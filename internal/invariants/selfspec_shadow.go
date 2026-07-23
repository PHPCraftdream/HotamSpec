// selfspec_shadow.go holds check_self_requirements_match_registry (task
// #345, RAC-A): a SHADOW-MODE check that internal/selfspec.Requirements (the
// Go registry mirror of domains/hotam-spec-self/graph.json's Requirement
// nodes — see that package's own doc comment) is byte-for-byte structurally
// current against the graph it mirrors.
//
// SHADOW, NEVER A GATE: deliberately NOT registered via All.MustRegister, so
// it never appears in invariants.AllViolations, never blocks `hotam
// all-violations`'s exit code, and never blocks internal/proposal/apply.go's
// proposal gate. It mirrors HonoredSkipWarnings (authored_links.go) and
// AuthoredProseSnapshotWarnings (authored_prose_snapshot.go) — a plain
// function returning []Violation that lives in this package for its shared
// machinery (the Violation shape, the graph types) but is wired into
// cmd/hotam's non-blocking ADVISORY section (all_violations.go's
// printAdvisorySection) by name, exactly the same "advisory band" those two
// checks already establish (see printAdvisorySection's own doc comment).
//
// WHY SHADOW AND NOT A GATE (task #345's explicit brief): the registry is a
// MIRROR, not yet the authority — flipping authority (a `hotam sync-self`
// command that writes graph.json FROM the registry, making a registry/graph
// mismatch a real, actionable drift) is task #346 (RAC-B), deliberately out
// of this phase's scope. Registering this as a blocking All invariant today
// would misrepresent that boundary: it would make internal/selfspec's own
// staleness (a registry someone forgot to regenerate after a graph.json
// edit) into a hard CI failure for every consumer domain and for this
// engine's own repo, when the actual authority (graph.json) may be
// perfectly fine — the registry is what is stale, not the graph. SHADOW
// mode surfaces that staleness as a visible, non-blocking signal instead.
//
// WHY SelfHosting-GATED (mirrors frameworkScopedInvariantNames' identical
// posture in all_violations.go, applied here via a direct field check rather
// than that map — this check is not a candidate() member of the ordinary
// All-registry fan-out, so it has no natural home in that specific map):
// internal/selfspec.Requirements mirrors ONE graph — THIS repo's own
// domains/hotam-spec-self/graph.json — by construction (see
// .scratch/selfspec-codegen, the throwaway generator that produced
// requirements_<topic>.go from that exact file). Running this comparison
// against any OTHER domain (a consumer's domains/prat, domains/gpsm-sm, or
// even this repo's own domains/hotam-dev, which does not set
// self_hosting=true — see domains/hotam-dev/manifest.json) would report
// every one of the registry's 301 IDs as "registered but absent from this
// graph" — not a real signal, pure noise from comparing two unrelated
// domains' requirement sets. g.SelfHosting is exactly the field that
// answers "does this graph claim to BE the framework modeling itself" (see
// R-critical-core-per-domain's sibling gate in all_violations.go's own doc
// comment), so gating on it here reuses that established boundary rather
// than inventing a new one.
package invariants

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/selfspec"
)

// SelfRequirementsMatchRegistryWarnings is the exported entry point
// cmd/hotam's printAdvisorySection (all_violations.go) calls directly by
// name — mirrors HonoredSkipWarnings/AuthoredProseSnapshotWarnings' identical
// exported-wrapper shape, since this check is never registered into the All
// registry (see this file's package doc comment for why).
func SelfRequirementsMatchRegistryWarnings(g *ontology.Graph) []Violation {
	return checkSelfRequirementsMatchRegistry(g)
}

// checkSelfRequirementsMatchRegistry is the SHADOW comparison itself: an
// honest no-op for any graph that is not g.SelfHosting (see package doc
// comment), and otherwise a three-way diff between
// internal/selfspec.Requirements and g.Requirements:
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
