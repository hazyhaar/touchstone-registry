// CLAUDE:SUMMARY Import adapter for INSEE legal forms (catégories juridiques France, ~90 entries).
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
	Register(&legalFormsFRAdapter{})
}

type legalFormsFRAdapter struct{}

func (a *legalFormsFRAdapter) ID() string      { return "insee-legal-forms" }
func (a *legalFormsFRAdapter) DictID() string  { return "legal-forms-fr" }
func (a *legalFormsFRAdapter) Description() string {
	return "INSEE — categories juridiques (SARL, SAS, SA, etc.)"
}
func (a *legalFormsFRAdapter) DefaultURL() string {
	return "https://gist.githubusercontent.com/johanricher/de408d0ff610989d6c1f8146e37f668c/raw"
}
func (a *legalFormsFRAdapter) License() string { return "CC0" }

func (a *legalFormsFRAdapter) Import(ctx context.Context, sourceURL, outputDir string) error {
	dlDir := filepath.Join(outputDir, "_download")
	if err := ensureDir(dlDir); err != nil {
		return err
	}
	defer os.RemoveAll(dlDir)

	csvPath := filepath.Join(dlDir, "cj.csv")
	fmt.Printf("  telechargement %s...\n", sourceURL)
	if err := downloadFile(ctx, sourceURL, csvPath); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	entries, err := parseLegalFormsFR(csvPath)
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
		Jurisdiction: "fr",
		EntityType:   "legal_form",
		Source:       "INSEE categories juridiques",
		SourceURL:    sourceURL,
		License:      "CC0",
		DataFile:     "data.db",
		Format:       dict.FormatSpec{Normalize: "lowercase_ascii"},
	})
}

func parseLegalFormsFR(path string) (map[string]*dict.Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.Comma = '\t'
	r.LazyQuotes = true
	r.FieldsPerRecord = -1

	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	// Try comma if tab gives single column
	if len(header) <= 1 {
		if _, seekErr := f.Seek(0, io.SeekStart); seekErr != nil {
			return nil, fmt.Errorf("seek: %w", seekErr)
		}
		r = csv.NewReader(f)
		r.Comma = ','
		r.LazyQuotes = true
		r.FieldsPerRecord = -1
		header, err = r.Read()
		if err != nil {
			return nil, fmt.Errorf("read header (comma): %w", err)
		}
	}
	// Try semicolon
	if len(header) <= 1 {
		if _, seekErr := f.Seek(0, io.SeekStart); seekErr != nil {
			return nil, fmt.Errorf("seek: %w", seekErr)
		}
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

	codeCol := colByNames(colIdx, "code", "cj")
	labelCol := colByNames(colIdx, "libelle", "libellé", "label")

	entries := make(map[string]*dict.Entry, 200)
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		code := strings.TrimSpace(safeCol(record, codeCol))
		label := strings.TrimSpace(safeCol(record, labelCol))
		if label == "" {
			continue
		}

		meta := map[string]string{
			"code":  code,
			"label": label,
		}

		if code != "" {
			entries[strings.ToLower(code)] = &dict.Entry{Metadata: meta}
		}
		entries[dict.NormalizeLowercaseASCII(label)] = &dict.Entry{Metadata: meta}
	}

	fmt.Printf("  %d formes juridiques\n", len(entries))
	return entries, nil
}
