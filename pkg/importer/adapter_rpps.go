// CLAUDE:SUMMARY Import adapter for RPPS (health professionals registry, France, 1.8M entries).
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
	Register(&rppsAdapter{})
}

type rppsAdapter struct{}

func (a *rppsAdapter) ID() string      { return "rpps" }
func (a *rppsAdapter) DictID() string  { return "rpps-fr" }
func (a *rppsAdapter) Description() string {
	return "RPPS — professionnels de sante France (medecins, infirmiers, etc.)"
}
func (a *rppsAdapter) DefaultURL() string {
	return "https://annuaire.sante.fr/web/site/extractions/ps"
}
func (a *rppsAdapter) License() string { return "Licence Ouverte v2" }

func (a *rppsAdapter) Import(ctx context.Context, sourceURL, outputDir string) error {
	dlDir := filepath.Join(outputDir, "_download")
	if err := ensureDir(dlDir); err != nil {
		return err
	}
	defer os.RemoveAll(dlDir)

	zipPath := filepath.Join(dlDir, "rpps.zip")
	fmt.Printf("  telechargement %s...\n", sourceURL)
	fmt.Println("  (fichier volumineux, patience...)")
	if err := downloadFile(ctx, sourceURL, zipPath); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	files, err := unzipFile(zipPath, dlDir)
	if err != nil {
		// Might be a direct CSV, not a ZIP
		entries, parseErr := parseRPPS(zipPath)
		if parseErr != nil {
			return fmt.Errorf("unzip: %w (also tried CSV: %v)", err, parseErr)
		}
		return a.saveEntries(entries, sourceURL, outputDir)
	}

	var csvPath string
	for _, f := range files {
		if strings.HasSuffix(strings.ToLower(f), ".csv") {
			csvPath = f
			break
		}
	}
	if csvPath == "" {
		return fmt.Errorf("no CSV found in RPPS archive")
	}

	entries, err := parseRPPS(csvPath)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	return a.saveEntries(entries, sourceURL, outputDir)
}

func (a *rppsAdapter) saveEntries(entries map[string]*dict.Entry, sourceURL, outputDir string) error {
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
		EntityType:   "health_professional",
		Source:       "RPPS (annuaire.sante.fr)",
		SourceURL:    sourceURL,
		License:      "Licence Ouverte v2",
		DataFile:     "data.gob",
		Format:       dict.FormatSpec{Normalize: "lowercase_ascii"},
		EntitySpec: &dict.EntitySpec{
			Pattern:     `^\d{11}$`,
			Sensitivity: "high",
		},
	})
}

func parseRPPS(path string) (map[string]*dict.Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.Comma = '|'
	r.LazyQuotes = true
	r.FieldsPerRecord = -1
	r.ReuseRecord = true

	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	// Try pipe, then semicolon, then comma
	if len(header) <= 1 {
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
	if len(header) <= 1 {
		if _, seekErr := f.Seek(0, io.SeekStart); seekErr != nil {
			return nil, fmt.Errorf("seek: %w", seekErr)
		}
		r = csv.NewReader(f)
		r.Comma = ','
		r.LazyQuotes = true
		r.FieldsPerRecord = -1
		r.ReuseRecord = true
		header, err = r.Read()
		if err != nil {
			return nil, fmt.Errorf("read header (comma): %w", err)
		}
	}

	colIdx := make(map[string]int)
	for i, h := range header {
		clean := strings.TrimSpace(strings.ToLower(h))
		// Strip BOM
		clean = strings.TrimPrefix(clean, "\xef\xbb\xbf")
		colIdx[clean] = i
	}

	rppsCol := colByNames(colIdx, "identification nationale pp", "rpps", "identifiant_pp", "numero rpps")
	nomCol := colByNames(colIdx, "nom d'exercice", "nom exercice", "nom", "nom_exercice")
	prenomCol := colByNames(colIdx, "prenom d'exercice", "prenom exercice", "prenom", "prenom_exercice")
	profCol := colByNames(colIdx, "libelle profession", "profession", "lib_profession")
	specCol := colByNames(colIdx, "libelle savoir-faire", "specialite", "savoir_faire")

	entries := make(map[string]*dict.Entry, 2000000)
	var count int
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		rpps := strings.TrimSpace(safeCol(record, rppsCol))
		nom := strings.TrimSpace(safeCol(record, nomCol))
		prenom := strings.TrimSpace(safeCol(record, prenomCol))

		if rpps == "" && nom == "" {
			continue
		}

		meta := map[string]string{
			"rpps":       rpps,
			"nom":        nom,
			"prenom":     prenom,
			"profession": safeCol(record, profCol),
			"specialite": safeCol(record, specCol),
		}

		if rpps != "" {
			entries[strings.ToLower(rpps)] = &dict.Entry{Metadata: meta}
		}
		fullName := strings.TrimSpace(nom + " " + prenom)
		if fullName != "" {
			entries[dict.NormalizeLowercaseASCII(fullName)] = &dict.Entry{Metadata: meta}
		}
		// Also index by last name alone
		if nom != "" {
			entries[dict.NormalizeLowercaseASCII(nom)] = &dict.Entry{Metadata: meta}
		}

		count++
		if count%500000 == 0 {
			fmt.Printf("  %d professionnels RPPS traites...\n", count)
		}
	}

	fmt.Printf("  %d professionnels RPPS\n", count)
	return entries, nil
}
