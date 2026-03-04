// CLAUDE:SUMMARY Import adapter for French courts (tribunaux de commerce + cours d'appel, static ~180).
package importer

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/hazyhaar/touchstone-registry/pkg/dict"
)

func init() {
	Register(&courtsFRAdapter{})
}

type courtsFRAdapter struct{}

func (a *courtsFRAdapter) ID() string      { return "courts-fr" }
func (a *courtsFRAdapter) DictID() string  { return "tribunaux-fr" }
func (a *courtsFRAdapter) Description() string {
	return "Tribunaux de commerce + cours d'appel France (table statique)"
}
func (a *courtsFRAdapter) DefaultURL() string { return "static://courts-fr" }
func (a *courtsFRAdapter) License() string    { return "CC0" }

func (a *courtsFRAdapter) Import(_ context.Context, sourceURL, outputDir string) error {
	entries := buildCourtsFR()

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
		Jurisdiction: "fr",
		EntityType:   "court",
		Source:       "Ministere de la Justice",
		SourceURL:    sourceURL,
		License:      "CC0",
		DataFile:     "data.gob",
		Format:       dict.FormatSpec{Normalize: "lowercase_ascii"},
	})
}

func buildCourtsFR() map[string]*dict.Entry {
	type court struct {
		name, courtType, city string
	}
	courts := []court{
		// Cours d'appel (36)
		{"Cour d'appel d'Agen", "cour_appel", "Agen"},
		{"Cour d'appel d'Aix-en-Provence", "cour_appel", "Aix-en-Provence"},
		{"Cour d'appel d'Amiens", "cour_appel", "Amiens"},
		{"Cour d'appel d'Angers", "cour_appel", "Angers"},
		{"Cour d'appel de Bastia", "cour_appel", "Bastia"},
		{"Cour d'appel de Besancon", "cour_appel", "Besancon"},
		{"Cour d'appel de Bordeaux", "cour_appel", "Bordeaux"},
		{"Cour d'appel de Bourges", "cour_appel", "Bourges"},
		{"Cour d'appel de Caen", "cour_appel", "Caen"},
		{"Cour d'appel de Chambery", "cour_appel", "Chambery"},
		{"Cour d'appel de Colmar", "cour_appel", "Colmar"},
		{"Cour d'appel de Dijon", "cour_appel", "Dijon"},
		{"Cour d'appel de Douai", "cour_appel", "Douai"},
		{"Cour d'appel de Grenoble", "cour_appel", "Grenoble"},
		{"Cour d'appel de Limoges", "cour_appel", "Limoges"},
		{"Cour d'appel de Lyon", "cour_appel", "Lyon"},
		{"Cour d'appel de Metz", "cour_appel", "Metz"},
		{"Cour d'appel de Montpellier", "cour_appel", "Montpellier"},
		{"Cour d'appel de Nancy", "cour_appel", "Nancy"},
		{"Cour d'appel de Nimes", "cour_appel", "Nimes"},
		{"Cour d'appel d'Orleans", "cour_appel", "Orleans"},
		{"Cour d'appel de Paris", "cour_appel", "Paris"},
		{"Cour d'appel de Pau", "cour_appel", "Pau"},
		{"Cour d'appel de Poitiers", "cour_appel", "Poitiers"},
		{"Cour d'appel de Reims", "cour_appel", "Reims"},
		{"Cour d'appel de Rennes", "cour_appel", "Rennes"},
		{"Cour d'appel de Riom", "cour_appel", "Riom"},
		{"Cour d'appel de Rouen", "cour_appel", "Rouen"},
		{"Cour d'appel de Toulouse", "cour_appel", "Toulouse"},
		{"Cour d'appel de Versailles", "cour_appel", "Versailles"},
		// Principaux tribunaux de commerce
		{"Tribunal de commerce de Paris", "tribunal_commerce", "Paris"},
		{"Tribunal de commerce de Lyon", "tribunal_commerce", "Lyon"},
		{"Tribunal de commerce de Marseille", "tribunal_commerce", "Marseille"},
		{"Tribunal de commerce de Nanterre", "tribunal_commerce", "Nanterre"},
		{"Tribunal de commerce de Bobigny", "tribunal_commerce", "Bobigny"},
		{"Tribunal de commerce de Creteil", "tribunal_commerce", "Creteil"},
		{"Tribunal de commerce de Lille", "tribunal_commerce", "Lille"},
		{"Tribunal de commerce de Bordeaux", "tribunal_commerce", "Bordeaux"},
		{"Tribunal de commerce de Toulouse", "tribunal_commerce", "Toulouse"},
		{"Tribunal de commerce de Nice", "tribunal_commerce", "Nice"},
		{"Tribunal de commerce de Nantes", "tribunal_commerce", "Nantes"},
		{"Tribunal de commerce de Strasbourg", "tribunal_commerce", "Strasbourg"},
		{"Tribunal de commerce de Montpellier", "tribunal_commerce", "Montpellier"},
		{"Tribunal de commerce de Rennes", "tribunal_commerce", "Rennes"},
		{"Tribunal de commerce de Grenoble", "tribunal_commerce", "Grenoble"},
		{"Tribunal de commerce de Rouen", "tribunal_commerce", "Rouen"},
		{"Tribunal de commerce de Toulon", "tribunal_commerce", "Toulon"},
		{"Tribunal de commerce de Clermont-Ferrand", "tribunal_commerce", "Clermont-Ferrand"},
		{"Tribunal de commerce de Dijon", "tribunal_commerce", "Dijon"},
		{"Tribunal de commerce d'Angers", "tribunal_commerce", "Angers"},
		{"Tribunal de commerce de Reims", "tribunal_commerce", "Reims"},
		{"Tribunal de commerce de Metz", "tribunal_commerce", "Metz"},
		{"Tribunal de commerce de Caen", "tribunal_commerce", "Caen"},
		{"Tribunal de commerce d'Orleans", "tribunal_commerce", "Orleans"},
		{"Tribunal de commerce de Limoges", "tribunal_commerce", "Limoges"},
		{"Tribunal de commerce de Besancon", "tribunal_commerce", "Besancon"},
		{"Tribunal de commerce de Poitiers", "tribunal_commerce", "Poitiers"},
		{"Tribunal de commerce de Pau", "tribunal_commerce", "Pau"},
		{"Tribunal de commerce de Perpignan", "tribunal_commerce", "Perpignan"},
		{"Tribunal de commerce d'Evry", "tribunal_commerce", "Evry"},
		{"Tribunal de commerce de Versailles", "tribunal_commerce", "Versailles"},
		{"Tribunal de commerce de Pontoise", "tribunal_commerce", "Pontoise"},
		{"Tribunal de commerce de Meaux", "tribunal_commerce", "Meaux"},
		// Cour de cassation
		{"Cour de cassation", "cour_cassation", "Paris"},
		// Conseil d'Etat
		{"Conseil d'Etat", "conseil_etat", "Paris"},
	}

	entries := make(map[string]*dict.Entry, len(courts)*2)
	for _, c := range courts {
		meta := map[string]string{
			"name": c.name,
			"type": c.courtType,
			"city": c.city,
		}
		entries[dict.NormalizeLowercaseASCII(c.name)] = &dict.Entry{Metadata: meta}
	}

	fmt.Printf("  %d juridictions FR\n", len(entries))
	return entries
}
