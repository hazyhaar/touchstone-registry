// CLAUDE:SUMMARY Import adapter for Eurostat NUTS 2024 (EU regions classification).
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
	Register(&nutsEUAdapter{})
}

type nutsEUAdapter struct{}

func (a *nutsEUAdapter) ID() string      { return "eurostat-nuts" }
func (a *nutsEUAdapter) DictID() string  { return "nuts-eu" }
func (a *nutsEUAdapter) Description() string {
	return "Eurostat NUTS 2024 — regions EU (niveaux 0-3)"
}
func (a *nutsEUAdapter) DefaultURL() string {
	return "https://gisco-services.ec.europa.eu/distribution/v2/nuts/csv/NUTS_AT_2024.csv"
}
func (a *nutsEUAdapter) License() string { return "CC BY 4.0" }

func (a *nutsEUAdapter) Import(ctx context.Context, sourceURL, outputDir string) error {
	dlDir := filepath.Join(outputDir, "_download")
	if err := ensureDir(dlDir); err != nil {
		return err
	}
	defer os.RemoveAll(dlDir)

	csvPath := filepath.Join(dlDir, "nuts.csv")
	fmt.Printf("  telechargement %s...\n", sourceURL)
	if err := downloadFile(ctx, sourceURL, csvPath); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	entries, err := parseNUTS(csvPath)
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
		Jurisdiction: "eu",
		EntityType:   "region",
		Source:       "Eurostat NUTS",
		SourceURL:    sourceURL,
		License:      "CC BY 4.0",
		DataFile:     "data.gob",
		Format:       dict.FormatSpec{Normalize: "lowercase_ascii"},
	})
}

func parseNUTS(path string) (map[string]*dict.Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.Comma = ';'
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

	codeCol := colByNames(colIdx, "NUTS CODE", "CODE", "NUTS_CODE", "NUTS_ID")
	nameCol := colByNames(colIdx, "NUTS LABEL", "LABEL", "NUTS_NAME", "DESCRIPTION", "NAME")
	levelCol := colByNames(colIdx, "NUTS LEVEL", "LEVEL", "NUTS_LEVEL")
	countryCol := colByNames(colIdx, "COUNTRY CODE", "CNTR_CODE", "COUNTRY")

	if codeCol < 0 || nameCol < 0 {
		// Try comma-separated fallback
		if _, seekErr := f.Seek(0, io.SeekStart); seekErr != nil {
			return nil, fmt.Errorf("seek: %w", seekErr)
		}
		r2 := csv.NewReader(f)
		r2.Comma = ','
		r2.LazyQuotes = true
		r2.FieldsPerRecord = -1
		header, err = r2.Read()
		if err != nil {
			return nil, fmt.Errorf("read header (comma): %w", err)
		}
		colIdx = make(map[string]int)
		for i, h := range header {
			colIdx[strings.TrimSpace(strings.ToUpper(h))] = i
		}
		codeCol = colByNames(colIdx, "NUTS CODE", "CODE", "NUTS_CODE", "NUTS_ID")
		nameCol = colByNames(colIdx, "NUTS LABEL", "LABEL", "NUTS_NAME", "DESCRIPTION", "NAME")
		levelCol = colByNames(colIdx, "NUTS LEVEL", "LEVEL", "NUTS_LEVEL")
		countryCol = colByNames(colIdx, "COUNTRY CODE", "CNTR_CODE", "COUNTRY")
		r = r2
	}

	if codeCol < 0 {
		return nil, fmt.Errorf("NUTS CODE column not found in header %v", header)
	}

	entries := make(map[string]*dict.Entry, 2000)
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		code := strings.TrimSpace(safeCol(record, codeCol))
		name := strings.TrimSpace(safeCol(record, nameCol))
		if code == "" {
			continue
		}

		meta := map[string]string{
			"nuts_code": code,
			"name":      name,
			"level":     safeCol(record, levelCol),
			"country":   safeCol(record, countryCol),
		}

		entries[strings.ToLower(code)] = &dict.Entry{Metadata: meta}
		if name != "" {
			entries[dict.NormalizeLowercaseASCII(name)] = &dict.Entry{Metadata: meta}
		}
	}

	fmt.Printf("  %d codes NUTS\n", len(entries))
	return entries, nil
}
