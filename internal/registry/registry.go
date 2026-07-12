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

// Update replaces the value already registered under name, in place,
// leaving registration order untouched. It exists so a later-loaded package
// (e.g. cmd/hotam wiring a Tool's Run field with a concrete command
// function) can patch a registry entry declared by an earlier, dependency-free
// package (e.g. internal/methodology, which must not import cmd/hotam) — the
// registering package stays ignorant of the patch, but every holder of the
// same *Registry[T] observes it, because All()/Get() read through the same
// pointer this method mutates. Update panics on an unknown name — patching a
// tool that was never declared is a wiring bug, not a runtime condition to
// tolerate silently.
func (r *Registry[T]) Update(name string, v T) {
	existing, ok := r.byName[name]
	if !ok {
		panic(fmt.Sprintf("registry: Update of unregistered name %q", name))
	}
	*existing = v
}
