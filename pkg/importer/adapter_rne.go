// CLAUDE:SUMMARY Import adapter for RNE (Registre National des Entreprises, ex-RCS, INPI France, 12M+).
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
	Register(&rneAdapter{})
}

type rneAdapter struct{}

func (a *rneAdapter) ID() string      { return "inpi-rne" }
func (a *rneAdapter) DictID() string  { return "rne-fr" }
func (a *rneAdapter) Description() string {
	return "RNE (ex-RCS) — Registre National des Entreprises INPI France (12M+)"
}
func (a *rneAdapter) DefaultURL() string {
	return "https://data.inpi.fr/export/IMR_Donnees_Ouvertes.csv"
}
func (a *rneAdapter) License() string { return "Licence Ouverte v2" }

func (a *rneAdapter) Import(ctx context.Context, sourceURL, outputDir string) error {
	dlDir := filepath.Join(outputDir, "_download")
	if err := ensureDir(dlDir); err != nil {
		return err
	}
	defer os.RemoveAll(dlDir)

	csvPath := filepath.Join(dlDir, "rne.csv")
	fmt.Printf("  telechargement %s...\n", sourceURL)
	fmt.Println("  (fichier tres volumineux, patience...)")
	if err := downloadFile(ctx, sourceURL, csvPath); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	entries, err := parseRNE(csvPath)
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
		EntityType:   "company",
		Source:       "INPI RNE",
		SourceURL:    sourceURL,
		License:      "Licence Ouverte v2",
		DataFile:     "data.db",
		Format:       dict.FormatSpec{Normalize: "lowercase_ascii"},
		EntitySpec: &dict.EntitySpec{
			Pattern:     `^\d{9}$`,
			Sensitivity: "public",
		},
	})
}

func parseRNE(path string) (map[string]*dict.Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.Comma = ','
	r.LazyQuotes = true
	r.FieldsPerRecord = -1
	r.ReuseRecord = true

	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	// Try semicolon
	if len(header) <= 2 {
		if _, seekErr := f.Seek(0, io.SeekStart); seekErr != nil {
			return nil, fmt.Errorf("seek: %w", seekErr)
		}
		r = csv.NewReader(f)
		r.Comma = ';'
		r.LazyQuotes = true
		r.FieldsPerRecord = -1
		r.ReuseRecord = true
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

	sirenCol := colByNames(colIdx, "siren", "numero_siren")
	nameCol := colByNames(colIdx, "denomination", "denomination_sociale", "nom_complet", "raison_sociale")
	nafCol := colByNames(colIdx, "activite_principale", "code_ape", "naf")
	formeCol := colByNames(colIdx, "nature_juridique", "forme_juridique", "categorie_juridique")
	cpCol := colByNames(colIdx, "code_postal", "adresse_code_postal")
	communeCol := colByNames(colIdx, "commune", "adresse_commune", "libelle_commune")

	entries := make(map[string]*dict.Entry, 15000000)
	var count int
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		siren := strings.TrimSpace(safeCol(record, sirenCol))
		name := strings.TrimSpace(safeCol(record, nameCol))

		if siren == "" && name == "" {
			continue
		}

		meta := map[string]string{
			"siren":   siren,
			"name":    name,
			"naf":     safeCol(record, nafCol),
			"forme":   safeCol(record, formeCol),
			"cp":      safeCol(record, cpCol),
			"commune": safeCol(record, communeCol),
		}

		if siren != "" {
			entries[strings.ToLower(siren)] = &dict.Entry{Metadata: meta}
		}
		if name != "" {
			entries[dict.NormalizeLowercaseASCII(name)] = &dict.Entry{Metadata: meta}
		}

		count++
		if count%2000000 == 0 {
			fmt.Printf("  %d entreprises RNE traitees...\n", count)
		}
	}

	fmt.Printf("  %d entreprises RNE\n", count)
	return entries, nil
}
