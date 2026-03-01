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
	_ = fs.Parse(args)

	// Open source DB and seed defaults.
	sourcesDBPath := filepath.Join(*outputDir, "sources.db")
	sdb, err := importer.OpenSourceDB(sourcesDBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Erreur ouverture sources.db: %v\n", err)
		os.Exit(1)
	}

	if runErr := runImport(sdb, *all, *source, *outputDir); runErr != nil {
		sdb.Close()
		fmt.Fprintf(os.Stderr, "%v\n", runErr)
		os.Exit(1)
	}
	sdb.Close()
}

func runImport(sdb *importer.SourceDB, all bool, source, outputDir string) error {
	if err := sdb.Seed(importer.All()); err != nil {
		return fmt.Errorf("Erreur seed sources: %w", err)
	}

	if !all && source == "" {
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
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()

	if all {
		for _, a := range importer.All() {
			url, urlErr := sdb.GetURL(a.ID())
			if urlErr != nil {
				fmt.Fprintf(os.Stderr, "[%s] ERREUR (URL): %v\n", a.ID(), urlErr)
				continue
			}
			fmt.Printf("[%s] Import en cours...\n", a.ID())
			if importErr := a.Import(ctx, url, outputDir); importErr != nil {
				fmt.Fprintf(os.Stderr, "[%s] ERREUR: %v\n", a.ID(), importErr)
				continue
			}
			fmt.Printf("[%s] OK\n", a.ID())
		}
		return nil
	}

	a, err := importer.Get(source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Erreur: %v\n", err)
		fmt.Println("\nSources disponibles :")
		for _, adapter := range importer.All() {
			fmt.Printf("  %s\n", adapter.ID())
		}
		return err
	}

	url, err := sdb.GetURL(a.ID())
	if err != nil {
		return fmt.Errorf("[%s] ERREUR (URL): %w", a.ID(), err)
	}

	fmt.Printf("[%s] Import en cours...\n", a.ID())
	if err := a.Import(ctx, url, outputDir); err != nil {
		return fmt.Errorf("[%s] ERREUR: %w", a.ID(), err)
	}
	fmt.Printf("[%s] OK -> %s/%s/\n", a.ID(), outputDir, a.DictID())
	return nil
}
