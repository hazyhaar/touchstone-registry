# CLAUDE.md — touchstone-registry

> **Règle n°1** — Un bug trouvé en audit mais pas par un test est d'abord une faille de test. Écrire le test rouge, puis fixer. Pas de fix sans test.

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
