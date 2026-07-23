// Package selfspec is Phase A (RAC-A, task #345) of the "requirements-as-code"
// migration: a Go registry of ontology.Requirement values that mirrors EVERY
// domains/hotam-spec-self/graph.json Requirement node (all 301: 253 SETTLED +
// 42 REJECTED + 6 DRAFT) — the byte-identity mechanism Phase 0 (RAC-0, task
// #344) proved on a ~18-entry hand-picked pilot subset, now scaled to full
// coverage, before any authority flip (Phase B, RAC-B, task #346).
//
// graph.json remains the universal on-disk format and the SOLE authority
// today — this package changes nothing about what is read or trusted at
// runtime. Requirements is a proven-byte-identical MIRROR: MergeIntoGraph
// (merge.go) can replace a graph Requirement's structural fields with the
// registry's value and re-serialize to the exact same bytes the committed
// graph.json already has. check_self_requirements_match_registry
// (selfcheck_shadow.go) reads this registry in SHADOW mode (advisory-only,
// never registered in internal/invariants' All — see that file's own doc
// comment for why); no CLI command writes through it yet (that is Phase B).
//
// The registration literals live in requirements_<topic>.go files (one per
// thematic bucket — operator, agent, domain, entity, conflict, and so on;
// see .scratch/selfspec-codegen's topicRules for the exact classification),
// mirroring internal/invariants' many-files-by-topic layout rather than one
// giant file. Every entry carries ONLY structural fields (Claim, Why, Owner,
// Status, Relations, Assumptions, Enforcement, EnforcedBy, Enforceability,
// MTag, Summary, CreatedAt, SettledAt, BlockedOn, ImplementedBy, VerifiedBy,
// SourceRefs, DeclOrder) — event fields (History, GateSignoffs,
// LastReviewedAt, ReviewAfter, Evidence) are deliberately absent, passed
// through untouched by MergeIntoGraph.
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

// Requirements is the Phase A full-coverage registry: it holds all 301
// requirement IDs from domains/hotam-spec-self/graph.json, registered across
// the requirements_<topic>.go files in this package. A registered ID that is
// absent from the graph is a MergeIntoGraph error (see merge.go) — this
// package never creates graph nodes, only mirrors existing ones.
var Requirements = registry.New[ontology.Requirement]()
