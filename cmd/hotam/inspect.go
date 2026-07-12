package main

import (
	"fmt"

	"github.com/PHPCraftdream/HotamSpecGo/internal/diagnose"
)

// cmdInspect implements `hotam inspect [--domain <path>] [--json] [--limit N]
// [--min-score N]`: a detailed, on-demand ADVISORY listing of semantic-conflict
// candidates — pairs/clusters of nodes that MIGHT be in tension, each with the
// evidence that flagged them and a fixed recommendation to consider a
// ProposedConflict.
//
// all-violations==0 only proves STRUCTURAL correctness (invariants package);
// whether two natural-language SETTLED claims actually contradict each
// other is a semantic question no structural check answers. `what-now`
// already surfaces a terse cluster signal + a freshness signal at
// PLatentConnector/PAdvisory priority (internal/diagnose/signal.go); this
// command does NOT duplicate that — it is the detailed drill-down a steward
// or agent reaches for on demand, backed by the SAME underlying detectors
// (internal/diagnose.AllCandidates) plus additional heuristics not surfaced
// in the terse what-now view (lexical claim overlap, axis co-reference).
//
// Score filtering: each heuristic assigns a Candidate.Score (see
// internal/diagnose/inspect.go). On the real hotam-spec-self domain ~81% of
// candidates carry score ≤4 — overwhelmingly low-signal lexical_claim_overlap
// pairs that drown the useful signal. --min-score (default 5) drops every
// candidate below the threshold out of the box and reports how many were
// suppressed; --min-score 0 restores the pre-threshold "show everything"
// behavior. Exit code is ALWAYS 0: inspect informs, it never gates
// (R-ai-presents-not-decides).
func cmdInspect(args []string) error {
	fs := newFlagSet("inspect")
	domain := fs.String("domain", "", "domain directory (default: "+defaultDomainRel+")")
	asJSON := fs.Bool("json", false, "emit machine-readable JSON")
	limit := fs.Int("limit", 20, "max candidates to print (0 = unlimited)")
	minScore := fs.Int("min-score", defaultInspectMinScore, "minimum candidate score to show (0 = show all); default 5 suppresses low-signal noise")
	fs.Parse(args)

	domainDir, err := resolveDomain(*domain)
	if err != nil {
		return err
	}
	g, err := loadDomainGraph(domainDir)
	if err != nil {
		return err
	}

	result := buildInspectResult(diagnose.AllCandidates(g), *minScore, *limit)

	if *asJSON {
		return printJSON(result)
	}
	fmt.Print(formatInspectReport(result))
	return nil
}

// defaultInspectMinScore is the score threshold applied when --min-score is
// not passed. On the real hotam-spec-self domain ~81% of inspect candidates
// carry score ≤4 (overwhelmingly low-signal lexical_claim_overlap pairs), so 5
// is chosen to suppress that noise out of the box rather than only when a user
// remembers to ask. --min-score 0 restores the pre-threshold behavior (show
// every candidate regardless of score).
const defaultInspectMinScore = 5

// inspectResult is the structured payload shared by the text and JSON
// renderers: the pre-filter total, the score-threshold suppression count, the
// threshold applied, and the candidates actually shown (after score filter AND
// display limit). Keeping the counting in one place means the text report, the
// JSON object, and the tests all read the same numbers.
type inspectResult struct {
	TotalCandidates int                  `json:"total_candidates"`
	SuppressedCount int                  `json:"suppressed_count"`
	MinScore        int                  `json:"min_score"`
	Candidates      []diagnose.Candidate `json:"candidates"`
}

// buildInspectResult applies the score threshold and the display limit to the
// full candidate list and returns the structured result consumed by both
// renderers. Score filtering runs BEFORE the limit: the limit caps how many of
// the score-survivors are printed, it does not change which candidates the
// threshold dropped (so SuppressedCount is independent of --limit — that is
// the invariant the tests pin: SuppressedCount + len(Candidates) ==
// TotalCandidates whenever limit does not bite).
func buildInspectResult(all []diagnose.Candidate, minScore, limit int) inspectResult {
	passed := make([]diagnose.Candidate, 0, len(all))
	suppressed := 0
	for _, c := range all {
		if c.Score < minScore {
			suppressed++
			continue
		}
		passed = append(passed, c)
	}
	shown := passed
	if limit > 0 && len(passed) > limit {
		shown = passed[:limit]
	}
	return inspectResult{
		TotalCandidates: len(all),
		SuppressedCount: suppressed,
		MinScore:        minScore,
		Candidates:      shown,
	}
}

func formatInspectReport(r inspectResult) string {
	if r.TotalCandidates == 0 {
		return "hotam inspect — no conflict candidates found (advisory; exit code always 0).\n"
	}
	passedCount := r.TotalCandidates - r.SuppressedCount
	var out string
	if r.SuppressedCount > 0 {
		out = fmt.Sprintf("hotam inspect — %d conflict candidate(s) at score≥%d (showing %d), advisory only:\n\n",
			passedCount, r.MinScore, len(r.Candidates))
	} else {
		out = fmt.Sprintf("hotam inspect — %d conflict candidate(s) found (showing %d), advisory only:\n\n",
			r.TotalCandidates, len(r.Candidates))
	}
	for i, c := range r.Candidates {
		out += fmt.Sprintf("%d. [%s] %s\n", i+1, c.Heuristic, c.ID)
		out += fmt.Sprintf("   members: %v\n", c.Members)
		out += fmt.Sprintf("   evidence: %s\n", c.Evidence)
		if c.Score > 0 {
			out += fmt.Sprintf("   score: %d\n", c.Score)
		}
		out += fmt.Sprintf("   %s\n\n", c.Recommendation)
	}
	if passedCount > len(r.Candidates) {
		out += fmt.Sprintf("… and %d more (raise --limit to see them).\n", passedCount-len(r.Candidates))
	}
	if r.SuppressedCount > 0 {
		out += fmt.Sprintf("suppressed %d candidate(s) below score threshold %d (use --min-score 0 to see all).\n",
			r.SuppressedCount, r.MinScore)
	}
	return out
}
