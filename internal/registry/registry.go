package registry

import "fmt"

type Registry[T any] struct {
	order  []string
	byName map[string]*T
}

func New[T any]() *Registry[T] {
	return &Registry[T]{byName: make(map[string]*T)}
}

func (r *Registry[T]) MustRegister(name string, v T) *T {
	if _, exists := r.byName[name]; exists {
		panic(fmt.Sprintf("registry: duplicate name %q", name))
	}
	cv := v
	r.byName[name] = &cv
	r.order = append(r.order, name)
	return &cv
}

func (r *Registry[T]) All() []T {
	out := make([]T, len(r.order))
	for i, name := range r.order {
		out[i] = *r.byName[name]
	}
	return out
}

func (r *Registry[T]) Get(name string) (*T, bool) {
	v, ok := r.byName[name]
	return v, ok
}
