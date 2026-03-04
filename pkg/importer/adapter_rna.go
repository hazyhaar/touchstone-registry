// CLAUDE:SUMMARY Import adapter for French RNA (Repertoire National des Associations) from data.gouv.fr.
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
	Register(&rnaAdapter{})
}

type rnaAdapter struct{}

func (a *rnaAdapter) ID() string      { return "rna-associations-fr" }
func (a *rnaAdapter) DictID() string  { return "associations-fr" }
func (a *rnaAdapter) Description() string {
	return "RNA Repertoire National des Associations (data.gouv.fr)"
}
func (a *rnaAdapter) DefaultURL() string {
	return "https://media.interieur.gouv.fr/rna/rna_waldec_20250901.zip"
}
func (a *rnaAdapter) License() string { return "Licence Ouverte v2" }

func (a *rnaAdapter) Import(ctx context.Context, sourceURL, outputDir string) error {
	dlDir := filepath.Join(outputDir, "_download")
	if err := ensureDir(dlDir); err != nil {
		return err
	}
	defer os.RemoveAll(dlDir)

	zipPath := filepath.Join(dlDir, "rna.zip")
	fmt.Printf("  telechargement %s...\n", sourceURL)
	fmt.Println("  (fichier volumineux ~300 Mo, patience...)")
	if err := downloadFile(ctx, sourceURL, zipPath); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	files, err := unzipFile(zipPath, dlDir)
	if err != nil {
		return fmt.Errorf("unzip: %w", err)
	}

	// Find the main CSV (rna_import_*.csv).
	var csvPath string
	for _, f := range files {
		base := strings.ToLower(filepath.Base(f))
		if strings.HasSuffix(base, ".csv") && strings.Contains(base, "rna") {
			csvPath = f
			break
		}
	}
	if csvPath == "" && len(files) > 0 {
		// Fallback: largest CSV.
		for _, f := range files {
			if strings.HasSuffix(strings.ToLower(f), ".csv") {
				csvPath = f
				break
			}
		}
	}
	if csvPath == "" {
		return fmt.Errorf("no CSV found in RNA ZIP")
	}

	entries, err := parseRNA(csvPath)
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
		EntityType:   "association",
		Source:       "RNA (Ministere de l'Interieur)",
		SourceURL:    sourceURL,
		License:      "Licence Ouverte v2",
		DataFile:     "data.gob",
		Format:       dict.FormatSpec{Normalize: "lowercase_ascii"},
	})
}

// parseRNA reads the RNA CSV (semicolon-delimited).
// Key columns: id (RNA W number), titre, objet, adrs_codepostal, adrs_libcommune.
func parseRNA(path string) (map[string]*dict.Entry, error) {
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

	colIdx := make(map[string]int)
	for i, h := range header {
		colIdx[strings.TrimSpace(strings.ToLower(h))] = i
	}

	idCol := colByNames(colIdx, "id")
	titreCol := colByNames(colIdx, "titre")
	cpCol := colByNames(colIdx, "adrs_codepostal")
	communeCol := colByNames(colIdx, "adrs_libcommune")

	if titreCol < 0 {
		return nil, fmt.Errorf("column 'titre' not found in header %v", header)
	}

	entries := make(map[string]*dict.Entry, 500000)
	var count int
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		titre := safeCol(record, titreCol)
		if titre == "" {
			continue
		}

		meta := make(map[string]string, 3)
		if rnaID := safeCol(record, idCol); rnaID != "" {
			meta["rna_id"] = rnaID
		}
		if cp := safeCol(record, cpCol); cp != "" {
			meta["code_postal"] = cp
		}
		if commune := safeCol(record, communeCol); commune != "" {
			meta["commune"] = commune
		}

		key := dict.NormalizeLowercaseASCII(titre)
		entries[key] = &dict.Entry{Metadata: meta}
		count++

		if count%500000 == 0 {
			fmt.Printf("  %d associations traitees...\n", count)
		}
	}

	fmt.Printf("  %d associations RNA\n", len(entries))
	return entries, nil
}
