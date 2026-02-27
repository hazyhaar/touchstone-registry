# touchstone-registry-audit

Responsabilité: Registre linguistique et audit par dictionnaires — valide du contenu textuel contre 14 catégories de dictionnaires spécialisés.
Module: `github.com/hazyhaar/touchstone-registry`
État: Actif, mises à jour récentes (Feb 2026)

## Index

| Fichier/Dir | Rôle |
|-------------|------|
| `cmd/server/` | Entry point serveur |
| `pkg/` | Bibliothèques internes |
| `dicts/` | 14 sous-dossiers de dictionnaires embarqués |
| `config.yaml` | Configuration YAML |
| `AUDIT.md` | Documentation audit |

Dépend de: `github.com/hazyhaar/pkg`

## Build

```bash
CGO_ENABLED=0 go build -o bin/touchstone ./cmd/server/
```

## Invariants

- Config via **YAML** (pas env vars)
- Dictionnaires embarqués dans `dicts/`

## NE PAS

- Utiliser env vars pour la config (YAML est le standard ici)
