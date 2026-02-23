# CLAUDE.md — touchstone-registry

## Ce que c'est

Registre linguistique et outil d'audit par dictionnaires. Valide du contenu textuel contre des dictionnaires spécialisés (14 catégories). Utilisé pour la conformité et la qualité linguistique.

**Module** : `github.com/hazyhaar/touchstone-registry`
**État** : Actif, mises à jour récentes (Feb 2026)

## Structure

```
touchstone-registry-audit/
├── cmd/server/            # Entry point serveur
├── pkg/                   # Bibliothèques internes
├── dicts/                 # 14 sous-dossiers de dictionnaires
├── config.yaml            # Configuration YAML
├── AUDIT.md               # Documentation audit
└── go.mod
```

## Build

```bash
CGO_ENABLED=0 go build -o bin/touchstone ./cmd/server/
```

## Particularités

- Config via **YAML** (pas env vars)
- Dictionnaires embarqués dans `dicts/` (14 catégories)
- Dépend de `github.com/hazyhaar/pkg`
