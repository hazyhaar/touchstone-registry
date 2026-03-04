// CLAUDE:SUMMARY Import adapter for ISO 4217 currency codes and names from DataHub open dataset.
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
	Register(&isoCurrenciesAdapter{})
}

type isoCurrenciesAdapter struct{}

func (a *isoCurrenciesAdapter) ID() string      { return "iso-4217-currencies" }
func (a *isoCurrenciesAdapter) DictID() string  { return "currencies" }
func (a *isoCurrenciesAdapter) Description() string {
	return "ISO 4217 currency codes (code, name, country)"
}
func (a *isoCurrenciesAdapter) DefaultURL() string {
	return "https://datahub.io/core/currency-codes/r/codes-all.csv"
}
func (a *isoCurrenciesAdapter) License() string { return "PDDL" }

func (a *isoCurrenciesAdapter) Import(ctx context.Context, sourceURL, outputDir string) error {
	dlDir := filepath.Join(outputDir, "_download")
	if err := ensureDir(dlDir); err != nil {
		return err
	}
	defer os.RemoveAll(dlDir)

	csvPath := filepath.Join(dlDir, "currencies.csv")
	fmt.Printf("  telechargement %s...\n", sourceURL)
	if err := downloadFile(ctx, sourceURL, csvPath); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	entries, err := parseISOCurrencies(csvPath)
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
		ID:         a.DictID(),
		Version:    "2026-03",
		EntityType: "currency",
		Source:     "ISO 4217",
		SourceURL:  sourceURL,
		License:    "PDDL",
		DataFile:   "data.db",
		Format:     dict.FormatSpec{Normalize: "lowercase_ascii"},
	})
}

// parseISOCurrencies reads the ISO 4217 CSV.
// Columns: Entity, Currency, AlphabeticCode, NumericCode, MinorUnit, WithdrawalDate
func parseISOCurrencies(path string) (map[string]*dict.Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.LazyQuotes = true

	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	colIdx := make(map[string]int)
	for i, h := range header {
		colIdx[strings.TrimSpace(h)] = i
	}

	entityCol := colByNames(colIdx, "Entity")
	currencyCol := colByNames(colIdx, "Currency")
	codeCol := colByNames(colIdx, "AlphabeticCode")
	numericCol := colByNames(colIdx, "NumericCode")

	entries := make(map[string]*dict.Entry)
	seen := make(map[string]bool)
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		code := safeCol(record, codeCol)
		currency := safeCol(record, currencyCol)
		if code == "" || seen[code] {
			continue
		}
		seen[code] = true

		meta := map[string]string{
			"code":     code,
			"name":     currency,
			"entity":   safeCol(record, entityCol),
			"numeric":  safeCol(record, numericCol),
		}

		// Index by code (e.g., "eur") and by currency name.
		entries[strings.ToLower(code)] = &dict.Entry{Metadata: meta}
		if currency != "" {
			key := dict.NormalizeLowercaseASCII(currency)
			if _, exists := entries[key]; !exists {
				entries[key] = &dict.Entry{Metadata: meta}
			}
		}
	}

	fmt.Printf("  %d devises ISO 4217\n", len(entries))
	return entries, nil
}

func colByNames(idx map[string]int, names ...string) int {
	for _, n := range names {
		if v, ok := idx[n]; ok {
			return v
		}
	}
	return -1
}

func safeCol(record []string, col int) string {
	if col >= 0 && col < len(record) {
		return strings.TrimSpace(record[col])
	}
	return ""
}
