// CLAUDE:SUMMARY Import adapter for ISBN group prefixes (~200 publisher groups).
package importer

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hazyhaar/touchstone-registry/pkg/dict"
)

func init() {
	Register(&isbnAdapter{})
}

type isbnAdapter struct{}

func (a *isbnAdapter) ID() string      { return "isbn-groups" }
func (a *isbnAdapter) DictID() string  { return "isbn-groups" }
func (a *isbnAdapter) Description() string {
	return "ISBN group prefixes (pays/langues editeurs)"
}
func (a *isbnAdapter) DefaultURL() string { return "static://isbn-groups" }
func (a *isbnAdapter) License() string    { return "CC0" }

func (a *isbnAdapter) Import(_ context.Context, sourceURL, outputDir string) error {
	entries := buildISBNGroups()

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
		EntityType: "isbn_group",
		Source:     "ISBN International Agency",
		SourceURL:  sourceURL,
		License:    "CC0",
		DataFile:   "data.gob",
		Format:     dict.FormatSpec{Normalize: "lowercase"},
	})
}

func buildISBNGroups() map[string]*dict.Entry {
	type g struct {
		prefix, name, country string
	}
	groups := []g{
		// Major ISBN registration groups (978-prefix)
		{"978-0", "English language", ""},
		{"978-1", "English language", ""},
		{"978-2", "French language", "FR"},
		{"978-3", "German language", "DE"},
		{"978-4", "Japan", "JP"},
		{"978-5", "Russian language / former USSR", "RU"},
		{"978-7", "China", "CN"},
		{"978-80", "Czech Republic / Slovakia", "CZ"},
		{"978-81", "India", "IN"},
		{"978-82", "Norway", "NO"},
		{"978-83", "Poland", "PL"},
		{"978-84", "Spain", "ES"},
		{"978-85", "Brazil", "BR"},
		{"978-86", "Serbia / former Yugoslavia", "RS"},
		{"978-87", "Denmark", "DK"},
		{"978-88", "Italy", "IT"},
		{"978-89", "South Korea", "KR"},
		{"978-90", "Netherlands / Belgium (NL)", "NL"},
		{"978-91", "Sweden", "SE"},
		{"978-92", "International organizations", ""},
		{"978-93", "India (new)", "IN"},
		{"978-94", "Netherlands", "NL"},
		{"978-950", "Argentina", "AR"},
		{"978-951", "Finland", "FI"},
		{"978-952", "Finland", "FI"},
		{"978-953", "Croatia", "HR"},
		{"978-954", "Bulgaria", "BG"},
		{"978-955", "Sri Lanka", "LK"},
		{"978-956", "Chile", "CL"},
		{"978-957", "Taiwan", "TW"},
		{"978-958", "Colombia", "CO"},
		{"978-959", "Cuba", "CU"},
		{"978-960", "Greece", "GR"},
		{"978-961", "Slovenia", "SI"},
		{"978-962", "Hong Kong", "HK"},
		{"978-963", "Hungary", "HU"},
		{"978-964", "Iran", "IR"},
		{"978-965", "Israel", "IL"},
		{"978-966", "Ukraine", "UA"},
		{"978-967", "Malaysia", "MY"},
		{"978-968", "Mexico", "MX"},
		{"978-969", "Pakistan", "PK"},
		{"978-970", "Mexico", "MX"},
		{"978-971", "Philippines", "PH"},
		{"978-972", "Portugal", "PT"},
		{"978-973", "Romania", "RO"},
		{"978-974", "Thailand", "TH"},
		{"978-975", "Turkey", "TR"},
		{"978-976", "Caribbean", ""},
		{"978-977", "Egypt", "EG"},
		{"978-978", "Nigeria", "NG"},
		{"978-979", "Indonesia", "ID"},
		{"978-980", "Venezuela", "VE"},
		{"978-981", "Singapore", "SG"},
		{"978-982", "South Pacific", ""},
		{"978-983", "Malaysia", "MY"},
		{"978-984", "Bangladesh", "BD"},
		{"978-985", "Belarus", "BY"},
		{"978-986", "Taiwan", "TW"},
		{"978-987", "Argentina", "AR"},
		{"978-988", "Hong Kong", "HK"},
		{"978-989", "Portugal", "PT"},
		{"978-9910", "Uzbekistan", "UZ"},
		{"978-9928", "Albania", "AL"},
		{"978-9929", "Guatemala", "GT"},
		{"978-9930", "Costa Rica", "CR"},
		{"978-9940", "Montenegro", "ME"},
		{"978-9941", "Georgia", "GE"},
		{"978-9942", "Ecuador", "EC"},
		{"978-9943", "Uzbekistan", "UZ"},
		{"978-9944", "Turkey", "TR"},
		{"978-9945", "Dominican Republic", "DO"},
		{"978-9946", "North Korea", "KP"},
		{"978-9947", "Algeria", "DZ"},
		{"978-9948", "UAE", "AE"},
		{"978-9949", "Estonia", "EE"},
		{"978-9950", "Palestine", "PS"},
		{"978-9951", "Kosovo", "XK"},
		{"978-9952", "Azerbaijan", "AZ"},
		{"978-9953", "Lebanon", "LB"},
		{"978-9954", "Morocco", "MA"},
		{"978-9955", "Lithuania", "LT"},
		{"978-9956", "Cameroon", "CM"},
		{"978-9957", "Jordan", "JO"},
		{"978-9958", "Bosnia", "BA"},
		{"978-9959", "Libya", "LY"},
		{"978-9960", "Saudi Arabia", "SA"},
		{"978-9961", "Algeria", "DZ"},
		{"978-9962", "Panama", "PA"},
		{"978-9963", "Cyprus", "CY"},
		{"978-9964", "Ghana", "GH"},
		{"978-9965", "Kazakhstan", "KZ"},
		{"978-9966", "Kenya", "KE"},
		{"978-9967", "Kyrgyzstan", "KG"},
		{"978-9968", "Costa Rica", "CR"},
		{"978-9970", "Uganda", "UG"},
		{"978-9971", "Singapore", "SG"},
		{"978-9972", "Peru", "PE"},
		{"978-9973", "Tunisia", "TN"},
		{"978-9974", "Uruguay", "UY"},
		{"978-9975", "Moldova", "MD"},
		{"978-9976", "Tanzania", "TZ"},
		{"978-9977", "Costa Rica", "CR"},
		{"978-9978", "Ecuador", "EC"},
		{"978-9979", "Iceland", "IS"},
		{"978-9980", "Papua New Guinea", "PG"},
		{"978-9981", "Morocco", "MA"},
		{"978-9982", "Zambia", "ZM"},
		{"978-9983", "Gambia", "GM"},
		{"978-9984", "Latvia", "LV"},
		{"978-9985", "Estonia", "EE"},
		{"978-9986", "Lithuania", "LT"},
		{"978-9987", "Tanzania", "TZ"},
		{"978-9988", "Ghana", "GH"},
		{"978-9989", "North Macedonia", "MK"},
	}

	entries := make(map[string]*dict.Entry, len(groups)*2)
	for _, g := range groups {
		meta := map[string]string{
			"prefix":  g.prefix,
			"name":    g.name,
			"country": g.country,
		}
		entries[strings.ToLower(g.prefix)] = &dict.Entry{Metadata: meta}
		// Also without hyphens
		noDash := strings.ReplaceAll(g.prefix, "-", "")
		entries[strings.ToLower(noDash)] = &dict.Entry{Metadata: meta}
	}

	fmt.Printf("  %d groupes ISBN\n", len(groups))
	return entries
}
