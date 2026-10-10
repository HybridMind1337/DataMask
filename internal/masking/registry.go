package masking

import (
	"fmt"
	"sync"
)

// Registry manages registered masking transformers.
type Registry struct {
	mu           sync.RWMutex
	transformers map[string]Transformer
}

// NewRegistry initializes an empty transformer registry.
func NewRegistry() *Registry {
	return &Registry{
		transformers: make(map[string]Transformer),
	}
}

// Register adds a transformer to the registry.
func (r *Registry) Register(t Transformer) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := t.Name()
	if _, exists := r.transformers[name]; exists {
		return fmt.Errorf("transformer %q is already registered", name)
	}
	r.transformers[name] = t
	return nil
}

// Get retrieves a transformer by name.
func (r *Registry) Get(name string) (Transformer, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	t, ok := r.transformers[name]
	return t, ok
}

// List returns all registered transformer names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.transformers))
	for name := range r.transformers {
		names = append(names, name)
	}
	return names
}
