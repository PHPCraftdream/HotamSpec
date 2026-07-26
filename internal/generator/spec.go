// spec.go re-exports docs/gen/SPEC.md's generator surface
// (PLAN-scenario-generated-spec.md §2 D2/§3 W1.3): the generated NORMATIVE
// TEXT projection -- a requirement's claim (still the short AUTHORED intent
// from graph.json, D2 — never invented here) followed by the GENERATED prose
// narrative of its verified_by scenario(s), rendered from the ACTUAL
// Given/When/Then/Value steps a real, passing `go test` run just recorded
// via internal/recorder/canon's hotamspec API.
//
// LAYERING (W2.3): the actual data-collection and rendering logic moved to
// internal/gate/spec_build.go -- see that file's own doc comment for the
// full reasoning. In short: task W2.3 needed a mechanical staleness check
// (check_spec_md_current) living in internal/invariants, but
// internal/invariants must never import internal/generator (a real import
// cycle: internal/generator -> internal/diagnose -> internal/invariants
// already exists, and internal/generator's own fixture_test.go imports
// internal/invariants directly). internal/gate is a true leaf both
// internal/generator and internal/invariants already depend on directly, so
// the shared logic lives there once; this file is now a thin re-export layer
// so every existing caller (cmd/hotam/gen_spec.go,
// internal/generator/traceability.go, internal/generator/coverage.go) and
// every existing test (internal/generator/spec_test.go) is unaffected --
// same package-level names, same signatures, same behavior, zero call-site
// changes required outside this file.
package generator

import (
	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// SpecRow is a type alias (not a new/wrapping type) for gate.SpecRow, so a
// generator.SpecRow value IS a gate.SpecRow value -- no conversion needed at
// any existing call site.
type SpecRow = gate.SpecRow

// ScenarioVerdict is a type alias for gate.ScenarioVerdict. NEITHER
// BuildTraceability NOR BuildCoverage (this package) has accepted a verdicts
// parameter since task #317 deliberately removed that overlay to keep both
// docs mode-independent (see gate.ScenarioVerdict's own doc comment,
// "HISTORICAL NOTE", for the full story) -- this alias is kept only for this
// package's existing public surface / test coverage. task #370's
// internal/selfspec.RequirementState is the current, general-purpose home
// for a real execution-based per-requirement proof verdict.
type ScenarioVerdict = gate.ScenarioVerdict

// CollectSpecRows re-exports gate.CollectSpecRows -- see that function's own
// doc comment (internal/gate/spec_build.go) for the full contract.
func CollectSpecRows(g *ontology.Graph) map[string]SpecRow {
	return gate.CollectSpecRows(g)
}

// ScenarioVerdictsFromRows re-exports gate.ScenarioVerdictsFromRows.
func ScenarioVerdictsFromRows(rows map[string]SpecRow) map[string]ScenarioVerdict {
	return gate.ScenarioVerdictsFromRows(rows)
}

// BuildSpec re-exports gate.BuildSpec -- see that function's own doc comment
// (internal/gate/spec_build.go) for BuildSpec's full rendering contract
// (three honest outcomes per requirement, determinism/byte-identical
// guarantee, etc.).
func BuildSpec(g *ontology.Graph) string {
	return gate.BuildSpec(g)
}

// BuildSpecFromRows re-exports gate.BuildSpecFromRows.
func BuildSpecFromRows(g *ontology.Graph, rows map[string]SpecRow) string {
	return gate.BuildSpecFromRows(g, rows)
}
