// CLAUDE:SUMMARY Import adapter for BAN (Base Adresse Nationale) France — 25M addresses.
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
	Register(&banAdapter{})
}

type banAdapter struct{}

func (a *banAdapter) ID() string      { return "ban-addresses" }
func (a *banAdapter) DictID() string  { return "addresses-fr" }
func (a *banAdapter) Description() string {
	return "BAN — Base Adresse Nationale France (25M adresses)"
}
func (a *banAdapter) DefaultURL() string {
	return "https://adresse.data.gouv.fr/data/ban/adresses/latest/csv/adresses-france.csv.gz"
}
func (a *banAdapter) License() string { return "Licence Ouverte v2" }

func (a *banAdapter) Import(ctx context.Context, sourceURL, outputDir string) error {
	dlDir := filepath.Join(outputDir, "_download")
	if err := ensureDir(dlDir); err != nil {
		return err
	}
	defer os.RemoveAll(dlDir)

	gzPath := filepath.Join(dlDir, "ban.csv.gz")
	fmt.Printf("  telechargement %s...\n", sourceURL)
	fmt.Println("  (fichier tres volumineux ~1.5 Go, patience...)")
	if err := downloadFile(ctx, sourceURL, gzPath); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	// Decompress gzip
	csvPath := filepath.Join(dlDir, "ban.csv")
	if err := gunzipFile(gzPath, csvPath); err != nil {
		return fmt.Errorf("gunzip: %w", err)
	}

	entries, err := parseBAN(csvPath)
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
		EntityType:   "address",
		Source:       "BAN (Base Adresse Nationale)",
		SourceURL:    sourceURL,
		License:      "Licence Ouverte v2",
		DataFile:     "data.gob",
		Format:       dict.FormatSpec{Normalize: "lowercase_ascii"},
		EntitySpec: &dict.EntitySpec{
			Sensitivity: "high",
		},
	})
}

func parseBAN(path string) (map[string]*dict.Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.Comma = ';'
	r.LazyQuotes = true
	r.FieldsPerRecord = -1
	r.ReuseRecord = true

	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	if len(header) <= 2 {
		f.Seek(0, io.SeekStart)
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
		colIdx[strings.TrimSpace(strings.ToLower(h))] = i
	}

	// BAN columns: id, id_fantoir, numero, rep, nom_voie, code_postal, code_insee, nom_commune, ...
	voieCol := colByNames(colIdx, "nom_voie", "voie")
	cpCol := colByNames(colIdx, "code_postal")
	communeCol := colByNames(colIdx, "nom_commune", "commune")
	inseeCol := colByNames(colIdx, "code_insee", "code_commune")

	// We index street names (voies) — the most useful for PII detection in addresses
	// We deduplicate by (voie, cp) to avoid 25M entries
	type voieKey struct{ voie, cp string }
	seen := make(map[voieKey]bool, 500000)

	entries := make(map[string]*dict.Entry, 1000000)
	var count, total int
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		total++

		voie := strings.TrimSpace(safeCol(record, voieCol))
		cp := strings.TrimSpace(safeCol(record, cpCol))
		commune := strings.TrimSpace(safeCol(record, communeCol))

		if voie == "" {
			continue
		}

		// Deduplicate: only keep unique (voie, cp) pairs
		vk := voieKey{voie: strings.ToLower(voie), cp: cp}
		if seen[vk] {
			continue
		}
		seen[vk] = true

		meta := map[string]string{
			"voie":         voie,
			"code_postal":  cp,
			"commune":      commune,
			"code_commune": safeCol(record, inseeCol),
		}

		// Index by normalized street name
		key := dict.NormalizeLowercaseASCII(voie)
		if _, exists := entries[key]; !exists {
			entries[key] = &dict.Entry{Metadata: meta}
		}

		// Index by "voie, cp commune" (full address line)
		if cp != "" && commune != "" {
			fullAddr := fmt.Sprintf("%s %s %s", voie, cp, commune)
			addrKey := dict.NormalizeLowercaseASCII(fullAddr)
			entries[addrKey] = &dict.Entry{Metadata: meta}
		}

		count++
		if count%200000 == 0 {
			fmt.Printf("  %d voies BAN traitees (sur %d lignes)...\n", count, total)
		}
	}

	fmt.Printf("  %d voies BAN uniques (de %d lignes)\n", count, total)
	return entries, nil
}
