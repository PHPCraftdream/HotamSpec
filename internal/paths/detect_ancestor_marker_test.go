package paths

import (
	"testing"
)

// TestDetectAncestorMarker_ReliableMarkerFound plants a RELIABLE marker
// (domains/) at a known ancestor of a synthetic leaf and confirms the walk
// finds exactly that ancestor (not a shallower or deeper one), reporting the
// marker by name. This mirrors hasNativeMarker's reliable-marker path.
func TestDetectAncestorMarker_ReliableMarkerFound(t *testing.T) {
	root := t.TempDir()
	makeDir(t, root, "domains")
	leaf := makeDir(t, root, "a", "b", "c") // root/a/b/c — 3 levels below root

	anc, matched, found := DetectAncestorMarker(leaf)
	if !found {
		t.Fatalf("expected to find the domains/ marker ancestor from leaf %s", leaf)
	}
	if anc != root {
		t.Errorf("ancestor = %q, want %q (the dir carrying domains/)", anc, root)
	}
	if !sliceContains(matched, "domains") {
		t.Errorf("matched = %v, want to include %q", matched, "domains")
	}
}

// TestDetectAncestorMarker_SecondaryQuorumFound plants NO reliable marker but
// the SECONDARY-marker quorum (CLAUDE.md + .claude, i.e. >=
// SecondaryMarkerMinCount) and confirms the walk still resolves — proving
// DetectAncestorMarker applies the SAME reliable-OR-secondary rule as
// hasNativeMarker (not a stricter reliable-only rule).
func TestDetectAncestorMarker_SecondaryQuorumFound(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "CLAUDE.md", "# stray\n")
	makeDir(t, root, ".claude")
	leaf := makeDir(t, root, "deep", "leaf")

	anc, matched, found := DetectAncestorMarker(leaf)
	if !found {
		t.Fatalf("expected the SECONDARY-quorum (CLAUDE.md + .claude) to resolve")
	}
	if anc != root {
		t.Errorf("ancestor = %q, want %q", anc, root)
	}
	if !sliceContains(matched, "CLAUDE.md") || !sliceContains(matched, ".claude") {
		t.Errorf("matched = %v, want both CLAUDE.md and .claude", matched)
	}
}

// TestDetectAncestorMarker_CleanTreeNotFound builds a genuinely marker-free
// synthetic subtree (no domains/, no delegations/, fewer than the secondary
// quorum anywhere within the walk) and confirms DetectAncestorMarker reports
// nothing — the negative case tests must be able to rely on.
func TestDetectAncestorMarker_CleanTreeNotFound(t *testing.T) {
	root := t.TempDir()
	leaf := makeDir(t, root, "x", "y", "z")

	anc, _, found := DetectAncestorMarker(leaf)
	if found {
		t.Errorf("expected no marker in a clean synthetic tree, found ancestor %q", anc)
	}
}

func sliceContains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
