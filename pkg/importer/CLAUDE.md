# importer

Responsabilite: Framework d'import de dictionnaires depuis des sources publiques — interface Adapter avec registry global, adaptateurs concrets (INSEE prenoms/patronymes/communes, SSA babynames, Census surnames, Companies House, SIRENE), source DB SQLite pour le tracking des URLs et disponibilite, checker periodique HEAD, et helpers HTTP/ZIP.
Depend de: `pkg/dict`, `modernc.org/sqlite`, `gopkg.in/yaml.v3`
Dependants: `cmd/server`
Point d'entree: `adapter.go` (Register, Get, All), `sourcedb.go` (OpenSourceDB), `checker.go` (NewChecker)
Types cles: `Adapter` (interface), `SourceDB`, `Source`, `Checker`
Invariants: Les adaptateurs s'enregistrent dans un registry global via init(). La source DB est en WAL mode. Le checker fait des HEAD requests (pas GET) pour verifier la disponibilite. Les downloads ont 3 retries avec backoff exponentiel.
NE PAS: Creer un adaptateur sans l'enregistrer via Register() dans init(). Faire des GET complets dans le checker (HEAD uniquement). Telecharger sans retry.
