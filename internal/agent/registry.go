package agent

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"sync"
)

// ErrUnknownAgent is returned when a requested agent type is not registered.
var ErrUnknownAgent = errors.New("agent: unknown agent type") //nolint:gochecknoglobals // sentinel error

// BackendFactory creates an AgentBackend for a given agent type.
type BackendFactory func(runtime *DockerRuntime) (AgentBackend, error)

// Registry manages agent backend factories.
type Registry struct {
	mu        sync.RWMutex
	factories map[string]BackendFactory
}

func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]BackendFactory),
	}
}

// Register adds a backend factory for an agent type.
func (r *Registry) Register(agentType string, factory BackendFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[agentType] = factory
}

// Create instantiates a backend for the given agent type.
func (r *Registry) Create(agentType string, runtime *DockerRuntime) (AgentBackend, error) {
	r.mu.RLock()
	factory, ok := r.factories[agentType]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("agent.Registry.Create(%q): %w", agentType, ErrUnknownAgent)
	}

	backend, err := factory(runtime)
	if err != nil {
		return nil, fmt.Errorf("agent.Registry.Create(%q): %w", agentType, err)
	}

	return backend, nil
}

// Available returns registered agent type names in sorted order.
func (r *Registry) Available() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := slices.Collect(func(yield func(string) bool) {
		for name := range r.factories {
			if !yield(name) {
				return
			}
		}
	})
	sort.Strings(names)

	return names
}
