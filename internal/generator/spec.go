// Shared SPEC rendering lives in gate so generators and invariants use the
// same authored document, selected examples, and execution evidence.
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

// BuildSpec renders the authored document when configured; otherwise it
// renders the requirement projection.
func BuildSpec(g *ontology.Graph) string {
	return gate.BuildSpec(g)
}

// BuildSpecFromRows re-exports gate.BuildSpecFromRows.
func BuildSpecFromRows(g *ontology.Graph, rows map[string]SpecRow) string {
	return gate.BuildSpecFromRows(g, rows)
}
