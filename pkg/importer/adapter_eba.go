// CLAUDE:SUMMARY Import adapter for EBA credit institutions register (EU banks, ~5K).
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
	Register(&ebaAdapter{})
}

type ebaAdapter struct{}

func (a *ebaAdapter) ID() string      { return "eba-credit-institutions" }
func (a *ebaAdapter) DictID() string  { return "credit-institutions-eu" }
func (a *ebaAdapter) Description() string {
	return "EBA — registre des etablissements de credit EU"
}
func (a *ebaAdapter) DefaultURL() string {
	return "https://www.eba.europa.eu/risk-analysis-and-data/credit-institutions-register/register-payment-and-e-money/download"
}
func (a *ebaAdapter) License() string { return "Public" }

func (a *ebaAdapter) Import(ctx context.Context, sourceURL, outputDir string) error {
	dlDir := filepath.Join(outputDir, "_download")
	if err := ensureDir(dlDir); err != nil {
		return err
	}
	defer os.RemoveAll(dlDir)

	csvPath := filepath.Join(dlDir, "eba.csv")
	fmt.Printf("  telechargement %s...\n", sourceURL)
	if err := downloadFile(ctx, sourceURL, csvPath); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	entries, err := parseEBA(csvPath)
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
		EntityType:   "credit_institution",
		Source:       "EBA Credit Institutions Register",
		SourceURL:    sourceURL,
		License:      "Public",
		DataFile:     "data.gob",
		Format:       dict.FormatSpec{Normalize: "lowercase_ascii"},
	})
}

func parseEBA(path string) (map[string]*dict.Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.Comma = ','
	r.LazyQuotes = true
	r.FieldsPerRecord = -1

	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	if len(header) <= 2 {
		f.Seek(0, io.SeekStart)
		r = csv.NewReader(f)
		r.Comma = ';'
		r.LazyQuotes = true
		r.FieldsPerRecord = -1
		header, err = r.Read()
		if err != nil {
			return nil, fmt.Errorf("read header (semicolon): %w", err)
		}
	}

	colIdx := make(map[string]int)
	for i, h := range header {
		clean := strings.TrimSpace(strings.ToLower(h))
		clean = strings.TrimPrefix(clean, "\xef\xbb\xbf")
		colIdx[clean] = i
	}

	nameCol := colByNames(colIdx, "name", "institution name", "entity_name")
	leiCol := colByNames(colIdx, "lei", "lei_code")
	countryCol := colByNames(colIdx, "country", "country_code", "country of establishment")
	typeCol := colByNames(colIdx, "type", "entity_type", "institution type")

	entries := make(map[string]*dict.Entry, 6000)
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		name := strings.TrimSpace(safeCol(record, nameCol))
		if name == "" {
			continue
		}

		meta := map[string]string{
			"name":    name,
			"lei":     safeCol(record, leiCol),
			"country": safeCol(record, countryCol),
			"type":    safeCol(record, typeCol),
		}

		entries[dict.NormalizeLowercaseASCII(name)] = &dict.Entry{Metadata: meta}
		if lei := meta["lei"]; lei != "" {
			entries[strings.ToLower(lei)] = &dict.Entry{Metadata: meta}
		}
	}

	fmt.Printf("  %d etablissements de credit EU\n", len(entries))
	return entries, nil
}
