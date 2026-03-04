// CLAUDE:SUMMARY Import adapter for French NAF/APE activity codes (732 entries) from INSEE open data.
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
	Register(&nafAdapter{})
}

type nafAdapter struct{}

func (a *nafAdapter) ID() string      { return "insee-naf-fr" }
func (a *nafAdapter) DictID() string  { return "naf-fr" }
func (a *nafAdapter) Description() string {
	return "INSEE NAF Rev.2 codes d'activite francais (732 codes)"
}
func (a *nafAdapter) DefaultURL() string {
	return "https://data.iledefrance.fr/api/explore/v2.1/catalog/datasets/nomenclature-dactivites-francaise-naf-rev-2-code-ape/exports/csv?delimiter=%3B&list_separator=%2C&quote_all=false&with_bom=true"
}
func (a *nafAdapter) License() string { return "CC0" }

func (a *nafAdapter) Import(ctx context.Context, sourceURL, outputDir string) error {
	dlDir := filepath.Join(outputDir, "_download")
	if err := ensureDir(dlDir); err != nil {
		return err
	}
	defer os.RemoveAll(dlDir)

	csvPath := filepath.Join(dlDir, "naf.csv")
	fmt.Printf("  telechargement %s...\n", sourceURL)
	if err := downloadFile(ctx, sourceURL, csvPath); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	entries, err := parseNAF(csvPath)
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
		EntityType:   "activity_code",
		Source:       "INSEE NAF Rev.2",
		SourceURL:    sourceURL,
		License:      "CC0",
		DataFile:     "data.gob",
		Format:       dict.FormatSpec{Normalize: "lowercase_ascii"},
	})
}

// parseNAF reads the INSEE NAF CSV. Expected columns: code;libelle (semicolon-delimited).
func parseNAF(path string) (map[string]*dict.Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Skip BOM if present.
	bom := make([]byte, 3)
	if n, _ := f.Read(bom); n == 3 && bom[0] == 0xEF && bom[1] == 0xBB && bom[2] == 0xBF {
		// BOM consumed, continue.
	} else {
		// Not a BOM, seek back.
		_, _ = f.Seek(0, 0)
	}

	r := csv.NewReader(f)
	r.Comma = ';'
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

	// Find code and label columns (various header formats exist).
	codeCol := -1
	labelCol := -1
	for k, v := range colIdx {
		switch {
		case strings.Contains(k, "code") || k == "naf":
			if codeCol < 0 {
				codeCol = v
			}
		case strings.Contains(k, "libelle") || strings.Contains(k, "intitule") || strings.Contains(k, "label"):
			if labelCol < 0 {
				labelCol = v
			}
		}
	}

	// Fallback: first column = code, second = label.
	if codeCol < 0 && len(header) >= 2 {
		codeCol = 0
		labelCol = 1
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

		code := ""
		label := ""
		if codeCol >= 0 && codeCol < len(record) {
			code = strings.TrimSpace(record[codeCol])
		}
		if labelCol >= 0 && labelCol < len(record) {
			label = strings.TrimSpace(record[labelCol])
		}
		if code == "" {
			continue
		}

		meta := map[string]string{
			"code":  code,
			"label": label,
		}

		lowerCode := strings.ToLower(code)
		entries[lowerCode] = &dict.Entry{Metadata: meta}

		// Also index without letter suffix (e.g., "6201z" → "6201").
		stripped := strings.TrimRight(lowerCode, "abcdefghijklmnopqrstuvwxyz")
		if stripped != lowerCode {
			entries[stripped] = &dict.Entry{Metadata: meta}
		}

		// Also index with dot format (e.g., "6201" → "62.01").
		if len(stripped) == 4 {
			dotted := stripped[:2] + "." + stripped[2:]
			entries[dotted] = &dict.Entry{Metadata: meta}
		} else if len(stripped) == 5 {
			dotted := stripped[:2] + "." + stripped[2:]
			entries[dotted] = &dict.Entry{Metadata: meta}
		}
		if label != "" {
			key := dict.NormalizeLowercaseASCII(label)
			if _, exists := entries[key]; !exists {
				entries[key] = &dict.Entry{Metadata: meta}
			}
		}
	}

	fmt.Printf("  %d codes NAF\n", len(entries))
	return entries, nil
}
