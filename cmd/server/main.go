// CLAUDE:SUMMARY Entry point dispatching serve/import subcommands, wiring HTTP+MCP server with dictionary registry and graceful shutdown.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/hazyhaar/pkg/audit"
	"github.com/hazyhaar/pkg/chassis"
	"github.com/hazyhaar/touchstone-registry/pkg/admin"
	"github.com/hazyhaar/touchstone-registry/pkg/api"
	"github.com/hazyhaar/touchstone-registry/pkg/dict"
	"github.com/hazyhaar/touchstone-registry/pkg/importer"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gopkg.in/yaml.v3"

	_ "modernc.org/sqlite"
)

type config struct {
	Addr       string `yaml:"addr"`
	DictsDir   string `yaml:"dicts_dir"`
	CertFile   string `yaml:"cert_file"`
	KeyFile    string `yaml:"key_file"`
	AdminToken string `yaml:"admin_token"`
	AdminDB    string `yaml:"admin_db"`
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		cmdServe(os.Args[2:])
	case "import":
		cmdImport(os.Args[2:])
	case "migrate-gob":
		cmdMigrateGob(os.Args[2:])
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: touchstone <command>\n\nCommands:\n  serve        Start the server (HTTP/1.1+2, HTTP/3, MCP-over-QUIC)\n  import       Download and build dictionaries from public sources\n  migrate-gob  Convert data.gob files to data.db (SQLite)\n")
}

func cmdServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	cfgPath := fs.String("config", "config.yaml", "path to config file")
	_ = fs.Parse(args)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg := loadConfig(*cfgPath, logger)

	// Source availability checker.
	sdb, err := importer.OpenSourceDB(filepath.Join(cfg.DictsDir, "sources.db"))
	if err != nil {
		logger.Error("failed to open sources database", "error", err)
		os.Exit(1)
	}

	deps := initServer(sdb, cfg, logger)

	// SIGHUP: hot reload dictionaries.
	// SIGINT/SIGTERM: graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	defer deps.reg.Close()
	defer sdb.Close()
	if deps.adminDB != nil {
		defer deps.adminDB.Close()
	}
	if deps.auditor != nil {
		defer deps.auditor.Close()
	}

	sighup := make(chan os.Signal, 1)
	signal.Notify(sighup, syscall.SIGHUP)
	go func() {
		for range sighup {
			logger.Info("SIGHUP received, reloading dictionaries")
			if reloadErr := deps.reg.Reload(); reloadErr != nil {
				logger.Error("reload failed", "error", reloadErr)
			} else {
				logger.Info("dictionaries reloaded", "count", deps.reg.DictCount(), "entries", deps.reg.TotalEntries())
			}
		}
	}()

	go deps.checker.Start(ctx)

	// Start chassis (TCP + QUIC).
	go func() {
		if startErr := deps.srv.Start(ctx); startErr != nil {
			logger.Error("chassis error", "error", startErr)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if stopErr := deps.srv.Stop(shutCtx); stopErr != nil {
		logger.Error("shutdown error", "error", stopErr)
	}
}

type serverDeps struct {
	reg     *dict.Registry
	srv     *chassis.Server
	checker *importer.Checker
	adminDB *sql.DB
	auditor *audit.SQLiteLogger
}

func initServer(sdb *importer.SourceDB, cfg config, logger *slog.Logger) *serverDeps {
	if err := sdb.Seed(importer.All()); err != nil {
		sdb.Close()
		logger.Error("failed to seed import sources", "error", err)
		os.Exit(1)
	}

	// Load dictionaries.
	reg := dict.NewRegistry(cfg.DictsDir)
	if err := reg.Load(); err != nil {
		sdb.Close()
		logger.Error("failed to load dictionaries", "error", err)
		os.Exit(1)
	}
	logger.Info("dictionaries loaded", "count", reg.DictCount(), "entries", reg.TotalEntries())

	// Combined HTTP mux: public API + admin.
	topMux := http.NewServeMux()

	// Public API routes.
	apiRouter := api.NewRouter(reg)
	topMux.Handle("/v1/", apiRouter)
	topMux.Handle("/v1/health", apiRouter)

	// Admin DB + admin routes.
	var adminDB *sql.DB
	var auditor *audit.SQLiteLogger
	if cfg.AdminToken != "" {
		dbPath := cfg.AdminDB
		if dbPath == "" {
			dbPath = filepath.Join(cfg.DictsDir, "admin.db")
		}

		var err error
		adminDB, err = sql.Open("sqlite", dbPath+"?_txlock=immediate&_pragma=journal_mode(WAL)&_pragma=busy_timeout(10000)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)")
		if err != nil {
			logger.Error("admin db open failed", "error", err)
			os.Exit(1)
		}
		if _, err := adminDB.Exec(admin.Schema); err != nil {
			logger.Error("admin schema init failed", "error", err)
			os.Exit(1)
		}

		auditor = audit.NewSQLiteLogger(adminDB)
		if err := auditor.Init(); err != nil {
			logger.Error("audit init failed", "error", err)
			os.Exit(1)
		}

		adminSvc := admin.NewService(adminDB, auditor)

		// Sync on-disk dictionaries and legacy sources into admin DB.
		if err := adminSvc.SyncFromRegistry(reg); err != nil {
			logger.Warn("admin sync from registry failed", "error", err)
		}
		if err := adminSvc.MigrateFromSourceDB(sdb); err != nil {
			logger.Warn("admin migrate from source db failed", "error", err)
		}

		adminAPIRouter := admin.NewRouter(adminSvc, cfg.AdminToken)
		topMux.Handle("/admin/v1/", adminAPIRouter)

		// Admin panel (HTML pages, no auth for convenience — protect via network).
		panelRouter := admin.NewPanelRouter(adminSvc, reg)
		topMux.Handle("/admin/", panelRouter)
		logger.Info("admin API + panel enabled")
	}

	// MCP server with Touchstone tools.
	mcpSrv := mcp.NewServer(&mcp.Implementation{Name: "touchstone", Version: "0.1.0"}, nil)
	api.RegisterMCPTools(mcpSrv, reg)

	// Chassis: dual-transport (TCP+QUIC) with TLS, security headers, MCP.
	srv, err := chassis.New(chassis.Config{
		Addr:      cfg.Addr,
		Handler:   topMux,
		MCPServer: mcpSrv,
		CertFile:  cfg.CertFile,
		KeyFile:   cfg.KeyFile,
		Logger:    logger,
	})
	if err != nil {
		sdb.Close()
		logger.Error("chassis init failed", "error", err)
		os.Exit(1)
	}

	// Source availability checker (every 24h).
	checker := importer.NewChecker(sdb, logger, 24*time.Hour)

	return &serverDeps{reg: reg, srv: srv, checker: checker, adminDB: adminDB, auditor: auditor}
}

func loadConfig(path string, logger *slog.Logger) config {
	cfg := config{
		Addr:     ":8420",
		DictsDir: "dicts",
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Info("no config file, using defaults", "path", path)
			return cfg
		}
		logger.Error("read config", "error", err)
		os.Exit(1)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		logger.Error("parse config", "error", err)
		os.Exit(1)
	}
	return cfg
}
