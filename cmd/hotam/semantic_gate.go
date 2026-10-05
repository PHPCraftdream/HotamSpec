package main

import (
	"fmt"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/diagnose"
	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/proposal"
)

// landAckOptions carries the human-decision evidence that can override a
// matching unresolved formal Conflict carrier.
//
// AckConflict must name one of the carriers whose members include both the
// candidate requirement ID and another conflict member. DecisionRef records a
// human decision reference on the landed requirement. Neither option turns
// lexical or metadata-linked suspicions into blockers.
type landAckOptions struct {
	AckConflict string
	DecisionRef string
}

// hasAck reports whether either form of human-decision evidence was supplied.
func (o landAckOptions) hasAck() bool {
	return o.AckConflict != "" || o.DecisionRef != ""
}

// hasOverride reports whether EITHER a landAckOptions flag (--ack-conflict /
// --decision-ref) OR a typed ProposedRequirement.Signoff (task #335,
// R4F-req-signoff) was supplied — the full set of evidence forms that can
// override semanticConflictGate's refusal. A typed, Stakeholder-resolved
// signoff is strictly stronger evidence than free text (its decided_by is
// mechanically verified against the domain's declared Stakeholders at
// mutate time, and check_history_signoff_has_provenance /
// check_history_signoff_decided_by_is_known_stakeholder keep it honest going
// forward), so it is accepted on par with the CLI flags: a resolver who
// already recorded a typed signoff on the requirement itself should not also
// have to pass --decision-ref to get past this gate.
func hasOverride(o landAckOptions, p proposal.Proposal) bool {
	if o.hasAck() {
		return true
	}
	if pr, ok := p.(proposal.ProposedRequirement); ok && pr.Signoff != nil {
		return true
	}
	return false
}

// semanticConflictGate blocks only when an explicit unresolved Conflict node
// names both the candidate requirement ID and at least one other member ID.
// Confront's lexical markers, token overlap, implementation/source links, and
// relations are emitted as suspicions for review; none establishes semantic
// contradiction or gates a landing.
//
// The formal carrier is checked regardless of lexical score or marker text.
// DETECTED and ACKNOWLEDGED carriers are unresolved per ontology.Conflict's
// lifecycle contract. DECIDED, HELD, and REVISIT_WHEN carriers are not hard
// blockers. This gate does not create, resolve, or infer Conflict records.
//
// The gate runs before transactional snapshot/apply in landProposalValue, so a
// refusal leaves graph and docs untouched. It applies to land and propose
// --land through their shared pipeline. Batch paths use the same
// ConfrontRequirement/IsBlockingHit contract via batchConflictChecker, but
// have no per-item decision override.
//
// Returns hadConflict=true iff at least one matching unresolved carrier was
// found, regardless of whether the operator recorded an override. The caller
// uses this to avoid writing false conflict-acknowledgment history when flags
// were supplied for a candidate with no formal blocker.
func semanticConflictGate(domainDir string, p proposal.Proposal, ackOpts landAckOptions) (hadConflict bool, err error) {
	pr, ok := p.(proposal.ProposedRequirement)
	if !ok {
		return false, nil // gate applies only to requirement claims
	}

	g, err := loadDomainGraph(domainDir)
	if err != nil {
		return false, fmt.Errorf("semantic-conflict gate: %w", err)
	}

	// Validate existence first so a typo'd C-id cannot be treated as a
	// citation. Once formal blockers are known below, the supplied ID must
	// also match one of their ConflictIDs.
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

	result := diagnose.ConfrontRequirement(g, requirementFromProposal(pr))

	var blockers []diagnose.ConfrontHit
	for _, h := range result.FormalConflicts {
		if diagnose.IsBlockingHit(h) {
			blockers = append(blockers, h)
		}
	}
	if len(blockers) == 0 {
		return false, nil
	}

	if ackOpts.AckConflict != "" && !ackNamesFormalBlocker(ackOpts.AckConflict, blockers) {
		return true, fmt.Errorf(
			"--ack-conflict %q does not match an unresolved Conflict whose members cover this requirement and a blocker",
			ackOpts.AckConflict)
	}

	if hasOverride(ackOpts, pr) {
		return true, nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "refusing to land %s: an unresolved formal Conflict names this requirement and %d other member(s):\n", pr.ID, len(blockers))
	for _, h := range blockers {
		fmt.Fprintf(&b, "  - %s: %q\n     conflicts: [%s]\n",
			h.ID, h.Claim, strings.Join(h.ConflictIDs, ", "))
	}
	b.WriteString("a human decision must be recorded before this can land. Use one of:\n")
	b.WriteString("  'signoff' on the proposal itself   attach a typed, Stakeholder-resolved signoff to this update\n")
	b.WriteString("  --ack-conflict <C-id>               cite one of the unresolved Conflict nodes listed above\n")
	b.WriteString("  --decision-ref <text>               record a reference to where the decision was made\n")
	b.WriteString("\nLexical markers, shared subjects, and metadata links are advisory only; they do not establish a semantic contradiction.")
	return true, fmt.Errorf("%s", b.String())
}

func requirementFromProposal(pr proposal.ProposedRequirement) ontology.Requirement {
	var candidate ontology.Requirement
	candidate.ID = pr.ID
	candidate.Claim = pr.Claim
	candidate.Status = pr.Status
	candidate.Relations = pr.Relations
	candidate.ImplementedBy = pr.ImplementedBy
	candidate.VerifiedBy = pr.VerifiedBy
	candidate.SourceLinks = pr.SourceLinks
	candidate.Coverage = pr.Coverage
	return candidate
}

func ackNamesFormalBlocker(conflictID string, blockers []diagnose.ConfrontHit) bool {
	for _, blocker := range blockers {
		for _, id := range blocker.ConflictIDs {
			if id == conflictID {
				return true
			}
		}
	}
	return false
}

// batchConflictChecker is injected into proposal.ApplyBatch. It receives the
// full candidate Requirement so source links and explicit relations participate
// in advisory output, while only matching unresolved Conflict members block.
func batchConflictChecker(g *ontology.Graph, candidate ontology.Requirement) error {
	result := diagnose.ConfrontRequirement(g, candidate)
	for _, h := range result.FormalConflicts {
		if diagnose.IsBlockingHit(h) {
			return fmt.Errorf(
				"matches unresolved formal Conflict member %s (%s) — "+
					"batch mode has no per-item decision override: pull this item out and land it individually with a recorded decision",
				h.ID, strings.Join(h.ConflictIDs, ", "))
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
// appendAckHistory writes the --ack-conflict / --decision-ref free-text
// audit entry described above appendAckHistory's declaration (see the
// landAckOptions doc comment). It is a no-op — for BOTH the "not a
// Requirement" case and the "no ack flag set" case — when ackOpts.hasAck()
// is false: a ProposedRequirement carrying its OWN typed Signoff (task
// #335, R4F-req-signoff) is a separate, stronger override that
// semanticConflictGate/hasOverride already accepts on its own, and its
// HistoryEntry (DecidedBy + Signoff, with the real decided_by/verbatim) was
// already written by ProposedRequirement.mutate at apply time — writing a
// SECOND, free-text "semantic conflict acknowledged" entry for the SAME
// decision here would be a redundant duplicate, not a complementary record.
// When BOTH ackOpts.AckConflict and a Requirement Signoff are present for
// the same landing (a resolver citing an existing Conflict node ALONGSIDE
// their own typed signoff), this function still fires for the
// --ack-conflict citation — that citation names the matching unresolved
// Conflict carrier covering the candidate/member pair, separate from the
// Requirement's own signoff (a decision record about THIS specific UPDATE).
func appendAckHistory(graphPath string, p proposal.Proposal, today string, ackOpts landAckOptions) error {
	pr, ok := p.(proposal.ProposedRequirement)
	if !ok {
		return nil
	}
	if !ackOpts.hasAck() {
		// Only a Requirement Signoff overrode the gate (no --ack-conflict /
		// --decision-ref flag) — its own HistoryEntry was already written by
		// mutate(); nothing further to append here.
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
