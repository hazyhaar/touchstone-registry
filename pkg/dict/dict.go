package dict

import (
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/transform"
)

// Entry is a single term in a dictionary, with optional metadata.
type Entry struct {
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Dictionary is one loaded dictionary with its manifest and in-memory hashmap.
type Dictionary struct {
	Manifest  *Manifest        `json:"manifest"`
	Entries   map[string]*Entry `json:"-"`
	normalize Normalizer
	patterns  *patternMatcher
}

// LoadDictionary reads a manifest.yaml and loads data from gob, csv, or patterns.
func LoadDictionary(dir string) (*Dictionary, error) {
	manifestPath := filepath.Join(dir, "manifest.yaml")
	manifest, err := LoadManifest(manifestPath)
	if err != nil {
		return nil, err
	}

	d := &Dictionary{
		Manifest:  manifest,
		Entries:   make(map[string]*Entry),
		normalize: GetNormalizer(manifest.Format.Normalize),
	}

	// Pattern-based dictionaries: compile regexes, no data file.
	if manifest.Method == "pattern" {
		pm, err := compilePatterns(manifest.Patterns)
		if err != nil {
			return nil, fmt.Errorf("dict %s: %w", manifest.ID, err)
		}
		d.patterns = pm
		return d, nil
	}

	// Gob takes priority over CSV.
	gobPath := filepath.Join(dir, "data.gob")
	if _, err := os.Stat(gobPath); err == nil {
		if err := d.loadGob(gobPath); err != nil {
			return nil, fmt.Errorf("dict %s: %w", manifest.ID, err)
		}
		return d, nil
	}

	// Fallback: CSV (original behaviour).
	dataPath := filepath.Join(dir, manifest.DataFile)
	if err := d.loadCSV(dataPath); err != nil {
		return nil, fmt.Errorf("dict %s: %w", manifest.ID, err)
	}
	return d, nil
}

// Classify matches a term against patterns or falls back to lookup.
func (d *Dictionary) Classify(term string) (*Entry, bool) {
	if d.patterns != nil {
		name, ok := d.patterns.match(term)
		if !ok {
			return nil, false
		}
		return &Entry{Metadata: map[string]string{"pattern": name}}, true
	}
	return d.Lookup(term)
}

func (d *Dictionary) loadCSV(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open data file: %w", err)
	}
	defer f.Close()

	// Transcode non-UTF-8 encodings declared in the manifest.
	var reader io.Reader = f
	if enc := d.Manifest.Format.Encoding; enc != "" && !isUTF8(enc) {
		e, err := htmlindex.Get(enc)
		if err != nil {
			return fmt.Errorf("unsupported encoding %q: %w", enc, err)
		}
		reader = transform.NewReader(f, e.NewDecoder())
	}

	r := csv.NewReader(reader)

	if delim := d.Manifest.Format.Delimiter; delim != "" {
		r.Comma = []rune(delim)[0]
	}
	r.LazyQuotes = true
	r.TrimLeadingSpace = true

	// Read header if present.
	var header []string
	if d.Manifest.Format.HasHeader {
		header, err = r.Read()
		if err != nil {
			return fmt.Errorf("read header: %w", err)
		}
		for i := range header {
			header[i] = strings.TrimSpace(header[i])
		}
	}

	// Resolve key column index.
	keyIdx := 0
	if col := d.Manifest.Format.KeyColumn; col != "" && header != nil {
		found := false
		for i, h := range header {
			if h == col {
				keyIdx = i
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("key column %q not found in header %v", col, header)
		}
	}

	// Resolve metadata column indices.
	metaIdx := make(map[string]int)
	for _, mc := range d.Manifest.MetadataCols {
		if header != nil {
			for i, h := range header {
				if h == mc.Column {
					metaIdx[mc.Name] = i
					break
				}
			}
		}
	}

	// Read all rows into the hashmap.
	var collisions int
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read row: %w", err)
		}
		if keyIdx >= len(record) {
			continue
		}

		key := d.normalize(strings.TrimSpace(record[keyIdx]))
		if key == "" {
			continue
		}

		entry := &Entry{}
		if len(metaIdx) > 0 {
			entry.Metadata = make(map[string]string, len(metaIdx))
			for name, idx := range metaIdx {
				if idx < len(record) {
					entry.Metadata[name] = strings.TrimSpace(record[idx])
				}
			}
		}
		if _, exists := d.Entries[key]; exists {
			collisions++
		}
		d.Entries[key] = entry
	}

	if collisions > 0 {
		slog.Warn("key collisions after normalization", "dict", d.Manifest.ID, "collisions", collisions)
	}

	return nil
}

// Lookup searches for a term in this dictionary after normalization.
func (d *Dictionary) Lookup(term string) (*Entry, bool) {
	e, ok := d.Entries[d.normalize(term)]
	return e, ok
}

// NormalizeTerm applies this dictionary's normalizer to a term.
func (d *Dictionary) NormalizeTerm(term string) string {
	return d.normalize(term)
}

func isUTF8(enc string) bool {
	e := strings.ToLower(strings.ReplaceAll(enc, "-", ""))
	return e == "utf8" || e == ""
}
