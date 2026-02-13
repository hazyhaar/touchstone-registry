package importer

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// Adapter defines a data source importer that downloads, transforms, and
// serializes a dictionary into gob format.
type Adapter interface {
	// ID returns the unique identifier of this adapter (e.g. "insee-prenoms-fr").
	ID() string
	// DictID returns the target dictionary ID (e.g. "prenoms-fr").
	DictID() string
	// Description returns a human-readable description.
	Description() string
	// DefaultURL returns the default source URL used for seeding the database.
	DefaultURL() string
	// License returns the license identifier for this source (e.g. "CC0", "OGL v3").
	License() string
	// Import downloads the source from sourceURL, transforms it, and writes
	// data.gob + manifest.yaml into a subdirectory of outputDir named after DictID().
	Import(ctx context.Context, sourceURL, outputDir string) error
}

var (
	registryMu sync.RWMutex
	adapters   = make(map[string]Adapter)
)

// Register adds an adapter to the global registry.
func Register(a Adapter) {
	registryMu.Lock()
	defer registryMu.Unlock()
	adapters[a.ID()] = a
}

// Get returns a registered adapter by ID, or an error if not found.
func Get(id string) (Adapter, error) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	a, ok := adapters[id]
	if !ok {
		return nil, fmt.Errorf("unknown import source: %q", id)
	}
	return a, nil
}

// All returns all registered adapters sorted by ID.
func All() []Adapter {
	registryMu.RLock()
	defer registryMu.RUnlock()
	result := make([]Adapter, 0, len(adapters))
	for _, a := range adapters {
		result = append(result, a)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID() < result[j].ID() })
	return result
}
