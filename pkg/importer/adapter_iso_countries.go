// CLAUDE:SUMMARY Import adapter for ISO 3166-1 country names and codes from DataHub open dataset.
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
	Register(&isoCountriesAdapter{})
}

type isoCountriesAdapter struct{}

func (a *isoCountriesAdapter) ID() string      { return "iso-3166-countries" }
func (a *isoCountriesAdapter) DictID() string  { return "countries" }
func (a *isoCountriesAdapter) Description() string {
	return "ISO 3166-1 countries (name, alpha-2, alpha-3, numeric)"
}
func (a *isoCountriesAdapter) DefaultURL() string {
	return "https://raw.githubusercontent.com/lukes/ISO-3166-Countries-with-Regional-Codes/master/all/all.csv"
}
func (a *isoCountriesAdapter) License() string { return "CC0" }

func (a *isoCountriesAdapter) Import(ctx context.Context, sourceURL, outputDir string) error {
	dlDir := filepath.Join(outputDir, "_download")
	if err := ensureDir(dlDir); err != nil {
		return err
	}
	defer os.RemoveAll(dlDir)

	csvPath := filepath.Join(dlDir, "countries.csv")
	fmt.Printf("  telechargement %s...\n", sourceURL)
	if err := downloadFile(ctx, sourceURL, csvPath); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	entries, err := parseISOCountries(csvPath)
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
		ID:         a.DictID(),
		Version:    "2026-03",
		EntityType: "country",
		Source:     "ISO 3166-1",
		SourceURL:  sourceURL,
		License:    "CC0",
		DataFile:   "data.gob",
		Format:     dict.FormatSpec{Normalize: "lowercase_ascii"},
	})
}

// parseISOCountries reads the ISO 3166 CSV.
// Columns: name, alpha-2, alpha-3, country-code, iso_3166-2, region, sub-region, ...
func parseISOCountries(path string) (map[string]*dict.Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.LazyQuotes = true

	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	colIdx := make(map[string]int)
	for i, h := range header {
		colIdx[strings.TrimSpace(strings.ToLower(h))] = i
	}

	nameCol, hasName := colIdx["name"]
	alpha2Col := colIdx["alpha-2"]
	alpha3Col := colIdx["alpha-3"]
	numericCol := colIdx["country-code"]
	regionCol := colIdx["region"]
	subRegionCol := colIdx["sub-region"]
	if !hasName {
		return nil, fmt.Errorf("column 'name' not found in header %v", header)
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

		name := strings.TrimSpace(record[nameCol])
		if name == "" {
			continue
		}

		meta := make(map[string]string)
		if alpha2Col < len(record) {
			meta["alpha2"] = strings.TrimSpace(record[alpha2Col])
		}
		if alpha3Col < len(record) {
			meta["alpha3"] = strings.TrimSpace(record[alpha3Col])
		}
		if numericCol < len(record) {
			meta["numeric"] = strings.TrimSpace(record[numericCol])
		}
		if regionCol < len(record) {
			meta["region"] = strings.TrimSpace(record[regionCol])
		}
		if subRegionCol < len(record) {
			meta["sub_region"] = strings.TrimSpace(record[subRegionCol])
		}

		key := dict.NormalizeLowercaseASCII(name)
		entries[key] = &dict.Entry{Metadata: meta}

		// Also index by alpha-2 and alpha-3 codes.
		if a2 := meta["alpha2"]; a2 != "" {
			entries[strings.ToLower(a2)] = &dict.Entry{Metadata: meta}
		}
		if a3 := meta["alpha3"]; a3 != "" {
			entries[strings.ToLower(a3)] = &dict.Entry{Metadata: meta}
		}
	}

	fmt.Printf("  %d entries pays ISO 3166\n", len(entries))
	return entries, nil
}
