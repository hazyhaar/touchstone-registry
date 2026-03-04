// CLAUDE:SUMMARY Import adapter for honorifics / titres de civilité (static multilingual list, ~200).
package importer

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hazyhaar/touchstone-registry/pkg/dict"
)

func init() {
	Register(&honorificsAdapter{})
}

type honorificsAdapter struct{}

func (a *honorificsAdapter) ID() string      { return "honorifics" }
func (a *honorificsAdapter) DictID() string  { return "honorifics" }
func (a *honorificsAdapter) Description() string {
	return "Titres de civilite / honorifiques multilangue (Mr, Mme, Dr, Prof, etc.)"
}
func (a *honorificsAdapter) DefaultURL() string { return "static://honorifics" }
func (a *honorificsAdapter) License() string    { return "CC0" }

func (a *honorificsAdapter) Import(_ context.Context, sourceURL, outputDir string) error {
	entries := buildHonorifics()

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
		EntityType: "honorific",
		Source:     "Static table (multilingual honorifics)",
		SourceURL:  sourceURL,
		License:    "CC0",
		DataFile:   "data.db",
		Format:     dict.FormatSpec{Normalize: "lowercase_ascii"},
	})
}

func buildHonorifics() map[string]*dict.Entry {
	// Comprehensive multilingual honorifics list
	type h struct {
		title, lang, gender, category string
	}
	titles := []h{
		// French
		{"Monsieur", "fr", "M", "civil"}, {"Madame", "fr", "F", "civil"},
		{"Mademoiselle", "fr", "F", "civil"}, {"M.", "fr", "M", "civil"},
		{"Mme", "fr", "F", "civil"}, {"Mlle", "fr", "F", "civil"},
		{"Maître", "fr", "", "legal"}, {"Me", "fr", "", "legal"},
		{"Docteur", "fr", "", "medical"}, {"Dr", "fr", "", "medical"},
		{"Professeur", "fr", "", "academic"}, {"Pr", "fr", "", "academic"},
		{"Prof.", "fr", "", "academic"},
		// English
		{"Mr", "en", "M", "civil"}, {"Mr.", "en", "M", "civil"},
		{"Mrs", "en", "F", "civil"}, {"Mrs.", "en", "F", "civil"},
		{"Ms", "en", "F", "civil"}, {"Ms.", "en", "F", "civil"},
		{"Miss", "en", "F", "civil"}, {"Dr", "en", "", "medical"},
		{"Dr.", "en", "", "medical"}, {"Prof", "en", "", "academic"},
		{"Prof.", "en", "", "academic"}, {"Sir", "en", "M", "nobility"},
		{"Dame", "en", "F", "nobility"}, {"Lord", "en", "M", "nobility"},
		{"Lady", "en", "F", "nobility"}, {"Rev", "en", "", "religious"},
		{"Rev.", "en", "", "religious"},
		// German
		{"Herr", "de", "M", "civil"}, {"Frau", "de", "F", "civil"},
		{"Doktor", "de", "", "medical"}, {"Professor", "de", "", "academic"},
		// Spanish
		{"Señor", "es", "M", "civil"}, {"Señora", "es", "F", "civil"},
		{"Señorita", "es", "F", "civil"}, {"Sr.", "es", "M", "civil"},
		{"Sra.", "es", "F", "civil"}, {"Srta.", "es", "F", "civil"},
		{"Don", "es", "M", "civil"}, {"Doña", "es", "F", "civil"},
		// Italian
		{"Signor", "it", "M", "civil"}, {"Signore", "it", "M", "civil"},
		{"Signora", "it", "F", "civil"}, {"Signorina", "it", "F", "civil"},
		{"Sig.", "it", "M", "civil"}, {"Sig.ra", "it", "F", "civil"},
		{"Dott.", "it", "", "medical"}, {"Dottore", "it", "M", "medical"},
		{"Dottoressa", "it", "F", "medical"}, {"Avv.", "it", "", "legal"},
		{"Avvocato", "it", "", "legal"}, {"Ing.", "it", "", "professional"},
		// Portuguese
		{"Senhor", "pt", "M", "civil"}, {"Senhora", "pt", "F", "civil"},
		{"Sr.", "pt", "M", "civil"}, {"Sra.", "pt", "F", "civil"},
		// Dutch
		{"Meneer", "nl", "M", "civil"}, {"Mevrouw", "nl", "F", "civil"},
		{"Mijnheer", "nl", "M", "civil"}, {"Dhr.", "nl", "M", "civil"},
		{"Mw.", "nl", "F", "civil"},
		// Polish
		{"Pan", "pl", "M", "civil"}, {"Pani", "pl", "F", "civil"},
		// Swedish/Danish/Norwegian
		{"Herre", "sv", "M", "civil"}, {"Fru", "sv", "F", "civil"},
		{"Herr", "sv", "M", "civil"}, {"Fröken", "sv", "F", "civil"},
		{"Hr.", "da", "M", "civil"}, {"Fru", "da", "F", "civil"},
		// Common abbreviations
		{"Mgr", "", "", "religious"}, {"Cdt", "", "", "military"},
		{"Col", "", "", "military"}, {"Col.", "", "", "military"},
		{"Cpt", "", "", "military"}, {"Gen", "", "", "military"},
		{"Gen.", "", "", "military"}, {"Lt", "", "", "military"},
		{"Sgt", "", "", "military"},
		// Academic / Professional
		{"PhD", "", "", "academic"}, {"Esq", "en", "", "legal"},
		{"Esq.", "en", "", "legal"}, {"Hon", "en", "", "political"},
		{"Hon.", "en", "", "political"},
	}

	entries := make(map[string]*dict.Entry, len(titles)*2)
	for _, t := range titles {
		meta := map[string]string{
			"title":    t.title,
			"lang":     t.lang,
			"gender":   t.gender,
			"category": t.category,
		}
		key := dict.NormalizeLowercaseASCII(t.title)
		entries[key] = &dict.Entry{Metadata: meta}
		// Also lowercase without dots
		noDots := strings.ReplaceAll(key, ".", "")
		if noDots != key {
			entries[noDots] = &dict.Entry{Metadata: meta}
		}
	}

	fmt.Printf("  %d honorifiques\n", len(entries))
	return entries
}
