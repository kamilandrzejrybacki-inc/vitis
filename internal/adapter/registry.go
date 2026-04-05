package adapter

import "fmt"

type Registry struct {
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
	r.adapters[item.ID()] = item
}

func (r *Registry) Get(id string) (Adapter, error) {
	item, ok := r.adapters[id]
	if !ok {
		return nil, fmt.Errorf("unknown provider %q", id)
	}
	return item, nil
}
