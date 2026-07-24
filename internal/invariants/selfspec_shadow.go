// selfspec_shadow.go holds check_self_requirements_match_registry: a BOAT
// invariant (registered in All, gates `hotam all-violations`/the proposal
// gate exactly like any other framework-scoped check) that
// internal/selfspec.Requirements (the Go registry mirror of
// domains/hotam-spec-self/graph.json's Requirement nodes — see that
// package's own doc comment) is byte-for-byte structurally current against
// the graph it mirrors.
//
// FORMERLY SHADOW (task #345/RAC-A), NOW A GATE (task #351/RAC-B4): from
// task #345 through task #350 this check was deliberately NOT registered via
// All.MustRegister — the registry was a MIRROR, not yet the authority, so a
// registry/graph mismatch could only mean the registry had gone stale, never
// that the graph itself was wrong, and promoting it to a blocking gate then
// would have misrepresented that boundary. RAC-B (task #346) flips that
// boundary in four steps: RAC-B1 (task #348) built the sync primitives
// (SyncGraph/StructuralFieldDiffs/VerifyAppendOnly), RAC-B2 (task #349) built
// `hotam sync-self` — the one sanctioned path that projects the registry ONTO
// the graph — RAC-B3 (task #350) closed `apply-proposal`/`land` to
// Requirement/Rejection edits on self-hosting domains (so `hotam sync-self`
// became the ONLY way to change a Requirement's structural fields in
// hotam-spec-self), and this step (RAC-B4) is the load-bearing consequence:
// now that a real, gated, append-only-verified write path exists, a
// registry/graph mismatch is real, actionable drift — either someone landed
// a hand-edit RAC-B3 didn't catch, or `hotam sync-self` was never re-run
// after a registry edit — and CI should say so, not stay silent. Both the
// exported wrapper (SelfRequirementsMatchRegistryWarnings, still used
// directly by internal/selfspec_test.go-style callers and this file's own
// tests) and the All-registered entry point below call the exact same
// checkSelfRequirementsMatchRegistry function, so there is exactly one
// comparison, never two that could disagree.
//
// WHY SelfHosting-GATED, DOUBLY (both an internal `!g.SelfHosting → nil`
// early-return AND an entry in frameworkScopedInvariantNames,
// all_violations.go): internal/selfspec.Requirements mirrors ONE graph —
// THIS repo's own domains/hotam-spec-self/graph.json — by construction (see
// .scratch/selfspec-codegen, the throwaway generator that produced
// requirements_<topic>.go from that exact file). Running this comparison
// against any OTHER domain (a consumer's domains/prat, domains/gpsm-sm, or
// even this repo's own domains/hotam-dev, which does not set
// self_hosting=true — see domains/hotam-dev/manifest.json) would report
// every one of the registry's 301+ IDs as "registered but absent from this
// graph" — not a real signal, pure noise from comparing two unrelated
// domains' requirement sets. g.SelfHosting is exactly the field that
// answers "does this graph claim to BE the framework modeling itself" (see
// R-critical-core-per-domain's sibling gate in all_violations.go's own doc
// comment), so gating on it here reuses that established boundary rather
// than inventing a new one. The internal early-return keeps the function
// itself an honest no-op even if called directly (as the exported wrapper
// still allows); the frameworkScopedInvariantNames entry is belt-and-braces
// so the All-registered fan-out never even calls Check for a non-self-hosting
// graph — mirroring check_bijection_r_to_enforcer's identical double-gated
// posture (self_reference.go), the requirement this very check is now
// enforced_by for (R-no-hand-edit-graph, see requirements_deterministic.go).
package invariants

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/selfspec"
)

// SelfRequirementsMatchRegistryWarnings is a thin exported wrapper around
// checkSelfRequirementsMatchRegistry, kept for callers (including this
// file's own tests) that want the check's result without going through the
// All registry's fan-out — the identical exported-wrapper shape
// HonoredSkipWarnings/AuthoredProseSnapshotWarnings still use for their own
// (still-shadow) advisory checks. Unlike those two, this check's PRIMARY
// entry point is now the All-registered Invariant below — this wrapper is a
// convenience, not the only way the check runs.
func SelfRequirementsMatchRegistryWarnings(g *ontology.Graph) []Violation {
	return checkSelfRequirementsMatchRegistry(g)
}

// checkSelfRequirementsMatchRegistry is the comparison itself: an honest
// no-op for any graph that is not g.SelfHosting (see package doc comment),
// and otherwise a three-way diff between internal/selfspec.Requirements and
// g.Requirements:
//
//  1. every registered ID absent from the graph (selfspec.MergeIntoGraph
//     would hard-error on this — surfaced here as an advisory instead, so a
//     shadow run never panics/exits the whole all-violations pass over a
//     registry/graph mismatch that a blocking check would make fatal),
//  2. every graph Requirement ID NOT in the registry (real, uncontroversial
//     coverage debt today; task #345 registers all 301 known IDs, so a
//     non-empty result here means a NEW Requirement landed in graph.json
//     since the registry was last regenerated),
//  3. for every ID present in BOTH, a structural-field mismatch (the exact
//     fields selfspec.MergeIntoGraph replaces — see that function's own doc
//     comment for the authoritative field list) between the registry's
//     value and the graph's current value — the registry's own copy going
//     stale relative to a hand-edited (or proposal-landed) graph.json.
func checkSelfRequirementsMatchRegistry(g *ontology.Graph) []Violation {
	if g == nil || !g.SelfHosting {
		return nil
	}

	inGraph := make(map[string]ontology.Requirement, len(g.Requirements))
	for _, r := range g.Requirements {
		inGraph[r.ID] = r
	}

	registered := selfspec.Requirements.All()
	inRegistry := make(map[string]ontology.Requirement, len(registered))
	for _, r := range registered {
		inRegistry[r.ID] = r
	}

	var out []Violation

	var missingFromGraph []string
	for id := range inRegistry {
		if _, ok := inGraph[id]; !ok {
			missingFromGraph = append(missingFromGraph, id)
		}
	}
	sort.Strings(missingFromGraph)
	for _, id := range missingFromGraph {
		out = append(out, Violation{
			Check:   "check_self_requirements_match_registry",
			ID:      id,
			Message: "registered in internal/selfspec.Requirements but absent from domains/hotam-spec-self/graph.json — the registry is stale (mirrors a node that no longer exists, or was renamed) and needs regeneration",
		})
	}

	var missingFromRegistry []string
	for id := range inGraph {
		if _, ok := inRegistry[id]; !ok {
			missingFromRegistry = append(missingFromRegistry, id)
		}
	}
	sort.Strings(missingFromRegistry)
	for _, id := range missingFromRegistry {
		out = append(out, Violation{
			Check:   "check_self_requirements_match_registry",
			ID:      id,
			Message: "present in domains/hotam-spec-self/graph.json but not registered in internal/selfspec.Requirements — a new Requirement landed since the registry was last regenerated (see .scratch/selfspec-codegen's throwaway generator, task #345/RAC-A)",
		})
	}

	var common []string
	for id := range inRegistry {
		if _, ok := inGraph[id]; ok {
			common = append(common, id)
		}
	}
	sort.Strings(common)
	for _, id := range common {
		reg := inRegistry[id]
		graphReq := inGraph[id]
		if diff := firstStructuralFieldDiff(reg, graphReq); diff != "" {
			out = append(out, Violation{
				Check:   "check_self_requirements_match_registry",
				ID:      id,
				Message: fmt.Sprintf("internal/selfspec.Requirements structural fields disagree with domains/hotam-spec-self/graph.json: %s — the registry is stale and needs regeneration", diff),
			})
		}
	}

	return out
}

// firstStructuralFieldDiff compares EXACTLY the structural fields
// selfspec.MergeIntoGraph replaces (see that function's own doc comment for
// the authoritative list — Claim, Owner, Status, Why, Assumptions,
// Relations, Enforcement, EnforcedBy, MTag, Enforceability, Summary,
// CreatedAt, SettledAt, SourceRefs, DeclOrder, BlockedOn, ImplementedBy,
// VerifiedBy) between reg (a registry entry) and graphReq (the graph's
// current node with the SAME ID), deliberately IGNORING the event fields
// (History, GateSignoffs, LastReviewedAt, ReviewAfter, Evidence) that
// MergeIntoGraph passes through untouched — those legitimately differ
// between two Requirement values that are otherwise a perfect structural
// match, so comparing them here would produce permanent, meaningless noise.
// Returns a human-readable "field: registry=... graph=..." description of
// the FIRST field that differs (empty string if none), not an exhaustive
// list — the advisory message points a reader at the regeneration action,
// not a full diff.
func firstStructuralFieldDiff(reg, graphReq ontology.Requirement) string {
	type fieldCheck struct {
		name       string
		regValue   any
		graphValue any
	}
	checks := []fieldCheck{
		{"claim", reg.Claim, graphReq.Claim},
		{"owner", reg.Owner, graphReq.Owner},
		{"status", reg.Status, graphReq.Status},
		{"why", reg.Why, graphReq.Why},
		{"assumptions", reg.Assumptions, graphReq.Assumptions},
		{"relations", reg.Relations, graphReq.Relations},
		{"enforcement", reg.Enforcement, graphReq.Enforcement},
		{"enforced_by", reg.EnforcedBy, graphReq.EnforcedBy},
		{"m_tag", reg.MTag, graphReq.MTag},
		{"enforceability", reg.Enforceability, graphReq.Enforceability},
		{"summary", reg.Summary, graphReq.Summary},
		{"created_at", reg.CreatedAt, graphReq.CreatedAt},
		{"settled_at", reg.SettledAt, graphReq.SettledAt},
		{"source_refs", reg.SourceRefs, graphReq.SourceRefs},
		{"decl_order", reg.DeclOrder, graphReq.DeclOrder},
		{"blocked_on", reg.BlockedOn, graphReq.BlockedOn},
		{"implemented_by", reg.ImplementedBy, graphReq.ImplementedBy},
		{"verified_by", reg.VerifiedBy, graphReq.VerifiedBy},
	}
	for _, c := range checks {
		if !reflect.DeepEqual(c.regValue, c.graphValue) {
			return fmt.Sprintf("%s: registry=%v graph=%v", c.name, c.regValue, c.graphValue)
		}
	}
	return ""
}

var _ = All.MustRegister("check_self_requirements_match_registry", Invariant{
	Name:  "check_self_requirements_match_registry",
	Canon: methodology.Invariants,
	Claim: "for the self-hosting domain (g.SelfHosting), internal/selfspec.Requirements is structurally byte-identical to g.Requirements — no ID missing from either side, no differing structural field.",
	Rule: "an honest no-op for any graph where g.SelfHosting is false (see this file's package doc comment for why: the registry mirrors exactly ONE graph, this repo's own domains/hotam-spec-self/graph.json, by construction). " +
		"For a SelfHosting graph, three sub-checks: (1) every ID registered in internal/selfspec.Requirements MUST exist in g.Requirements — a registered ID absent from the graph means the registry mirrors a node that " +
		"no longer exists or was renamed; (2) every Requirement ID in g.Requirements MUST be registered in internal/selfspec.Requirements — a graph node absent from the registry means a new Requirement landed (via " +
		"`hotam sync-self`, the only sanctioned path for a self-hosting domain's Requirement/Rejection structural fields — R-no-hand-edit-graph) without the registry being updated to match; (3) for every ID present in " +
		"BOTH, the structural fields selfspec.MergeIntoGraph/StructuralFieldDiffs replace wholesale (Claim, Owner, Status, Why, Assumptions, Relations, Enforcement, EnforcedBy, MTag, Enforceability, Summary, CreatedAt, " +
		"SettledAt, SourceRefs, DeclOrder, BlockedOn, ImplementedBy, VerifiedBy) MUST agree byte-for-byte between the registry and the graph. Event fields (History, GateSignoffs, LastReviewedAt, ReviewAfter, Evidence) are " +
		"deliberately excluded — they legitimately differ between two otherwise-identical values and comparing them would be permanent, meaningless noise.",
	Why: "task #345 (RAC-A) registered this exact comparison as a SHADOW check — advisory-only, deliberately never gating — because at that point the registry was a MIRROR, not yet the authority: a mismatch could only " +
		"mean the registry had gone stale, and there was no sanctioned write path that could make the mismatch mean anything more actionable. RAC-B (task #346) built that path across three prior steps: RAC-B1 (task #348) " +
		"built the sync primitives (SyncGraph/StructuralFieldDiffs/VerifyAppendOnly), RAC-B2 (task #349) built `hotam sync-self` — the command that projects the registry ONTO the graph through the same confront/pre-post- " +
		"violation/append-only gate sequence `hotam land` uses — and RAC-B3 (task #350) closed `apply-proposal`/`land` to Requirement/Rejection structural edits on self-hosting domains, making `hotam sync-self` the ONLY " +
		"sanctioned path for such an edit. With that path in place, a registry/graph mismatch is no longer ambiguous staleness — it is either an edit that bypassed `hotam sync-self` (a hand-edit RAC-B3's gate didn't catch, " +
		"or a direct graph.json write) or a registry edit nobody synced yet, and CI should say so rather than stay silent. Doubly self-hosting-gated (an internal early-return here AND an entry in " +
		"frameworkScopedInvariantNames, all_violations.go) for the same reason RAC-A's shadow version was: the registry mirrors ONE graph by construction, so running this comparison against any other domain would report " +
		"every registered ID as spurious noise, not a real signal. This check is itself the mechanism R-no-hand-edit-graph's Claim/Why now describe: it is what makes a hand-edit to domains/hotam-spec-self/graph.json's " +
		"Requirement nodes mechanically detectable, closing the orphan-enforcer gap check_bijection_r_to_enforcer would otherwise flag (a registered check_* function named by no SETTLED/ENFORCED requirement's enforced_by).",
	Check: checkSelfRequirementsMatchRegistry,
})
