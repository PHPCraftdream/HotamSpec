// Package graphfacts holds LIVE graph-fact readers: small, pure functions
// that tally a current, computed truth off *ontology.Graph — as opposed to a
// static keyword string frozen at authoring time. It exists to close a real
// gap the Orientation-FAQ invariant exposed (task #321/R3-semantic-faq): a
// manifest FAQ entry's keyword phrase (e.g. "27 of 32 requirements") can stay
// lexically PRESENT in the crystal long after the graph itself moved past
// that number, and the keyword-only check has no way to know the phrase went
// stale — it only proves the phrase is a SUBSTRING of the crystal, never that
// the phrase is still semantically TRUE.
//
// This package imports ONLY internal/ontology (a leaf package with no
// project-internal imports of its own). It is deliberately placed OUTSIDE
// internal/query: internal/query is a PERIPHERY consumer package
// (internal/selfcheck/imports_test.go's peripheryConsumers set —
// R-core-periphery-import-ratchet), and internal/invariants is CORE
// (corePackages) — the core/periphery dependency arrow must point one way
// only (consumers depend on core, never the reverse), so a core package
// (invariants) may never import a periphery package (query). graphfacts is
// instead "shared low-level machinery" like gate/methodology/registry/paths
// (see peripheryConsumers' own doc comment): it is in neither set, so both
// internal/invariants (core) and internal/generator (periphery) may import
// it freely, and it may never itself import either.
package graphfacts

import (
	"sort"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// GateTally is a live SIGNED/DEFERRED count pair for one gate stage.
type GateTally struct {
	Signed   int
	Deferred int
}

// lastSignoffAtStage computes, for every Requirement in g (by index) and
// every stage name known to stageIndex, the LAST (highest-array-index, i.e.
// most recently appended — GateSignoffBatch.mutate only ever appends, see
// internal/proposal/mutate.go) GateSignoff.State recorded for that
// Requirement at that stage. A single Requirement can carry BOTH a
// superseded DEFERRED entry and a later SIGNED entry for the SAME stage
// (e.g. an early deferral once its blocking Conflict resolved) — counting
// every entry would double-count that Requirement at the stage; only the
// LAST entry reflects the Requirement's CURRENT gate state at that stage.
// This is the exact dedup rule internal/generator/claudemd.go's
// renderDomainMapBlockWithViolations computed inline before this package
// existed — extracted here verbatim, not reimplemented, so a subtle
// double-counting bug cannot creep in via divergent reimplementations.
//
// run, when non-empty, restricts every considered GateSignoff to those whose
// PipelineRun exactly equals run — the dedup rule then applies WITHIN that
// one run only, never mixing entries across distinct pipeline_run values. An
// empty run considers every GateSignoff regardless of PipelineRun — the
// exact pre-existing (task #321-era) behavior, byte-identical, preserved as
// the default for every call site that has not opted into run-scoping.
//
// frontierIdx is the highest stage index (into order) at which ANY
// Requirement carries a recorded GateSignoff (matching run, when declared) —
// the "furthest stage the pipeline has touched so far" — or -1 if no
// Requirement carries any matching GateSignoff whose Stage is known to order
// at all.
func lastSignoffAtStage(g *ontology.Graph, order []string, run string) (last map[reqStage]string, frontierIdx int) {
	stageIndex := make(map[string]int, len(order))
	for i, s := range order {
		stageIndex[s] = i
	}
	last = make(map[reqStage]string)
	frontierIdx = -1
	for ri, r := range g.Requirements {
		for _, gs := range r.GateSignoffs {
			if run != "" && gs.PipelineRun != run {
				continue
			}
			idx, known := stageIndex[gs.Stage]
			if !known {
				continue
			}
			if idx > frontierIdx {
				frontierIdx = idx
			}
			last[reqStage{reqIdx: ri, stage: gs.Stage}] = gs.State
		}
	}
	return last, frontierIdx
}

// reqStage is the composite key lastSignoffAtStage dedups on: one
// Requirement (by index into g.Requirements), one gate stage name.
type reqStage struct {
	reqIdx int
	stage  string
}

// GateSignoffTally returns the live SIGNED/DEFERRED tally for stage,
// counting each Requirement in g AT MOST ONCE (its LAST recorded
// GateSignoff.State at that stage — see lastSignoffAtStage's doc comment for
// the dedup rule and why it matters). order is the domain's declared
// gate_stage_order (loader.ResolveGateStageOrder) — a stage name not present
// in order is simply never matched by any Requirement's GateSignoff.Stage,
// so it tallies (0, 0), the honest empty result for an unknown/undeclared
// stage.
//
// run, when non-empty, restricts the tally to GateSignoffs whose PipelineRun
// exactly equals run — see lastSignoffAtStage's doc comment. An empty run
// tallies across ALL pipeline runs, the exact pre-existing behavior every
// call site predating task #330's multi-run guard relies on
// (internal/generator/claudemd.go's DOMAIN-MAP renderer and
// internal/generator/pipeline.go's Live-state renderer both pass "" here,
// unchanged).
func GateSignoffTally(g *ontology.Graph, order []string, stage, run string) GateTally {
	last, _ := lastSignoffAtStage(g, order, run)
	var t GateTally
	for ri := range g.Requirements {
		switch last[reqStage{reqIdx: ri, stage: stage}] {
		case ontology.GateSignoffStateSigned:
			t.Signed++
		case ontology.GateSignoffStateDeferred:
			t.Deferred++
		}
	}
	return t
}

// PipelineRunsAtStage returns the distinct GateSignoff.PipelineRun values
// among every Requirement in g whose GateSignoff.Stage equals stage (Stage
// need not be known to order — this reader answers "which runs touched this
// stage name at all", the raw signal the multi-run ambiguity guard in
// internal/invariants/orientation_faq_assert.go needs BEFORE it can even
// decide whether order/stage validation is the relevant failure). Results
// are sorted for deterministic, reproducible output (gen-spec/violation
// output must never depend on map iteration order). An empty/nil order does
// not affect this reader — it scans every GateSignoff on every Requirement
// directly by Stage string match, independent of any declared stage
// vocabulary.
func PipelineRunsAtStage(g *ontology.Graph, order []string, stage string) []string {
	_ = order // stage is matched directly; order is accepted for call-site symmetry with GateSignoffTally/GateFrontier, not consulted.
	seen := make(map[string]struct{})
	for _, r := range g.Requirements {
		for _, gs := range r.GateSignoffs {
			if gs.Stage != stage {
				continue
			}
			seen[gs.PipelineRun] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for run := range seen {
		out = append(out, run)
	}
	sort.Strings(out)
	return out
}

// CohortCount returns the count of Requirements in g for which member(r) is
// true — a trivial counted filter, extracted here (rather than inlined at
// each call site) so a "which requirements count toward a declared cohort"
// predicate has exactly ONE implementation, matching this package's own
// established "extracted so divergent reimplementations can't creep"
// discipline (see this file's package doc comment, and lastSignoffAtStage's
// doc comment for the identical rationale applied to the gate-tally dedup
// rule).
func CohortCount(g *ontology.Graph, member func(r ontology.Requirement) bool) int {
	count := 0
	for _, r := range g.Requirements {
		if member(r) {
			count++
		}
	}
	return count
}

// GateFrontier returns the FRONTIER gate stage — the furthest stage (by
// position in order) any Requirement in g has recorded a GateSignoff for —
// plus that stage's live SIGNED/DEFERRED tally. ok is false when no
// Requirement carries any GateSignoff whose Stage is known to order (order
// itself empty/nil, or no GateSignoff matches any declared stage), mirroring
// the honest no-op every sibling gate-aware reader in this codebase already
// establishes for "this domain has no staged-gate methodology, or has never
// recorded a single matching GateSignoff yet."
func GateFrontier(g *ontology.Graph, order []string) (stage string, t GateTally, ok bool) {
	last, frontierIdx := lastSignoffAtStage(g, order, "")
	if frontierIdx < 0 {
		return "", GateTally{}, false
	}
	stage = order[frontierIdx]
	for ri := range g.Requirements {
		switch last[reqStage{reqIdx: ri, stage: stage}] {
		case ontology.GateSignoffStateSigned:
			t.Signed++
		case ontology.GateSignoffStateDeferred:
			t.Deferred++
		}
	}
	return stage, t, true
}

// ConflictLifecycleTally returns the live count of g.Conflicts whose
// lifecycle_class predicate matches class ("DECIDED", "HELD", or
// "UNRESOLVED" — mirroring ontology.Conflict.IsDecided/IsHeld/IsUnresolved),
// plus total = len(g.Conflicts). An unrecognized class returns (0, total,
// err) — a fail-closed signal for the caller (internal/invariants'
// evalOrientationAssert) to surface as a violation rather than silently
// tallying zero for a typo'd class name.
func ConflictLifecycleTally(g *ontology.Graph, class string) (count, total int, err error) {
	total = len(g.Conflicts)
	switch class {
	case "DECIDED":
		for _, c := range g.Conflicts {
			if c.IsDecided() {
				count++
			}
		}
	case "HELD":
		for _, c := range g.Conflicts {
			if c.IsHeld() {
				count++
			}
		}
	case "UNRESOLVED":
		for _, c := range g.Conflicts {
			if c.IsUnresolved() {
				count++
			}
		}
	default:
		return 0, total, &UnknownPredicateError{Kind: "lifecycle_class", Value: class}
	}
	return count, total, nil
}

// RequirementStatusTally returns the live count of g.Requirements matching
// BOTH status (one of ontology.StatusDRAFT/StatusSETTLED/StatusREJECTED, or
// the ontology.StatusOPENPrefix family checked via IsOpen — see below) and,
// when enforcement is non-empty, the requirement's Enforcement level (one of
// ontology.EnforcementPROSE/STRUCTURAL/ENFORCED) — plus total =
// len(g.Requirements). An empty enforcement means "match on status alone,
// any enforcement level." An unrecognized status or enforcement value
// returns (0, total, err) — fail-closed, mirroring
// ConflictLifecycleTally's identical contract.
//
// status == ontology.StatusOPENPrefix ("OPEN") matches via r.IsOpen()
// (prefix match, since OPEN carries suffixed variants like "OPEN_BLOCKED" in
// this graph's vocabulary) rather than an exact string comparison, the same
// distinction ontology.Requirement.IsOpen() itself already draws.
func RequirementStatusTally(g *ontology.Graph, status, enforcement string) (count, total int, err error) {
	total = len(g.Requirements)
	if status != ontology.StatusDRAFT && status != ontology.StatusSETTLED &&
		status != ontology.StatusREJECTED && status != ontology.StatusOPENPrefix {
		return 0, total, &UnknownPredicateError{Kind: "status", Value: status}
	}
	if enforcement != "" {
		if _, known := ontology.EnforcementLevels[enforcement]; !known {
			return 0, total, &UnknownPredicateError{Kind: "enforcement", Value: enforcement}
		}
	}
	for _, r := range g.Requirements {
		statusMatch := false
		if status == ontology.StatusOPENPrefix {
			statusMatch = r.IsOpen()
		} else {
			statusMatch = r.Status == status
		}
		if !statusMatch {
			continue
		}
		if enforcement != "" && r.Enforcement != enforcement {
			continue
		}
		count++
	}
	return count, total, nil
}

// UnknownPredicateError reports an unrecognized predicate value passed to a
// tally function (an unknown lifecycle_class, status, or enforcement) — the
// fail-closed signal internal/invariants' evalOrientationAssert turns into a
// violation naming the offending FAQ entry, rather than a tally silently
// returning zero for a typo.
type UnknownPredicateError struct {
	Kind  string // "lifecycle_class", "status", or "enforcement"
	Value string
}

func (e *UnknownPredicateError) Error() string {
	return "graphfacts: unknown " + e.Kind + " value " + quote(e.Value)
}

func quote(s string) string {
	return "\"" + s + "\""
}
