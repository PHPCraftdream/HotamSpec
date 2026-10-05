package main

import (
	"fmt"

	"github.com/PHPCraftdream/HotamSpec/internal/diagnose"
	"github.com/PHPCraftdream/HotamSpec/internal/proposal"
)

// confrontBeforeApply prints the confrontation evidence report immediately
// before the separate formal-carrier gate. It shares the direct apply and land
// paths; propose --land prints its own report inside runPropose. The report is
// advisory by itself; semanticConflictGate blocks only a matching unresolved
// Conflict carrier. Graph-load failures are returned because apply would also
// fail to load the graph.
func confrontBeforeApply(domainDir string, p proposal.Proposal, asJSON bool) error {
	g, err := loadDomainGraph(domainDir)
	if err != nil {
		return err
	}
	result := confrontProposal(g, p)
	fmt.Fprint(landOut(asJSON), formatConfrontReport(result))
	return nil
}

// confrontBatchDigestItem is the compact summary of one proposal's lexical,
// metadata-linked, and explicit formal-conflict hits, including the top hit's
// classification.
type confrontBatchDigestItem struct {
	Anchor            string `json:"anchor"`
	Kind              string `json:"kind"`
	Hits              int    `json:"hits"`
	TopID             string `json:"top_id"`
	TopScore          int    `json:"top_score"`
	TopClassification string `json:"top_classification,omitempty"`
}

// confrontBatchSummary renders advisory suspicions and explicit formal
// conflicts against the starting graph snapshot. The ApplyBatch gate separately
// evaluates every item against the rolling graph and blocks only a matching
// unresolved Conflict carrier.
func confrontBatchSummary(domainDir string, proposals []proposal.Proposal, asJSON bool) error {
	g, err := loadDomainGraph(domainDir)
	if err != nil {
		return err
	}
	var flagged []confrontBatchDigestItem
	for _, p := range proposals {
		r := confrontProposal(g, p)
		if r.Clear {
			continue
		}
		item := confrontBatchDigestItem{
			Anchor: p.TargetAnchor(),
			Kind:   p.Kind(),
			Hits:   len(r.Settled) + len(r.Rejected) + len(r.FormalConflicts),
		}
		for _, group := range [][]diagnose.ConfrontHit{r.Settled, r.Rejected, r.FormalConflicts} {
			for _, h := range group {
				if item.TopID == "" || h.Score > item.TopScore {
					item.TopScore = h.Score
					item.TopID = h.ID
					item.TopClassification = h.Classification
				}
			}
		}
		flagged = append(flagged, item)
	}

	out := landOut(asJSON)
	if len(flagged) == 0 {
		fmt.Fprintf(out, "confront batch: %d/%d proposals have no lexical, metadata-linked, or formal-conflict evidence\n", len(proposals), len(proposals))
		return nil
	}
	fmt.Fprintf(out, "confront batch: %d/%d proposals have advisory or formal evidence:\n", len(flagged), len(proposals))
	for _, it := range flagged {
		fmt.Fprintf(out, "  - %s %s: %d hit(s) (top: %s, %s, score %d)\n", it.Kind, it.Anchor, it.Hits, it.TopID, it.TopClassification, it.TopScore)
	}
	fmt.Fprintln(out, "  lexical and metadata hits are advisory; explicit unresolved Conflict carriers are checked by the apply gate")
	return nil
}
