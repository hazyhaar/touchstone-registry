// CLAUDE:SUMMARY Import adapter for European Parliament MEPs open data (~720 entries).
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
	Register(&mepsEUAdapter{})
}

type mepsEUAdapter struct{}

func (a *mepsEUAdapter) ID() string      { return "europarl-meps" }
func (a *mepsEUAdapter) DictID() string  { return "meps-eu" }
func (a *mepsEUAdapter) Description() string {
	return "Parlement Europeen — deputes europeens (MEPs)"
}
func (a *mepsEUAdapter) DefaultURL() string {
	return "https://data.europarl.europa.eu/api/v2/meps?format=csv&offset=0&limit=800"
}
func (a *mepsEUAdapter) License() string { return "CC BY 4.0" }

func (a *mepsEUAdapter) Import(ctx context.Context, sourceURL, outputDir string) error {
	dlDir := filepath.Join(outputDir, "_download")
	if err := ensureDir(dlDir); err != nil {
		return err
	}
	defer os.RemoveAll(dlDir)

	csvPath := filepath.Join(dlDir, "meps.csv")
	fmt.Printf("  telechargement %s...\n", sourceURL)
	if err := downloadFile(ctx, sourceURL, csvPath); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	entries, err := parseMEPs(csvPath)
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
		Jurisdiction: "eu",
		EntityType:   "politician",
		Source:       "European Parliament",
		SourceURL:    sourceURL,
		License:      "CC BY 4.0",
		DataFile:     "data.gob",
		Format:       dict.FormatSpec{Normalize: "lowercase_ascii"},
	})
}

func parseMEPs(path string) (map[string]*dict.Entry, error) {
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
		clean := strings.TrimSpace(strings.ToLower(h))
		clean = strings.TrimPrefix(clean, "\xef\xbb\xbf")
		colIdx[clean] = i
	}

	nameCol := colByNames(colIdx, "fullname", "name", "label", "mep_name")
	familyCol := colByNames(colIdx, "familyname", "family_name", "last_name", "lastname")
	givenCol := colByNames(colIdx, "givenname", "given_name", "first_name", "firstname")
	countryCol := colByNames(colIdx, "country", "country_of_representation", "countryofrepresentation")
	groupCol := colByNames(colIdx, "political_group", "politicalgroup", "group")

	entries := make(map[string]*dict.Entry, 1500)
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		fullName := strings.TrimSpace(safeCol(record, nameCol))
		family := strings.TrimSpace(safeCol(record, familyCol))
		given := strings.TrimSpace(safeCol(record, givenCol))

		// Build full name if not available
		if fullName == "" && (family != "" || given != "") {
			fullName = strings.TrimSpace(given + " " + family)
		}
		if fullName == "" {
			continue
		}

		meta := map[string]string{
			"name":    fullName,
			"family":  family,
			"given":   given,
			"country": safeCol(record, countryCol),
			"group":   safeCol(record, groupCol),
		}

		entries[dict.NormalizeLowercaseASCII(fullName)] = &dict.Entry{Metadata: meta}
		// Also index by family name
		if family != "" {
			entries[dict.NormalizeLowercaseASCII(family)] = &dict.Entry{Metadata: meta}
		}
	}

	fmt.Printf("  %d deputes europeens\n", len(entries))
	return entries, nil
}
