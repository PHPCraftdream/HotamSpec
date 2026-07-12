package diagnose

import (
	"sort"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// Confront is the CONFRONT step of the mediation loop, made callable: it takes
// an EXTERNAL candidate text (a draft claim an operator is about to propose)
// and checks it for lexical overlap with every SETTLED requirement (duplicate
// guard) and every REJECTED requirement (anti-relitigation guard) in the graph.
//
// It reuses the SAME tokenization (claimTokens), opposite-marker detection
// (markerHits), and overlap thresholds (MinLexicalOverlapTokens /
// MinLexicalOverlapTokensWithMarker) as InspectLexicalClaimOverlap — the only
// difference is that one side of the comparison is an external string rather
// than a second graph node, so the "different owner" branch of the inspect
// heuristic does not apply (a candidate has no owner). What remains is the
// signal that matters for CONFRONT: significant shared tokens, optionally
// strengthened by an opposite marker (never/always, must/must not, only/any).
//
// The result is ALWAYS advisory (R-ai-presents-not-decides): confront never
// gates, never decides, and never changes exit code. It gives the operator a
// deterministic shortlist of "looks like X already" before anything is written.
type ConfrontHit struct {
	ID         string   `json:"id"`
	Claim      string   `json:"claim"`
	Score      int      `json:"score"`
	Shared     []string `json:"shared"`
	ReplacedBy []string `json:"replaced_by,omitempty"`
}

// ConfrontResult is the full output of one Confront check: the candidate text
// (echoed back so JSON consumers can correlate), the duplicate-suspect hits
// against SETTLED requirements, the re-litigation-suspect hits against REJECTED
// requirements, and a Clear flag that is true iff NO significant overlap was
// found on either side (the "green light to propose" signal).
type ConfrontResult struct {
	Candidate string        `json:"candidate"`
	Settled   []ConfrontHit `json:"settled"`
	Rejected  []ConfrontHit `json:"rejected"`
	Clear     bool          `json:"clear"`
}

// Confront checks candidateText for lexical overlap with the SETTLED and
// REJECTED requirements of g. A hit fires when the shared-significant-token
// count reaches the same threshold InspectLexicalClaimOverlap uses
// (MinLexicalOverlapTokens, lowered to MinLexicalOverlapTokensWithMarker when
// an opposite marker is also present). REJECTED hits additionally carry any
// known REPLACES successor (via ontology.ReplacesMap) so the operator can cite
// the replacement instead of re-deriving the rejected idea.
func Confront(g *ontology.Graph, candidateText string) ConfrontResult {
	candTokens := claimTokens(candidateText)
	candLower := strings.ToLower(candidateText)
	candMarks := markerHits(candLower)

	replaces := ontology.ReplacesMap(g)

	var settled, rejected []ConfrontHit
	for _, r := range g.Requirements {
		switch r.Status {
		case ontology.StatusSETTLED, ontology.StatusREJECTED:
		default:
			continue
		}
		hit := confrontHit(candTokens, candMarks, r)
		if hit == nil {
			continue
		}
		if r.Status == ontology.StatusREJECTED {
			if succ, ok := replaces[r.ID]; ok && len(succ) > 0 {
				cp := make([]string, len(succ))
				copy(cp, succ)
				hit.ReplacedBy = cp
			}
			rejected = append(rejected, *hit)
		} else {
			settled = append(settled, *hit)
		}
	}

	sortConfrontHits(settled)
	sortConfrontHits(rejected)

	return ConfrontResult{
		Candidate: candidateText,
		Settled:   settled,
		Rejected:  rejected,
		Clear:     len(settled) == 0 && len(rejected) == 0,
	}
}

// confrontHit returns a populated *ConfrontHit for candidate vs r when the
// overlap clears the inspect threshold, or nil when it does not. The threshold
// and scoring mirror InspectLexicalClaimOverlap exactly (minus the
// different-owner term, which is undefined for an owner-less candidate): the
// overlap bar is MinLexicalOverlapTokens (2) normally, lowered to
// MinLexicalOverlapTokensWithMarker (1) when an opposite marker is present.
func confrontHit(candTokens map[string]struct{}, candMarks map[string]string, r ontology.Requirement) *ConfrontHit {
	reqTokens := claimTokens(r.Claim)
	var shared []string
	for t := range candTokens {
		if _, ok := reqTokens[t]; ok {
			shared = append(shared, t)
		}
	}
	sort.Strings(shared)

	opposite := oppositeMarkerBetween(candMarks, markerHits(strings.ToLower(r.Claim)))

	threshold := MinLexicalOverlapTokens
	if opposite != "" {
		threshold = MinLexicalOverlapTokensWithMarker
	}
	if len(shared) < threshold {
		return nil
	}

	score := len(shared)
	if opposite != "" {
		score += 3
	}
	return &ConfrontHit{
		ID:     r.ID,
		Claim:  r.Claim,
		Score:  score,
		Shared: shared,
	}
}

// oppositeMarkerBetween returns the human-readable "a vs b" label for the first
// oppositeMarkerPair whose two sides are split across the two mark maps (one
// side in a, the other in b), or "" when no such split exists. It is the same
// comparison InspectLexicalClaimOverlap inlines over two settled requirements.
func oppositeMarkerBetween(a, b map[string]string) string {
	for key, sideA := range a {
		sideB, ok := b[key]
		if !ok || sideB == sideA {
			continue
		}
		parts := strings.SplitN(key, "|", 2)
		if len(parts) == 2 {
			return parts[0] + " vs " + parts[1]
		}
	}
	return ""
}

func sortConfrontHits(hits []ConfrontHit) {
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].ID < hits[j].ID
	})
}
