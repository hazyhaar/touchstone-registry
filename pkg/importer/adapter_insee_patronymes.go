// CLAUDE:SUMMARY Import adapter for INSEE French surnames (patronymes) from tab-delimited national file.
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
	Register(&inseePatronymesAdapter{})
}

type inseePatronymesAdapter struct{}

func (a *inseePatronymesAdapter) ID() string          { return "insee-patronymes-fr" }
func (a *inseePatronymesAdapter) DictID() string      { return "patronymes-fr" }
func (a *inseePatronymesAdapter) Description() string { return "INSEE patronymes francais (noms de famille)" }
func (a *inseePatronymesAdapter) DefaultURL() string   { return "https://www.insee.fr/fr/statistiques/fichier/3536630/noms2008nat_txt.zip" }
func (a *inseePatronymesAdapter) License() string      { return "CC0" }

func (a *inseePatronymesAdapter) Import(ctx context.Context, sourceURL, outputDir string) error {
	dlDir := filepath.Join(outputDir, "_download")
	if err := ensureDir(dlDir); err != nil {
		return err
	}
	defer os.RemoveAll(dlDir)

	zipPath := filepath.Join(dlDir, "patronymes.zip")
	fmt.Printf("  telechargement %s...\n", sourceURL)
	if err := downloadFile(ctx, sourceURL, zipPath); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	files, err := unzipFile(zipPath, dlDir)
	if err != nil {
		return fmt.Errorf("unzip: %w", err)
	}

	var dataPath string
	for _, f := range files {
		lower := strings.ToLower(f)
		if strings.HasSuffix(lower, ".txt") || strings.HasSuffix(lower, ".csv") {
			dataPath = f
			break
		}
	}
	if dataPath == "" {
		return fmt.Errorf("no data file found in ZIP")
	}

	entries, err := parseINSEEPatronymes(dataPath)
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
		EntityType:   "surname",
		Source:       "INSEE fichier des noms de famille",
		SourceURL:    sourceURL,
		License:      "CC0",
		DataFile:     "data.gob",
		Format:       dict.FormatSpec{Normalize: "lowercase_ascii"},
	})
}

// parseINSEEPatronymes reads the INSEE patronymes file (tab-delimited).
// Columns: NOM;NOMBRE (or tab-separated). Aggregates by normalized name.
func parseINSEEPatronymes(path string) (map[string]*dict.Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.Comma = '\t'
	r.LazyQuotes = true
	r.FieldsPerRecord = -1

	// Read header.
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	colIdx := make(map[string]int)
	for i, h := range header {
		colIdx[strings.TrimSpace(strings.ToUpper(h))] = i
	}

	// The file uses NOM and NOMBRE columns.
	nameCol := -1
	nombreCol := -1
	for k, v := range colIdx {
		if strings.Contains(k, "NOM") && !strings.Contains(k, "NOMBRE") {
			nameCol = v
		}
		if strings.Contains(k, "NOMBRE") || strings.Contains(k, "FREQ") {
			nombreCol = v
		}
	}
	if nameCol < 0 {
		// Fallback: first column is name.
		nameCol = 0
	}

	type agg struct {
		total int
	}
	aggregated := make(map[string]*agg)

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		if nameCol >= len(record) {
			continue
		}

		name := strings.TrimSpace(record[nameCol])
		if name == "" {
			continue
		}
		key := dict.NormalizeLowercaseASCII(name)

		var nombre int
		if nombreCol >= 0 && nombreCol < len(record) {
			fmt.Sscanf(strings.TrimSpace(record[nombreCol]), "%d", &nombre)
		}

		if existing, ok := aggregated[key]; ok {
			existing.total += nombre
		} else {
			aggregated[key] = &agg{total: nombre}
		}
	}

	entries := make(map[string]*dict.Entry, len(aggregated))
	for key, a := range aggregated {
		meta := make(map[string]string)
		if a.total > 0 {
			meta["frequency"] = fmt.Sprintf("%d", a.total)
		}
		entries[key] = &dict.Entry{Metadata: meta}
	}

	fmt.Printf("  %d patronymes uniques\n", len(entries))
	return entries, nil
}
