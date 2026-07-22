// gate_signoff_checks.go holds the invariants for
// ontology.Requirement.GateSignoffs, the single typed carrier for
// per-Requirement gate-passage facts (see internal/ontology/gate_signoff.go).
//
// HONEST NO-OP (the same shape every sibling opt-in check already
// establishes): a domain whose manifest.json declares NO "gate_stage_order"
// field contributes ZERO violations from check_gate_signoff_monotonic
// regardless of how many GateSignoff entries its requirements carry —
// exactly like check_orientation_faq_answered (no orientation_faq list = no
// orientability obligation) and check_settled_requires_scenario (no
// discipline:"full" = no scenario obligation). Every other check in this
// file (check_gate_signoff_deferred_reason_present,
// check_gate_signoff_deferred_conflict_resolves,
// check_gate_signoff_signed_has_provenance,
// check_gate_signoff_decided_by_is_known_stakeholder) is NOT gated on
// gate_stage_order (they police GateSignoff shape, not stage order) but are
// themselves a no-op whenever no requirement in the graph carries any
// GateSignoffs at all — the same "opt in by using the field" honesty the
// rest of this package's optional-field checks share.
//
// check_gate_signoff_signed_has_provenance and
// check_gate_signoff_decided_by_is_known_stakeholder (task #319,
// R3-signoff-strict) are deliberately ONGOING all-violations invariants,
// not proposal-time-only checks — mirroring
// check_gate_signoff_deferred_reason_present's own choice to run
// unconditionally over the graph rather than only at
// ProposedGateSignoffBatch.validate() time. This is safe against existing
// landed data: prat/gpsm-sm's 64 SIGNED gate_signoffs (P-G1, task #312/#318)
// already carry a full signoff (decided_by/verbatim) and non-empty evidence
// on every entry, and every decided_by used ("gpsm-pm", "marat-karamullin")
// resolves to a real Stakeholder in that domain's graph — verified by
// direct inspection of domains/gpsm-sm/graph.json in a sibling repo before
// this check was wired up as ongoing rather than proposal-time-only.
package invariants

import (
	"fmt"
	"path/filepath"
	"regexp"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// checkGateSignoffMonotonic is the ordering gate: within one Requirement's
// GateSignoffs, restricted to one PipelineRun, every SIGNED entry's Stage
// must appear in the domain-declared gate_stage_order at an index no
// greater than 1 + the highest index already SIGNED in that same run — i.e.
// a requirement cannot be SIGNED at stage N in a run without every earlier
// stage (index < N) ALSO being SIGNED in that same run. Honest no-op when
// the domain has not declared gate_stage_order (loader.
// ResolveGateStageOrder returns nil) — see the package doc comment.
func checkGateSignoffMonotonic(g *ontology.Graph) []Violation {
	if g.DomainDir == "" {
		// No on-disk domain (an in-memory fixture graph built without
		// loader.LoadGraph) — honest no-op, mirroring
		// check_orientation_faq_answered's identical DomainDir guard.
		return nil
	}
	order := loader.ResolveGateStageOrder(filepath.Join(g.DomainDir, "graph.json"))
	if len(order) == 0 {
		// This domain has not opted into the gate-stage discipline — honest
		// no-op ("no committed opt-in = no lie").
		return nil
	}
	stageIndex := make(map[string]int, len(order))
	for i, s := range order {
		stageIndex[s] = i
	}

	var out []Violation
	for _, r := range g.Requirements {
		if len(r.GateSignoffs) == 0 {
			continue
		}
		// Group this requirement's SIGNED entries by PipelineRun, preserving
		// declaration order within each run.
		byRun := map[string][]ontology.GateSignoff{}
		var runOrder []string
		for _, gs := range r.GateSignoffs {
			if gs.State != ontology.GateSignoffStateSigned {
				continue
			}
			if _, seen := byRun[gs.PipelineRun]; !seen {
				runOrder = append(runOrder, gs.PipelineRun)
			}
			byRun[gs.PipelineRun] = append(byRun[gs.PipelineRun], gs)
		}
		for _, run := range runOrder {
			signedIdx := map[int]struct{}{}
			for _, gs := range byRun[run] {
				idx, known := stageIndex[gs.Stage]
				if !known {
					// A stage not in the declared order cannot be checked
					// for monotonicity — that is a separate malformed-data
					// concern this check does not police (a domain that
					// declares gate_stage_order but signs off on an
					// undeclared stage name is a typo, not an ordering
					// violation; report it distinctly so the message stays
					// actionable).
					out = append(out, Violation{
						Check: "check_gate_signoff_monotonic",
						ID:    r.ID,
						Message: fmt.Sprintf(
							"requirement %s has a SIGNED gate_signoff for stage %q (pipeline_run %q) which is not "+
								"declared in this domain's manifest.json gate_stage_order %v",
							r.ID, gs.Stage, run, order),
					})
					continue
				}
				signedIdx[idx] = struct{}{}
			}
			for idx := range signedIdx {
				for earlier := 0; earlier < idx; earlier++ {
					if _, ok := signedIdx[earlier]; !ok {
						out = append(out, Violation{
							Check: "check_gate_signoff_monotonic",
							ID:    r.ID,
							Message: fmt.Sprintf(
								"requirement %s is SIGNED at stage %q (index %d, pipeline_run %q) but stage %q "+
									"(index %d) is NOT SIGNED in the same pipeline_run — gate_stage_order %v requires "+
									"every earlier stage SIGNED first",
								r.ID, order[idx], idx, run, order[earlier], earlier, order),
						})
					}
				}
			}
		}
	}
	return out
}

var _ = All.MustRegister("check_gate_signoff_monotonic", Invariant{
	Name:  "check_gate_signoff_monotonic",
	Canon: methodology.Requirement,
	Claim: "within one Requirement and one pipeline_run, SIGNED gate_signoffs appear in the domain-declared gate_stage_order — no stage SIGNED before every earlier stage is SIGNED too.",
	Rule: "IF a domain's manifest.json declares a non-empty \"gate_stage_order\" list (loader.ResolveGateStageOrder), THEN " +
		"for every Requirement.gate_signoffs entry with state=SIGNED, grouped by pipeline_run, the SET of SIGNED stage " +
		"indices (looked up in gate_stage_order) within one pipeline_run MUST be a prefix-closed set: if stage at index N " +
		"is SIGNED, every stage at index < N MUST ALSO be SIGNED in that SAME pipeline_run. A SIGNED entry naming a stage " +
		"not present in gate_stage_order fires its own violation (undeclared stage, not an ordering defect). A domain with " +
		"no gate_stage_order declared is a pure HONEST NO-OP — zero violations regardless of what any requirement's " +
		"gate_signoffs contain.",
	Why: "gate_stage_order is domain DATA, never an engine-known enum (the engine serves domains with entirely different " +
		"or absent staged-gate methodologies — hotam-spec-self, hotam-dev — alongside a domain like prat/gpsm-sm that runs " +
		"P-G0..P-G4). Declaring the order is the domain's own opt-in; once declared, this check makes 'gate passage is " +
		"monotonic' a mechanically verified property of the graph rather than a convention a resolver must remember to " +
		"honor by hand across dozens of Requirements. pipeline_run scoping matters because a re-run of the pipeline is a " +
		"fresh attempt — a requirement SIGNED through P-G2 in an earlier abandoned run does not retroactively justify " +
		"SIGNED at P-G3 in a brand-new run that never re-passed P-G0/P-G1/P-G2.",
	Check: checkGateSignoffMonotonic,
})

// checkGateSignoffDeferredReasonPresent requires a non-empty DeferredReason
// on every GateSignoff with State=DEFERRED — a deferral with no recorded
// reason is drift, not a decision (mirrors ProposedAssumptionTransition's
// identical requirement for its own Reason field). Unconditional (not gated
// on gate_stage_order): this is a shape check on the GateSignoff payload
// itself, independent of whether the domain has declared a stage order.
func checkGateSignoffDeferredReasonPresent(g *ontology.Graph) []Violation {
	var out []Violation
	for _, r := range g.Requirements {
		for _, gs := range r.GateSignoffs {
			if gs.State == ontology.GateSignoffStateDeferred && gs.DeferredReason == "" {
				out = append(out, Violation{
					Check: "check_gate_signoff_deferred_reason_present",
					ID:    r.ID,
					Message: fmt.Sprintf(
						"requirement %s has a DEFERRED gate_signoff for stage %q (pipeline_run %q) with an empty "+
							"deferred_reason — a deferral with no recorded reason is drift, not a decision",
						r.ID, gs.Stage, gs.PipelineRun),
				})
			}
		}
	}
	return out
}

var _ = All.MustRegister("check_gate_signoff_deferred_reason_present", Invariant{
	Name:  "check_gate_signoff_deferred_reason_present",
	Canon: methodology.Requirement,
	Claim: "every GateSignoff with state=DEFERRED carries a non-empty deferred_reason.",
	Rule: "for every Requirement.gate_signoffs[*] entry with state==\"DEFERRED\", deferred_reason MUST be non-empty. No-ops " +
		"trivially when no requirement carries any gate_signoffs.",
	Why: "a DEFERRED gate passage with no recorded reason is indistinguishable from an unexplained silent gap — the same " +
		"discipline ProposedAssumptionTransition.validate already enforces for Assumption status transitions " +
		"('reason' is required and must be non-empty — an assumption status change with no recorded reason is drift, not " +
		"a decision'). Applying the identical rule to GateSignoff keeps the two record-a-decision shapes consistent.",
	Check: checkGateSignoffDeferredReasonPresent,
})

// checkGateSignoffSignedHasProvenance requires a SIGNED GateSignoff to carry
// a populated Signoff with non-empty DecidedBy/Verbatim, plus a non-empty
// Evidence list on the GateSignoff itself (task #319, R3-signoff-strict) — a
// SIGNED gate passage records a genuine human decision (WHO decided, WHAT
// they said, WHY it is trustworthy), and before this check the engine
// allowed a SIGNED record with none of that: only Stage/State/PipelineRun
// were required. Mirrors checkGateSignoffDeferredReasonPresent's shape
// exactly (unconditional, not gated on gate_stage_order — this polices the
// GateSignoff payload itself, independent of whether the domain has
// declared a stage order) and is ALSO enforced at proposal-validate() time
// (ProposedGateSignoffBatch.validate in internal/proposal/validate.go) so a
// malformed batch is rejected before it ever lands — this invariant is the
// ongoing all-violations twin that also catches any already-landed record,
// the same belt-and-suspenders relationship
// check_gate_signoff_deferred_reason_present already has with its own
// proposal-time validate() check.
func checkGateSignoffSignedHasProvenance(g *ontology.Graph) []Violation {
	var out []Violation
	for _, r := range g.Requirements {
		for _, gs := range r.GateSignoffs {
			if gs.State != ontology.GateSignoffStateSigned {
				continue
			}
			var missing []string
			if gs.Signoff == nil || gs.Signoff.DecidedBy == "" {
				missing = append(missing, "signoff.decided_by")
			}
			if gs.Signoff == nil || gs.Signoff.Verbatim == "" {
				missing = append(missing, "signoff.verbatim")
			}
			if len(gs.Evidence) == 0 {
				missing = append(missing, "evidence")
			}
			if len(missing) > 0 {
				out = append(out, Violation{
					Check: "check_gate_signoff_signed_has_provenance",
					ID:    r.ID,
					Message: fmt.Sprintf(
						"requirement %s has a SIGNED gate_signoff for stage %q (pipeline_run %q) missing %v — a "+
							"SIGNED gate passage with no recorded provenance (who decided, what they said, "+
							"supporting evidence) is indistinguishable from an unattributed silent pass",
						r.ID, gs.Stage, gs.PipelineRun, missing),
				})
			}
		}
	}
	return out
}

var _ = All.MustRegister("check_gate_signoff_signed_has_provenance", Invariant{
	Name:  "check_gate_signoff_signed_has_provenance",
	Canon: methodology.Requirement,
	Claim: "every GateSignoff with state=SIGNED carries a populated Signoff (decided_by + verbatim) and non-empty evidence.",
	Rule: "for every Requirement.gate_signoffs[*] entry with state==\"SIGNED\", `signoff` MUST be non-nil with non-empty " +
		"decided_by and verbatim, AND `evidence` MUST be a non-empty list. No-ops trivially when no requirement carries " +
		"any gate_signoffs.",
	Why: "a SIGNED gate passage represents a genuine human decision the same way a DECIDED Conflict does " +
		"(R-decided-needs-human-signoff) — before this check the engine required NOTHING beyond the bare " +
		"stage/state/pipeline_run fields for SIGNED, letting a gate-signoff land with zero provenance about who decided " +
		"it, what they said, or why. This matters most for a domain whose gate signoffs represent real personal " +
		"decisions (not just process bookkeeping), where provenance must be guaranteed before the record lands.",
	Check: checkGateSignoffSignedHasProvenance,
})

// checkGateSignoffDecidedByIsKnownStakeholder requires a SIGNED
// GateSignoff's signoff.decided_by, when non-empty, to resolve to a real
// ontology.Stakeholder.id — mirrors checkDecidedByIsKnownStakeholder
// (conflict_decided_held.go), applied to GateSignoff's own DecidedBy field.
// Deliberately does NOT also mirror checkDecidedByNotMemberOwner: that check
// exists because a Conflict has MEMBERS (requirements whose owners must stay
// outside the decision to keep the resolver-distinct rule intact) — a
// GateSignoff has no such membership concept, it is a single-Requirement
// fact, so there is no analogous "owner of a member" to exclude the decider
// from. Left to checkGateSignoffSignedHasProvenance to require decided_by
// non-empty in the first place; this check only adds the referential-
// integrity half once it is present.
func checkGateSignoffDecidedByIsKnownStakeholder(g *ontology.Graph) []Violation {
	sids := ontology.StakeholderIDs(g)
	var out []Violation
	for _, r := range g.Requirements {
		for _, gs := range r.GateSignoffs {
			if gs.State != ontology.GateSignoffStateSigned || gs.Signoff == nil || gs.Signoff.DecidedBy == "" {
				continue
			}
			if _, ok := sids[gs.Signoff.DecidedBy]; !ok {
				out = append(out, Violation{
					Check: "check_gate_signoff_decided_by_is_known_stakeholder",
					ID:    r.ID,
					Message: fmt.Sprintf(
						"requirement %s has a SIGNED gate_signoff for stage %q (pipeline_run %q) whose "+
							"signoff.decided_by %q is not a known Stakeholder",
						r.ID, gs.Stage, gs.PipelineRun, gs.Signoff.DecidedBy),
				})
			}
		}
	}
	return out
}

var _ = All.MustRegister("check_gate_signoff_decided_by_is_known_stakeholder", Invariant{
	Name:  "check_gate_signoff_decided_by_is_known_stakeholder",
	Canon: methodology.Requirement,
	Claim: "a SIGNED GateSignoff's signoff.decided_by resolves to a known Stakeholder.",
	Rule: "for every Requirement.gate_signoffs[*] entry with state==\"SIGNED\" and a non-nil signoff with non-empty " +
		"decided_by, decided_by MUST be in stakeholder_ids(g). An unresolvable decider is a dangling reference that " +
		"cannot be audited.",
	Why: "mirrors check_decided_by_is_known_stakeholder (Conflict) applied to GateSignoff — free-text decided_by (a " +
		"typo'd name, an email address, a role label with no Stakeholder node) cannot be traced back to a real " +
		"accountable person, exactly the gap check_decided_by_is_known_stakeholder already closes for Conflict " +
		"resolution.",
	Check: checkGateSignoffDecidedByIsKnownStakeholder,
})

// conflictIDPattern matches the exact shape ontology.ConflictIdentity
// produces: "C-" followed by 8 lowercase hex digits (the first 8 hex chars
// of a sha256 sum, see internal/ontology/conflict.go).
var conflictIDPattern = regexp.MustCompile(`C-[0-9a-f]{8}`)

// checkGateSignoffDeferredConflictResolves validates that when a
// GateSignoff's DeferredReason contains a Conflict-id-shaped token
// (C-[0-9a-f]{8}), that id actually resolves to a real Conflict node in the
// graph — a deferral that blames an open Conflict for the delay must name a
// REAL Conflict, not a stale or typo'd reference. Unconditional (not gated
// on gate_stage_order): this is a referential-integrity check on the
// DeferredReason payload itself.
func checkGateSignoffDeferredConflictResolves(g *ontology.Graph) []Violation {
	var out []Violation
	var idx *ontology.GraphIndex
	for _, r := range g.Requirements {
		for _, gs := range r.GateSignoffs {
			if gs.State != ontology.GateSignoffStateDeferred {
				continue
			}
			ref := conflictIDPattern.FindString(gs.DeferredReason)
			if ref == "" {
				continue
			}
			if idx == nil {
				idx = ontology.BuildIndex(g)
			}
			if _, ok := idx.ConflictByID[ref]; !ok {
				out = append(out, Violation{
					Check: "check_gate_signoff_deferred_conflict_resolves",
					ID:    r.ID,
					Message: fmt.Sprintf(
						"requirement %s has a DEFERRED gate_signoff for stage %q whose deferred_reason references "+
							"conflict id %q, which does not resolve to any Conflict in the graph",
						r.ID, gs.Stage, ref),
				})
			}
		}
	}
	return out
}

var _ = All.MustRegister("check_gate_signoff_deferred_conflict_resolves", Invariant{
	Name:  "check_gate_signoff_deferred_conflict_resolves",
	Canon: methodology.Requirement,
	Claim: "a GateSignoff.deferred_reason that references a Conflict id (C-[0-9a-f]{8}) must resolve to a real Conflict node.",
	Rule: "for every Requirement.gate_signoffs[*] entry with state==\"DEFERRED\" whose deferred_reason contains a substring " +
		"matching C-[0-9a-f]{8} (the exact shape ontology.ConflictIdentity produces), that id MUST be present in " +
		"ontology.BuildIndex(g).ConflictByID. A deferred_reason with no Conflict-id-shaped substring is unaffected (this " +
		"check only fires when a Conflict is actually referenced). No-ops trivially when no requirement carries any " +
		"gate_signoffs.",
	Why: "a resolver deferring a gate for 'blocked on C-a1b2c3d4' is making a referential claim exactly like a " +
		"Requirement.assumptions[*] entry or a Conflict.members[*] entry — the dangling-id family " +
		"(check_entity_instance_refs_resolve and siblings) already treats an unresolvable typed reference as a structural " +
		"defect, not merely a lint nit, because a dangling pointer looks resolved to a casual reader but resolves to " +
		"nothing on inspection. Applying the same discipline here means a stale or typo'd Conflict reference in a " +
		"deferral reason is caught by `hotam all-violations`, not discovered manually months later when someone tries to " +
		"follow the pointer and finds nothing.",
	Check: checkGateSignoffDeferredConflictResolves,
})
