// CLAUDE:SUMMARY Entry point dispatching serve/import subcommands, wiring HTTP+MCP server with dictionary registry and graceful shutdown.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/hazyhaar/touchstone-registry/pkg/api"
	"github.com/hazyhaar/pkg/chassis"
	"github.com/hazyhaar/touchstone-registry/pkg/dict"
	"github.com/hazyhaar/touchstone-registry/pkg/importer"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gopkg.in/yaml.v3"
)

type config struct {
	Addr     string `yaml:"addr"`
	DictsDir string `yaml:"dicts_dir"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
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
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: touchstone <command>\n\nCommands:\n  serve    Start the server (HTTP/1.1+2, HTTP/3, MCP-over-QUIC)\n  import   Download and build dictionaries from public sources\n")
}

func cmdServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	cfgPath := fs.String("config", "config.yaml", "path to config file")
	fs.Parse(args)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg := loadConfig(*cfgPath, logger)

	// Source availability checker.
	sdb, err := importer.OpenSourceDB(filepath.Join(cfg.DictsDir, "sources.db"))
	if err != nil {
		logger.Error("failed to open sources database", "error", err)
		os.Exit(1)
	}
	defer sdb.Close()

	if err := sdb.Seed(importer.All()); err != nil {
		logger.Error("failed to seed import sources", "error", err)
		os.Exit(1)
	}

	// Load dictionaries.
	reg := dict.NewRegistry(cfg.DictsDir)
	if err := reg.Load(); err != nil {
		logger.Error("failed to load dictionaries", "error", err)
		os.Exit(1)
	}
	logger.Info("dictionaries loaded", "count", reg.DictCount(), "entries", reg.TotalEntries())

	// HTTP router.
	router := api.NewRouter(reg)

	// MCP server with Touchstone tools.
	mcpSrv := mcp.NewServer(&mcp.Implementation{Name: "touchstone", Version: "0.1.0"}, nil)
	api.RegisterMCPTools(mcpSrv, reg)

	// Chassis: dual-transport (TCP+QUIC) with TLS, security headers, MCP.
	srv, err := chassis.New(chassis.Config{
		Addr:      cfg.Addr,
		Handler:   router,
		MCPServer: mcpSrv,
		CertFile:  cfg.CertFile,
		KeyFile:   cfg.KeyFile,
		Logger:    logger,
	})
	if err != nil {
		logger.Error("chassis init failed", "error", err)
		os.Exit(1)
	}

	// Start source availability checker (every 24h).
	checker := importer.NewChecker(sdb, logger, 24*time.Hour)

	// SIGHUP: hot reload dictionaries.
	// SIGINT/SIGTERM: graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	sighup := make(chan os.Signal, 1)
	signal.Notify(sighup, syscall.SIGHUP)
	go func() {
		for range sighup {
			logger.Info("SIGHUP received, reloading dictionaries")
			if err := reg.Reload(); err != nil {
				logger.Error("reload failed", "error", err)
			} else {
				logger.Info("dictionaries reloaded", "count", reg.DictCount(), "entries", reg.TotalEntries())
			}
		}
	}()

	go checker.Start(ctx)

	// Start chassis (TCP + QUIC).
	go func() {
		if err := srv.Start(ctx); err != nil {
			logger.Error("chassis error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Stop(shutCtx); err != nil {
		logger.Error("shutdown error", "error", err)
	}
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
