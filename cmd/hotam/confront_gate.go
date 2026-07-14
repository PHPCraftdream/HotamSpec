package main

import (
	"fmt"

	"github.com/PHPCraftdream/HotamSpec/internal/diagnose"
	"github.com/PHPCraftdream/HotamSpec/internal/proposal"
)

// confrontBeforeApply is the advisory confront-at-gate check shared by the
// single-file, DIRECT invocations of `hotam apply-proposal <file>` and
// `hotam land <file>` (their command-level branches). It loads the domain
// graph, runs the SAME lexical-overlap confront that `hotam confront` /
// `hotam propose` run (diagnose.Confront over the proposal's candidate text,
// extracted via proposeConfrontText), and prints the human-readable report
// (formatConfrontReport) BEFORE the apply outcome is reported.
//
// It lives ONLY at the command level (never inside applyProposalValue /
// landProposalValue), so `hotam propose --land` — which runs its OWN confront
// inside runPropose before calling landProposalFile — is unaffected and prints
// exactly one report (no double-confront). This is the design constraint that
// keeps the warn-only visibility from task #124 (propose) extended to the APPLY
// path (this task) without printing the same report twice on the propose --land
// path.
//
// Advisory: a high-overlap hit is a WARNING, never a block
// (R-ai-presents-not-decides). The apply/land always proceeds regardless of
// what confront finds. A graph-load failure is returned (the apply would fail
// to load the graph anyway), matching propose.go's confront behavior.
func confrontBeforeApply(domainDir string, p proposal.Proposal, asJSON bool) error {
	g, err := loadDomainGraph(domainDir)
	if err != nil {
		return err
	}
	result := diagnose.Confront(g, proposeConfrontText(p))
	fmt.Fprint(landOut(asJSON), formatConfrontReport(result))
	return nil
}

// confrontBatchDigestItem is one FLAGGED proposal's compact confront digest,
// printed by the batch summary. It carries the target anchor, the total hit
// count (SETTLED + REJECTED), and the single highest-scoring hit (id + score)
// across both sides. Fields are JSON-taggable so a future `--json` flag (task
// #128, R5-m's "--json everywhere" audit) can serialize the batch summary
// without re-designing the shape. (No --json flag is added by this task — see
// its scope note.)
type confrontBatchDigestItem struct {
	Anchor   string `json:"anchor"`
	Kind     string `json:"kind"`
	Hits     int    `json:"hits"`
	TopID    string `json:"top_id"`
	TopScore int    `json:"top_score"`
}

// confrontBatchSummary is the batch-mode confront-at-gate check, shared by
// `apply-proposal --batch` and `land --batch`. It loads the domain graph ONCE
// and runs confront for every proposal's candidate text against that SAME
// starting graph state (not progressively updated per-item — the batch's own
// apply semantics are atomic-against-one-snapshot, and the confront check
// mirrors that: one pass, N candidates, one graph).
//
// It prints a SHORT summary rather than N full confront reports (which would
// drown a large batch in noise): when every proposal is clear, a single
// "N/N clear" line; otherwise the count of flagged proposals plus a one-line
// digest (anchor + hit count + top hit) for EACH flagged one. Silence is never
// the output — the operator must know the check ran and what it concluded
// (matching formatConfrontReport's own "always an explicit verdict" contract).
//
// Advisory only: never blocks the batch apply (R-ai-presents-not-decides).
func confrontBatchSummary(domainDir string, proposals []proposal.Proposal, asJSON bool) error {
	g, err := loadDomainGraph(domainDir)
	if err != nil {
		return err
	}
	var flagged []confrontBatchDigestItem
	for _, p := range proposals {
		r := diagnose.Confront(g, proposeConfrontText(p))
		if r.Clear {
			continue
		}
		item := confrontBatchDigestItem{
			Anchor: p.TargetAnchor(),
			Kind:   p.Kind(),
			Hits:   len(r.Settled) + len(r.Rejected),
		}
		// Top hit by score across SETTLED then REJECTED. Hits always carry a
		// positive score (they clear MinLexicalOverlapTokens / the marker
		// variant), so the first hit sets TopID; the guard below also tolerates
		// a hypothetical all-zero-score edge.
		for _, h := range r.Settled {
			if h.Score > item.TopScore {
				item.TopScore = h.Score
				item.TopID = h.ID
			}
		}
		for _, h := range r.Rejected {
			if h.Score > item.TopScore {
				item.TopScore = h.Score
				item.TopID = h.ID
			}
		}
		flagged = append(flagged, item)
	}

	out := landOut(asJSON)
	if len(flagged) == 0 {
		fmt.Fprintf(out, "confront batch: %d/%d proposals clear — no overlap detected\n", len(proposals), len(proposals))
		return nil
	}
	fmt.Fprintf(out, "confront batch: %d/%d proposals flagged for possible overlap:\n", len(flagged), len(proposals))
	for _, it := range flagged {
		if it.TopID != "" {
			fmt.Fprintf(out, "  - %s %s: %d hit(s) (top: %s, score %d)\n", it.Kind, it.Anchor, it.Hits, it.TopID, it.TopScore)
		} else {
			fmt.Fprintf(out, "  - %s %s: %d hit(s)\n", it.Kind, it.Anchor, it.Hits)
		}
	}
	fmt.Fprintln(out, "  (advisory; batch proceeds regardless)")
	return nil
}
