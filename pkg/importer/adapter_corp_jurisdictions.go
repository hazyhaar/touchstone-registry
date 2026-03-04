// CLAUDE:SUMMARY Import adapter for corporate jurisdictions codes (OpenCorporates-style, ~150 entries).
package importer

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hazyhaar/touchstone-registry/pkg/dict"
)

func init() {
	Register(&corpJurisdictionsAdapter{})
}

type corpJurisdictionsAdapter struct{}

func (a *corpJurisdictionsAdapter) ID() string      { return "corp-jurisdictions" }
func (a *corpJurisdictionsAdapter) DictID() string  { return "corp-jurisdictions" }
func (a *corpJurisdictionsAdapter) Description() string {
	return "Corporate registry jurisdictions (ISO-based, OpenCorporates-style)"
}
func (a *corpJurisdictionsAdapter) DefaultURL() string { return "static://corp-jurisdictions" }
func (a *corpJurisdictionsAdapter) License() string    { return "CC0" }

func (a *corpJurisdictionsAdapter) Import(_ context.Context, sourceURL, outputDir string) error {
	entries := buildCorpJurisdictions()

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
		EntityType: "jurisdiction",
		Source:     "Corporate registries compilation",
		SourceURL:  sourceURL,
		License:    "CC0",
		DataFile:   "data.db",
		Format:     dict.FormatSpec{Normalize: "lowercase"},
	})
}

func buildCorpJurisdictions() map[string]*dict.Entry {
	type j struct {
		code, name, registry, openData string
	}
	jurisdictions := []j{
		{"fr", "France", "SIRENE / RNE (INPI)", "yes"},
		{"gb", "United Kingdom", "Companies House", "yes"},
		{"de", "Germany", "Handelsregister", "partial"},
		{"nl", "Netherlands", "KvK (Kamer van Koophandel)", "no"},
		{"be", "Belgium", "BCE/KBO (Banque-Carrefour des Entreprises)", "yes"},
		{"lu", "Luxembourg", "RCS Luxembourg", "partial"},
		{"it", "Italy", "Registro Imprese (InfoCamere)", "partial"},
		{"es", "Spain", "Registro Mercantil Central", "no"},
		{"pt", "Portugal", "Portal da Justiça", "partial"},
		{"ie", "Ireland", "CRO (Companies Registration Office)", "yes"},
		{"dk", "Denmark", "CVR (Det Centrale Virksomhedsregister)", "yes"},
		{"se", "Sweden", "Bolagsverket", "partial"},
		{"fi", "Finland", "PRH (Patentti- ja rekisterihallitus)", "yes"},
		{"no", "Norway", "Brønnøysundregistrene", "yes"},
		{"ch", "Switzerland", "Zefix (SECO)", "yes"},
		{"at", "Austria", "Firmenbuch", "no"},
		{"pl", "Poland", "KRS (Krajowy Rejestr Sądowy)", "yes"},
		{"cz", "Czech Republic", "ARES", "yes"},
		{"sk", "Slovakia", "ORSR (Obchodný register)", "partial"},
		{"hu", "Hungary", "Céginformációs Szolgálat", "partial"},
		{"ro", "Romania", "ONRC", "partial"},
		{"bg", "Bulgaria", "Targovski registar", "partial"},
		{"hr", "Croatia", "Sudski registar", "partial"},
		{"si", "Slovenia", "AJPES", "yes"},
		{"ee", "Estonia", "Äriregister", "yes"},
		{"lv", "Latvia", "Lursoft", "no"},
		{"lt", "Lithuania", "Registrų centras", "yes"},
		{"cy", "Cyprus", "Department of Registrar of Companies", "no"},
		{"mt", "Malta", "Malta Business Registry", "partial"},
		{"gr", "Greece", "GEMI", "partial"},
		{"us", "United States", "State Secretaries of State", "varies"},
		{"us_de", "US Delaware", "DDOS (Delaware Division of Corporations)", "no"},
		{"us_ny", "US New York", "New York Department of State", "partial"},
		{"us_ca", "US California", "California Secretary of State", "partial"},
		{"ca", "Canada", "Corporations Canada", "yes"},
		{"au", "Australia", "ASIC", "partial"},
		{"nz", "New Zealand", "NZBN (New Zealand Business Number)", "yes"},
		{"jp", "Japan", "法務局 (Legal Affairs Bureau)", "partial"},
		{"kr", "South Korea", "CRETOP", "no"},
		{"cn", "China", "SAIC / SAMR", "no"},
		{"hk", "Hong Kong", "CR (Companies Registry)", "partial"},
		{"sg", "Singapore", "ACRA", "yes"},
		{"in", "India", "MCA (Ministry of Corporate Affairs)", "partial"},
		{"br", "Brazil", "CNPJ (Receita Federal)", "partial"},
		{"mx", "Mexico", "SIEM / SAT", "partial"},
		{"za", "South Africa", "CIPC", "partial"},
	}

	entries := make(map[string]*dict.Entry, len(jurisdictions)*3)
	for _, j := range jurisdictions {
		meta := map[string]string{
			"code":      j.code,
			"name":      j.name,
			"registry":  j.registry,
			"open_data": j.openData,
		}
		entries[strings.ToLower(j.code)] = &dict.Entry{Metadata: meta}
		entries[dict.NormalizeLowercaseASCII(j.name)] = &dict.Entry{Metadata: meta}
	}

	fmt.Printf("  %d juridictions corporate\n", len(jurisdictions))
	return entries
}
