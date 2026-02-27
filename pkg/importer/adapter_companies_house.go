// CLAUDE:SUMMARY Import adapter for UK Companies House active company names from their bulk CSV download.
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
	Register(&companiesHouseAdapter{})
}

type companiesHouseAdapter struct{}

func (a *companiesHouseAdapter) ID() string     { return "companies-house-uk" }
func (a *companiesHouseAdapter) DictID() string { return "companies-uk" }
func (a *companiesHouseAdapter) Description() string {
	return "Companies House UK (entreprises actives)"
}
func (a *companiesHouseAdapter) DefaultURL() string { return "https://download.companieshouse.gov.uk/BasicCompanyDataAsOneFile-2024-01-01.zip" }
func (a *companiesHouseAdapter) License() string    { return "OGL v3" }

func (a *companiesHouseAdapter) Import(ctx context.Context, sourceURL, outputDir string) error {
	dlDir := filepath.Join(outputDir, "_download")
	if err := ensureDir(dlDir); err != nil {
		return err
	}
	defer os.RemoveAll(dlDir)

	zipPath := filepath.Join(dlDir, "companies.zip")
	fmt.Printf("  telechargement %s...\n", sourceURL)
	fmt.Println("  (fichier volumineux ~2 Go, patience...)")
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

	entries, err := parseCompaniesHouse(csvPath)
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
		Jurisdiction: "uk",
		EntityType:   "company",
		Source:       "Companies House",
		SourceURL:    sourceURL,
		License:      "OGL v3",
		DataFile:     "data.gob",
		Format:       dict.FormatSpec{Normalize: "lowercase_ascii"},
	})
}

// parseCompaniesHouse reads the Companies House CSV in streaming mode.
// Filters only active companies (CompanyStatus = Active).
// Key columns: CompanyName, CompanyNumber, CompanyStatus.
func parseCompaniesHouse(path string) (map[string]*dict.Entry, error) {
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

	nameCol, hasName := colIdx["CompanyName"]
	if !hasName {
		// Try alternative casing.
		for k, v := range colIdx {
			if strings.EqualFold(k, "companyname") {
				nameCol = v
				hasName = true
				break
			}
		}
	}
	if !hasName {
		return nil, fmt.Errorf("column 'CompanyName' not found in header")
	}

	statusCol := -1
	for k, v := range colIdx {
		if strings.EqualFold(k, "companystatus") {
			statusCol = v
			break
		}
	}

	numberCol := -1
	for k, v := range colIdx {
		if strings.EqualFold(k, "companynumber") {
			numberCol = v
			break
		}
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

		// Filter: only active companies.
		if statusCol >= 0 && statusCol < len(record) {
			status := strings.TrimSpace(record[statusCol])
			if !strings.EqualFold(status, "Active") {
				skipped++
				continue
			}
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
		if numberCol >= 0 && numberCol < len(record) {
			meta["company_number"] = strings.TrimSpace(record[numberCol])
		}
		entries[key] = &dict.Entry{Metadata: meta}
	}

	fmt.Printf("  %d entreprises actives UK (%d inactives ignorees)\n", len(entries), skipped)
	return entries, nil
}
