# dict

Responsabilite: Coeur du registre linguistique — chargement de dictionnaires depuis gob/CSV/patterns, normalisation de texte (lowercase+strip-accents, lowercase-only, none), pattern matching avec validateurs de checksums (IBAN mod97, Luhn, NIR), registry thread-safe pour les queries de classification cross-dictionnaires.
Depend de: `golang.org/x/text/...`, `gopkg.in/yaml.v3`
Dependants: `pkg/api`, `pkg/importer`, `cmd/server`
Point d'entree: `registry.go` (NewRegistry, Load, Classify), `dict.go` (LoadDictionary)
Types cles: `Registry`, `Dictionary`, `Entry`, `Manifest`, `Normalizer`, `ClassifyResult`, `ClassifyOptions`, `DictInfo`, `PatternSpec`, `patternMatcher`
Invariants: Le registry est thread-safe (sync.RWMutex). Gob prend priorite sur CSV au chargement. La normalisation est configurable par dictionnaire via manifest.yaml. Les pattern matchers supportent les validateurs IBAN, Luhn, NIR.
NE PAS: Charger un dictionnaire sans manifest.yaml. Modifier les normalizers sans verifier les tests existants. Acceder aux entries du dictionnaire sans passer par le registry.
