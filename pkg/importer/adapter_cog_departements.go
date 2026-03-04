// CLAUDE:SUMMARY Import adapter for INSEE COG French departments with region codes and chef-lieu.
package importer

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/hazyhaar/touchstone-registry/pkg/dict"
)

func init() {
	Register(&cogDepartementsAdapter{})
}

type cogDepartementsAdapter struct{}

func (a *cogDepartementsAdapter) ID() string      { return "insee-cog-departements" }
func (a *cogDepartementsAdapter) DictID() string  { return "departements-fr" }
func (a *cogDepartementsAdapter) Description() string {
	return "INSEE COG departements de France (codes, regions, chefs-lieux)"
}
func (a *cogDepartementsAdapter) DefaultURL() string {
	return "https://www.insee.fr/fr/statistiques/fichier/7766585/v_departement_2024.csv"
}
func (a *cogDepartementsAdapter) License() string { return "CC0" }

func (a *cogDepartementsAdapter) Import(ctx context.Context, sourceURL, outputDir string) error {
	dlDir := filepath.Join(outputDir, "_download")
	if err := ensureDir(dlDir); err != nil {
		return err
	}
	defer os.RemoveAll(dlDir)

	csvPath := filepath.Join(dlDir, "departements.csv")
	fmt.Printf("  telechargement %s...\n", sourceURL)
	if err := downloadFile(ctx, sourceURL, csvPath); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	entries, err := parseCOGDepartements(csvPath)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	dictDir := filepath.Join(outputDir, a.DictID())
	if err := ensureDir(dictDir); err != nil {
		return err
	}

	if err := dict.SaveSQLite(entries, filepath.Join(dictDir, "data.db")); err != nil {
		return fmt.Errorf("save sqlite: %w", err)
	}

	return writeManifest(dictDir, &dict.Manifest{
		ID:           a.DictID(),
		Version:      "2026-03",
		Jurisdiction: "fr",
		EntityType:   "department",
		Source:       "INSEE COG departements",
		SourceURL:    sourceURL,
		License:      "CC0",
		DataFile:     "data.db",
		Format:       dict.FormatSpec{Normalize: "lowercase_ascii"},
	})
}

// parseCOGDepartements reads the INSEE COG departements CSV.
// Columns include: DEP, REG, CHEFLIEU, TNCC, NCC, NCCENR, LIBELLE.
func parseCOGDepartements(path string) (map[string]*dict.Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.Comma = ','
	r.LazyQuotes = true
	r.FieldsPerRecord = -1

	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	colIdx := make(map[string]int)
	for i, h := range header {
		colIdx[strings.TrimSpace(strings.ToUpper(h))] = i
	}

	depCol := colByNames(colIdx, "DEP")
	regCol := colByNames(colIdx, "REG")
	cheflieuCol := colByNames(colIdx, "CHEFLIEU")
	libelleCol := colByNames(colIdx, "LIBELLE", "NCCENR", "NCC")

	if libelleCol < 0 {
		return nil, fmt.Errorf("no name column found in header %v", header)
	}

	entries := make(map[string]*dict.Entry)
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		name := safeCol(record, libelleCol)
		if name == "" {
			continue
		}

		dep := safeCol(record, depCol)
		meta := map[string]string{
			"code":      dep,
			"region":    safeCol(record, regCol),
			"chef_lieu": safeCol(record, cheflieuCol),
		}

		key := dict.NormalizeLowercaseASCII(name)
		entries[key] = &dict.Entry{Metadata: meta}

		// Also index by department code (e.g., "75", "2a").
		if dep != "" {
			entries[strings.ToLower(dep)] = &dict.Entry{Metadata: meta}
		}
	}

	fmt.Printf("  %d departements\n", len(entries))
	return entries, nil
}
