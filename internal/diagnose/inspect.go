package diagnose

import (
	"fmt"
	"math"
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

// SharedAssumptionClusterBaselineScore is the score floor for the minimal
// InspectSharedAssumptionClusters case: exactly ONE pair (2 requirements
// sharing one specific, non-generic assumption — see
// ontology.GenericAssumptionThreshold — with no mediating Conflict node).
// Re-examined against InspectLexicalClaimOverlap's own scale (task's
// explicit ask; that heuristic's formula itself stays untouched): a shared
// assumption ID is an EXACT structural match (same node, referenced by both
// requirements), not a fuzzy token co-occurrence — at minimum comparable to
// InspectLexicalClaimOverlap's plain 2-token/different-owner floor (score
// 2+1=3), but deliberately kept BELOW defaultInspectMinScore=5 at the
// 1-pair floor: a single shared assumption between exactly two requirements
// is real but still thin evidence on its own (it says "these two lean on the
// same fact," not "these two actively entangle several others around that
// fact"). Cluster size (Pairs count) is where the genuine density gradient
// lives — see clusterDensityBonus below — so a cluster only needs to grow
// past the single-pair floor to clear the default threshold, rather than
// scoring len(cl.Pairs) directly the way the pre-fix formula did (which
// scored the SAME minimal single-pair case at 1, an order of magnitude below
// even a weak lexical hit, despite being an exact rather than fuzzy match).
const SharedAssumptionClusterBaselineScore = 4

// clusterDensityBonus rewards a cluster growing beyond the minimal single
// contributing pair: each additional pair means one more requirement (or one
// more connection among the same requirement set) tangled around the same
// specific shared assumption — a broader latent connector, not just a
// coincidental one-off link. Floors at 0 so the baseline is never reduced.
func clusterDensityBonus(pairs int) int {
	bonus := pairs - 1
	if bonus < 0 {
		bonus = 0
	}
	return bonus
}

// InspectSharedAssumptionClusters reuses ontology.LatentConnectorClusters —
// the SAME data DiagnoseSignals renders at PLatentConnector priority — and
// reshapes each cluster into a Candidate. No detection logic is duplicated
// here; only the presentation differs (what-now shows one summary line per
// cluster, inspect shows the full cluster + every contributing pair as
// evidence). See SharedAssumptionClusterBaselineScore's doc comment for the
// scoring reasoning.
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
			Score:          SharedAssumptionClusterBaselineScore + clusterDensityBonus(len(cl.Pairs)),
			Recommendation: advice(cl.Requirements),
		})
	}
	return out
}

// EntityStateConflictBaselineScore is the flat score every
// InspectEntityStateConflicts candidate carries. ontology.LatentSuspect (the
// type EntityStateConflictSuspects returns) carries no numeric gradient of
// its own — Left/Right/Hint only — so there is no per-pair signal to scale
// against; every suspect is the SAME shape of evidence: two DIFFERENT
// Processes drive the SAME EntityType to disjoint terminal (resting) states
// (see ontology.EntityStateConflictSuspects' disjoint-destination check).
// That is graph-structural fact, not a heuristic guess — two lifecycles
// converging on states that can never agree is a stronger, more concrete
// signal than an N-token lexical overlap, which only gestures at shared
// topic. Calibrated against InspectLexicalClaimOverlap's own scoring scale
// (deliberately not touched by this task, only read for comparison):
// MinLexicalOverlapTokens=2 (no marker, different owner) scores 2+1=3;
// MinLexicalOverlapTokensWithMarker=1 (with an opposite marker) scores
// 1+3+[+1 if different owner]=4-5. A same-entity/disjoint-destination
// structural suspect is set one point above that ceiling (6) — strong enough
// to reliably clear defaultInspectMinScore=5 out of the box (it is real
// structure, not inferred prose), without being so high it would swamp a
// high-scoring lexical hit (marker + multi-token overlap can still exceed
// it). No real domain in this repo currently has an EntityType driven by 2+
// Processes to exercise this path with live data (hotam-dev has 1 EntityType
// / 0 Processes; hotam-spec-self has 0 EntityTypes) — the baseline is
// therefore a reasoned constant, not empirically fit to a candidate set that
// does not yet exist; revisit with real measurements once a domain grows a
// Process-driven EntityType with branching resting states.
const EntityStateConflictBaselineScore = 6

// InspectEntityStateConflicts reuses ontology.EntityStateConflictSuspects —
// the same detector DiagnoseSignals renders at PLatentConnector priority —
// and reshapes each suspect pair into a Candidate. See
// EntityStateConflictBaselineScore's doc comment for why every candidate
// carries the same flat score.
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
			Score:          EntityStateConflictBaselineScore,
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

// tokenRE tokenizes on Unicode letters/digits (\p{L}\p{N}), not just ASCII
// [a-z0-9]. This is a necessary companion to the caps-token marker fix
// below: IsBlockingHit (blocking_hit.go) requires an opposite marker AND at
// least one shared token that is NOT itself a marker word (the "topical
// anchor" requirement — see its doc comment) before a hit can BLOCK a
// land/apply. An ASCII-only tokenizer made every pure-Cyrillic claim
// tokenize to the empty set, so two Russian claims sharing an obvious
// topical anchor ("Экспорт записей") would still score zero shared tokens
// and never clear IsBlockingHit's topical-anchor bar, no matter how strong
// the caps-token opposite-marker signal was — silently defeating the whole
// point of this task's Russian-claim blocking demonstration (task
// #156/R10-b). \p{L}\p{N} is a strict superset of [a-z0-9] for tokenization
// purposes (every ASCII letter/digit is also \p{L}/\p{N}), so this is
// additive: no existing English-only claim's token set changes.
var tokenRE = regexp.MustCompile(`[\p{L}\p{N}]+`)

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
//
// Anti-false-negative validation (task #99, 2026-07-13): checked all 8 real
// steward-confirmed Conflict node member pairs in hotam-spec-self's own graph
// against InspectLexicalClaimOverlap's output. 7 of 8 never fired via lexical
// overlap even with this filter fully disabled (they were surfaced by the
// steward through other channels — see R-tension-audit-shortlist-tool's own
// "0 of 8 conflicts machine-surfaced" history note). Exactly ONE pair,
// C-c3911f28 (R-content-free-framework + R-empty-content-is-legitimate), is a
// genuine false negative caused by this filter: its 3 shared tokens
// ("content" 5.5%, "graph" 19.9%, "spec" 7.6% document frequency) all clear
// the 5% ceiling, so post-filter overlap drops to 0. A threshold sweep showed
// rescuing it is not worth the cost — raising the ceiling to 0.06 (to let
// "content" back in) alone grows total candidates from 232 to 316 (+36%);
// fully rescuing it needs 0.20, which explodes candidates to 1186, undoing
// nearly all of the 1231→234 noise fix. Accepted as a documented tradeoff:
// a Conflict node is already graph ground truth (no heuristic needs to
// "discover" it), and inspect's stated purpose is surfacing NEW undiscovered
// tension candidates, not reproducing every already-known Conflict. Pinned
// by TestInspectLexicalClaimOverlap_KnownConflictGroundTruth in
// inspect_test.go, which fails loud if a NEW Conflict pair silently joins the
// miss set beyond this manually-verified allow-list.
const CorpusCommonTokenFraction = 0.05

// MinCorpusSizeForFrequencyFilter is the minimum number of SETTLED claims a
// graph must carry before corpus-frequency exclusion (corpusCommonTokens)
// activates. Below this size, callers fall back to plain stop-word-only
// filtering (claimTokens' original, still-correct behavior).
//
// This is NOT an arbitrary "smaller sample = noisier" round number — it is
// derived from CorpusCommonTokenFraction because the exclusion test itself
// (float64(n) > ceiling, where ceiling = CorpusCommonTokenFraction *
// len(settled)) is mathematically incapable of leaving even a single token
// un-excluded until len(settled) is large enough for ceiling to reach 1.0.
// Below that point, ANY token occurring in just ONE SETTLED requirement
// (n=1) already satisfies n > ceiling and gets excluded as "corpus-common" —
// which, since every content token appears in at least one claim by
// definition, means the filter wipes out 100% of the corpus vocabulary, not
// occasionally but as a guaranteed consequence of the formula. Concretely,
// with CorpusCommonTokenFraction=0.05, any corpus with 8-19 SETTLED
// requirements had ceiling < 1.0 (e.g. N=13 -> ceiling=0.65), so n=1 > 0.65
// excluded EVERY token — silently defeating both InspectLexicalClaimOverlap
// and, more seriously, the semantic-conflict land-time gate
// (IsBlockingHit's topical-shared-token requirement in blocking_hit.go),
// which could never find a surviving topical token to block on. Bug found
// while pilot-testing against an external domain with exactly 13 SETTLED
// requirements (squarely in the hole); hotam-dev (this repo's own
// self-hosted secondary domain, 9 SETTLED) sat in the same hole.
//
// ceiling first reaches >= 1.0 (so a single-occurrence token, n=1, no longer
// automatically clears n > ceiling) at len(settled) =
// ceil(1/CorpusCommonTokenFraction) = 20. Deriving the guard from the
// fraction (rather than a bare literal 20) keeps the two constants from
// silently desyncing if CorpusCommonTokenFraction is ever retuned.
var MinCorpusSizeForFrequencyFilter = int(math.Ceil(1.0 / CorpusCommonTokenFraction))

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

// wordBoundaryRE compiles pattern into a whole-word (ASCII word-boundary
// aware) matcher: the pattern must not be immediately preceded or followed
// by an ASCII letter or digit. This is what fixes the substring
// false-positive bug — plain strings.Contains(lowerClaim, "any") matches
// inside "company", "many", "anybody"; a `\b`-anchored regexp does not,
// because Go's RE2 `\b` is itself an ASCII-word-boundary (it does not treat
// non-ASCII letters, e.g. Cyrillic, as word characters at all — this same
// property is deliberately reused by capsTokenRE below, where a caps token
// glued to Cyrillic on either side must still match).
func wordBoundaryRE(pattern string) *regexp.Regexp {
	return regexp.MustCompile(`\b` + pattern + `\b`)
}

// lowerMarkerRE holds the compiled whole-word matchers for the lowercase
// English heuristic, keyed the same way as oppositeMarkerPairs' literal
// strings (spaces inside a phrase like "must not" become `\s+` so minor
// whitespace variation still matches; "mustn't" is handled separately in
// markerHits, unchanged from before).
var lowerMarkerRE = map[string]*regexp.Regexp{
	"never":    wordBoundaryRE(`never`),
	"always":   wordBoundaryRE(`always`),
	"must":     wordBoundaryRE(`must`),
	"must not": wordBoundaryRE(`must\s+not`),
	"only":     wordBoundaryRE(`only`),
	"any":      wordBoundaryRE(`any`),
}

// capsTokenRE holds the compiled whole-word matchers for the ADDITIVE,
// language-independent reserved-token pass (see capsTokenMarkerHits' doc
// comment): exact-case ALL-CAPS ASCII tokens, matched with the same `\b`
// word-boundary rule as lowerMarkerRE — since `\b` is ASCII-word-aware, a
// token immediately adjacent to another ASCII letter/digit (e.g.
// "ALWAYSNESS") does NOT match, while a token glued to a non-ASCII letter
// (e.g. Cyrillic "ВСЕГДАALWAYS") DOES match, because Cyrillic is not an
// ASCII word character as far as RE2's `\b` is concerned.
var capsTokenRE = map[string]*regexp.Regexp{
	"NEVER":    wordBoundaryRE(`NEVER`),
	"ALWAYS":   wordBoundaryRE(`ALWAYS`),
	"MUST":     wordBoundaryRE(`MUST`),
	"MUST NOT": wordBoundaryRE(`MUST\s+NOT`),
	"ONLY":     wordBoundaryRE(`ONLY`),
	"ANY":      wordBoundaryRE(`ANY`),
}

// capsTokenMarkerHits is the ADDITIVE, language-independent detection pass:
// it scans the ORIGINAL (non-lowercased) claim text for exact-case
// whole-word matches of the six reserved tokens (ALWAYS, NEVER, MUST NOT,
// MUST, ONLY, ANY — "MUST NOT" checked before bare "MUST" so it is not
// double counted as the positive pole, mirroring the existing lowercase
// precedence logic) and returns hits in the same "a|b" -> side shape
// markerHits already produces, so the two signals can be unioned directly.
//
// This is the mechanism behind the meta-language design (steward design
// discussion, task #156/R10-b): a business requirement is authored in plain
// natural language, in ANY language — the reserved CAPS tokens are never
// written by the business author. They are inserted by the AGENT during the
// TRANSLATE step of the mediation loop, when it converts a plain-language
// business claim into a ProposedRequirement's claim text and recognizes a
// hard universal/prohibition modality (always/never/must/must-not/only-any,
// in whatever language the source used). See RenderMediationLoopBlock's
// TRANSLATE step text (claudemd_static.go) for the agent-facing guidance
// this vocabulary exists to support — the same established pattern as RFC
// 2119's MUST/SHALL/SHOULD/MAY reserved keywords embedded in prose, except
// the embedding step here is an agent responsibility, not a human authoring
// convention.
//
// Detection is UNIONED with markerHits' lowercase-English heuristic, never a
// replacement for it: a claim trips a marker pair if EITHER signal fires.
func capsTokenMarkerHits(claim string) map[string]string {
	hits := map[string]string{}
	hasMustNot := capsTokenRE["MUST NOT"].MatchString(claim)
	for _, pair := range oppositeMarkerPairs {
		a, b := pair[0], pair[1]
		capsA, capsB := strings.ToUpper(a), strings.ToUpper(b)
		switch {
		case a == "must" && b == "must not":
			if hasMustNot {
				hits[a+"|"+b] = b
			} else if capsTokenRE["MUST"].MatchString(claim) {
				hits[a+"|"+b] = a
			}
		default:
			hasA := capsTokenRE[capsA].MatchString(claim)
			hasB := capsTokenRE[capsB].MatchString(claim)
			if hasA && !hasB {
				hits[a+"|"+b] = a
			} else if hasB && !hasA {
				hits[a+"|"+b] = b
			}
		}
	}
	return hits
}

// markerHits returns which side of each opposite-marker pair appears in
// claim, as the UNION of two independent signals: the lowercase-English
// heuristic (matched against strings.ToLower(claim), whole-word — see
// wordBoundaryRE for why "company"/"many"/"anybody" no longer
// false-positive-match "any", the substring bug this fixes) and the
// caps-token heuristic (capsTokenMarkerHits, matched against the ORIGINAL
// case, language-independent). "must not"/"MUST NOT" is checked before bare
// "must"/"MUST" in each pass so it isn't double counted as the positive
// pole. When both signals fire for the same pair with the SAME side, the
// lowercase result wins (arbitrary — the sides agree, so it is a no-op
// choice); a caller never observes disagreement between the two signals for
// a single pair on a single claim, since each pair has exactly two sides.
func markerHits(claim string) map[string]string {
	lowerClaim := strings.ToLower(claim)
	hits := map[string]string{}
	hasMustNot := lowerMarkerRE["must not"].MatchString(lowerClaim) || strings.Contains(lowerClaim, "mustn't")
	for _, pair := range oppositeMarkerPairs {
		a, b := pair[0], pair[1]
		switch {
		case a == "must" && b == "must not":
			if hasMustNot {
				hits[a+"|"+b] = b
			} else if lowerMarkerRE["must"].MatchString(lowerClaim) {
				hits[a+"|"+b] = a
			}
		default:
			hasA := lowerMarkerRE[a].MatchString(lowerClaim)
			hasB := lowerMarkerRE[b].MatchString(lowerClaim)
			if hasA && !hasB {
				hits[a+"|"+b] = a
			} else if hasB && !hasA {
				hits[a+"|"+b] = b
			}
		}
	}
	for key, side := range capsTokenMarkerHits(claim) {
		if _, already := hits[key]; !already {
			hits[key] = side
		}
	}
	return hits
}

// MinLexicalOverlapTokens is the minimum shared-significant-token count
// required when the ONLY signal is "different owner" (no opposite marker).
// Kept low deliberately — this is an ADVISORY signal meant to be
// over-inclusive (false positives are cheap: a steward glances and
// dismisses; false negatives silently hide real tension).
//
// Worked example of this tradeoff (task #99, 2026-07-13): `hotam confront
// "The system shall enforce every requirement structurally"` hits
// R-requirement-freshness-fields on shared tokens "system" (2.1% document
// frequency) and "structurally" (0.8%) — both genuinely rare, well under
// CorpusCommonTokenFraction's 5% ceiling, so this is NOT corpus-filter noise
// leaking through. It is a real (if weak, score=2, the minimum) two-token
// overlap correctly surfaced at this deliberately low bar. Confirmed
// defensible rather than tuned away: confront never gates (exit code always
// 0), so the cost of a steward glancing at one weak hit and dismissing it is
// cheap, exactly the tradeoff this comment already commits to.
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
		pre[i] = tokenized{req: r, tokens: claimTokens(r.Claim, common), lower: lower, marks: markerHits(r.Claim)}
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

// AxisCoReferenceBaselineScore is the minimum score any InspectAxisCoReference
// candidate can carry, awarded purely for the structural fact itself: two
// SEPARATE Conflict nodes — each already a steward-attended, first-class
// connector node in the graph, not a heuristic guess — reference the SAME
// Axis. That is graph ground truth, arguably stronger evidence of real
// tension-worth-a-look than any token-overlap heuristic can produce, since no
// inference step is involved at all (the co-reference either exists in the
// graph or it doesn't). Every real Conflict has at least 2 Members (a
// "conflict" needs at least a pair to connect), so the smallest possible
// pair of co-referencing Conflicts carries 2+2=4 combined members — that
// floor case is deliberately set to land exactly AT defaultInspectMinScore=5
// (baseline 5, +0 extra members at the floor), so this structural signal is
// visible by default rather than accidentally always suppressed the way it
// was before this fix (Score was unset / always 0). See
// AxisCoReferenceMemberBonus below for how broader entanglement scores
// higher than the floor.
const AxisCoReferenceBaselineScore = 5

// AxisCoReferenceMemberFloor is the minimum combined Members count
// (len(c1.Members)+len(c2.Members)) any two real Conflict nodes can carry —
// 2 members each is the smallest a Conflict can be. Combined member count
// above this floor is extra entanglement breadth beyond the minimum
// structural case, and is rewarded 1 score point per extra member (more
// requirements pulled into the two co-referencing conflicts = a broader,
// more consequential tangle worth a steward's attention sooner).
const AxisCoReferenceMemberFloor = 4

// axisCoReferenceScore combines AxisCoReferenceBaselineScore (awarded for the
// bare structural fact) with a 1-point-per-member bonus for combined Conflict
// membership beyond AxisCoReferenceMemberFloor, so two co-referencing
// Conflicts entangling many requirements score visibly higher than the
// minimal 2-member+2-member floor case, while never scoring below the
// baseline.
func axisCoReferenceScore(c1, c2 ontology.Conflict) int {
	combined := len(c1.Members) + len(c2.Members)
	bonus := combined - AxisCoReferenceMemberFloor
	if bonus < 0 {
		bonus = 0
	}
	return AxisCoReferenceBaselineScore + bonus
}

// InspectAxisCoReference is heuristic (c) from the task: requirements that
// are members of DIFFERENT Conflict nodes which nonetheless share the same
// Axis are "co-referencing" one tension dimension from separate connector
// nodes — worth a steward glance to decide whether they are really one
// conflict split in two, or genuinely independent tensions that happen to
// share a vocabulary axis. See AxisCoReferenceBaselineScore's doc comment
// for the scoring reasoning.
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
					Score:          axisCoReferenceScore(c1, c2),
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
