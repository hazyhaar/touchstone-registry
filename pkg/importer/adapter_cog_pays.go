// CLAUDE:SUMMARY Import adapter for INSEE COG countries (pays et territories) with ISO codes and sovereignty.
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
	Register(&cogPaysAdapter{})
}

type cogPaysAdapter struct{}

func (a *cogPaysAdapter) ID() string      { return "insee-cog-pays" }
func (a *cogPaysAdapter) DictID() string  { return "pays-fr" }
func (a *cogPaysAdapter) Description() string {
	return "INSEE COG pays et territories (codes, continents, sovereignty)"
}
func (a *cogPaysAdapter) DefaultURL() string {
	return "https://www.insee.fr/fr/statistiques/fichier/7766585/v_pays_territoire_2024.csv"
}
func (a *cogPaysAdapter) License() string { return "CC0" }

func (a *cogPaysAdapter) Import(ctx context.Context, sourceURL, outputDir string) error {
	dlDir := filepath.Join(outputDir, "_download")
	if err := ensureDir(dlDir); err != nil {
		return err
	}
	defer os.RemoveAll(dlDir)

	csvPath := filepath.Join(dlDir, "pays.csv")
	fmt.Printf("  telechargement %s...\n", sourceURL)
	if err := downloadFile(ctx, sourceURL, csvPath); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	entries, err := parseCOGPays(csvPath)
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
		Version:      "2026-03",
		Jurisdiction: "fr",
		EntityType:   "country",
		Source:       "INSEE COG pays",
		SourceURL:    sourceURL,
		License:      "CC0",
		DataFile:     "data.gob",
		Format:       dict.FormatSpec{Normalize: "lowercase_ascii"},
	})
}

// parseCOGPays reads the INSEE COG pays CSV.
// Columns include: COG, ACTUAL, LIBCOG, LIBENR, CODEISO2, CODEISO3, CODENUM3.
func parseCOGPays(path string) (map[string]*dict.Entry, error) {
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

	cogCol := colByNames(colIdx, "COG")
	libelleCol := colByNames(colIdx, "LIBCOG", "LIBENR")
	iso2Col := colByNames(colIdx, "CODEISO2")
	iso3Col := colByNames(colIdx, "CODEISO3")
	actualCol := colByNames(colIdx, "ACTUAL")

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

		// Skip historical entries (ACTUAL != 1).
		if actual := safeCol(record, actualCol); actual != "" && actual != "1" {
			continue
		}

		name := safeCol(record, libelleCol)
		if name == "" {
			continue
		}

		meta := map[string]string{
			"cog":   safeCol(record, cogCol),
			"iso2":  safeCol(record, iso2Col),
			"iso3":  safeCol(record, iso3Col),
		}

		key := dict.NormalizeLowercaseASCII(name)
		entries[key] = &dict.Entry{Metadata: meta}

		// Also index by COG code.
		if cog := meta["cog"]; cog != "" {
			entries[strings.ToLower(cog)] = &dict.Entry{Metadata: meta}
		}
	}

	fmt.Printf("  %d pays COG\n", len(entries))
	return entries, nil
}
