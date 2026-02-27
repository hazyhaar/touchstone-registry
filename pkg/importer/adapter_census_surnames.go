// CLAUDE:SUMMARY Import adapter for US Census Bureau 2010 surnames with rank and frequency metadata.
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
	Register(&censusSurnamesAdapter{})
}

type censusSurnamesAdapter struct{}

func (a *censusSurnamesAdapter) ID() string     { return "census-surnames-us" }
func (a *censusSurnamesAdapter) DictID() string { return "surnames-us" }
func (a *censusSurnamesAdapter) Description() string {
	return "US Census Bureau surnames (2010 census)"
}
func (a *censusSurnamesAdapter) DefaultURL() string { return "https://www2.census.gov/topics/genealogy/2010surnames/names.zip" }
func (a *censusSurnamesAdapter) License() string    { return "Public Domain" }

func (a *censusSurnamesAdapter) Import(ctx context.Context, sourceURL, outputDir string) error {
	dlDir := filepath.Join(outputDir, "_download")
	if err := ensureDir(dlDir); err != nil {
		return err
	}
	defer os.RemoveAll(dlDir)

	zipPath := filepath.Join(dlDir, "surnames.zip")
	fmt.Printf("  telechargement %s...\n", sourceURL)
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

	entries, err := parseCensusSurnames(csvPath)
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
		Jurisdiction: "us",
		EntityType:   "surname",
		Source:       "US Census Bureau 2010",
		SourceURL:    sourceURL,
		License:      "Public Domain",
		DataFile:     "data.gob",
		Format:       dict.FormatSpec{Normalize: "lowercase_ascii"},
	})
}

// parseCensusSurnames reads the Census surnames CSV.
// Columns: name,rank,count,prop100k,cum_prop100k,pctwhite,...
func parseCensusSurnames(path string) (map[string]*dict.Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.LazyQuotes = true
	r.FieldsPerRecord = -1

	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	colIdx := make(map[string]int)
	for i, h := range header {
		colIdx[strings.TrimSpace(strings.ToLower(h))] = i
	}

	nameCol, hasName := colIdx["name"]
	if !hasName {
		return nil, fmt.Errorf("column 'name' not found in header %v", header)
	}
	rankCol := colIdx["rank"]
	countCol := colIdx["count"]

	entries := make(map[string]*dict.Entry)
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

		meta := make(map[string]string)
		if rankCol < len(record) {
			meta["rank"] = strings.TrimSpace(record[rankCol])
		}
		if countCol < len(record) {
			meta["frequency"] = strings.TrimSpace(record[countCol])
		}
		entries[key] = &dict.Entry{Metadata: meta}
	}

	fmt.Printf("  %d noms de famille US\n", len(entries))
	return entries, nil
}
