package main

import (
	"fmt"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/diagnose"
	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/proposal"
)

// landAckOptions carries the two forms of human-decision evidence an operator
// can supply to override the semantic-conflict gate (semanticConflictGate):
//
//   - AckConflict: the ID of an EXISTING Conflict node in the graph whose
//     Members plausibly cover the tension between the new requirement and the
//     SETTLED requirement(s) it contradicts. The tool validates EXISTENCE only
//     (the node is graph ground truth — its coverage of this specific tension
//     is the steward's judgment, not the tool's, per R-ai-presents-not-decides
//     and R-decided-needs-human-signoff). The Conflict node itself is the
//     durable record; the requirement additionally gets a HistoryEntry audit
//     trail noting which Conflict was cited (see appendAckHistory).
//
//   - DecisionRef: a free-text reference to where a human decision was
//     recorded (a ticket link, a meeting note, a steward's name+date). The
//     CONTENT is not validated, only its PRESENCE — persisted as a HistoryEntry
//     on the landed requirement so the audit trail lives on the node it
//     concerns, avoiding the heavier Conflict-node machinery when a full
//     Conflict would be overkill for the specific case.
type landAckOptions struct {
	AckConflict string
	DecisionRef string
}

// hasAck reports whether either form of human-decision evidence was supplied.
func (o landAckOptions) hasAck() bool {
	return o.AckConflict != "" || o.DecisionRef != ""
}

// semanticConflictGate is the land-time gate that closes the review-8 R8-d gap:
// "the system checks structure but not meaning." When landing a
// ProposedRequirement whose claim triggers a HIGH-CONFIDENCE semantic-conflict
// signal against one or more EXISTING SETTLED requirements, the land REFUSES
// (non-zero exit) unless the operator supplied evidence a human already made a
// decision about this tension (landAckOptions).
//
// HIGH-CONFIDENCE SIGNAL — opposite marker only:
// The signal is a ConfrontHit that carries an OppositeMarker (never/always,
// must/must not, only/any) split across the candidate claim and an existing
// SETTLED requirement's claim. This is deliberately the ONLY trigger:
//
//   - An opposite marker is a PRECISE indicator of genuine semantic
//     contradiction — one side asserts a universal where the other asserts a
//     prohibition (the review's own worked example: "always encrypt" vs
//     "never encrypt").
//
//   - A high token-overlap score WITHOUT an opposite marker is often just
//     "these two requirements are about the same subject" (relatedness, not
//     contradiction). Using raw score alone would generate false positives on
//     every pair of requirements that share domain vocabulary but agree.
//
//   - No pure-score fallback threshold is used. This is the conservative
//     choice the task asks for ("err toward few false positives, clear escape
//     hatches"): the opposite-marker controlled vocabulary (established in
//     inspect.go's oppositeMarkerPairs) is narrow enough that a steward who
//     triggers it almost always has a real tension on their hands, while the
//     overwhelming majority of ordinary requirement lands (which never carry
//     an opposite marker against an existing SETTLED requirement) pass through
//     the gate untouched.
//
// This gate does NOT decide semantic correctness — it requires a DECISION to be
// RECORDED before proceeding, which is exactly what R-ai-presents-not-decides
// and R-decided-needs-human-signoff establish as the pattern for Conflict
// nodes. The confront machinery it reuses is the SAME diagnose.Confront the
// land/propose confront-at-gate already runs (warn-only visibility); this gate
// adds only the refusal-on-no-ack behavior on top of it.
//
// The gate runs BEFORE the transactional snapshot/apply in landProposalValue,
// so a refusal leaves the graph and docs completely untouched. It applies to
// BOTH `hotam land <file>` and `hotam propose requirement --land` (both funnel
// through landProposalValue) and is NOT duplicated in two places. The batch
// path (`--batch <dir>`) does NOT call this function — its BLOCKING half runs
// inside internal/proposal.ApplyBatch (same diagnose.IsBlockingHit predicate,
// against the rolling in-memory graph); only the ack-OVERRIDE half is absent
// in batch mode (see cmdLandBatch's and cmdApplyProposal's doc comments).
//
// Returns hadConflict=true IFF a high-confidence signal (blockers) was found,
// regardless of whether an ack overrode the refusal. The caller uses this to
// gate appendAckHistory: the audit trail must be written ONLY when a real
// conflict existed, not merely because ack flags were passed (landing a
// non-conflicting requirement with --decision-ref must NOT record a false
// "semantic conflict acknowledged" entry).
func semanticConflictGate(domainDir string, p proposal.Proposal, ackOpts landAckOptions) (hadConflict bool, err error) {
	pr, ok := p.(proposal.ProposedRequirement)
	if !ok {
		return false, nil // gate applies only to requirement claims
	}

	g, err := loadDomainGraph(domainDir)
	if err != nil {
		return false, fmt.Errorf("semantic-conflict gate: %w", err)
	}

	// Validate --ack-conflict references a real Conflict node BEFORE using it
	// to override the gate, so a typo'd C-id does not silently bypass the
	// refusal. Existence-only: the node is graph ground truth; whether it
	// plausibly covers THIS tension is the steward's judgment.
	if ackOpts.AckConflict != "" {
		found := false
		for _, c := range g.Conflicts {
			if c.ID == ackOpts.AckConflict {
				found = true
				break
			}
		}
		if !found {
			return false, fmt.Errorf(
				"--ack-conflict %q does not match any Conflict node in the graph — "+
					"provide an existing C-... id (create one via `hotam apply-proposal <conflict.json>` first)",
				ackOpts.AckConflict)
		}
	}

	// Run the SAME confront the land/propose confront-at-gate already uses.
	result := diagnose.Confront(g, pr.Claim)

	// Collect high-confidence hits: SETTLED requirements that IsBlockingHit
	// flags (opposite marker + topical shared token). The blocking predicate
	// lives in internal/diagnose (diagnose.IsBlockingHit) so the batch path —
	// internal/proposal.ApplyBatch, which cannot import cmd/hotam — runs the
	// SAME check the single-file gate runs.
	var blockers []diagnose.ConfrontHit
	for _, h := range result.Settled {
		if diagnose.IsBlockingHit(h) {
			blockers = append(blockers, h)
		}
	}
	if len(blockers) == 0 {
		return false, nil // no high-confidence semantic conflict
	}

	// Ack provided (either form) → proceed. The tool required a decision to be
	// recorded; it does not verify the decision's correctness. hadConflict is
	// true: a real signal WAS found, even though the ack overrides the refusal,
	// so the audit trail is legitimately written.
	if ackOpts.hasAck() {
		return true, nil
	}

	// Refuse: name the SPECIFIC conflicting anchor(s) and their claims, and
	// suggest BOTH remediation paths.
	var b strings.Builder
	fmt.Fprintf(&b, "refusing to land %s: its claim semantically contradicts %d SETTLED requirement(s) "+
		"(opposite-marker signal):\n", pr.ID, len(blockers))
	for _, h := range blockers {
		fmt.Fprintf(&b, "  - %s: %q\n     opposite markers: %s; shared tokens: [%s]\n",
			h.ID, h.Claim, h.OppositeMarker, strings.Join(h.Shared, ", "))
	}
	b.WriteString("a human decision must be recorded before this can land. Use one of:\n")
	b.WriteString("  --ack-conflict <C-id>   cite an existing Conflict node whose members cover this tension\n")
	b.WriteString(fmt.Sprintf("  --decision-ref <text>   record a free-text reference to where the decision was made (e.g. ticket, meeting, steward+date)\n"))
	b.WriteString("\nThis gate does not decide correctness — it requires that a decision be RECORDED first ")
	b.WriteString("(R-ai-presents-not-decides, R-decided-needs-human-signoff).")
	return true, fmt.Errorf("%s", b.String())
}

// batchConflictChecker builds the proposal.ConflictChecker that
// internal/proposal.ApplyBatch invokes for each ProposedRequirement in a
// batch. internal/proposal (core) cannot import internal/diagnose
// (periphery) — see R-core-periphery-import-ratchet, enforced by
// internal/selfcheck/imports_test.go's TestCorePeriphery_ImportRatchet — so
// cmd/hotam, which already imports both packages, builds this closure and
// injects it into ApplyBatch. It runs the SAME diagnose.Confront +
// diagnose.IsBlockingHit check the single-file semanticConflictGate runs
// (against g's SETTLED requirements), and — unlike semanticConflictGate —
// has no ack-override: batch mode has no per-file flag mechanism, so a
// conflicting item must be pulled out and landed individually with an
// explicit ack.
func batchConflictChecker(g *ontology.Graph, claim string) error {
	result := diagnose.Confront(g, claim)
	for _, h := range result.Settled {
		if diagnose.IsBlockingHit(h) {
			return fmt.Errorf(
				"semantically contradicts SETTLED requirement %s: %q "+
					"(opposite-marker signal; shared tokens: [%s]) — "+
					"batch mode has no --ack-conflict/--decision-ref override: "+
					"pull this item out and land it individually with an explicit ack",
				h.ID, h.Claim, strings.Join(h.Shared, ", "))
		}
	}
	return nil
}

// appendAckHistory persists the human-decision audit trail on the landed
// requirement's History field, AFTER apply wrote the node but BEFORE regen
// renders the docs (so the History entry appears in the generated output).
//
// For --ack-conflict: records which Conflict node was cited. The Conflict node
// itself is the primary durable record; this HistoryEntry is a convenience
// pointer so a future reader of the requirement knows its landing explicitly
// acknowledged a named tension without having to cross-reference every
// Conflict's Members.
//
// For --decision-ref: records the free-text reference verbatim. This is the
// SOLE persistence of the decision-ref — there is no Conflict node — so the
// History field is its home (see landAckOptions' doc comment for why History
// is the right place rather than a new field).
//
// Only ProposedRequirement carries a claim the gate can fire on, so only
// ProposedRequirement gets an audit entry; a non-Requirement proposal with ack
// options set (a user error) is a silent no-op here.
func appendAckHistory(graphPath string, p proposal.Proposal, today string, ackOpts landAckOptions) error {
	pr, ok := p.(proposal.ProposedRequirement)
	if !ok {
		return nil
	}

	var summary string
	switch {
	case ackOpts.AckConflict != "" && ackOpts.DecisionRef != "":
		summary = fmt.Sprintf("semantic conflict acknowledged via Conflict %s; decision ref: %s",
			ackOpts.AckConflict, ackOpts.DecisionRef)
	case ackOpts.AckConflict != "":
		summary = fmt.Sprintf("semantic conflict acknowledged via Conflict %s", ackOpts.AckConflict)
	default: // DecisionRef != "" (hasAck guarantees at least one)
		summary = fmt.Sprintf("semantic conflict acknowledged — human decision recorded: %s", ackOpts.DecisionRef)
	}

	g, err := loader.LoadGraph(graphPath)
	if err != nil {
		return fmt.Errorf("load graph for ack history: %w", err)
	}
	idx := -1
	for i, r := range g.Requirements {
		if r.ID == pr.ID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("ack history: requirement %s not found in graph after apply", pr.ID)
	}
	g.Requirements[idx].History = append(g.Requirements[idx].History, ontology.HistoryEntry{
		At:      today,
		Summary: summary,
	})
	if err := loader.WriteGraph(graphPath, g); err != nil {
		return fmt.Errorf("write graph for ack history: %w", err)
	}
	return nil
}
