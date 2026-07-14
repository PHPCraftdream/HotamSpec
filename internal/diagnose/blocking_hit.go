package diagnose

import "strings"

// IsBlockingHit reports whether a ConfrontHit represents a high-confidence
// semantic conflict that should block a land/apply. A hit blocks when it
// carries an opposite marker (never/always, must/must not, only/any) AND has
// at least one shared token that is NOT itself a marker word.
//
// This is the EXACT relocation of the logic cmd/hotam/semantic_gate.go
// previously inlined as markerWordSet + hasTopicalSharedToken, lifted into
// internal/diagnose so internal/proposal.ApplyBatch — which cannot import
// cmd/hotam (wrong dependency direction) but CAN import internal/diagnose —
// runs the SAME blocking check the single-file semanticConflictGate runs. The
// single-file gate now calls this function too, so "what counts as a blocking
// hit" is defined in exactly one place.
//
// Why opposite-marker + topical token (not raw score):
//
//   - An opposite marker is a PRECISE indicator of genuine semantic
//     contradiction — one side asserts a universal where the other asserts a
//     prohibition (the canonical worked example: "always encrypt" vs
//     "never encrypt").
//
//   - A high token-overlap score WITHOUT an opposite marker is often just
//     "these two requirements are about the same subject" (relatedness, not
//     contradiction). Using raw score alone would generate false positives on
//     every pair of requirements that share domain vocabulary but agree.
//
//   - The topical-token requirement is what keeps the "must/must not" pair from
//     firing on every pair where one says "must" and the other "must not":
//     "must" is standard requirement prose (nearly every claim uses it), so
//     sharing only "must" proves relatedness to the MARKER, not to the SUBJECT.
//     A genuine contradiction shares a topical anchor (encrypt, cache, export,
//     …) in ADDITION to the opposing polarity.
func IsBlockingHit(h ConfrontHit) bool {
	return h.HasOppositeMarker() && hasTopicalSharedToken(h)
}

// markerWordSet extracts the individual words from an opposite-marker label
// like "must vs must not" → {"must", "not"}, or "never vs always" →
// {"never", "always"}. These are the words whose presence in BOTH claims is
// already accounted for by the opposite-marker signal itself, so they should
// not ALSO count toward the shared-token overlap (doing so would let "must"
// — a near-universal modal in requirement prose — be the sole shared token
// for a "must/must not" hit, firing the gate on unrelated claims).
func markerWordSet(oppositeMarker string) map[string]struct{} {
	set := map[string]struct{}{}
	for _, part := range strings.Split(oppositeMarker, " vs ") {
		for _, w := range strings.Fields(part) {
			set[w] = struct{}{}
		}
	}
	return set
}

// hasTopicalSharedToken reports whether a ConfrontHit has at least one shared
// token that is NOT one of the opposite-marker words. This distinguishes a
// genuine semantic contradiction (the two claims share a topical anchor AND
// have opposing polarity) from a coincidental marker match (the two claims
// share only the modal "must" because every requirement uses "must", but are
// otherwise about unrelated subjects). Without this filter, the "must/must
// not" marker pair would fire on nearly every pair where one uses "must" and
// the other "must not", since "must" itself counts as a shared token — an
// unacceptably high false-positive rate for a check that REFUSES the apply.
//
// When the hit has no opposite marker, any shared token counts (the caller
// guards against this via IsBlockingHit, but the function is safe to call
// regardless).
func hasTopicalSharedToken(h ConfrontHit) bool {
	if !h.HasOppositeMarker() {
		return len(h.Shared) > 0
	}
	exclude := markerWordSet(h.OppositeMarker)
	for _, t := range h.Shared {
		if _, isMarker := exclude[t]; !isMarker {
			return true
		}
	}
	return false
}
