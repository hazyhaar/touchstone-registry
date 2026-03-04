// CLAUDE:SUMMARY CLI subcommand to migrate data.gob dictionaries to data.db (SQLite).
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hazyhaar/touchstone-registry/pkg/dict"
)

func cmdMigrateGob(args []string) {
	fs := flag.NewFlagSet("migrate-gob", flag.ExitOnError)
	dictsDir := fs.String("dicts-dir", "dicts", "path to dictionaries directory")
	removeGob := fs.Bool("remove-gob", false, "remove data.gob after successful conversion")
	_ = fs.Parse(args)

	entries, err := os.ReadDir(*dictsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read dicts dir: %v\n", err)
		os.Exit(1)
	}

	var converted, skipped, failed int
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(*dictsDir, entry.Name())
		gobPath := filepath.Join(dir, "data.gob")
		dbPath := filepath.Join(dir, "data.db")

		// Skip if no gob file.
		if _, err := os.Stat(gobPath); err != nil {
			continue
		}
		// Skip if db already exists.
		if _, err := os.Stat(dbPath); err == nil {
			fmt.Printf("  skip %s (data.db already exists)\n", entry.Name())
			skipped++
			continue
		}

		fmt.Printf("  converting %s...", entry.Name())
		start := time.Now()

		// Load gob into memory.
		d := &dict.Dictionary{Entries: make(map[string]*dict.Entry)}
		if err := d.LoadGobFile(gobPath); err != nil {
			fmt.Printf(" FAILED (load gob: %v)\n", err)
			failed++
			continue
		}

		// Save as SQLite.
		if err := dict.SaveSQLite(d.Entries, dbPath); err != nil {
			fmt.Printf(" FAILED (save sqlite: %v)\n", err)
			_ = os.Remove(dbPath)
			failed++
			continue
		}

		elapsed := time.Since(start)
		fmt.Printf(" OK (%d entries, %v)\n", len(d.Entries), elapsed.Round(time.Millisecond))
		converted++

		if *removeGob {
			if err := os.Remove(gobPath); err != nil {
				fmt.Printf("    warning: remove gob: %v\n", err)
			}
		}
	}

	fmt.Printf("\nMigration complete: %d converted, %d skipped, %d failed\n", converted, skipped, failed)
	if failed > 0 {
		os.Exit(1)
	}
}
