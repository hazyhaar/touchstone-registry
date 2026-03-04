// CLAUDE:SUMMARY Import adapter for ANSM public drug database (medicaments France, ~15K).
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
	Register(&medicamentsAdapter{})
}

type medicamentsAdapter struct{}

func (a *medicamentsAdapter) ID() string      { return "ansm-medicaments" }
func (a *medicamentsAdapter) DictID() string  { return "medicaments-fr" }
func (a *medicamentsAdapter) Description() string {
	return "Base publique des medicaments ANSM (France)"
}
func (a *medicamentsAdapter) DefaultURL() string {
	return "https://www.data.gouv.fr/api/1/datasets/r/056b6732-cbaf-447f-9f20-e3b5f655919a"
}
func (a *medicamentsAdapter) License() string { return "Licence Ouverte v2" }

func (a *medicamentsAdapter) Import(ctx context.Context, sourceURL, outputDir string) error {
	dlDir := filepath.Join(outputDir, "_download")
	if err := ensureDir(dlDir); err != nil {
		return err
	}
	defer os.RemoveAll(dlDir)

	path := filepath.Join(dlDir, "medicaments.txt")
	fmt.Printf("  telechargement %s...\n", sourceURL)
	if err := downloadFile(ctx, sourceURL, path); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	entries, err := parseMedicaments(path)
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
		EntityType:   "medication",
		Source:       "ANSM base publique medicaments",
		SourceURL:    sourceURL,
		License:      "Licence Ouverte v2",
		DataFile:     "data.gob",
		Format:       dict.FormatSpec{Normalize: "lowercase_ascii"},
	})
}

func parseMedicaments(path string) (map[string]*dict.Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// ANSM format: tab-separated, no header line
	// Columns: CIS, denomination, forme, voie, statut_AMM, type_procedure, etat, date_AMM, statut_bdm, numero_autorisation, titulaire, surveillance_renforcee
	r := csv.NewReader(f)
	r.Comma = '\t'
	r.LazyQuotes = true
	r.FieldsPerRecord = -1
	r.ReuseRecord = true

	entries := make(map[string]*dict.Entry, 20000)
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		if len(record) < 2 {
			continue
		}

		cis := strings.TrimSpace(record[0])
		denomination := strings.TrimSpace(record[1])
		if denomination == "" {
			continue
		}

		var forme, titulaire string
		if len(record) > 2 {
			forme = strings.TrimSpace(record[2])
		}
		if len(record) > 10 {
			titulaire = strings.TrimSpace(record[10])
		}

		meta := map[string]string{
			"cis":          cis,
			"denomination": denomination,
			"forme":        forme,
			"titulaire":    titulaire,
		}

		// Index by CIS code
		if cis != "" {
			entries[strings.ToLower(cis)] = &dict.Entry{Metadata: meta}
		}

		// Index by denomination (extract first word = active ingredient usually)
		key := dict.NormalizeLowercaseASCII(denomination)
		entries[key] = &dict.Entry{Metadata: meta}

		// Also index by short name (before first comma)
		if idx := strings.Index(denomination, ","); idx > 0 {
			shortName := strings.TrimSpace(denomination[:idx])
			entries[dict.NormalizeLowercaseASCII(shortName)] = &dict.Entry{Metadata: meta}
		}
	}

	fmt.Printf("  %d medicaments\n", len(entries))
	return entries, nil
}
