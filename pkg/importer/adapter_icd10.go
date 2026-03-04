// CLAUDE:SUMMARY Import adapter for ICD-10 (CIM-10) medical classification codes.
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
	Register(&icd10Adapter{})
}

type icd10Adapter struct{}

func (a *icd10Adapter) ID() string      { return "icd10" }
func (a *icd10Adapter) DictID() string  { return "icd10-fr" }
func (a *icd10Adapter) Description() string {
	return "CIM-10 (ICD-10) — classification internationale des maladies, FR"
}
func (a *icd10Adapter) DefaultURL() string {
	return "https://raw.githubusercontent.com/gr0g/CMA/master/cim10.csv"
}
func (a *icd10Adapter) License() string { return "OMS / Public" }

func (a *icd10Adapter) Import(ctx context.Context, sourceURL, outputDir string) error {
	dlDir := filepath.Join(outputDir, "_download")
	if err := ensureDir(dlDir); err != nil {
		return err
	}
	defer os.RemoveAll(dlDir)

	path := filepath.Join(dlDir, "icd10.txt")
	fmt.Printf("  telechargement %s...\n", sourceURL)
	if err := downloadFile(ctx, sourceURL, path); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	entries, err := parseICD10(path)
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
		Jurisdiction: "fr",
		EntityType:   "medical_code",
		Source:       "ATIH CIM-10",
		SourceURL:    sourceURL,
		License:      "OMS / Public",
		DataFile:     "data.gob",
		Format:       dict.FormatSpec{Normalize: "lowercase_ascii"},
	})
}

func parseICD10(path string) (map[string]*dict.Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// CMA/CIM-10 format: semicolon-separated with header
	r := csv.NewReader(f)
	r.Comma = ';'
	r.LazyQuotes = true
	r.FieldsPerRecord = -1
	r.ReuseRecord = true

	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	// If semicolon doesn't work, try tab
	if len(header) <= 2 {
		if _, seekErr := f.Seek(0, io.SeekStart); seekErr != nil {
			return nil, fmt.Errorf("seek: %w", seekErr)
		}
		r = csv.NewReader(f)
		r.Comma = '\t'
		r.LazyQuotes = true
		r.FieldsPerRecord = -1
		r.ReuseRecord = true
		header, err = r.Read()
		if err != nil {
			return nil, fmt.Errorf("read header (tab): %w", err)
		}
	}

	colIdx := make(map[string]int)
	for i, h := range header {
		clean := strings.TrimSpace(strings.ToLower(h))
		clean = strings.TrimPrefix(clean, "\xef\xbb\xbf")
		colIdx[clean] = i
	}

	codeCol := colByNames(colIdx, "diag_code", "code", "cim_code")
	labelCol := colByNames(colIdx, "diag_libelle", "libelle", "label", "cim_libelle")
	chapCol := colByNames(colIdx, "chapitre_libelle", "chapter")
	familyCol := colByNames(colIdx, "famille_code", "family")

	entries := make(map[string]*dict.Entry, 20000)
	seen := make(map[string]bool, 20000)
	var count int
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

		if code == "" || strings.HasPrefix(code, "#") || strings.HasPrefix(code, "(") {
			continue
		}
		// Deduplicate
		if seen[code] {
			continue
		}
		seen[code] = true

		meta := map[string]string{
			"code":    code,
			"label":   label,
			"chapter": safeCol(record, chapCol),
			"family":  safeCol(record, familyCol),
		}

		entries[strings.ToLower(code)] = &dict.Entry{Metadata: meta}
		entries[strings.ToLower(strings.ReplaceAll(code, ".", ""))] = &dict.Entry{Metadata: meta}
		if label != "" {
			entries[dict.NormalizeLowercaseASCII(label)] = &dict.Entry{Metadata: meta}
		}

		count++
	}

	fmt.Printf("  %d codes CIM-10\n", count)
	return entries, nil
}
