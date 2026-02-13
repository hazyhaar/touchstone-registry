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
	Register(&inseeCommunesAdapter{})
}

type inseeCommunesAdapter struct{}

func (a *inseeCommunesAdapter) ID() string          { return "insee-communes-fr" }
func (a *inseeCommunesAdapter) DictID() string      { return "communes-fr" }
func (a *inseeCommunesAdapter) Description() string { return "INSEE COG communes de France" }
func (a *inseeCommunesAdapter) DefaultURL() string   { return "https://www.insee.fr/fr/statistiques/fichier/7766585/v_commune_2024.csv" }
func (a *inseeCommunesAdapter) License() string      { return "CC0" }

func (a *inseeCommunesAdapter) Import(ctx context.Context, sourceURL, outputDir string) error {
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

	entries, err := parseINSEECommunes(csvPath)
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
		EntityType:   "city",
		Source:       "INSEE COG",
		SourceURL:    sourceURL,
		License:      "CC0",
		DataFile:     "data.gob",
		Format:       dict.FormatSpec{Normalize: "lowercase_ascii"},
	})
}

// parseINSEECommunes reads the INSEE COG CSV (comma-delimited).
// Columns include: COM, TYPECOM, LIBELLE, DEP.
func parseINSEECommunes(path string) (map[string]*dict.Entry, error) {
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

	libelleCol := -1
	depCol := -1
	comCol := -1
	typecomCol := -1

	for k, v := range colIdx {
		switch {
		case k == "LIBELLE" || k == "NCC" || k == "NCCENR":
			if libelleCol < 0 {
				libelleCol = v
			}
		case k == "DEP":
			depCol = v
		case k == "COM":
			comCol = v
		case k == "TYPECOM":
			typecomCol = v
		}
	}
	if libelleCol < 0 {
		return nil, fmt.Errorf("no name column found in header %v", header)
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

		// Only keep actual communes (TYPECOM = COM).
		if typecomCol >= 0 && typecomCol < len(record) {
			if tc := strings.TrimSpace(record[typecomCol]); tc != "" && tc != "COM" {
				continue
			}
		}

		if libelleCol >= len(record) {
			continue
		}
		name := strings.TrimSpace(record[libelleCol])
		if name == "" {
			continue
		}
		key := dict.NormalizeLowercaseASCII(name)

		meta := make(map[string]string)
		if depCol >= 0 && depCol < len(record) {
			meta["departement"] = strings.TrimSpace(record[depCol])
		}
		if comCol >= 0 && comCol < len(record) {
			meta["code_commune"] = strings.TrimSpace(record[comCol])
		}

		entries[key] = &dict.Entry{Metadata: meta}
	}

	fmt.Printf("  %d communes\n", len(entries))
	return entries, nil
}
