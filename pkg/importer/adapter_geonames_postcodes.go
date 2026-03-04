// CLAUDE:SUMMARY Import adapter for Geonames worldwide postal codes (~5M entries).
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
	Register(&geonamesPostcodesAdapter{})
}

type geonamesPostcodesAdapter struct{}

func (a *geonamesPostcodesAdapter) ID() string      { return "geonames-postcodes" }
func (a *geonamesPostcodesAdapter) DictID() string  { return "postcodes-world" }
func (a *geonamesPostcodesAdapter) Description() string {
	return "Geonames — codes postaux monde entier (5M+)"
}
func (a *geonamesPostcodesAdapter) DefaultURL() string {
	return "https://download.geonames.org/export/zip/allCountries.zip"
}
func (a *geonamesPostcodesAdapter) License() string { return "CC BY 4.0" }

func (a *geonamesPostcodesAdapter) Import(ctx context.Context, sourceURL, outputDir string) error {
	dlDir := filepath.Join(outputDir, "_download")
	if err := ensureDir(dlDir); err != nil {
		return err
	}
	defer os.RemoveAll(dlDir)

	zipPath := filepath.Join(dlDir, "allCountries.zip")
	fmt.Printf("  telechargement %s...\n", sourceURL)
	fmt.Println("  (fichier volumineux, patience...)")
	if err := downloadFile(ctx, sourceURL, zipPath); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	files, err := unzipFile(zipPath, dlDir)
	if err != nil {
		return fmt.Errorf("unzip: %w", err)
	}

	var txtPath string
	for _, f := range files {
		base := strings.ToLower(filepath.Base(f))
		if base == "allcountries.txt" || strings.HasSuffix(base, ".txt") {
			txtPath = f
			break
		}
	}
	if txtPath == "" {
		return fmt.Errorf("no text file found in Geonames ZIP")
	}

	entries, err := parseGeonamesPostcodes(txtPath)
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
		EntityType: "postcode",
		Source:     "Geonames",
		SourceURL:  sourceURL,
		License:    "CC BY 4.0",
		DataFile:   "data.gob",
		Format:     dict.FormatSpec{Normalize: "lowercase"},
	})
}

func parseGeonamesPostcodes(path string) (map[string]*dict.Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Geonames postal codes format: tab-separated
	// country_code, postal_code, place_name, admin_name1, admin_code1, ...
	r := csv.NewReader(f)
	r.Comma = '\t'
	r.LazyQuotes = true
	r.FieldsPerRecord = -1
	r.ReuseRecord = true

	// Deduplicate by (country, postcode) to keep size manageable
	type pcKey struct{ country, code string }
	seen := make(map[pcKey]bool, 3000000)

	entries := make(map[string]*dict.Entry, 5000000)
	var count int
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		if len(record) < 3 {
			continue
		}

		country := strings.TrimSpace(record[0])
		postcode := strings.TrimSpace(record[1])
		placeName := strings.TrimSpace(record[2])

		if postcode == "" || country == "" {
			continue
		}

		pk := pcKey{country: country, code: postcode}
		if seen[pk] {
			continue
		}
		seen[pk] = true

		meta := map[string]string{
			"country":  country,
			"postcode": postcode,
			"place":    placeName,
		}

		// Index by "CC-POSTCODE" (e.g., "fr-75001")
		key := strings.ToLower(country + "-" + postcode)
		entries[key] = &dict.Entry{Metadata: meta}

		// Also index by postcode alone (may collide across countries)
		pcKey2 := strings.ToLower(postcode)
		if _, exists := entries[pcKey2]; !exists {
			entries[pcKey2] = &dict.Entry{Metadata: meta}
		}

		count++
		if count%1000000 == 0 {
			fmt.Printf("  %d codes postaux monde traites...\n", count)
		}
	}

	fmt.Printf("  %d codes postaux monde\n", count)
	return entries, nil
}
