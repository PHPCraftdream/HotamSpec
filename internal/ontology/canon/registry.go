package hotamontology

import "fmt"

// Registry mirrors internal/registry.Registry[T] (internal/registry/
// registry.go) -- same shape, same algorithm, unchanged: New/MustRegister/
// All/Get. Vendored here (rather than imported) for the same reason
// Requirement above is vendored rather than imported -- a consumer domain's
// spec/ module is a separate Go module and per NEW-2-bis this engine never
// wires a cross-module `replace` to bridge that gap. A consumer domain uses
// this Registry[Requirement] to declare its own Requirement literals in Go
// code, the same registry shape internal/selfspec's own requirements_*.go
// files already use engine-side.
//
// Deliberately NOT vendoring Update (internal/registry.Registry's fourth
// method): Update exists engine-side to let a later-loaded package patch an
// earlier package's registry entry in place (e.g. cmd/hotam wiring a Tool's
// Run field) -- a wiring pattern with no counterpart in a consumer domain's
// own flat Requirement-literal registration, so this mirror stays exactly
// the four methods a consumer domain actually needs.
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
