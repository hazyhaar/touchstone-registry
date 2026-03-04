// CLAUDE:SUMMARY Import adapter for GLEIF LEI (Legal Entity Identifiers) golden copy CSV from gleif.org.
package importer

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hazyhaar/touchstone-registry/pkg/dict"
)

func init() {
	Register(&gleifAdapter{})
}

type gleifAdapter struct{}

func (a *gleifAdapter) ID() string      { return "gleif-lei" }
func (a *gleifAdapter) DictID() string  { return "lei" }
func (a *gleifAdapter) Description() string {
	return "GLEIF LEI (Legal Entity Identifiers) golden copy, entites financieres mondiales"
}
func (a *gleifAdapter) DefaultURL() string {
	return "https://goldencopy.gleif.org/api/v2/golden-copies/publishes/lei2/latest?type=full_file&format=csv"
}
func (a *gleifAdapter) License() string { return "CC0" }

func (a *gleifAdapter) Import(ctx context.Context, sourceURL, outputDir string) error {
	dlDir := filepath.Join(outputDir, "_download")
	if err := ensureDir(dlDir); err != nil {
		return err
	}
	defer os.RemoveAll(dlDir)

	// The GLEIF API returns JSON with the actual download URL.
	actualURL, err := resolveGLEIFURL(ctx, sourceURL)
	if err != nil {
		return fmt.Errorf("resolve GLEIF URL: %w", err)
	}

	zipPath := filepath.Join(dlDir, "lei.zip")
	fmt.Printf("  telechargement %s...\n", actualURL)
	fmt.Println("  (fichier volumineux ~450 Mo, patience...)")
	if dlErr := downloadFile(ctx, actualURL, zipPath); dlErr != nil {
		return fmt.Errorf("download: %w", dlErr)
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
		return fmt.Errorf("no CSV found in GLEIF ZIP")
	}

	entries, err := parseGLEIF(csvPath)
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
		EntityType: "legal_entity",
		Source:     "GLEIF Golden Copy",
		SourceURL:  sourceURL,
		License:    "CC0",
		DataFile:   "data.gob",
		Format:     dict.FormatSpec{Normalize: "lowercase_ascii"},
		EntitySpec: &dict.EntitySpec{
			Pattern:    `^[A-Z0-9]{20}$`,
			Checksum:   "mod97",
			Sensitivity: "public",
		},
	})
}

// resolveGLEIFURL queries the GLEIF API to get the actual CSV ZIP download URL.
func resolveGLEIFURL(ctx context.Context, apiURL string) (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "touchstone-registry/1.0 (open-data-import)")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GLEIF API HTTP %d", resp.StatusCode)
	}

	var result struct {
		Data struct {
			FullFile struct {
				CSV struct {
					URL string `json:"url"`
				} `json:"csv"`
			} `json:"full_file"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode GLEIF response: %w", err)
	}

	url := result.Data.FullFile.CSV.URL
	if url == "" {
		return "", fmt.Errorf("no CSV URL in GLEIF API response")
	}

	return url, nil
}

// parseGLEIF reads the GLEIF LEI CSV.
// Key columns: LEI, Entity.LegalName, Entity.LegalJurisdiction, Entity.EntityStatus.
func parseGLEIF(path string) (map[string]*dict.Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.LazyQuotes = true
	r.FieldsPerRecord = -1
	r.ReuseRecord = true

	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	colIdx := make(map[string]int)
	for i, h := range header {
		colIdx[strings.TrimSpace(h)] = i
	}

	leiCol := colByNames(colIdx, "LEI")
	nameCol := colByNames(colIdx, "Entity.LegalName")
	jurisdictionCol := colByNames(colIdx, "Entity.LegalJurisdiction")
	statusCol := colByNames(colIdx, "Entity.EntityStatus")
	countryCol := colByNames(colIdx, "Entity.LegalAddress.Country")
	categoryCol := colByNames(colIdx, "Entity.EntityCategory")

	if leiCol < 0 || nameCol < 0 {
		return nil, fmt.Errorf("required columns (LEI, Entity.LegalName) not found in header")
	}

	entries := make(map[string]*dict.Entry, 500000)
	var count, skipped int
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		// Only active entities.
		status := safeCol(record, statusCol)
		if status != "" && status != "ACTIVE" {
			skipped++
			continue
		}

		lei := safeCol(record, leiCol)
		name := safeCol(record, nameCol)
		if lei == "" || name == "" {
			continue
		}

		meta := map[string]string{
			"lei":          lei,
			"jurisdiction": safeCol(record, jurisdictionCol),
			"country":      safeCol(record, countryCol),
		}
		if cat := safeCol(record, categoryCol); cat != "" {
			meta["category"] = cat
		}

		// Index by LEI code.
		entries[strings.ToLower(lei)] = &dict.Entry{Metadata: meta}

		// Also index by company name.
		key := dict.NormalizeLowercaseASCII(name)
		entries[key] = &dict.Entry{Metadata: meta}

		count++
		if count%500000 == 0 {
			fmt.Printf("  %d entites GLEIF traitees...\n", count)
		}
	}

	fmt.Printf("  %d entites LEI actives (%d inactives ignorees)\n", count, skipped)
	return entries, nil
}
