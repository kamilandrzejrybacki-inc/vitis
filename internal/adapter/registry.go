package adapter

import (
	"fmt"
	"sync"
)

type Registry struct {
	mu       sync.RWMutex
	adapters map[string]Adapter
}

func NewRegistry(items ...Adapter) *Registry {
	r := &Registry{adapters: map[string]Adapter{}}
	for _, item := range items {
		r.Register(item)
	}
	return r
}

func (r *Registry) Register(item Adapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters[item.ID()] = item
}

func (r *Registry) Get(id string) (Adapter, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.adapters[id]
	if !ok {
		return nil, fmt.Errorf("unknown provider %q", id)
	}
	return item, nil
}
