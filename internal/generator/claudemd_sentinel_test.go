package generator

import (
	"strings"
	"testing"
)

// TestSentinelOps_WrapExtract round-trips a block through WrapBlock/ExtractBlock
// and asserts ExtractBlock returns the inner content with surrounding newlines
// stripped, plus its not-found and malformed-ordering guards.
func TestSentinelOps_WrapExtract(t *testing.T) {
	t.Parallel()
	name := "LIVE-STATE"
	wrapped := WrapBlock(name, "inner line 1\ninner line 2")

	got, ok := ExtractBlock(wrapped, name)
	if !ok {
		t.Fatalf("ExtractBlock: expected ok=true for a wrapped block")
	}
	if got != "inner line 1\ninner line 2" {
		t.Errorf("ExtractBlock content = %q, want the inner text only", got)
	}
	// sentinels themselves must not appear in the extracted inner text
	if strings.Contains(got, ":BEGIN") || strings.Contains(got, ":END") {
		t.Errorf("ExtractBlock leaked sentinel markers: %q", got)
	}
}

func TestSentinelOps_ExtractMissingSentinels(t *testing.T) {
	t.Parallel()
	// neither sentinel present
	if _, ok := ExtractBlock("plain text with no markers", "ABSENT"); ok {
		t.Errorf("ExtractBlock must return ok=false when sentinels are absent")
	}
	// END precedes BEGIN → malformed ordering
	reversed := EndSentinel("X") + " junk " + BeginSentinel("X")
	if _, ok := ExtractBlock(reversed, "X"); ok {
		t.Errorf("ExtractBlock must return ok=false when END precedes BEGIN")
	}
}

func TestSentinelOps_ReplacePreservesSurroundings(t *testing.T) {
	t.Parallel()
	name := "BLOCK"
	src := "header line\n" + WrapBlock(name, "old") + "\nfooter line"
	out, err := ReplaceBlock(src, name, "new content")
	if err != nil {
		t.Fatalf("ReplaceBlock: %v", err)
	}
	if !strings.HasPrefix(out, "header line") {
		t.Errorf("ReplaceBlock must preserve text before BEGIN: %q", out)
	}
	if !strings.HasSuffix(out, "footer line") {
		t.Errorf("ReplaceBlock must preserve text after END: %q", out)
	}
	extracted, ok := ExtractBlock(out, name)
	if !ok || extracted != "new content" {
		t.Errorf("ReplaceBlock inner = %q (ok=%v), want %q", extracted, ok, "new content")
	}
}

func TestSentinelOps_ReplaceMissingSentinelsErrors(t *testing.T) {
	t.Parallel()
	_, err := ReplaceBlock("no markers here", "MISSING", "x")
	if err == nil {
		t.Fatalf("ReplaceBlock must error when a sentinel is absent")
	}
	msg := err.Error()
	if !strings.Contains(msg, "MISSING") || !strings.Contains(msg, "not found") {
		t.Errorf("ReplaceBlock error should name the block + corruption hint, got: %s", msg)
	}
}

func TestSentinelOps_InsertBlockAfter(t *testing.T) {
	t.Parallel()
	anchor := "ANCHOR"
	src := "pre\n" + WrapBlock(anchor, "a") + "\npost"
	out, err := InsertBlockAfter(src, anchor, "NEW", "n")
	if err != nil {
		t.Fatalf("InsertBlockAfter: %v", err)
	}
	// the NEW block must sit immediately after the ANCHOR END sentinel
	idxAnchor := strings.Index(out, EndSentinel(anchor))
	idxNew := strings.Index(out, BeginSentinel("NEW"))
	if idxAnchor == -1 || idxNew == -1 || idxNew < idxAnchor {
		t.Errorf("NEW block must follow the ANCHOR END sentinel:\n%s", out)
	}
	// the trailing "post" text must still be present after the NEW block
	if !strings.HasSuffix(out, "\npost") {
		t.Errorf("InsertBlockAfter must preserve trailing text, got tail: %q", out[len(out)-40:])
	}
}

func TestSentinelOps_InsertBlockAfterMissingAnchorErrors(t *testing.T) {
	t.Parallel()
	_, err := InsertBlockAfter("no anchor here", "NOPE", "NEW", "n")
	if err == nil {
		t.Fatalf("InsertBlockAfter must error when the anchor END sentinel is absent")
	}
	msg := err.Error()
	if !strings.Contains(msg, "NOPE") || !strings.Contains(msg, "Cannot insert") {
		t.Errorf("InsertBlockAfter error should name anchor + the block being inserted, got: %s", msg)
	}
}
