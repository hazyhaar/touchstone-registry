// CLAUDE:SUMMARY Import adapter for Geonames alternate names filtered to first names (~300K international).
package importer

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hazyhaar/touchstone-registry/pkg/dict"
)

func init() {
	Register(&geonamesFirstnamesAdapter{})
}

type geonamesFirstnamesAdapter struct{}

func (a *geonamesFirstnamesAdapter) ID() string      { return "geonames-firstnames" }
func (a *geonamesFirstnamesAdapter) DictID() string  { return "firstnames-intl" }
func (a *geonamesFirstnamesAdapter) Description() string {
	return "Geonames / wikidata — prenoms internationaux"
}
func (a *geonamesFirstnamesAdapter) DefaultURL() string {
	return "https://raw.githubusercontent.com/sigpwned/popular-names-by-country-dataset/main/common-forenames-by-country.csv"
}
func (a *geonamesFirstnamesAdapter) License() string { return "CC0" }

func (a *geonamesFirstnamesAdapter) Import(ctx context.Context, sourceURL, outputDir string) error {
	dlDir := filepath.Join(outputDir, "_download")
	if err := ensureDir(dlDir); err != nil {
		return err
	}
	defer os.RemoveAll(dlDir)

	path := filepath.Join(dlDir, "names.csv")
	fmt.Printf("  telechargement %s...\n", sourceURL)
	if err := downloadFile(ctx, sourceURL, path); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	entries, err := parseGeonamesFirstnames(path)
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
		ID:         a.DictID(),
		Version:    "2026-03",
		EntityType: "firstname",
		Source:     "Global first names dataset",
		SourceURL:  sourceURL,
		License:    "CC0",
		DataFile:   "data.gob",
		Format:     dict.FormatSpec{Normalize: "lowercase_ascii"},
	})
}

func parseGeonamesFirstnames(path string) (map[string]*dict.Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	entries := make(map[string]*dict.Entry, 300000)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	first := true
	for scanner.Scan() {
		line := scanner.Text()
		if first {
			first = false
			// Skip header if present
			lower := strings.ToLower(line)
			if strings.Contains(lower, "name") || strings.Contains(lower, "prenom") {
				continue
			}
		}

		// Handle CSV or simple one-name-per-line
		fields := strings.Split(line, ",")
		name := strings.TrimSpace(fields[0])
		name = strings.Trim(name, "\"")
		if name == "" || len(name) < 2 {
			continue
		}

		var gender, country string
		if len(fields) > 1 {
			gender = strings.TrimSpace(fields[1])
		}
		if len(fields) > 2 {
			country = strings.TrimSpace(fields[2])
		}

		meta := map[string]string{
			"name": name,
		}
		if gender != "" {
			meta["gender"] = gender
		}
		if country != "" {
			meta["country"] = country
		}

		key := dict.NormalizeLowercaseASCII(name)
		if _, exists := entries[key]; !exists {
			entries[key] = &dict.Entry{Metadata: meta}
		}
	}

	fmt.Printf("  %d prenoms internationaux\n", len(entries))
	return entries, nil
}
