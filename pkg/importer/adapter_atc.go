// CLAUDE:SUMMARY Import adapter for ATC (Anatomical Therapeutic Chemical) drug classification codes (~6K).
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
	Register(&atcAdapter{})
}

type atcAdapter struct{}

func (a *atcAdapter) ID() string      { return "who-atc" }
func (a *atcAdapter) DictID() string  { return "atc-eu" }
func (a *atcAdapter) Description() string {
	return "OMS ATC — classification anatomique therapeutique chimique des medicaments"
}
func (a *atcAdapter) DefaultURL() string {
	// WHOCC ATC index export
	return "https://raw.githubusercontent.com/fabkury/atcd/master/WHO%20ATC-DDD%202021-12-03.csv"
}
func (a *atcAdapter) License() string { return "CC0" }

func (a *atcAdapter) Import(ctx context.Context, sourceURL, outputDir string) error {
	dlDir := filepath.Join(outputDir, "_download")
	if err := ensureDir(dlDir); err != nil {
		return err
	}
	defer os.RemoveAll(dlDir)

	path := filepath.Join(dlDir, "atc.csv")
	fmt.Printf("  telechargement %s...\n", sourceURL)
	if err := downloadFile(ctx, sourceURL, path); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	entries, err := parseATC(path)
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
		EntityType: "drug_code",
		Source:     "WHO ATC/DDD",
		SourceURL:  sourceURL,
		License:    "CC0",
		DataFile:   "data.db",
		Format:     dict.FormatSpec{Normalize: "lowercase_ascii"},
	})
}

func parseATC(path string) (map[string]*dict.Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	entries := make(map[string]*dict.Entry, 8000)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	first := true
	for scanner.Scan() {
		line := scanner.Text()
		if first {
			first = false
			lower := strings.ToLower(line)
			if strings.Contains(lower, "atc") || strings.Contains(lower, "code") {
				continue
			}
		}

		// CSV format: atc_code,atc_name,ddd,uom,adm_r,note
		fields := strings.SplitN(line, ",", 6)
		if len(fields) < 2 {
			continue
		}

		code := strings.TrimSpace(strings.Trim(fields[0], "\""))
		name := strings.TrimSpace(strings.Trim(fields[1], "\""))
		if code == "" || name == "" {
			continue
		}

		var ddd, uom string
		if len(fields) > 2 {
			ddd = strings.TrimSpace(strings.Trim(fields[2], "\""))
		}
		if len(fields) > 3 {
			uom = strings.TrimSpace(strings.Trim(fields[3], "\""))
		}

		meta := map[string]string{
			"atc_code": code,
			"name":     name,
		}
		if ddd != "" {
			meta["ddd"] = ddd
		}
		if uom != "" {
			meta["uom"] = uom
		}

		entries[strings.ToLower(code)] = &dict.Entry{Metadata: meta}
		entries[dict.NormalizeLowercaseASCII(name)] = &dict.Entry{Metadata: meta}
	}

	fmt.Printf("  %d codes ATC\n", len(entries))
	return entries, nil
}
