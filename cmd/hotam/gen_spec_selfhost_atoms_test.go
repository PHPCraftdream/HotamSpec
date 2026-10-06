package main

import (
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/registry"
	"github.com/PHPCraftdream/HotamSpec/internal/selfspec"
)

// swapSelfspecRegistry replaces the package-global selfspec.Requirements with
// a fresh registry seeded with one marker requirement, restoring the original
// on cleanup.
func swapSelfspecRegistry(t *testing.T) {
	t.Helper()
	old := selfspec.Requirements
	marker := ontology.Requirement{ID: "R-test-marker-atom"}
	fresh := registry.New[ontology.Requirement]()
	fresh.MustRegister(marker.ID, marker)
	selfspec.Requirements = fresh
	t.Cleanup(func() { selfspec.Requirements = old })
}

// TestAtomOverridesSelfHostingUsesInProcessRegistry: a self-hosting graph
// must take overrides from selfspec.Requirements even when domainDir has no
// spec/ at all (the subprocess path would fail there); consumer graphs keep
// the subprocess behavior.
func TestAtomOverridesSelfHostingUsesInProcessRegistry(t *testing.T) {
	swapSelfspecRegistry(t)
	g := &ontology.Graph{SelfHosting: true}
	overrides, err := atomOverrides(g, t.TempDir())
	if err != nil {
		t.Fatalf("self-hosting overrides: unexpected error: %v", err)
	}
	if _, ok := overrides.Get("R-test-marker-atom"); !ok {
		t.Fatal("self-hosting overrides not equivalent to selfspec.Requirements: marker missing")
	}
}

// TestRequiresAtomDiscovered: consumer domains (empty packages) keep the
// legacy all-non-REJECTED-must-be-atoms rule; root-module mode only demands
// atoms for requirements implemented inside a listed atom package.
func TestRequiresAtomDiscovered(t *testing.T) {
	manual := ontology.Requirement{
		ID:            "R-manual",
		ImplementedBy: []string{"internal/other/x.go:Sym"},
	}
	atom := ontology.Requirement{
		ID:            "R-atom",
		ImplementedBy: []string{"internal/localization/atoms.go:Catalog.Supported"},
	}
	pkgs := []string{"internal/localization"}

	if !requiresAtomDiscovered(manual, nil) {
		t.Error("empty packages: manual requirement must still require discovery (legacy behavior)")
	}
	if requiresAtomDiscovered(manual, pkgs) {
		t.Error("manual requirement outside atom packages must be skipped")
	}
	if !requiresAtomDiscovered(atom, pkgs) {
		t.Error("atom requirement inside a listed package must require discovery")
	}
	if !requiresAtomDiscovered(ontology.Requirement{ImplementedBy: []string{"internal/localization/x.go:Y"}}, pkgs) {
		t.Error("file directly at package root must match")
	}
	if requiresAtomDiscovered(ontology.Requirement{}, pkgs) {
		t.Error("requirement without implemented_by is not an atom")
	}
	if requiresAtomDiscovered(ontology.Requirement{ImplementedBy: []string{"internal/localization-other/x.go:Y"}}, pkgs) {
		t.Error("prefix collision must not match (package boundary required)")
	}
}
