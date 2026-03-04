// CLAUDE:SUMMARY Import adapter for EU institutions and bodies (static table ~50).
package importer

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/hazyhaar/touchstone-registry/pkg/dict"
)

func init() {
	Register(&euInstitutionsAdapter{})
}

type euInstitutionsAdapter struct{}

func (a *euInstitutionsAdapter) ID() string      { return "eu-institutions" }
func (a *euInstitutionsAdapter) DictID() string  { return "eu-institutions" }
func (a *euInstitutionsAdapter) Description() string {
	return "Institutions et organes de l'Union europeenne (table statique)"
}
func (a *euInstitutionsAdapter) DefaultURL() string { return "static://eu-institutions" }
func (a *euInstitutionsAdapter) License() string    { return "CC0" }

func (a *euInstitutionsAdapter) Import(_ context.Context, sourceURL, outputDir string) error {
	entries := buildEUInstitutions()

	dictDir := filepath.Join(outputDir, a.DictID())
	if err := ensureDir(dictDir); err != nil {
		return err
	}

	if err := dict.SaveSQLite(entries, filepath.Join(dictDir, "data.db")); err != nil {
		return fmt.Errorf("save sqlite: %w", err)
	}

	return writeManifest(dictDir, &dict.Manifest{
		ID:           a.DictID(),
		Version:      "2026-03",
		Jurisdiction: "eu",
		EntityType:   "institution",
		Source:       "EU institutions directory",
		SourceURL:    sourceURL,
		License:      "CC0",
		DataFile:     "data.db",
		Format:       dict.FormatSpec{Normalize: "lowercase_ascii"},
	})
}

func buildEUInstitutions() map[string]*dict.Entry {
	type inst struct {
		name, nameFR, abbr, category string
	}
	institutions := []inst{
		{"European Commission", "Commission europeenne", "EC", "institution"},
		{"European Parliament", "Parlement europeen", "EP", "institution"},
		{"Council of the European Union", "Conseil de l'Union europeenne", "Council", "institution"},
		{"European Council", "Conseil europeen", "", "institution"},
		{"Court of Justice of the European Union", "Cour de justice de l'Union europeenne", "CJUE", "institution"},
		{"European Central Bank", "Banque centrale europeenne", "ECB/BCE", "institution"},
		{"European Court of Auditors", "Cour des comptes europeenne", "ECA", "institution"},
		{"European External Action Service", "Service europeen d'action exterieure", "EEAS/SEAE", "body"},
		{"European Economic and Social Committee", "Comite economique et social europeen", "EESC/CESE", "advisory"},
		{"European Committee of the Regions", "Comite europeen des regions", "CoR/CdR", "advisory"},
		{"European Investment Bank", "Banque europeenne d'investissement", "EIB/BEI", "financial"},
		{"European Ombudsman", "Mediateur europeen", "", "institution"},
		{"European Data Protection Supervisor", "Controleur europeen de la protection des donnees", "EDPS/CEPD", "body"},
		{"European Data Protection Board", "Comite europeen de la protection des donnees", "EDPB/CEPD", "body"},
		{"European Banking Authority", "Autorite bancaire europeenne", "EBA/ABE", "agency"},
		{"European Securities and Markets Authority", "Autorite europeenne des marches financiers", "ESMA/AEMF", "agency"},
		{"European Insurance and Occupational Pensions Authority", "Autorite des assurances et pensions professionnelles", "EIOPA/AEAPP", "agency"},
		{"European Medicines Agency", "Agence europeenne des medicaments", "EMA/AEM", "agency"},
		{"European Food Safety Authority", "Autorite europeenne de securite des aliments", "EFSA/AESA", "agency"},
		{"European Environment Agency", "Agence europeenne pour l'environnement", "EEA/AEE", "agency"},
		{"Europol", "Europol", "Europol", "agency"},
		{"Eurojust", "Eurojust", "Eurojust", "agency"},
		{"Frontex", "Frontex", "Frontex", "agency"},
		{"European Union Agency for Cybersecurity", "Agence de l'Union europeenne pour la cybersecurite", "ENISA", "agency"},
		{"European Union Intellectual Property Office", "Office de l'Union europeenne pour la propriete intellectuelle", "EUIPO/OHMI", "agency"},
		{"European Chemicals Agency", "Agence europeenne des produits chimiques", "ECHA", "agency"},
		{"European Aviation Safety Agency", "Agence europeenne de la securite aerienne", "EASA/AESA", "agency"},
		{"European Space Agency", "Agence spatiale europeenne", "ESA/ASE", "agency"},
		{"Eurostat", "Eurostat", "Eurostat", "body"},
		{"European Anti-Fraud Office", "Office europeen de lutte antifraude", "OLAF", "body"},
	}

	entries := make(map[string]*dict.Entry, len(institutions)*4)
	for _, i := range institutions {
		meta := map[string]string{
			"name":     i.name,
			"name_fr":  i.nameFR,
			"abbr":     i.abbr,
			"category": i.category,
		}
		entries[dict.NormalizeLowercaseASCII(i.name)] = &dict.Entry{Metadata: meta}
		entries[dict.NormalizeLowercaseASCII(i.nameFR)] = &dict.Entry{Metadata: meta}
		if i.abbr != "" {
			// Handle abbreviations with slash (e.g., "ECB/BCE")
			for _, a := range splitAbbreviations(i.abbr) {
				entries[dict.NormalizeLowercaseASCII(a)] = &dict.Entry{Metadata: meta}
			}
		}
	}

	fmt.Printf("  %d institutions EU\n", len(entries))
	return entries
}

func splitAbbreviations(abbr string) []string {
	parts := make([]string, 0, 2)
	for _, p := range []string{"/"} {
		for _, a := range split(abbr, p) {
			a = trimSpace(a)
			if a != "" {
				parts = append(parts, a)
			}
		}
		if len(parts) > 1 {
			return parts
		}
		parts = parts[:0]
	}
	return []string{abbr}
}

func split(s, sep string) []string {
	result := make([]string, 0, 2)
	for {
		i := indexOf(s, sep)
		if i < 0 {
			result = append(result, s)
			break
		}
		result = append(result, s[:i])
		s = s[i+len(sep):]
	}
	return result
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
