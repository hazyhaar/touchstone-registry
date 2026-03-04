// CLAUDE:SUMMARY Import adapter for INSEE COG arrondissements municipaux France (~45 entries).
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
	Register(&arrondissementsAdapter{})
}

type arrondissementsAdapter struct{}

func (a *arrondissementsAdapter) ID() string      { return "insee-arrondissements" }
func (a *arrondissementsAdapter) DictID() string  { return "arrondissements-fr" }
func (a *arrondissementsAdapter) Description() string {
	return "INSEE COG — arrondissements municipaux (Paris, Lyon, Marseille)"
}
func (a *arrondissementsAdapter) DefaultURL() string {
	return "https://www.insee.fr/fr/statistiques/fichier/8377162/v_commune_2025.csv"
}
func (a *arrondissementsAdapter) License() string { return "CC0" }

func (a *arrondissementsAdapter) Import(ctx context.Context, sourceURL, outputDir string) error {
	dlDir := filepath.Join(outputDir, "_download")
	if err := ensureDir(dlDir); err != nil {
		return err
	}
	defer os.RemoveAll(dlDir)

	csvPath := filepath.Join(dlDir, "communes.csv")
	fmt.Printf("  telechargement %s...\n", sourceURL)
	if err := downloadFile(ctx, sourceURL, csvPath); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	entries, err := parseArrondissements(csvPath)
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
		EntityType:   "arrondissement",
		Source:       "INSEE COG communes",
		SourceURL:    sourceURL,
		License:      "CC0",
		DataFile:     "data.db",
		Format:       dict.FormatSpec{Normalize: "lowercase_ascii"},
	})
}

func parseArrondissements(path string) (map[string]*dict.Entry, error) {
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

	colIdx := make(map[string]int)
	for i, h := range header {
		colIdx[strings.TrimSpace(strings.ToUpper(h))] = i
	}

	typecomCol := colByNames(colIdx, "TYPECOM")
	codeCol := colByNames(colIdx, "COM", "CODE")
	nameCol := colByNames(colIdx, "LIBELLE", "NCC", "NCCENR")
	actualCol := colByNames(colIdx, "ACTUAL")

	entries := make(map[string]*dict.Entry, 100)
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		// Only current entries
		if actual := safeCol(record, actualCol); actual != "" && actual != "1" {
			continue
		}

		// Only arrondissements municipaux (TYPECOM = ARM)
		typecom := safeCol(record, typecomCol)
		if typecom != "ARM" {
			continue
		}

		code := safeCol(record, codeCol)
		name := safeCol(record, nameCol)
		if name == "" {
			continue
		}

		meta := map[string]string{
			"code": code,
			"name": name,
			"type": "arrondissement",
		}

		entries[dict.NormalizeLowercaseASCII(name)] = &dict.Entry{Metadata: meta}
		if code != "" {
			entries[strings.ToLower(code)] = &dict.Entry{Metadata: meta}
		}
	}

	fmt.Printf("  %d arrondissements\n", len(entries))
	return entries, nil
}
