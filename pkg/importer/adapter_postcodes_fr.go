// CLAUDE:SUMMARY Import adapter for La Poste French postal codes (codes postaux).
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
	Register(&postcodesFRAdapter{})
}

type postcodesFRAdapter struct{}

func (a *postcodesFRAdapter) ID() string      { return "laposte-postcodes-fr" }
func (a *postcodesFRAdapter) DictID() string  { return "postcodes-fr" }
func (a *postcodesFRAdapter) Description() string {
	return "La Poste — codes postaux France avec communes"
}
func (a *postcodesFRAdapter) DefaultURL() string {
	return "https://datanova.laposte.fr/data-fair/api/v1/datasets/laposte-hexasmal/metadata-attachments/base-officielle-codes-postaux.csv"
}
func (a *postcodesFRAdapter) License() string { return "Licence Ouverte v2" }

func (a *postcodesFRAdapter) Import(ctx context.Context, sourceURL, outputDir string) error {
	dlDir := filepath.Join(outputDir, "_download")
	if err := ensureDir(dlDir); err != nil {
		return err
	}
	defer os.RemoveAll(dlDir)

	csvPath := filepath.Join(dlDir, "postcodes.csv")
	fmt.Printf("  telechargement %s...\n", sourceURL)
	if err := downloadFile(ctx, sourceURL, csvPath); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	entries, err := parsePostcodesFR(csvPath)
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
		EntityType:   "postcode",
		Source:       "La Poste (Datanova)",
		SourceURL:    sourceURL,
		License:      "Licence Ouverte v2",
		DataFile:     "data.gob",
		Format:       dict.FormatSpec{Normalize: "lowercase_ascii"},
	})
}

func parsePostcodesFR(path string) (map[string]*dict.Entry, error) {
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

	// Try semicolon if comma gives single column
	if len(header) <= 2 {
		if _, seekErr := f.Seek(0, io.SeekStart); seekErr != nil {
			return nil, fmt.Errorf("seek: %w", seekErr)
		}
		r = csv.NewReader(f)
		r.Comma = ';'
		r.LazyQuotes = true
		r.FieldsPerRecord = -1
		header, err = r.Read()
		if err != nil {
			return nil, fmt.Errorf("read header (semicolon): %w", err)
		}
	}

	colIdx := make(map[string]int)
	for i, h := range header {
		colIdx[strings.TrimSpace(strings.ToLower(h))] = i
	}

	codeCol := colByNames(colIdx, "code_postal")
	communeCol := colByNames(colIdx, "nom_de_la_commune", "nom_commune", "libelle_d_acheminement")
	codeINSEECol := colByNames(colIdx, "code_commune_insee")

	if codeCol < 0 {
		return nil, fmt.Errorf("column code_postal not found in header %v", header)
	}

	entries := make(map[string]*dict.Entry, 70000)
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		code := safeCol(record, codeCol)
		commune := safeCol(record, communeCol)
		if code == "" {
			continue
		}

		meta := map[string]string{
			"postcode":     code,
			"commune":      commune,
			"code_commune": safeCol(record, codeINSEECol),
		}

		entries[strings.ToLower(code)] = &dict.Entry{Metadata: meta}
		if commune != "" {
			key := dict.NormalizeLowercaseASCII(commune)
			entries[key] = &dict.Entry{Metadata: meta}
		}
	}

	fmt.Printf("  %d codes postaux FR\n", len(entries))
	return entries, nil
}
