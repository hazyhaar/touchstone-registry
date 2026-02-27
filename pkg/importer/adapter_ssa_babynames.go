// CLAUDE:SUMMARY Import adapter for SSA baby names (US first names) aggregated across all yearly files.
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
	Register(&ssaBabyNamesAdapter{})
}

type ssaBabyNamesAdapter struct{}

func (a *ssaBabyNamesAdapter) ID() string          { return "ssa-babynames-us" }
func (a *ssaBabyNamesAdapter) DictID() string      { return "firstnames-us" }
func (a *ssaBabyNamesAdapter) Description() string { return "SSA baby names US (Social Security Administration)" }
func (a *ssaBabyNamesAdapter) DefaultURL() string   { return "https://www.ssa.gov/oact/babynames/names.zip" }
func (a *ssaBabyNamesAdapter) License() string      { return "Public Domain" }

func (a *ssaBabyNamesAdapter) Import(ctx context.Context, sourceURL, outputDir string) error {
	dlDir := filepath.Join(outputDir, "_download")
	if err := ensureDir(dlDir); err != nil {
		return err
	}
	defer os.RemoveAll(dlDir)

	zipPath := filepath.Join(dlDir, "names.zip")
	fmt.Printf("  telechargement %s...\n", sourceURL)
	if err := downloadFile(ctx, sourceURL, zipPath); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	files, err := unzipFile(zipPath, dlDir)
	if err != nil {
		return fmt.Errorf("unzip: %w", err)
	}

	// Parse all yobYYYY.txt files and aggregate.
	type agg struct {
		sex   string
		total int
	}
	aggregated := make(map[string]*agg)

	for _, f := range files {
		base := filepath.Base(f)
		if !strings.HasPrefix(base, "yob") || !strings.HasSuffix(base, ".txt") {
			continue
		}

		file, err := os.Open(f)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			// Format: Name,Sex,Count
			parts := strings.SplitN(line, ",", 3)
			if len(parts) != 3 {
				continue
			}
			name := strings.TrimSpace(parts[0])
			sex := strings.TrimSpace(parts[1])
			var count int
			fmt.Sscanf(strings.TrimSpace(parts[2]), "%d", &count)

			key := dict.NormalizeLowercaseASCII(name)
			if existing, ok := aggregated[key]; ok {
				existing.total += count
			} else {
				aggregated[key] = &agg{sex: sex, total: count}
			}
		}
		file.Close()
	}

	entries := make(map[string]*dict.Entry, len(aggregated))
	for key, a := range aggregated {
		meta := map[string]string{
			"frequency": fmt.Sprintf("%d", a.total),
		}
		if a.sex != "" {
			meta["sex"] = a.sex
		}
		entries[key] = &dict.Entry{Metadata: meta}
	}

	fmt.Printf("  %d prenoms uniques US\n", len(entries))

	dictDir := filepath.Join(outputDir, a.DictID())
	if err := ensureDir(dictDir); err != nil {
		return err
	}

	if err := dict.SaveGob(entries, filepath.Join(dictDir, "data.gob")); err != nil {
		return fmt.Errorf("save gob: %w", err)
	}

	return writeManifest(dictDir, &dict.Manifest{
		ID:           a.DictID(),
		Version:      "2026-02",
		Jurisdiction: "us",
		EntityType:   "first_name",
		Source:       "SSA Baby Names",
		SourceURL:    sourceURL,
		License:      "Public Domain",
		DataFile:     "data.gob",
		Format:       dict.FormatSpec{Normalize: "lowercase_ascii"},
	})
}
