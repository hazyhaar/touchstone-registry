// CLAUDE:SUMMARY Import adapter for MCC (Merchant Category Codes) from Cielo open dataset on GitHub.
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
	Register(&mccAdapter{})
}

type mccAdapter struct{}

func (a *mccAdapter) ID() string      { return "mcc-codes" }
func (a *mccAdapter) DictID() string  { return "mcc" }
func (a *mccAdapter) Description() string {
	return "MCC Merchant Category Codes (codes ISO 18245)"
}
func (a *mccAdapter) DefaultURL() string {
	return "https://raw.githubusercontent.com/greggles/mcc-codes/main/mcc_codes.csv"
}
func (a *mccAdapter) License() string { return "Public Domain" }

func (a *mccAdapter) Import(ctx context.Context, sourceURL, outputDir string) error {
	dlDir := filepath.Join(outputDir, "_download")
	if err := ensureDir(dlDir); err != nil {
		return err
	}
	defer os.RemoveAll(dlDir)

	csvPath := filepath.Join(dlDir, "mcc.csv")
	fmt.Printf("  telechargement %s...\n", sourceURL)
	if err := downloadFile(ctx, sourceURL, csvPath); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	entries, err := parseMCC(csvPath)
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
		EntityType: "merchant_category",
		Source:     "MCC Codes (ISO 18245)",
		SourceURL:  sourceURL,
		License:    "Public Domain",
		DataFile:   "data.gob",
		Format:     dict.FormatSpec{Normalize: "lowercase_ascii"},
	})
}

// parseMCC reads the MCC codes CSV.
// Columns: mcc, edited_description, combined_description, usda_description, irs_description, irs_reportable, id
func parseMCC(path string) (map[string]*dict.Entry, error) {
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

	codeCol := colByNames(colIdx, "mcc")
	descCol := colByNames(colIdx, "edited_description", "combined_description")
	irsDescCol := colByNames(colIdx, "irs_description")
	reportableCol := colByNames(colIdx, "irs_reportable")

	entries := make(map[string]*dict.Entry)
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		code := safeCol(record, codeCol)
		desc := safeCol(record, descCol)
		if code == "" {
			continue
		}

		meta := map[string]string{
			"code":        code,
			"description": desc,
		}
		if irsDesc := safeCol(record, irsDescCol); irsDesc != "" {
			meta["irs_description"] = irsDesc
		}
		if reportable := safeCol(record, reportableCol); reportable != "" {
			meta["irs_reportable"] = reportable
		}

		// Index by code.
		entries[strings.ToLower(code)] = &dict.Entry{Metadata: meta}

		// Also index by description.
		if desc != "" {
			key := dict.NormalizeLowercaseASCII(desc)
			if _, exists := entries[key]; !exists {
				entries[key] = &dict.Entry{Metadata: meta}
			}
		}
	}

	fmt.Printf("  %d codes MCC\n", len(entries))
	return entries, nil
}
