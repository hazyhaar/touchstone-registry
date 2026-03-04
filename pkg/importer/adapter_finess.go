// CLAUDE:SUMMARY Import adapter for FINESS (health establishments registry, France).
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
	Register(&finessAdapter{})
}

type finessAdapter struct{}

func (a *finessAdapter) ID() string      { return "finess" }
func (a *finessAdapter) DictID() string  { return "finess-fr" }
func (a *finessAdapter) Description() string {
	return "FINESS — etablissements sanitaires et sociaux France"
}
func (a *finessAdapter) DefaultURL() string {
	return "https://static.data.gouv.fr/resources/finess-extraction-du-fichier-des-etablissements/20260108-153415/etalab-cs1100507-stock-20260107-0342.csv"
}
func (a *finessAdapter) License() string { return "Licence Ouverte v2" }

func (a *finessAdapter) Import(ctx context.Context, sourceURL, outputDir string) error {
	dlDir := filepath.Join(outputDir, "_download")
	if err := ensureDir(dlDir); err != nil {
		return err
	}
	defer os.RemoveAll(dlDir)

	csvPath := filepath.Join(dlDir, "finess.csv")
	fmt.Printf("  telechargement %s...\n", sourceURL)
	if err := downloadFile(ctx, sourceURL, csvPath); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	entries, err := parseFINESS(csvPath)
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
		EntityType:   "health_establishment",
		Source:       "FINESS (data.gouv.fr)",
		SourceURL:    sourceURL,
		License:      "Licence Ouverte v2",
		DataFile:     "data.db",
		Format:       dict.FormatSpec{Normalize: "lowercase_ascii"},
	})
}

func parseFINESS(path string) (map[string]*dict.Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// FINESS etalab format: semicolon-separated, first line is metadata "finess;etalab;93;date"
	// No real header row — columns are positional:
	// 0:type, 1:nofinesset, 2:nofinessej, 3:rs, 4:rslongue, 5-7:address fields...
	// 18:categetab, 19:categetablib, ...
	r := csv.NewReader(f)
	r.Comma = ';'
	r.LazyQuotes = true
	r.FieldsPerRecord = -1
	r.ReuseRecord = true

	// Skip first line (metadata)
	first, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read first line: %w", err)
	}

	// Check if it's actually a header with column names.
	// The etalab metadata line starts with "finess;etalab;..." — do NOT match
	// on bare "finess" which would false-positive on the metadata line.
	hasHeader := false
	for _, h := range first {
		lower := strings.ToLower(strings.TrimSpace(h))
		if lower == "nofinesset" || lower == "rs" || lower == "rslongue" {
			hasHeader = true
			break
		}
	}

	if hasHeader {
		// Parse with header
		colIdx := make(map[string]int)
		for i, h := range first {
			colIdx[strings.TrimSpace(strings.ToLower(h))] = i
		}
		return parseFINESSWithHeader(r, colIdx)
	}

	// Positional parsing (etalab format)
	entries := make(map[string]*dict.Entry, 80000)
	processLine := func(record []string) {
		if len(record) < 5 {
			return
		}
		recType := strings.TrimSpace(record[0])
		if recType != "structureet" && recType != "geolocalisation" {
			return
		}
		if recType == "geolocalisation" {
			return // Skip geolocalisation lines
		}

		finess := strings.TrimSpace(record[1])
		shortName := strings.TrimSpace(record[3])
		longName := strings.TrimSpace(record[4])

		name := longName
		if name == "" {
			name = shortName
		}

		var catCode, catLib, dept, commune string
		if len(record) > 18 {
			catCode = strings.TrimSpace(record[18])
		}
		if len(record) > 19 {
			catLib = strings.TrimSpace(record[19])
		}
		if len(record) > 13 {
			dept = strings.TrimSpace(record[13])
		}
		if len(record) > 15 {
			commune = strings.TrimSpace(record[15])
		}

		meta := map[string]string{
			"finess":        finess,
			"name":          name,
			"short_name":    shortName,
			"category_code": catCode,
			"category":      catLib,
			"dept":          dept,
			"commune":       commune,
		}

		if finess != "" {
			entries[strings.ToLower(finess)] = &dict.Entry{Metadata: meta}
		}
		if name != "" {
			entries[dict.NormalizeLowercaseASCII(name)] = &dict.Entry{Metadata: meta}
		}
	}

	// Process first data line (it's the first record we already read, if not header)
	processLine(first)

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		processLine(record)
	}

	fmt.Printf("  %d etablissements FINESS\n", len(entries))
	return entries, nil
}

func parseFINESSWithHeader(r *csv.Reader, colIdx map[string]int) (map[string]*dict.Entry, error) {
	finessCol := colByNames(colIdx, "nofinesset", "finess")
	nameCol := colByNames(colIdx, "rs", "rslongue", "raison_sociale")
	catCol := colByNames(colIdx, "categetab", "categorie")
	deptCol := colByNames(colIdx, "departement", "dept")
	communeCol := colByNames(colIdx, "commune", "libcommune", "communeet")

	entries := make(map[string]*dict.Entry, 80000)
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		finess := strings.TrimSpace(safeCol(record, finessCol))
		name := strings.TrimSpace(safeCol(record, nameCol))

		meta := map[string]string{
			"finess":   finess,
			"name":     name,
			"category": safeCol(record, catCol),
			"dept":     safeCol(record, deptCol),
			"commune":  safeCol(record, communeCol),
		}

		if finess != "" {
			entries[strings.ToLower(finess)] = &dict.Entry{Metadata: meta}
		}
		if name != "" {
			entries[dict.NormalizeLowercaseASCII(name)] = &dict.Entry{Metadata: meta}
		}
	}

	fmt.Printf("  %d etablissements FINESS\n", len(entries))
	return entries, nil
}
