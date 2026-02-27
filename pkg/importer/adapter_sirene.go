// CLAUDE:SUMMARY Import adapter for French SIRENE company registry filtering active legal entities with SIREN numbers.
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
	Register(&sireneAdapter{})
}

type sireneAdapter struct{}

func (a *sireneAdapter) ID() string          { return "sirene-fr" }
func (a *sireneAdapter) DictID() string      { return "sirene-fr" }
func (a *sireneAdapter) Description() string { return "SIRENE FR (entreprises actives, base stock)" }
func (a *sireneAdapter) DefaultURL() string   { return "https://files.data.gouv.fr/insee-sirene/StockUniteLegale_utf8.zip" }
func (a *sireneAdapter) License() string      { return "Licence Ouverte v2" }

func (a *sireneAdapter) Import(ctx context.Context, sourceURL, outputDir string) error {
	dlDir := filepath.Join(outputDir, "_download")
	if err := ensureDir(dlDir); err != nil {
		return err
	}
	defer os.RemoveAll(dlDir)

	zipPath := filepath.Join(dlDir, "sirene.zip")
	fmt.Printf("  telechargement %s...\n", sourceURL)
	fmt.Println("  (fichier tres volumineux ~8 Go, patience...)")
	if err := downloadFile(ctx, sourceURL, zipPath); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	files, err := unzipFile(zipPath, dlDir)
	if err != nil {
		return fmt.Errorf("unzip: %w", err)
	}

	var csvPath string
	for _, f := range files {
		if strings.HasSuffix(strings.ToLower(f), ".csv") {
			csvPath = f
			break
		}
	}
	if csvPath == "" {
		return fmt.Errorf("no CSV found in ZIP")
	}

	entries, err := parseSIRENE(csvPath)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	dictDir := filepath.Join(outputDir, a.DictID())
	if err := ensureDir(dictDir); err != nil {
		return err
	}

	if err := dict.SaveGob(entries, filepath.Join(dictDir, "data.gob")); err != nil {
		return fmt.Errorf("save gob: %w", err)
	}

	return writeManifest(dictDir, &dict.Manifest{
		ID:           a.DictID(),
		Version:      "2026-02",
		Jurisdiction: "fr",
		EntityType:   "company",
		Source:       "SIRENE (INSEE)",
		SourceURL:    sourceURL,
		License:      "Licence Ouverte v2",
		DataFile:     "data.gob",
		Format:       dict.FormatSpec{Normalize: "lowercase_ascii"},
	})
}

// parseSIRENE reads the SIRENE StockUniteLegale CSV in streaming mode.
// Filters by etatAdministratifUniteLegale = "A" (active).
// Key columns: denominationUniteLegale (or denominationUsuelleUniteLegale), siren.
func parseSIRENE(path string) (map[string]*dict.Entry, error) {
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
		colIdx[strings.TrimSpace(h)] = i
	}

	denomCol := -1
	if v, ok := colIdx["denominationUniteLegale"]; ok {
		denomCol = v
	}
	denomUsuelCol := -1
	if v, ok := colIdx["denominationUsuelleUniteLegale"]; ok {
		denomUsuelCol = v
	}
	etatCol := -1
	if v, ok := colIdx["etatAdministratifUniteLegale"]; ok {
		etatCol = v
	}
	sirenCol := -1
	if v, ok := colIdx["siren"]; ok {
		sirenCol = v
	}

	if denomCol < 0 && denomUsuelCol < 0 {
		return nil, fmt.Errorf("no denomination column found in header")
	}

	entries := make(map[string]*dict.Entry)
	var skipped int
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		// Filter: only active entities.
		if etatCol >= 0 && etatCol < len(record) {
			etat := strings.TrimSpace(record[etatCol])
			if etat != "A" {
				skipped++
				continue
			}
		}

		// Get company name: prefer denominationUniteLegale, fallback to usuelle.
		var name string
		if denomCol >= 0 && denomCol < len(record) {
			name = strings.TrimSpace(record[denomCol])
		}
		if name == "" && denomUsuelCol >= 0 && denomUsuelCol < len(record) {
			name = strings.TrimSpace(record[denomUsuelCol])
		}
		if name == "" {
			continue
		}

		key := dict.NormalizeLowercaseASCII(name)

		meta := make(map[string]string)
		if sirenCol >= 0 && sirenCol < len(record) {
			meta["siren"] = strings.TrimSpace(record[sirenCol])
		}
		entries[key] = &dict.Entry{Metadata: meta}
	}

	fmt.Printf("  %d entreprises actives SIRENE (%d inactives ignorees)\n", len(entries), skipped)
	return entries, nil
}
