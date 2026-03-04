// CLAUDE:SUMMARY Import adapter for OurAirports worldwide airport database with IATA/ICAO codes.
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
	Register(&ourAirportsAdapter{})
}

type ourAirportsAdapter struct{}

func (a *ourAirportsAdapter) ID() string      { return "ourairports-world" }
func (a *ourAirportsAdapter) DictID() string  { return "airports" }
func (a *ourAirportsAdapter) Description() string {
	return "OurAirports worldwide (IATA, ICAO, name, country)"
}
func (a *ourAirportsAdapter) DefaultURL() string {
	return "https://davidmegginson.github.io/ourairports-data/airports.csv"
}
func (a *ourAirportsAdapter) License() string { return "Public Domain" }

func (a *ourAirportsAdapter) Import(ctx context.Context, sourceURL, outputDir string) error {
	dlDir := filepath.Join(outputDir, "_download")
	if err := ensureDir(dlDir); err != nil {
		return err
	}
	defer os.RemoveAll(dlDir)

	csvPath := filepath.Join(dlDir, "airports.csv")
	fmt.Printf("  telechargement %s...\n", sourceURL)
	if err := downloadFile(ctx, sourceURL, csvPath); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	entries, err := parseOurAirports(csvPath)
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
		EntityType: "airport",
		Source:     "OurAirports",
		SourceURL:  sourceURL,
		License:    "Public Domain",
		DataFile:   "data.gob",
		Format:     dict.FormatSpec{Normalize: "lowercase_ascii"},
	})
}

// parseOurAirports reads the OurAirports CSV.
// Columns: id, ident, type, name, latitude_deg, longitude_deg, ..., iso_country, ..., iata_code, ...
func parseOurAirports(path string) (map[string]*dict.Entry, error) {
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

	nameCol := colByNames(colIdx, "name")
	identCol := colByNames(colIdx, "ident")
	typeCol := colByNames(colIdx, "type")
	iataCol := colByNames(colIdx, "iata_code")
	countryCol := colByNames(colIdx, "iso_country")
	municipalityCol := colByNames(colIdx, "municipality")

	if nameCol < 0 {
		return nil, fmt.Errorf("column 'name' not found in header %v", header)
	}

	entries := make(map[string]*dict.Entry, 30000)
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		name := safeCol(record, nameCol)
		if name == "" {
			continue
		}

		// Skip closed airports.
		apType := safeCol(record, typeCol)
		if apType == "closed" {
			continue
		}

		iata := safeCol(record, iataCol)
		icao := safeCol(record, identCol)
		country := safeCol(record, countryCol)

		meta := map[string]string{
			"icao":    icao,
			"type":    apType,
			"country": country,
		}
		if iata != "" {
			meta["iata"] = iata
		}
		if municipality := safeCol(record, municipalityCol); municipality != "" {
			meta["city"] = municipality
		}

		// Index by name.
		key := dict.NormalizeLowercaseASCII(name)
		entries[key] = &dict.Entry{Metadata: meta}

		// Also index by IATA and ICAO codes.
		if iata != "" {
			entries[strings.ToLower(iata)] = &dict.Entry{Metadata: meta}
		}
		if icao != "" {
			entries[strings.ToLower(icao)] = &dict.Entry{Metadata: meta}
		}
	}

	fmt.Printf("  %d aeroports\n", len(entries))
	return entries, nil
}
