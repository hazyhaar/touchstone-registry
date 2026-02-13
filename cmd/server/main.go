package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/hazyhaar/touchstone-registry/pkg/api"
	"github.com/hazyhaar/touchstone-registry/pkg/dict"
	"gopkg.in/yaml.v3"
)

type config struct {
	Addr     string `yaml:"addr"`
	DictsDir string `yaml:"dicts_dir"`
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		cmdServe(os.Args[2:])
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: touchstone <command>\n\nCommands:\n  serve   Start the HTTP server\n")
}

func cmdServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	cfgPath := fs.String("config", "config.yaml", "path to config file")
	fs.Parse(args)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg := loadConfig(*cfgPath, logger)

	// Load dictionaries.
	reg := dict.NewRegistry(cfg.DictsDir)
	if err := reg.Load(); err != nil {
		logger.Error("failed to load dictionaries", "error", err)
		os.Exit(1)
	}
	logger.Info("dictionaries loaded", "count", reg.DictCount(), "entries", reg.TotalEntries())

	// HTTP router.
	router := api.NewRouter(reg)

	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: router,
	}

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

	// Start server.
	go func() {
		logger.Info("touchstone listening", "addr", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")
	srv.Shutdown(context.Background())
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
