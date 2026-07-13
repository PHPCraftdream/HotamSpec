// Package selfcheck hosts static self-checks of the Hotam-Spec framework's OWN
// source shape. It exists as a dedicated package (rather than tests inside
// internal/methodology or internal/ontology) because the properties it enforces
// are cross-cutting hygiene invariants about the whole framework tree
// (internal/* and cmd/hotam/*), not the behavior of any single package:
// content-freeness (no business data baked into framework source), the
// controlled import graph (stdlib-or-self only; core never reaches periphery;
// the framework never imports domain-owned agents/tools runtime dirs), the
// ontology's deliberate non-reification of certain types, the single shared
// tool registry, and the no-seed-graph behavioral guarantee.
//
// The checks use go/parser + go/ast (stdlib only — see go.mod) to mechanically
// scan .go files under internal/ and cmd/hotam/ at test time. They are real
// repo-source scans, not fixture-based: they fail the moment a violating
// import, type declaration, embedded business token, or unwired tool lands.
package selfcheck
