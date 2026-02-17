package notify

import (
	"github.com/gosuda/aira/internal/messenger"
)

// Registry is a simple map-based MessengerRegistry.
type Registry struct {
	messengers map[string]messenger.Messenger
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		messengers: make(map[string]messenger.Messenger),
	}
}

// Register adds a messenger for the given platform name.
func (r *Registry) Register(platform string, m messenger.Messenger) {
	r.messengers[platform] = m
}

// Get returns the messenger for the given platform, or false if not registered.
func (r *Registry) Get(platform string) (messenger.Messenger, bool) {
	m, ok := r.messengers[platform]
	return m, ok
}
