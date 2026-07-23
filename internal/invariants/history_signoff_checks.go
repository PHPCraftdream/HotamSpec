// history_signoff_checks.go holds the invariants for the shared
// ontology.HistoryEntry.Signoff field (task #335, R4F-req-signoff) -- the
// typed provenance payload (decided_by + date + verbatim + instrument;
// chosen_variant is Conflict-variant-only and must stay empty on a
// History-attached signoff, enforced at proposal-validate() time, see
// internal/proposal/validate.go's validateHistorySignoffShape) a History
// entry carries when it records a real human decision, closing the gap task
// #328's landed Requirement UPDATEs exposed: a real human approval recorded
// only as free text in History.Summary, with History.DecidedBy left "" even
// though a real human approved with a real verbatim quote (confirmed by
// direct read of domains/hotam-spec-self/graph.json's
// R-shared-projections-mode-independent / R-orientation-faq-answerable
// entries).
//
// Both checks below sweep EVERY HistoryEntry-carrying node type in the
// ontology package (Requirement, Assumption, Axis, EntityType, Process --
// see internal/ontology/{requirement,assumption,axis,entity,process}.go),
// not just Requirement, for future-proofing: any node kind gains this
// provenance guarantee automatically the day it starts using
// HistoryEntry.Signoff, with no new check to write.
//
// Both are deliberately ONGOING all-violations invariants, not
// proposal-time-only checks -- mirroring
// check_gate_signoff_signed_has_provenance /
// check_gate_signoff_decided_by_is_known_stakeholder's identical choice
// (task #319): the belt is proposal-time validate()/mutate() (rejects a
// malformed or unresolvable signoff before it ever lands), the suspenders
// are these two checks (catch any already-landed record, including one
// written by a future code path that bypasses internal/proposal entirely).
package invariants

import (
	"fmt"

	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// historySignoffEntry pairs one HistoryEntry with an identifying label for
// the node that carries it (e.g. "Requirement R-foo", "Assumption A-bar"),
// so both checks below can walk every HistoryEntry-carrying node type in the
// graph through one shared loop instead of five near-duplicate ones.
type historySignoffEntry struct {
	nodeLabel string
	entry     ontology.HistoryEntry
}

// allHistoryEntries collects every HistoryEntry from every node type in g
// that carries a History field (Requirement, Assumption, Axis, EntityType,
// Process), each paired with a human-readable label naming its owning node.
func allHistoryEntries(g *ontology.Graph) []historySignoffEntry {
	var out []historySignoffEntry
	for _, r := range g.Requirements {
		for _, h := range r.History {
			out = append(out, historySignoffEntry{nodeLabel: "Requirement " + r.ID, entry: h})
		}
	}
	for _, a := range g.Assumptions {
		for _, h := range a.History {
			out = append(out, historySignoffEntry{nodeLabel: "Assumption " + a.ID, entry: h})
		}
	}
	for _, ax := range g.Axes {
		for _, h := range ax.History {
			out = append(out, historySignoffEntry{nodeLabel: "Axis " + ax.Slug, entry: h})
		}
	}
	for _, et := range g.EntityTypes {
		for _, h := range et.History {
			out = append(out, historySignoffEntry{nodeLabel: "EntityType " + et.Slug, entry: h})
		}
	}
	for _, p := range g.Processes {
		for _, h := range p.History {
			out = append(out, historySignoffEntry{nodeLabel: "Process " + p.ID, entry: h})
		}
	}
	return out
}

// checkHistorySignoffHasProvenance requires every HistoryEntry with a
// non-nil Signoff to carry non-empty DecidedBy and Verbatim on that Signoff
// -- a signoff with no named human decider or no record of what they said is
// indistinguishable from an unattributed silent pass, the same shape
// check_gate_signoff_signed_has_provenance already enforces for
// GateSignoff.Signoff.
func checkHistorySignoffHasProvenance(g *ontology.Graph) []Violation {
	var out []Violation
	for _, he := range allHistoryEntries(g) {
		s := he.entry.Signoff
		if s == nil {
			continue
		}
		var missing []string
		if s.DecidedBy == "" {
			missing = append(missing, "signoff.decided_by")
		}
		if s.Verbatim == "" {
			missing = append(missing, "signoff.verbatim")
		}
		if len(missing) > 0 {
			out = append(out, Violation{
				Check: "check_history_signoff_has_provenance",
				ID:    he.nodeLabel,
				Message: fmt.Sprintf(
					"%s has a History entry at %q with a non-nil signoff missing %v -- a signoff with "+
						"no recorded provenance (who decided, what they said) is indistinguishable from "+
						"an unattributed silent pass",
					he.nodeLabel, he.entry.At, missing),
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_history_signoff_has_provenance", Invariant{
	Name:  "check_history_signoff_has_provenance",
	Canon: methodology.Signoff,
	Claim: "every HistoryEntry.Signoff, when non-nil, carries non-empty decided_by and verbatim.",
	Rule: "for every HistoryEntry (on any Requirement/Assumption/Axis/EntityType/Process) whose signoff is non-nil, " +
		"signoff.decided_by MUST be non-empty AND signoff.verbatim MUST be non-empty. No-ops trivially when no " +
		"HistoryEntry in the graph carries a signoff.",
	Why: "a typed History signoff exists to make a real human decision auditable (task #335, R4F-req-signoff, closing " +
		"the gap where task #328's landed Requirement UPDATEs recorded real human approval only as free text in " +
		"History.Summary, with History.DecidedBy left empty) -- a signoff record with no named decider or no record of " +
		"what they actually said would recreate the exact gap this field exists to close, just with an extra layer of " +
		"typed-but-empty indirection. Mirrors check_gate_signoff_signed_has_provenance's identical rule for " +
		"GateSignoff.Signoff.",
	Check: checkHistorySignoffHasProvenance,
})

// checkHistorySignoffDecidedByIsKnownStakeholder requires a HistoryEntry
// Signoff's DecidedBy, when non-empty, to resolve to a real
// ontology.Stakeholder.id -- mirrors
// check_gate_signoff_decided_by_is_known_stakeholder (GateSignoff.Signoff)
// and checkDecidedByIsKnownStakeholder (Conflict), applied here to
// HistoryEntry.Signoff. Skips (no violation) when DecidedBy is empty --
// check_history_signoff_has_provenance above already owns that case; this
// check only adds the referential-integrity half once DecidedBy is present,
// avoiding a double-report of the same underlying gap.
func checkHistorySignoffDecidedByIsKnownStakeholder(g *ontology.Graph) []Violation {
	sids := ontology.StakeholderIDs(g)
	var out []Violation
	for _, he := range allHistoryEntries(g) {
		s := he.entry.Signoff
		if s == nil || s.DecidedBy == "" {
			continue
		}
		if _, ok := sids[s.DecidedBy]; !ok {
			out = append(out, Violation{
				Check: "check_history_signoff_decided_by_is_known_stakeholder",
				ID:    he.nodeLabel,
				Message: fmt.Sprintf(
					"%s has a History entry at %q whose signoff.decided_by %q is not a known Stakeholder",
					he.nodeLabel, he.entry.At, s.DecidedBy),
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_history_signoff_decided_by_is_known_stakeholder", Invariant{
	Name:  "check_history_signoff_decided_by_is_known_stakeholder",
	Canon: methodology.Signoff,
	Claim: "a HistoryEntry.Signoff's decided_by, when non-empty, resolves to a known Stakeholder.",
	Rule: "for every HistoryEntry (on any Requirement/Assumption/Axis/EntityType/Process) with a non-nil signoff and a " +
		"non-empty signoff.decided_by, decided_by MUST be in stakeholder_ids(g). A HistoryEntry with an empty " +
		"decided_by is skipped here (check_history_signoff_has_provenance owns that case). An unresolvable decider is " +
		"a dangling reference that cannot be audited.",
	Why: "mirrors check_gate_signoff_decided_by_is_known_stakeholder (GateSignoff) and " +
		"check_decided_by_is_known_stakeholder (Conflict) applied to HistoryEntry.Signoff -- free-text decided_by (a " +
		"typo'd name, an email address, a role label with no Stakeholder node) cannot be traced back to a real " +
		"accountable person, exactly the gap those two sibling checks already close for their own node kinds.",
	Check: checkHistorySignoffDecidedByIsKnownStakeholder,
})
