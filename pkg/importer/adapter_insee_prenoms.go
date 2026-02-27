// CLAUDE:SUMMARY Import adapter for INSEE French first names aggregated by frequency across years.
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
	Register(&inseePrenomsAdapter{})
}

type inseePrenomsAdapter struct{}

func (a *inseePrenomsAdapter) ID() string          { return "insee-prenoms-fr" }
func (a *inseePrenomsAdapter) DictID() string      { return "prenoms-fr" }
func (a *inseePrenomsAdapter) Description() string { return "INSEE prenoms francais (fichier national)" }
func (a *inseePrenomsAdapter) DefaultURL() string   { return "https://www.insee.fr/fr/statistiques/fichier/2540004/nat2021_csv.zip" }
func (a *inseePrenomsAdapter) License() string      { return "CC0" }

func (a *inseePrenomsAdapter) Import(ctx context.Context, sourceURL, outputDir string) error {
	dlDir := filepath.Join(outputDir, "_download")
	if err := ensureDir(dlDir); err != nil {
		return err
	}
	defer os.RemoveAll(dlDir)

	zipPath := filepath.Join(dlDir, "prenoms.zip")
	fmt.Printf("  telechargement %s...\n", sourceURL)
	if err := downloadFile(ctx, sourceURL, zipPath); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	files, err := unzipFile(zipPath, dlDir)
	if err != nil {
		return fmt.Errorf("unzip: %w", err)
	}

	// Find the CSV file.
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

	entries, err := parseINSEEPrenoms(csvPath)
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
		Jurisdiction: "fr",
		EntityType:   "first_name",
		Source:       "INSEE fichier des prenoms",
		SourceURL:    sourceURL,
		License:      "CC0",
		DataFile:     "data.gob",
		Format:       dict.FormatSpec{Normalize: "lowercase_ascii"},
	})
}

// parseINSEEPrenoms reads the INSEE prenoms CSV (semicolon-delimited) and aggregates
// by first name, summing frequencies across years. Columns: sexe;preusuel;annais;nombre
func parseINSEEPrenoms(path string) (map[string]*dict.Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.Comma = ';'
	r.LazyQuotes = true

	// Read header.
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	colIdx := make(map[string]int)
	for i, h := range header {
		colIdx[strings.TrimSpace(strings.ToLower(h))] = i
	}

	nameCol, hasName := colIdx["preusuel"]
	sexeCol, hasSexe := colIdx["sexe"]
	nombreCol, hasNombre := colIdx["nombre"]
	if !hasName {
		return nil, fmt.Errorf("column 'preusuel' not found in header %v", header)
	}

	type agg struct {
		sexe  string
		total int
	}
	aggregated := make(map[string]*agg)

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		name := strings.TrimSpace(record[nameCol])
		if name == "" || strings.EqualFold(name, "_prenoms_rares") {
			continue
		}
		key := dict.NormalizeLowercaseASCII(name)

		var sexe string
		if hasSexe && sexeCol < len(record) {
			sexe = strings.TrimSpace(record[sexeCol])
		}

		var nombre int
		if hasNombre && nombreCol < len(record) {
			fmt.Sscanf(strings.TrimSpace(record[nombreCol]), "%d", &nombre)
		}

		if existing, ok := aggregated[key]; ok {
			existing.total += nombre
			if sexe != "" && existing.sexe == "" {
				existing.sexe = sexe
			}
		} else {
			aggregated[key] = &agg{sexe: sexe, total: nombre}
		}
	}

	entries := make(map[string]*dict.Entry, len(aggregated))
	for key, a := range aggregated {
		meta := make(map[string]string)
		if a.sexe != "" {
			meta["sexe"] = a.sexe
		}
		if a.total > 0 {
			meta["frequency"] = fmt.Sprintf("%d", a.total)
		}
		entries[key] = &dict.Entry{Metadata: meta}
	}

	fmt.Printf("  %d prenoms uniques\n", len(entries))
	return entries, nil
}
