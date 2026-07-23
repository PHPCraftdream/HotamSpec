// Package selfspec is Phase 0 (RAC-0, task #344) of the "requirements-as-code"
// migration: a Go registry of ontology.Requirement values that mirrors a
// hand-picked subset of domains/hotam-spec-self/graph.json's Requirement
// nodes, proving the byte-identity mechanism before it scales to all 301
// requirements (Phase A) and before any authority flip (Phase B).
//
// graph.json remains the universal on-disk format and the SOLE authority
// today — this package changes nothing about what is read or trusted at
// runtime. Requirements is a proven-byte-identical MIRROR: MergeIntoGraph
// (merge.go) can replace a graph Requirement's structural fields with the
// registry's value and re-serialize to the exact same bytes the committed
// graph.json already has. No invariant reads this registry yet (that starts,
// in shadow mode, in Phase A), and no CLI command writes through it (that is
// Phase B).
//
// Value proposition of this phase: authority-by-construction (a Requirement's
// structural shape becomes a reviewed Go diff, not a hand-edited JSON blob)
// and a reviewed-diff workflow for the highest-friction fields, NOT type
// safety — ImplementedBy/VerifiedBy stay []string permanently. Proof cannot
// be typed in Go at all, and execution-checking (does the named test
// actually exist and pass) is a strictly stronger guarantee than any type the
// Go compiler could enforce over a file:symbol string.
package selfspec

import (
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/registry"
)

// Requirements is the Phase 0 pilot registry: today it holds the hand-picked
// subset of requirement IDs registered in requirements_pilot.go (~15-20
// entries), not all 301. A registered ID that is absent from the graph is a
// MergeIntoGraph error (see merge.go) — Phase 0 never creates graph nodes,
// only mirrors existing ones.
var Requirements = registry.New[ontology.Requirement]()
