// CLAUDE:SUMMARY Import adapter for IANA TLD (Top Level Domains) list.
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
	Register(&ianaTLDAdapter{})
}

type ianaTLDAdapter struct{}

func (a *ianaTLDAdapter) ID() string          { return "iana-tld" }
func (a *ianaTLDAdapter) DictID() string      { return "tld" }
func (a *ianaTLDAdapter) Description() string { return "IANA Top Level Domain list" }
func (a *ianaTLDAdapter) DefaultURL() string {
	return "https://data.iana.org/TLD/tlds-alpha-by-domain.txt"
}
func (a *ianaTLDAdapter) License() string { return "Public Domain" }

func (a *ianaTLDAdapter) Import(ctx context.Context, sourceURL, outputDir string) error {
	dlDir := filepath.Join(outputDir, "_download")
	if err := ensureDir(dlDir); err != nil {
		return err
	}
	defer os.RemoveAll(dlDir)

	path := filepath.Join(dlDir, "tlds.txt")
	fmt.Printf("  telechargement %s...\n", sourceURL)
	if err := downloadFile(ctx, sourceURL, path); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	entries, err := parseTLDs(path)
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
		ID:         a.DictID(),
		Version:    "2026-03",
		EntityType: "tld",
		Source:     "IANA",
		SourceURL:  sourceURL,
		License:    "Public Domain",
		DataFile:   "data.db",
		Format:     dict.FormatSpec{Normalize: "lowercase"},
	})
}

func parseTLDs(path string) (map[string]*dict.Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	entries := make(map[string]*dict.Entry)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		tld := strings.ToLower(line)
		entries[tld] = &dict.Entry{Metadata: map[string]string{"tld": tld}}
		// Also index with dot prefix
		entries["."+tld] = &dict.Entry{Metadata: map[string]string{"tld": tld}}
	}

	fmt.Printf("  %d TLDs\n", len(entries)/2)
	return entries, nil
}
