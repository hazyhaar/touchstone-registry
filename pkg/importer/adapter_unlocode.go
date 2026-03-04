// CLAUDE:SUMMARY Import adapter for UN/LOCODE location codes from the UNECE CSV dataset.
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
	Register(&unlocodeAdapter{})
}

type unlocodeAdapter struct{}

func (a *unlocodeAdapter) ID() string      { return "unlocode" }
func (a *unlocodeAdapter) DictID() string  { return "locode" }
func (a *unlocodeAdapter) Description() string {
	return "UN/LOCODE codes de localisation (ports, aeroports, terminaux)"
}
func (a *unlocodeAdapter) DefaultURL() string {
	return "https://datahub.io/core/un-locode/r/code-list.csv"
}
func (a *unlocodeAdapter) License() string { return "PDDL" }

func (a *unlocodeAdapter) Import(ctx context.Context, sourceURL, outputDir string) error {
	dlDir := filepath.Join(outputDir, "_download")
	if err := ensureDir(dlDir); err != nil {
		return err
	}
	defer os.RemoveAll(dlDir)

	csvPath := filepath.Join(dlDir, "locode.csv")
	fmt.Printf("  telechargement %s...\n", sourceURL)
	if err := downloadFile(ctx, sourceURL, csvPath); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	entries, err := parseUNLOCODE(csvPath)
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
		ID:         a.DictID(),
		Version:    "2026-03",
		EntityType: "location",
		Source:     "UN/LOCODE (UNECE)",
		SourceURL:  sourceURL,
		License:    "PDDL",
		DataFile:   "data.db",
		Format:     dict.FormatSpec{Normalize: "lowercase_ascii"},
	})
}

// parseUNLOCODE reads the UN/LOCODE CSV from DataHub.
// Columns: Country, Location, Name, NameWoDiacritics, Subdivision, Status, Function, ...
func parseUNLOCODE(path string) (map[string]*dict.Entry, error) {
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
		colIdx[strings.TrimSpace(h)] = i
	}

	countryCol := colByNames(colIdx, "Country")
	locationCol := colByNames(colIdx, "Location")
	nameCol := colByNames(colIdx, "Name")
	nameAsciiCol := colByNames(colIdx, "NameWoDiacritics")
	subdivisionCol := colByNames(colIdx, "Subdivision")
	functionCol := colByNames(colIdx, "Function")

	if nameCol < 0 && nameAsciiCol < 0 {
		return nil, fmt.Errorf("no name column found in header %v", header)
	}

	entries := make(map[string]*dict.Entry, 100000)
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		country := safeCol(record, countryCol)
		location := safeCol(record, locationCol)
		name := safeCol(record, nameCol)
		if name == "" {
			name = safeCol(record, nameAsciiCol)
		}
		if name == "" || country == "" || location == "" {
			continue
		}

		locode := country + location // e.g. "FRPAR"
		meta := map[string]string{
			"locode":  locode,
			"country": country,
		}
		if sub := safeCol(record, subdivisionCol); sub != "" {
			meta["subdivision"] = sub
		}
		if fn := safeCol(record, functionCol); fn != "" {
			meta["function"] = fn
		}

		// Index by LOCODE.
		entries[strings.ToLower(locode)] = &dict.Entry{Metadata: meta}

		// Also index by name.
		key := dict.NormalizeLowercaseASCII(name)
		if _, exists := entries[key]; !exists {
			entries[key] = &dict.Entry{Metadata: meta}
		}
	}

	fmt.Printf("  %d localisations UN/LOCODE\n", len(entries))
	return entries, nil
}
