package diagnose

import (
	"testing"
)

// TestMarkerHits_SubstringFalsePositiveFixed proves the CONFIRMED bug named
// in task #156/R10-b: the old strings.Contains(lowerClaim, "any") check
// matched inside "company" ("comp-ANY"), "many" ("m-ANY"), "anybody"
// ("ANY-body") — unrelated English words that merely contain "any" as a
// substring. markerHits must NOT report an "only|any" hit for any of these,
// since none of them is a genuine use of the word "any" as a marker.
func TestMarkerHits_SubstringFalsePositiveFixed(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		claim string
	}{
		{"company", "the company shall report revenue"},
		{"many", "comparison of many vendors"},
		{"anybody", "anybody can access this"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			hits := markerHits(tc.claim)
			if side, ok := hits["only|any"]; ok {
				t.Fatalf("markerHits(%q) falsely fired only|any marker = %q; substring bug not fixed", tc.claim, side)
			}
		})
	}
}

// TestMarkerHits_WholeWordStillMatchesGenuineAny confirms the fix did not
// over-correct into never matching "any" at all: a genuine standalone use of
// the word must still be detected.
func TestMarkerHits_WholeWordStillMatchesGenuineAny(t *testing.T) {
	t.Parallel()
	hits := markerHits("any user may delete a project")
	if side, ok := hits["only|any"]; !ok || side != "any" {
		t.Fatalf("markerHits genuine 'any' claim = %v, want only|any -> any", hits)
	}
}

// TestMarkerHits_ExistingLowercaseEnglishFixturesUnaffected pins the hard
// no-regression bar: the self-hosted graph's own existing English fixtures
// ("export service must always encrypt records" / "must never encrypt
// records", see cmd/hotam/semantic_gate_test.go) must continue to trigger
// the must/must-not-style detection exactly as before this task.
func TestMarkerHits_ExistingLowercaseEnglishFixturesUnaffected(t *testing.T) {
	t.Parallel()
	always := markerHits("export service must always encrypt records")
	if side, ok := always["must|must not"]; !ok || side != "must" {
		t.Fatalf("markerHits(must always) = %v, want must|must not -> must", always)
	}
	if side, ok := always["never|always"]; !ok || side != "always" {
		t.Fatalf("markerHits(must always) = %v, want never|always -> always", always)
	}

	never := markerHits("export service must never encrypt records")
	// "must never encrypt" contains bare "must" (no "not"/"n't" adjacent) so
	// the must/must-not pair fires on the positive "must" pole - unchanged
	// pre-existing behavior, this test only pins it stays that way.
	if side, ok := never["must|must not"]; !ok || side != "must" {
		t.Fatalf("markerHits(must never) = %v, want must|must not -> must", never)
	}
	if side, ok := never["never|always"]; !ok || side != "never" {
		t.Fatalf("markerHits(must never) = %v, want never|always -> never", never)
	}
}

// TestMarkerHits_CapsTokenDetectsRussianClaims proves the ADDITIVE
// caps-token pass: a Russian claim carrying the literal reserved token
// ALWAYS (or NEVER) trips the corresponding marker pair even though every
// other word is Russian and the old lowercase-English heuristic has nothing
// to match.
func TestMarkerHits_CapsTokenDetectsRussianClaims(t *testing.T) {
	t.Parallel()

	always := markerHits("Экспорт записей ALWAYS шифруется")
	if side, ok := always["never|always"]; !ok || side != "always" {
		t.Fatalf("markerHits(Russian ALWAYS) = %v, want never|always -> always", always)
	}

	never := markerHits("Экспорт записей NEVER не шифруется")
	if side, ok := never["never|always"]; !ok || side != "never" {
		t.Fatalf("markerHits(Russian NEVER) = %v, want never|always -> never", never)
	}
}

// TestMarkerHits_RussianClaimWithoutAnyTokenDoesNotFire is the
// non-over-triggering guard: a Russian claim that contains neither a caps
// reserved token nor an English lowercase marker word must trip nothing at
// all.
func TestMarkerHits_RussianClaimWithoutAnyTokenDoesNotFire(t *testing.T) {
	t.Parallel()
	hits := markerHits("Экспорт записей должен шифроваться при передаче")
	if len(hits) != 0 {
		t.Fatalf("markerHits(plain Russian claim) = %v, want no hits", hits)
	}
}

// TestMarkerHits_CapsTokenRequiresExactCase proves the caps-token pass is
// genuinely case-sensitive: a lowercase "always" embedded in otherwise
// Russian prose must NOT trip the NEW caps-token signal. It happens to also
// not trip the OLD lowercase-English pass here because "always" occurs
// inside non-space-delimited Cyrillic text with no ASCII word boundary
// around it on one side in some phrasings — to isolate the claim precisely,
// this test uses a claim where lowercase "always" DOES appear as a genuine
// ASCII whole word (so the OLD lowercase pass legitimately fires — that's
// the union, fine) while proving the NEW caps pass specifically requires
// exact case by checking capsTokenMarkerHits directly, which must report
// nothing for lowercase input.
func TestMarkerHits_CapsTokenRequiresExactCase(t *testing.T) {
	t.Parallel()
	claim := "Экспорт always должен шифроваться"

	capsOnly := capsTokenMarkerHits(claim)
	if len(capsOnly) != 0 {
		t.Fatalf("capsTokenMarkerHits(lowercase 'always') = %v, want no hits (case must be exact)", capsOnly)
	}

	// The union via markerHits MAY still fire because the old lowercase pass
	// legitimately matches lowercase "always" as a whole word - that's the
	// documented union behavior, not a caps-token false positive.
	union := markerHits(claim)
	if side, ok := union["never|always"]; !ok || side != "always" {
		t.Fatalf("markerHits(lowercase 'always' in Russian prose) = %v, want the OLD lowercase pass to still fire (union)", union)
	}
}

// TestMarkerHits_CapsTokenWholeWordBoundary proves the boundary rule from
// the task: a caps token immediately adjacent to another ASCII letter (e.g.
// "ALWAYSNESS") must NOT match, while a caps token glued to Cyrillic on
// either side MUST still match (Cyrillic is not an ASCII word character, so
// \b does not require separation from it).
func TestMarkerHits_CapsTokenWholeWordBoundary(t *testing.T) {
	t.Parallel()

	t.Run("glued to ASCII word does not match", func(t *testing.T) {
		t.Parallel()
		hits := capsTokenMarkerHits("this claim has ALWAYSNESS as a made-up word")
		if _, ok := hits["never|always"]; ok {
			t.Fatalf("capsTokenMarkerHits(ALWAYSNESS) = %v, want no never|always hit (not whole-word)", hits)
		}
	})

	t.Run("glued to Cyrillic on both sides still matches", func(t *testing.T) {
		t.Parallel()
		hits := capsTokenMarkerHits("экспортВСЕГДАALWAYSшифруется")
		if side, ok := hits["never|always"]; !ok || side != "always" {
			t.Fatalf("capsTokenMarkerHits(Cyrillic-glued ALWAYS) = %v, want never|always -> always", hits)
		}
	})

	t.Run("SEMPERALWAYS glued ASCII prefix does not match", func(t *testing.T) {
		t.Parallel()
		hits := capsTokenMarkerHits("SEMPERALWAYS is not a real word")
		if _, ok := hits["never|always"]; ok {
			t.Fatalf("capsTokenMarkerHits(SEMPERALWAYS) = %v, want no never|always hit (not whole-word)", hits)
		}
	})
}

// TestMarkerHits_CapsTokenMustNotPrecedence mirrors the existing
// must/must-not precedence logic: MUST NOT must be detected as the negative
// pole, not double-counted as bare MUST also firing the positive pole.
func TestMarkerHits_CapsTokenMustNotPrecedence(t *testing.T) {
	t.Parallel()
	hits := capsTokenMarkerHits("Система MUST NOT логировать пароли")
	if side, ok := hits["must|must not"]; !ok || side != "must not" {
		t.Fatalf("capsTokenMarkerHits(MUST NOT) = %v, want must|must not -> must not", hits)
	}
}

// TestMarkerHits_CapsTokenOnlyAnyPair proves the ONLY/ANY caps pair fires
// independently of language, symmetric to the ALWAYS/NEVER coverage above.
func TestMarkerHits_CapsTokenOnlyAnyPair(t *testing.T) {
	t.Parallel()

	only := capsTokenMarkerHits("ONLY администраторы могут удалить проект")
	if side, ok := only["only|any"]; !ok || side != "only" {
		t.Fatalf("capsTokenMarkerHits(ONLY) = %v, want only|any -> only", only)
	}

	any_ := capsTokenMarkerHits("ANY пользователь может удалить проект")
	if side, ok := any_["only|any"]; !ok || side != "any" {
		t.Fatalf("capsTokenMarkerHits(ANY) = %v, want only|any -> any", any_)
	}
}
