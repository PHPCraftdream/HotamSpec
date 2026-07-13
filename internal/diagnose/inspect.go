package diagnose

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// Package-level doc for the `inspect` advisory layer: all-violations==0 only
// proves STRUCTURAL correctness (invariants.AllViolations). Whether two
// natural-language SETTLED claims actually contradict each other is a
// semantic question no structural check can answer; this file collects every
// heuristic that gestures at "these two might be in tension" into typed,
// evidence-carrying Candidate records. A Candidate is ALWAYS advisory: it
// never blocks a write, never changes exit code, and never decides anything
// (R-ai-presents-not-decides) — it is raw material for a steward or an
// agent's judgment, surfaced by `hotam inspect`.

// InspectHeuristic names the detection method that produced a Candidate, so
// callers can filter/group without parsing the evidence prose.
const (
	HeuristicSharedAssumptionCluster = "shared_assumption_cluster"
	HeuristicEntityStateConflict     = "entity_state_conflict"
	HeuristicLexicalClaimOverlap     = "lexical_claim_overlap"
	HeuristicAxisCoReference         = "axis_co_reference"
)

// Candidate is one conflict-candidate surfaced by `hotam inspect`: a pair or
// cluster of node ids, the heuristic that flagged them, the evidence that
// justifies the flag, and a fixed advisory recommendation. Exit code is
// always 0 regardless of how many candidates are found — inspect informs,
// it never gates (unlike `all-violations`/`gate`).
type Candidate struct {
	ID             string   `json:"id"`
	Heuristic      string   `json:"heuristic"`
	Members        []string `json:"members"`
	Evidence       string   `json:"evidence"`
	Score          int      `json:"score,omitempty"`
	Recommendation string   `json:"recommendation"`
}

func advice(members []string) string {
	return fmt.Sprintf(
		"ADVISORY: consider a ProposedConflict with axis=<pick one>, members=[%s] — this is ADVISORY only, the steward decides (R-ai-presents-not-decides, R-decided-needs-human-signoff).",
		strings.Join(members, ", "),
	)
}

// InspectSharedAssumptionClusters reuses ontology.LatentConnectorClusters —
// the SAME data DiagnoseSignals renders at PLatentConnector priority — and
// reshapes each cluster into a Candidate. No detection logic is duplicated
// here; only the presentation differs (what-now shows one summary line per
// cluster, inspect shows the full cluster + every contributing pair as
// evidence).
func InspectSharedAssumptionClusters(g *ontology.Graph) []Candidate {
	clusters := ontology.LatentConnectorClusters(g)
	out := make([]Candidate, 0, len(clusters))
	for i, cl := range clusters {
		var pairEvidence []string
		for _, p := range cl.Pairs {
			pairEvidence = append(pairEvidence, p.Left+"~"+p.Right)
		}
		out = append(out, Candidate{
			ID:        candidateID(HeuristicSharedAssumptionCluster, i, cl.Requirements),
			Heuristic: HeuristicSharedAssumptionCluster,
			Members:   cl.Requirements,
			Evidence: "shares assumption(s) [" + strings.Join(cl.Assumptions, ", ") +
				"] with no mediating Conflict node; contributing pairs: " + strings.Join(pairEvidence, ", "),
			Score:          len(cl.Pairs),
			Recommendation: advice(cl.Requirements),
		})
	}
	return out
}

// InspectEntityStateConflicts reuses ontology.EntityStateConflictSuspects —
// the same detector DiagnoseSignals renders at PLatentConnector priority —
// and reshapes each suspect pair into a Candidate.
func InspectEntityStateConflicts(g *ontology.Graph) []Candidate {
	suspects := ontology.EntityStateConflictSuspects(g)
	out := make([]Candidate, 0, len(suspects))
	for i, s := range suspects {
		members := []string{s.Left, s.Right}
		out = append(out, Candidate{
			ID:             candidateID(HeuristicEntityStateConflict, i, members),
			Heuristic:      HeuristicEntityStateConflict,
			Members:        members,
			Evidence:       s.Hint,
			Recommendation: advice(members),
		})
	}
	return out
}

// stopWords is a small, fixed English stop-word list for the lexical
// claim-overlap heuristic below. Stemming is deliberately NOT applied (task
// scope: "стемминг не нужен") — only lowercasing + stop-word removal. This
// list is deliberately GENERIC English closed-class vocabulary (articles,
// conjunctions, modal verbs) — it carries no business/domain content, so it
// stays framework-appropriate regardless of which domain graph is loaded
// (R-content-free-no-business-data). It catches the same noise in every
// domain; it does NOT and cannot catch a domain's own frequently-repeated
// OPEN-class nouns/verbs (e.g. "requirement", "enforce", "graph" in this
// project's own self-describing corpus) — that is what corpusCommonTokens
// below is for.
var stopWords = map[string]struct{}{
	"a": {}, "an": {}, "the": {}, "is": {}, "are": {}, "was": {}, "were": {},
	"be": {}, "been": {}, "being": {}, "to": {}, "of": {}, "in": {}, "on": {},
	"for": {}, "with": {}, "and": {}, "or": {}, "but": {}, "as": {}, "at": {},
	"by": {}, "from": {}, "that": {}, "this": {}, "these": {}, "those": {},
	"it": {}, "its": {}, "into": {}, "than": {}, "then": {}, "so": {}, "if": {},
	"when": {}, "while": {}, "shall": {}, "should": {}, "will": {}, "would": {},
	"can": {}, "could": {}, "may": {}, "might": {}, "not": {}, "no": {},
}

var tokenRE = regexp.MustCompile(`[a-z0-9]+`)

// MinCorpusSizeForFrequencyFilter is the minimum number of SETTLED claims a
// graph must carry before corpus-frequency exclusion (corpusCommonTokens)
// activates. Below this size, "frequency" is not a meaningful signal — a
// 2-3-requirement test fixture would have every shared token look
// "common" (document frequency 50-100%) purely from small-sample noise, which
// would silently zero out the token sets the existing table-driven tests
// depend on. Below the guard, callers fall back to plain stop-word-only
// filtering (claimTokens' original, still-correct behavior).
const MinCorpusSizeForFrequencyFilter = 8

// CorpusCommonTokenFraction is the document-frequency ceiling above which a
// token is treated as "corpus-common" and excluded from lexical-overlap
// scoring, exactly like a stop word. A token appearing in more than this
// fraction of SETTLED claims is, empirically, not a discriminative topical
// anchor for THIS domain — it is domain-frequent connective tissue (measured
// on hotam-spec-self's own 234 SETTLED requirements: "every" 26.5%, "graph"
// 19.7%, "operator" 19.7%, "requirement" 12.4%, "steward" 7.3%, "proposal"
// 6.4% all clear this ceiling, while genuinely narrower topical words like
// "system" 2.1% or "budget" 3.0% stay under it). This is computed fresh from
// whatever graph is loaded at call time — it is NOT a hardcoded word list, so
// it adapts automatically to any domain's own vocabulary without baking
// business content into framework source (R-content-free-no-business-data: a
// fixed list of THIS domain's frequent nouns living in internal/diagnose
// would itself be exactly the kind of content-in-framework-source violation
// batch A4 closed enforcement debt on). 5% was chosen empirically against the
// real hotam-spec-self graph: it cleared the noise the review flagged
// (lexical_claim_overlap candidates dropped 1231→234, spot-checked to
// confirm the survivors keep genuinely narrow shared vocabulary — see
// inspect_test.go's TestCorpusCommonTokens_* for the regression pin).
const CorpusCommonTokenFraction = 0.05

// corpusCommonTokens computes, from the SETTLED requirements of g, the set of
// tokens whose document frequency (fraction of SETTLED claims that contain
// the token at least once) exceeds CorpusCommonTokenFraction. Returns an
// empty (non-nil) set when the corpus has fewer than
// MinCorpusSizeForFrequencyFilter SETTLED requirements — see that constant's
// doc comment for why. The result is a pure function of the graph passed in;
// nothing here is baked into the framework's source across domains.
func corpusCommonTokens(g *ontology.Graph) map[string]struct{} {
	var settled []ontology.Requirement
	for _, r := range g.Requirements {
		if r.Status == ontology.StatusSETTLED {
			settled = append(settled, r)
		}
	}
	common := map[string]struct{}{}
	if len(settled) < MinCorpusSizeForFrequencyFilter {
		return common
	}
	docFreq := map[string]int{}
	for _, r := range settled {
		seen := map[string]struct{}{}
		for _, tok := range tokenRE.FindAllString(strings.ToLower(r.Claim), -1) {
			if _, stop := stopWords[tok]; stop {
				continue
			}
			if len(tok) < 3 {
				continue
			}
			seen[tok] = struct{}{}
		}
		for tok := range seen {
			docFreq[tok]++
		}
	}
	ceiling := CorpusCommonTokenFraction * float64(len(settled))
	for tok, n := range docFreq {
		if float64(n) > ceiling {
			common[tok] = struct{}{}
		}
	}
	return common
}

// claimTokens normalizes a claim string into a set of significant tokens:
// lowercase, split on non-alphanumeric, stop words dropped, and any token in
// common (corpus-common tokens computed by corpusCommonTokens, or nil to skip
// that layer) also dropped. No stemming. Corpus-common exclusion is an
// EXTRA filter layer beyond stopWords, not a replacement for it: a token can
// be domain-common without being a generic English stop word, and vice
// versa.
func claimTokens(claim string, common map[string]struct{}) map[string]struct{} {
	out := map[string]struct{}{}
	for _, tok := range tokenRE.FindAllString(strings.ToLower(claim), -1) {
		if _, stop := stopWords[tok]; stop {
			continue
		}
		if len(tok) < 3 {
			continue
		}
		if _, isCommon := common[tok]; isCommon {
			continue
		}
		out[tok] = struct{}{}
	}
	return out
}

// oppositeMarkerPairs is the controlled vocabulary of lexical markers whose
// PRESENCE-on-one-side-ABSENCE(or-opposite)-on-the-other is itself evidence
// of tension, independent of token overlap size — e.g. "cache must never
// store PII" vs "cache stores all fields" share few tokens but one asserts
// universal prohibition where the other asserts unconditional inclusion.
var oppositeMarkerPairs = [][2]string{
	{"never", "always"},
	{"must", "must not"},
	{"only", "any"},
}

// markerHits returns which side of each opposite-marker pair appears in the
// (already-lowercased) claim text. "must not" is checked before bare "must"
// so it isn't double counted as the positive pole.
func markerHits(lowerClaim string) map[string]string {
	hits := map[string]string{}
	hasMustNot := strings.Contains(lowerClaim, "must not") || strings.Contains(lowerClaim, "mustn't")
	for _, pair := range oppositeMarkerPairs {
		a, b := pair[0], pair[1]
		switch {
		case a == "must" && b == "must not":
			if hasMustNot {
				hits[a+"|"+b] = b
			} else if strings.Contains(lowerClaim, "must") {
				hits[a+"|"+b] = a
			}
		default:
			hasA := strings.Contains(lowerClaim, a)
			hasB := strings.Contains(lowerClaim, b)
			if hasA && !hasB {
				hits[a+"|"+b] = a
			} else if hasB && !hasA {
				hits[a+"|"+b] = b
			}
		}
	}
	return hits
}

// MinLexicalOverlapTokens is the minimum shared-significant-token count
// required when the ONLY signal is "different owner" (no opposite marker).
// Kept low deliberately — this is an ADVISORY signal meant to be
// over-inclusive (false positives are cheap: a steward glances and
// dismisses; false negatives silently hide real tension).
const MinLexicalOverlapTokens = 2

// MinLexicalOverlapTokensWithMarker is the (lower) minimum shared-token
// count required when an opposite marker (never/always, must/must not,
// only/any) is ALSO present. A single strong topical anchor is enough once
// the opposite-marker signal itself is doing most of the work — this is
// exactly the task's own canonical example: "cache must never store PII"
// vs "cache stores all fields" share only the single token "cache".
const MinLexicalOverlapTokensWithMarker = 1

// InspectLexicalClaimOverlap is heuristic (b) from the task: for every pair
// of SETTLED requirements, normalize their claims to token sets and flag the
// pair when they share a significant number of tokens AND (1) have
// different owners, OR (2) use opposite markers from oppositeMarkerPairs
// (never/always, must/must not, only/any). The token-overlap bar is lower
// when a marker hit is present (see MinLexicalOverlapTokensWithMarker),
// since the marker itself is strong evidence. Each hit becomes a Candidate
// carrying the shared tokens / marker words as evidence.
//
// Tokens are additionally filtered through corpusCommonTokens(g) — a
// document-frequency exclusion computed FRESH from this graph's own SETTLED
// claims (see its doc comment). This is what keeps this heuristic from
// mistaking two claims that share only domain-frequent connective vocabulary
// (e.g. "requirement", "graph", "enforce" in a methodology describing
// itself) for a genuine topical anchor: those tokens are dropped from BOTH
// the overlap-count gate and the score before pairwise comparison begins, so
// a pair surviving purely on generic domain nouns no longer clears the
// MinLexicalOverlapTokens bar at all (not just "scores lower"). This does
// NOT guarantee zero false negatives: a real tension whose ONLY shared
// vocabulary happens to be corpus-common words, with no opposite marker and
// no rarer shared token, will still be missed — exactly the same class of
// miss the plain stop-word list already accepted for generic English words,
// now extended to this domain's own frequent words. Below
// MinCorpusSizeForFrequencyFilter SETTLED requirements this exclusion is a
// no-op (empty set) and behavior is identical to stop-words-only.
func InspectLexicalClaimOverlap(g *ontology.Graph) []Candidate {
	common := corpusCommonTokens(g)
	var settled []ontology.Requirement
	for _, r := range g.Requirements {
		if r.Status == ontology.StatusSETTLED {
			settled = append(settled, r)
		}
	}
	sort.Slice(settled, func(i, j int) bool { return settled[i].ID < settled[j].ID })

	type tokenized struct {
		req    ontology.Requirement
		tokens map[string]struct{}
		lower  string
		marks  map[string]string
	}
	pre := make([]tokenized, len(settled))
	for i, r := range settled {
		lower := strings.ToLower(r.Claim)
		pre[i] = tokenized{req: r, tokens: claimTokens(r.Claim, common), lower: lower, marks: markerHits(lower)}
	}

	var out []Candidate
	for i := 0; i < len(pre); i++ {
		for j := i + 1; j < len(pre); j++ {
			a, b := pre[i], pre[j]
			var shared []string
			for t := range a.tokens {
				if _, ok := b.tokens[t]; ok {
					shared = append(shared, t)
				}
			}
			sort.Strings(shared)

			var oppositeMarker string
			for key, sideA := range a.marks {
				sideB, ok := b.marks[key]
				if !ok || sideB == sideA {
					continue
				}
				parts := strings.SplitN(key, "|", 2)
				oppositeMarker = parts[0] + " vs " + parts[1]
				break
			}

			differentOwner := a.req.Owner != b.req.Owner
			hasMarker := oppositeMarker != ""

			// Task spec (b): significant token overlap AND (different owner
			// OR opposite markers). Token overlap is still the gate in both
			// branches — it just takes fewer shared tokens to count as
			// "significant" when an opposite marker is also present, since
			// the marker itself is strong topical-anchor evidence (task's
			// own example: "cache must never store PII" vs "cache stores
			// all fields" share only the single token "cache").
			threshold := MinLexicalOverlapTokens
			if hasMarker {
				threshold = MinLexicalOverlapTokensWithMarker
			}
			enoughOverlap := len(shared) >= threshold

			fire := enoughOverlap && (differentOwner || hasMarker)
			if !fire {
				continue
			}

			members := []string{a.req.ID, b.req.ID}
			var evidence strings.Builder
			evidence.WriteString("shared claim tokens: [" + strings.Join(shared, ", ") + "]")
			if differentOwner {
				evidence.WriteString("; different owners: " + a.req.Owner + " vs " + b.req.Owner)
			}
			if oppositeMarker != "" {
				evidence.WriteString("; opposite markers: " + oppositeMarker)
			}

			score := len(shared)
			if oppositeMarker != "" {
				score += 3
			}
			if differentOwner {
				score++
			}

			out = append(out, Candidate{
				ID:             candidateID(HeuristicLexicalClaimOverlap, len(out), members),
				Heuristic:      HeuristicLexicalClaimOverlap,
				Members:        members,
				Evidence:       evidence.String(),
				Score:          score,
				Recommendation: advice(members),
			})
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// InspectAxisCoReference is heuristic (c) from the task: requirements that
// are members of DIFFERENT Conflict nodes which nonetheless share the same
// Axis are "co-referencing" one tension dimension from separate connector
// nodes — worth a steward glance to decide whether they are really one
// conflict split in two, or genuinely independent tensions that happen to
// share a vocabulary axis.
func InspectAxisCoReference(g *ontology.Graph) []Candidate {
	byAxis := ontology.ConflictsByAxis(g)

	axes := make([]string, 0, len(byAxis))
	for axis := range byAxis {
		axes = append(axes, axis)
	}
	sort.Strings(axes)

	var out []Candidate
	for _, axis := range axes {
		conflicts := byAxis[axis]
		if len(conflicts) < 2 {
			continue
		}
		sorted := append([]ontology.Conflict(nil), conflicts...)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })

		for i := 0; i < len(sorted); i++ {
			for j := i + 1; j < len(sorted); j++ {
				c1, c2 := sorted[i], sorted[j]
				members := []string{c1.ID, c2.ID}
				out = append(out, Candidate{
					ID:        candidateID(HeuristicAxisCoReference, len(out), members),
					Heuristic: HeuristicAxisCoReference,
					Members:   members,
					Evidence: "conflicts '" + c1.ID + "' (members: " + strings.Join(c1.Members, ", ") +
						") and '" + c2.ID + "' (members: " + strings.Join(c2.Members, ", ") +
						") both reference axis '" + axis + "' but are separate Conflict nodes",
					Recommendation: advice(members),
				})
			}
		}
	}
	return out
}

// AllCandidates runs every inspect heuristic in a fixed order — shared
// assumption clusters, entity-state suspects, lexical claim overlap, axis
// co-reference — and concatenates the results. Callers that want a subset
// (e.g. `hotam inspect --domain`) can filter afterward by Heuristic; nothing
// here decides relevance for them.
func AllCandidates(g *ontology.Graph) []Candidate {
	var out []Candidate
	out = append(out, InspectSharedAssumptionClusters(g)...)
	out = append(out, InspectEntityStateConflicts(g)...)
	out = append(out, InspectLexicalClaimOverlap(g)...)
	out = append(out, InspectAxisCoReference(g)...)
	return out
}

func candidateID(heuristic string, index int, members []string) string {
	sorted := append([]string(nil), members...)
	sort.Strings(sorted)
	return heuristic + "#" + strings.Join(sorted, "+")
}
