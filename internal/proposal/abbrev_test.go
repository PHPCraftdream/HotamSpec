package proposal

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestAbbrev_MultibyteBoundary_NeverProducesInvalidUTF8 proves that abbrev's
// truncation always lands on a rune boundary, not a byte boundary. Cyrillic
// (and other non-ASCII) characters are multi-byte in UTF-8; a byte-index
// slice (text[:limit-1]) can split a character's encoding in half, producing
// invalid UTF-8 that later round-trips through encoding/json as U+FFFD
// replacement characters. This was observed on live data in
// domains/prat/graph.json and domains/gpsm-sm/graph.json History entries.
func TestAbbrev_MultibyteBoundary_NeverProducesInvalidUTF8(t *testing.T) {
	// Each Cyrillic rune is 2 bytes in UTF-8. Pick limits that, under the old
	// byte-slice implementation, would land squarely inside a rune's encoding.
	text := "Экспорт всегда должен шифроваться данные пользователя надежно"

	for limit := 1; limit <= utf8.RuneCountInString(text)+5; limit++ {
		got := abbrev(text, limit)
		if !utf8.ValidString(got) {
			t.Fatalf("abbrev(%q, %d) = %q: not valid UTF-8", text, limit, got)
		}

		runeCount := utf8.RuneCountInString(got)
		srcRuneCount := utf8.RuneCountInString(strings.Join(strings.Fields(text), " "))
		if srcRuneCount > limit {
			// Truncated: body is limit-1 runes plus the "…" marker rune, so
			// the result must never exceed limit runes.
			if runeCount > limit {
				t.Fatalf("abbrev(%q, %d) = %q (%d runes): exceeds limit", text, limit, got, runeCount)
			}
			if !strings.HasSuffix(got, "…") {
				t.Fatalf("abbrev(%q, %d) = %q: expected ellipsis suffix when truncated", text, limit, got)
			}
		}
	}
}

// TestAbbrev_ExactMidCharacterLimit pins the originally observed failure
// mode: a limit that would have split a specific multi-byte character
// exactly in half under a byte-index slice.
func TestAbbrev_ExactMidCharacterLimit(t *testing.T) {
	text := "шифрование" // 10 Cyrillic runes, 20 bytes
	for limit := 1; limit < utf8.RuneCountInString(text); limit++ {
		got := abbrev(text, limit)
		if !utf8.ValidString(got) {
			t.Fatalf("abbrev(%q, %d) = %q: invalid UTF-8 (mid-character split)", text, limit, got)
		}
		if strings.ContainsRune(got, '�') {
			t.Fatalf("abbrev(%q, %d) = %q: contains replacement character", text, limit, got)
		}
	}
}

// TestAbbrev_ASCII_BehaviorPreserved keeps the historical ASCII behavior
// intact: for ASCII text, byte count == rune count, so truncation length and
// ellipsis placement must match the pre-fix behavior exactly.
func TestAbbrev_ASCII_BehaviorPreserved(t *testing.T) {
	cases := []struct {
		text  string
		limit int
		want  string
	}{
		{"short", 150, "short"},
		{"exactly ten", 11, "exactly ten"},
		{"this is longer than the limit", 10, "this is l…"},
		{"a", 1, "a"},
		{"ab", 1, "…"},
	}
	for _, c := range cases {
		got := abbrev(c.text, c.limit)
		if got != c.want {
			t.Errorf("abbrev(%q, %d) = %q, want %q", c.text, c.limit, got, c.want)
		}
	}
}
