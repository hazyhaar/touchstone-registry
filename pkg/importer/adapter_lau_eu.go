// CLAUDE:SUMMARY Import adapter for Eurostat LAU (Local Administrative Units) — all EU communes (~100K).
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
	Register(&lauEUAdapter{})
}

type lauEUAdapter struct{}

func (a *lauEUAdapter) ID() string      { return "eurostat-lau" }
func (a *lauEUAdapter) DictID() string  { return "lau-eu" }
func (a *lauEUAdapter) Description() string {
	return "Eurostat LAU 2023 — communes EU27 (~100K)"
}
func (a *lauEUAdapter) DefaultURL() string {
	return "https://gisco-services.ec.europa.eu/distribution/v2/lau/csv/LAU_RG_01M_2024_4326.csv"
}
func (a *lauEUAdapter) License() string { return "CC BY 4.0" }

func (a *lauEUAdapter) Import(ctx context.Context, sourceURL, outputDir string) error {
	dlDir := filepath.Join(outputDir, "_download")
	if err := ensureDir(dlDir); err != nil {
		return err
	}
	defer os.RemoveAll(dlDir)

	csvPath := filepath.Join(dlDir, "lau.csv")
	fmt.Printf("  telechargement %s...\n", sourceURL)
	if err := downloadFile(ctx, sourceURL, csvPath); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	entries, err := parseLAU(csvPath)
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
		Jurisdiction: "eu",
		EntityType:   "municipality",
		Source:       "Eurostat LAU",
		SourceURL:    sourceURL,
		License:      "CC BY 4.0",
		DataFile:     "data.db",
		Format:       dict.FormatSpec{Normalize: "lowercase_ascii"},
	})
}

func parseLAU(path string) (map[string]*dict.Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.Comma = ';'
	r.LazyQuotes = true
	r.FieldsPerRecord = -1
	r.ReuseRecord = true

	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	if len(header) <= 2 {
		if _, seekErr := f.Seek(0, io.SeekStart); seekErr != nil {
			return nil, fmt.Errorf("seek: %w", seekErr)
		}
		r = csv.NewReader(f)
		r.Comma = ','
		r.LazyQuotes = true
		r.FieldsPerRecord = -1
		r.ReuseRecord = true
		header, err = r.Read()
		if err != nil {
			return nil, fmt.Errorf("read header (comma): %w", err)
		}
	}

	colIdx := make(map[string]int)
	for i, h := range header {
		clean := strings.TrimSpace(strings.ToUpper(h))
		clean = strings.TrimPrefix(clean, "\xef\xbb\xbf")
		colIdx[clean] = i
	}

	lauCol := colByNames(colIdx, "LAU CODE", "LAU_CODE", "LAU_NAT_CODE", "LAU")
	nameCol := colByNames(colIdx, "LAU NAME NATIONAL", "LAU_NAME_NATIONAL", "LAU NAME LATIN", "LAU_NAME_LATIN", "LAU_NAME", "NAME")
	nutsCol := colByNames(colIdx, "NUTS 3 CODE", "NUTS3", "NUTS_3", "NUTS3_CODE")
	countryCol := colByNames(colIdx, "CNTR_CODE", "COUNTRY")
	popCol := colByNames(colIdx, "POPULATION", "POP_2021", "POP_2024", "TOTAL_POP")
	giscoCol := colByNames(colIdx, "GISCO_ID")

	entries := make(map[string]*dict.Entry, 120000)
	var count int
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		lau := strings.TrimSpace(safeCol(record, lauCol))
		name := strings.TrimSpace(safeCol(record, nameCol))
		country := strings.TrimSpace(safeCol(record, countryCol))

		// GISCO CSV: extract LAU code and country from GISCO_ID (e.g. "FR_02022")
		if lau == "" && giscoCol >= 0 {
			gisco := strings.TrimSpace(safeCol(record, giscoCol))
			if parts := strings.SplitN(gisco, "_", 2); len(parts) == 2 {
				if country == "" {
					country = parts[0]
				}
				lau = parts[1]
			}
		}

		if name == "" && lau == "" {
			continue
		}

		meta := map[string]string{
			"lau_code":   lau,
			"name":       name,
			"nuts3":      safeCol(record, nutsCol),
			"country":    country,
			"population": safeCol(record, popCol),
		}

		if lau != "" {
			entries[strings.ToLower(lau)] = &dict.Entry{Metadata: meta}
		}
		if name != "" {
			entries[dict.NormalizeLowercaseASCII(name)] = &dict.Entry{Metadata: meta}
		}

		count++
		if count%50000 == 0 {
			fmt.Printf("  %d communes LAU traitees...\n", count)
		}
	}

	fmt.Printf("  %d communes LAU EU\n", count)
	return entries, nil
}
