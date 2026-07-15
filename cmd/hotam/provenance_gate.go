package main

import (
	"fmt"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/proposal"
)

// provenanceGate is the land-time gate that closes task #158's gap: a bare
// SETTLED requirement with completely empty source_refs, evidence,
// last_reviewed_at, and review_after could land with no record of where it
// came from or when it needs re-checking — a real maintainability risk for a
// business-methodology bulk import. It is OPT-IN per domain via
// "require_provenance": true in manifest.json (loader.ResolveRequireProvenance)
// — every domain that does not set the flag (including both self-hosted
// domains, hotam-spec-self and hotam-dev) is completely unaffected, mirroring
// resolveSelfHosting/ResolveGenProfile's backward-compatible default.
//
// The gate applies ONLY to a ProposedRequirement whose Status is SETTLED
// (DRAFT/other statuses are provisional — provenance is only meaningful once
// a requirement is actually decided). It checks the SIMULATED POST-MERGE
// result of applying the proposal (proposal.SimulateRequirementResult), not
// the raw incoming proposal fields — see that function's doc comment for why:
// mutate()'s coalesce semantics mean an UPDATE that omits an already-set
// provenance field is preserving it, not clearing it, so checking the raw
// proposal would falsely refuse a legitimate content-only edit of an
// already-provenance-complete requirement.
//
// It requires the simulated result to carry non-empty SourceRefs and
// non-empty LastReviewedAt and non-empty ReviewAfter. Evidence is NOT
// separately checked here: the existing R-review-mark-carries-evidence rule
// (validate.go's ProposedRequirement.validate) already requires Evidence to
// be non-empty whenever LastReviewedAt or ReviewAfter is set on the
// PROPOSAL — and since a requirement's LastReviewedAt/ReviewAfter can only
// ever have been set (on this proposal or an earlier one) via a proposal that
// itself passed that validation, a simulated result with both dates
// non-empty transitively has Evidence recorded on graph.json too. Re-checking
// it here would be redundant, not a gap.
//
// On failure, the error names SPECIFICALLY which field(s) are missing (not a
// generic "provenance incomplete" message) and points at the manifest flag
// that gates this, mirroring semanticConflictGate's refusal-message style.
func provenanceGate(domainDir string, p proposal.Proposal) error {
	pr, ok := p.(proposal.ProposedRequirement)
	if !ok {
		return nil // gate applies only to requirement claims
	}
	if pr.Status != ontology.StatusSETTLED {
		return nil // provenance is only required for SETTLED
	}
	if !loader.ResolveRequireProvenance(graphPathForDomain(domainDir)) {
		return nil // opt-out is the default
	}

	g, err := loadDomainGraph(domainDir)
	if err != nil {
		return err
	}
	return provenanceCheck(g, "", pr)
}

// provenanceCheck simulates pr's post-merge result against g and reports a
// non-nil, ready-to-surface error naming the missing provenance field(s) if
// the result is incomplete. today is passed through to
// proposal.SimulateRequirementResult unchanged (it only affects CreatedAt/
// SettledAt stamping on a CREATE, neither of which this gate inspects).
//
// This is the shared core behind provenanceGate (single-file path) and
// batchProvenanceChecker (the proposal.ProvenanceChecker injected into
// internal/proposal.ApplyBatch for the batch path), so both call surfaces run
// the IDENTICAL check.
func provenanceCheck(g *ontology.Graph, today string, pr proposal.ProposedRequirement) error {
	if pr.Status != ontology.StatusSETTLED {
		return nil
	}
	result, err := proposal.SimulateRequirementResult(g, today, pr)
	if err != nil {
		return err
	}

	var missing []string
	if len(result.SourceRefs) == 0 {
		missing = append(missing, "source_refs")
	}
	if strings.TrimSpace(result.LastReviewedAt) == "" {
		missing = append(missing, "last_reviewed_at")
	}
	if strings.TrimSpace(result.ReviewAfter) == "" {
		missing = append(missing, "review_after")
	}
	if len(missing) == 0 {
		return nil
	}

	return errProvenanceIncomplete(pr.ID, missing)
}

func errProvenanceIncomplete(id string, missing []string) error {
	var b strings.Builder
	b.WriteString("refusing to land ")
	b.WriteString(id)
	b.WriteString(": SETTLED requires provenance, missing: ")
	b.WriteString(strings.Join(missing, ", "))
	b.WriteString(" — this domain's manifest.json sets \"require_provenance\": true, " +
		"which requires every SETTLED requirement to carry non-empty source_refs, " +
		"last_reviewed_at, and review_after (evidence is transitively required by " +
		"R-review-mark-carries-evidence once the dates are set). Supply the missing " +
		"field(s) on this proposal, or land it as DRAFT until provenance is available.")
	return fmt.Errorf("%s", b.String())
}

// batchProvenanceChecker builds the proposal.ProvenanceChecker that
// internal/proposal.ApplyBatch invokes for each ProposedRequirement in a
// batch, giving the provenance gate the SAME batch-path parity task #155
// established for the semantic-conflict gate (batchConflictChecker). It runs
// the SAME provenanceCheck against the ROLLING in-memory graph ApplyBatch
// already threads through — so an UPDATE later in a batch correctly sees
// provenance a CREATE earlier in the SAME batch just established.
//
// domainDir is captured by the closure only to resolve the opt-in flag
// (loader.ResolveRequireProvenance) once per checker construction — the
// flag is a per-domain manifest.json setting, not something that changes
// mid-batch.
func batchProvenanceChecker(domainDir string) proposal.ProvenanceChecker {
	requireProvenance := loader.ResolveRequireProvenance(graphPathForDomain(domainDir))
	return func(g *ontology.Graph, today string, pr proposal.ProposedRequirement) error {
		if !requireProvenance {
			return nil
		}
		return provenanceCheck(g, today, pr)
	}
}
