// Origin: one-time bulk extraction by .scratch/selfspec-codegen (task #345, RAC-A) from
// domains/hotam-spec-self/graph.json. That program was a THROWAWAY (run once, never
// committed to git) and is NOT a regeneration path -- it went graph->Go, the OPPOSITE of
// today's sync direction. These literals are HAND-MAINTAINED Go source: edit them directly,
// then mirror Go->graph via `hotam sync-self` (dry-run by default; --confirm-hash to write).
// The "generated" framing describes this file's ORIGIN (task #345's one-time bulk pull from
// the pre-existing JSON graph), not an ongoing regeneration workflow.
//
// Topic: Scope (operator sub-domain projection).
//
// STRUCTURAL fields only: Claim, Why, Owner, Status, Relations, Assumptions,
// Enforcement, EnforcedBy, Enforceability, MTag, Summary, CreatedAt, SettledAt,
// BlockedOn, ImplementedBy, VerifiedBy, SourceRefs, DeclOrder. EVENT fields
// (History, GateSignoffs, LastReviewedAt, ReviewAfter, Evidence) are
// deliberately absent — MergeIntoGraph passes them through from the graph's
// existing node untouched.
package selfspec

import "github.com/PHPCraftdream/HotamSpec/internal/ontology"

var _ = Requirements.MustRegister("R-backend-scope", ontology.Requirement{
	ID:             "R-backend-scope",
	Claim:          "The framework names no target backends: the core (graph/JSON proposals/CLI/Go test suite) stays backend-neutral by construction, and adapting the skin is the adopting agent's own concern.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "M37. Resolver verdict 2026-07-02 (verbatim): «не важно -- каждый умный агент под себя допишет» (English: 'it doesn't matter -- every smart agent will adapt it for itself'). This settles M37 by declining to name concrete backends (CI runner / alternate coding agent / programmatic or human resolver): the core surface (cmd/hotam CLIs, JSON Proposed* shapes, go test verification) already has no dependency on any one agent's runtime, so it stays backend-neutral by construction rather than by a designed OperatorBackend protocol. R-operator-backend-protocol remains gated/unbuilt -- the resolver's answer is that building it now would be speculative engineering against hypothetical backends R-speculative-aspects-frozen already warns against, not that the protocol is wrong in principle. REQUALIFIED 2026-07-02 (Wave 7 move 2 honesty pass): previously carried default enforceability=ENFORCEABLE, which listed it as closeable debt in UNENFORCED.md -- but no check_* can ever verify 'no target backend is named' or 'adapting the skin is the adopting agent's own concern' as a runtime property of the committed graph (it is a design-stance/non-decision about scope, not a structural fact). This is the same honesty class as R-initiator-supplies-domain-content and R-observation-evidence-scope: a real, permanent positional discipline that is INHERENTLY_PROSE, not ENFORCEABLE-but-unbuilt. The measurable SLICE of this claim already reached ENFORCED separately: R-core-imports-stdlib-or-hotam-spec-only mechanically verifies the CONSTRUCTION half -- go.mod proving the module imports nothing beyond the Go standard library -- so no backend dependency has silently crept into the core; R-backend-scope itself states the broader naming/positional stance, which stays INHERENTLY_PROSE.",
	Assumptions:    []string{"A-finite-context-operators"},
	Relations:      []ontology.Relation{{Kind: "replaces", Target: "R-operator-backend-protocol"}},
	Enforcement:    "PROSE",
	EnforcedBy:     []string{},
	MTag:           "",
	Enforceability: "INHERENTLY_PROSE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{"go.mod"},
	DeclOrder:      74,
})

var _ = Requirements.MustRegister("R-overlap-single-presenter", ontology.Requirement{
	ID:             "R-overlap-single-presenter",
	Claim:          "Every node contested by two or more operators' overlapping scope projections shall resolve to exactly one deterministic presenter.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "M18/P-scope. presenterForNode (internal/invariants/scope_process.go) returns the LEXICOGRAPHICALLY FIRST operator id among the operators contesting a node -- a stable, total, deterministic rule that does not depend on graph declaration order (which shifts under unrelated source reformatting) or on an arbitrary parent-hierarchy walk. check_scoped_node_has_single_presenter (internal/invariants/scope_process.go) walks every pair of operators with a non-empty Scope, computes their overlap via projectScope, and fires a Violation only if presenterForNode ever returned empty for a non-empty contesting set -- which cannot happen, making single-presentership PROVABLE rather than merely asserted. Registered in the invariant registry (internal/invariants/all_violations.go) and in the methodology as Canon §Scope.",
	Assumptions:    []string{"A-finite-context-operators"},
	Relations:      []ontology.Relation{{Kind: "replaces", Target: "R-partition-vs-border"}},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"check_scoped_node_has_single_presenter"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-07-02",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{"internal/invariants/scope_process.go", "internal/invariants/all_violations.go"},
	DeclOrder:      229,
})

var _ = Requirements.MustRegister("R-partition-vs-border", ontology.Requirement{
	ID:             "R-partition-vs-border",
	Claim:          "Operator sub-domains shall relate to the parent graph by a single declared discipline (strict partition or declared-border overlap).",
	Owner:          "framework-author",
	Status:         "REJECTED",
	Why:            "REJECTED -- REPLACES R-scope-is-projection + R-scope-overlap-generated + R-overlap-single-presenter. The open question ('strict partition, or overlap on explicitly-declared delegation borders?') presupposed a binary; the actual answer is a third option -- a PROJECTION. Operator sub-domains are neither a strict partition (which would forbid any shared node) nor an ad-hoc declared-border overlap (which would need hand-maintained edge lists); they are a computed, always-current id-set VIEW over the one shared graph (hotam_spec.scope_projection.project_scope), and any overlap between two operators' views is itself computed and rendered visibly (scope_overlap + the generator's OVERLAP block) rather than declared by hand or forbidden outright. A contested node under such an overlap resolves to exactly one deterministic presenter (R-overlap-single-presenter), closing the ambiguity the OPEN question was protecting against without adopting either extreme it named. -- (was: M18. Delegation (R-context-bounded-delegation) needs to know whether shared objects are forbidden (partition) or first-class borders; the two give different drift behavior.)",
	Assumptions:    []string{"A-finite-context-operators"},
	Relations:      []ontology.Relation{},
	Enforcement:    "PROSE",
	EnforcedBy:     []string{},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "",
	SourceRefs:     []string{},
	DeclOrder:      60,
})

var _ = Requirements.MustRegister("R-scope-is-projection", ontology.Requirement{
	ID:             "R-scope-is-projection",
	Claim:          "An operator's sub-domain shall be a computed PROJECTION (an id-set view derived by prefix match over the shared graph), never a copy of any node.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "M18/P-scope. Resolves R-partition-vs-border's open question by rejecting both extremes: a strict partition forbids any shared object (loses the ability to model deliberate cross-cutting delegation); an ad-hoc declared-border overlap invites hand-maintained edge lists that drift from the graph. The projection is projectScope(g, prefixes) in internal/invariants/scope_process.go: it builds a scopeView (sets of requirement+conflict ids) purely from the graph by id-prefix match (strings.HasPrefix(r.ID, p) for any p in the operator's Scope prefixes), on demand, with no copied node data. This is the exact prefix-match discipline the generator already uses for per-domain CONSTITUTION digests, so the projection can never fork from the single writer (R-no-hand-edit-graph). The declaring prefix tuple is carried by Operator.Scope (internal/ontology/operator.go). The projection is consumed by check_scoped_node_has_single_presenter to compute overlaps, so the projection is load-bearing in the invariant layer today even though only one OP-director (Scope=()) currently exists. ENFORCED 2026-07-13: TestProjectScopeIsIDProjectionNotCopy in internal/invariants/scope_process_test.go mechanically verifies this claim.",
	Assumptions:    []string{"A-finite-context-operators"},
	Relations:      []ontology.Relation{{Kind: "replaces", Target: "R-partition-vs-border"}},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"TestProjectScopeIsIDProjectionNotCopy"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-07-02",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{"internal/invariants/scope_process.go", "internal/ontology/operator.go"},
	DeclOrder:      227,
})

var _ = Requirements.MustRegister("R-scope-overlap-generated", ontology.Requirement{
	ID:             "R-scope-overlap-generated",
	Claim:          "When two operators' scope projections share a node, the overlap shall be computed (never hidden or silently merged); the rendering of that overlap into a per-operator crystal is deferred until a real second operator exists.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "M18/P-scope. The overlap is computed by scopeOverlapNodeIDs(a, b scopeView) in internal/invariants/scope_process.go -- the sorted set-intersection of two scopeViews over requirement+conflict ids -- and is consumed by check_scoped_node_has_single_presenter, which walks every unordered pair of operators with a non-empty Scope, projects each, and fires only if a contested node has no determinable presenter. So the COMPUTE half of the claim is live and load-bearing in the invariant layer. The RENDER half -- emitting one OVERLAP:BEGIN/END block per agent against every OTHER discovered agent's SCOPE -- is not implemented: the generator (internal/generator) produces a single consolidated root CLAUDE.md and has no per-agent crystal generation at all, because with the current meta-domain (one OP-director, SCOPE=()) there are no agents to render overlap for (R-claude-md-consolidates-when-single-agent). (Historical note: the original render was tools/gen_spec.py::_regenerate_agent_constitutions, which emitted one OVERLAP block per agent.) The glossary still defines §Scope as 'overlaps with another operator's projection are computed and rendered, never hidden' (internal/generator/glossary_terms_data.go); the compute is real, the per-agent render is deferred, not abandoned -- it materializes when R-sub-agent-crystal-triad's per-agent generation is implemented alongside a real second operator. ENFORCED 2026-07-13: TestScopeOverlapNodeIDs_DeterministicIntersection in internal/invariants/scope_process_test.go mechanically verifies this claim.",
	Assumptions:    []string{"A-finite-context-operators"},
	Relations:      []ontology.Relation{{Kind: "replaces", Target: "R-partition-vs-border"}},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"TestScopeOverlapNodeIDs_DeterministicIntersection"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-07-02",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{"internal/invariants/scope_process.go", "internal/generator/glossary_terms_data.go"},
	DeclOrder:      228,
})
