// CLAUDE:SUMMARY CLI subcommand that downloads and builds dictionaries from public data sources via import adapters.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hazyhaar/touchstone-registry/pkg/importer"
)

func cmdImport(args []string) {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	source := fs.String("source", "", "adapter ID to import (e.g. insee-prenoms-fr)")
	all := fs.Bool("all", false, "import all available sources")
	outputDir := fs.String("output-dir", "dicts", "output directory for dictionaries")
	fs.Parse(args)

	// Open source DB and seed defaults.
	sourcesDBPath := filepath.Join(*outputDir, "sources.db")
	sdb, err := importer.OpenSourceDB(sourcesDBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Erreur ouverture sources.db: %v\n", err)
		os.Exit(1)
	}
	defer sdb.Close()

	if err := sdb.Seed(importer.All()); err != nil {
		fmt.Fprintf(os.Stderr, "Erreur seed sources: %v\n", err)
		os.Exit(1)
	}

	if !*all && *source == "" {
		fmt.Println("Sources disponibles :")
		fmt.Println()
		sources, _ := sdb.ListSources()
		for _, src := range sources {
			status := ""
			if src.LastStatus != nil {
				status = fmt.Sprintf("  [%d]", *src.LastStatus)
			}
			fmt.Printf("  %-25s  %s  (-> %s)%s\n", src.AdapterID, src.Description, src.DictID, status)
		}
		fmt.Println()
		fmt.Println("Usage :")
		fmt.Println("  touchstone import --source <id> [--output-dir <dir>]")
		fmt.Println("  touchstone import --all [--output-dir <dir>]")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()

	if *all {
		for _, a := range importer.All() {
			url, err := sdb.GetURL(a.ID())
			if err != nil {
				fmt.Fprintf(os.Stderr, "[%s] ERREUR (URL): %v\n", a.ID(), err)
				continue
			}
			fmt.Printf("[%s] Import en cours...\n", a.ID())
			if err := a.Import(ctx, url, *outputDir); err != nil {
				fmt.Fprintf(os.Stderr, "[%s] ERREUR: %v\n", a.ID(), err)
				continue
			}
			fmt.Printf("[%s] OK\n", a.ID())
		}
		return
	}

	a, err := importer.Get(*source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Erreur: %v\n", err)
		fmt.Println("\nSources disponibles :")
		for _, a := range importer.All() {
			fmt.Printf("  %s\n", a.ID())
		}
		os.Exit(1)
	}

	url, err := sdb.GetURL(a.ID())
	if err != nil {
		fmt.Fprintf(os.Stderr, "[%s] ERREUR (URL): %v\n", a.ID(), err)
		os.Exit(1)
	}

	fmt.Printf("[%s] Import en cours...\n", a.ID())
	if err := a.Import(ctx, url, *outputDir); err != nil {
		fmt.Fprintf(os.Stderr, "[%s] ERREUR: %v\n", a.ID(), err)
		os.Exit(1)
	}
	fmt.Printf("[%s] OK -> %s/%s/\n", a.ID(), *outputDir, a.DictID())
}
