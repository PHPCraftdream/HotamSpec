package loader

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// realManifests are the two real, committed manifest.json files this repository
// owns: hotam-spec-self (self-hosting root, carries orientation_faq + explicit
// null parent) and hotam-dev (consumer child, declares a string parent, no
// orientation_faq). The byte-identity proof below runs against BOTH — the point
// of Phase 1 (task #341, R5-manifest-object) is proving the typed DomainManifest
// is a faithful projection of REAL authored data, not a small synthetic fixture.
// (internal/generator/testdata/manifest.json is intentionally excluded: it is a
// minimal {"parent": null} fixture, not a real domain manifest.)
var realManifests = []struct {
	name string
	path string
}{
	{"hotam-spec-self", "../../domains/hotam-spec-self/manifest.json"},
	{"hotam-dev", "../../domains/hotam-dev/manifest.json"},
}

// TestLoadWriteManifest_ByteIdenticalRoundTrip is the entire point of Phase 1:
// load each real committed manifest.json into a DomainManifest, re-serialize it
// through WriteManifest (reusing the exact canonical encoder graph.json is
// written through, not a reimplementation), and assert the output is
// BYTE-IDENTICAL to the committed file. If the typed projection's field order,
// omitempty choices, or nesting do not exactly match what is already on disk,
// this test fails and prints a usable diff hint (first differing line, with
// context) — the same shape internal/selfspec/merge_test.go's
// TestMergeIntoGraph_ByteIdenticalRoundTrip provides for the Requirements
// registry.
func TestLoadWriteManifest_ByteIdenticalRoundTrip(t *testing.T) {
	for _, tc := range realManifests {
		t.Run(tc.name, func(t *testing.T) {
			want, err := os.ReadFile(tc.path)
			if err != nil {
				t.Fatalf("read %s: %v", tc.path, err)
			}

			m, err := LoadManifest(tc.path)
			if err != nil {
				t.Fatalf("LoadManifest(%s): %v", tc.path, err)
			}

			dir := t.TempDir()
			outPath := filepath.Join(dir, "manifest.json")
			if err := WriteManifest(outPath, m); err != nil {
				t.Fatalf("WriteManifest: %v", err)
			}
			got, err := os.ReadFile(outPath)
			if err != nil {
				t.Fatalf("read written manifest: %v", err)
			}

			manifestDiffReport(t, tc.name, string(got), string(want))
		})
	}
}

// TestLoadWriteManifest_Idempotent proves writing twice produces the same bytes
// as writing once: LoadManifest→WriteManifest is a pure function of the file's
// content (no clock, no random, no global state), so a second pass over an
// already-written manifest must be a no-op.
func TestLoadWriteManifest_Idempotent(t *testing.T) {
	for _, tc := range realManifests {
		t.Run(tc.name, func(t *testing.T) {
			m, err := LoadManifest(tc.path)
			if err != nil {
				t.Fatalf("LoadManifest(%s): %v", tc.path, err)
			}
			dir := t.TempDir()
			p1 := filepath.Join(dir, "once.json")
			p2 := filepath.Join(dir, "twice.json")
			if err := WriteManifest(p1, m); err != nil {
				t.Fatalf("WriteManifest once: %v", err)
			}
			// reload the once-written file and write it again
			m2, err := LoadManifest(p1)
			if err != nil {
				t.Fatalf("LoadManifest(once): %v", err)
			}
			if err := WriteManifest(p2, m2); err != nil {
				t.Fatalf("WriteManifest twice: %v", err)
			}
			once, _ := os.ReadFile(p1)
			twice, _ := os.ReadFile(p2)
			manifestDiffReport(t, tc.name+" idempotence", string(twice), string(once))
		})
	}
}

// TestLoadManifest_CapturesRealFields proves the typed object actually captures
// the authored fields (non-vacuity): a byte-identical round-trip would also
// pass for an object that round-trips bytes without ever decoding a single
// field — so this test asserts the decoded DomainManifest carries the real
// values, field by field, for the two real manifests. If a future field is
// added to a manifest but not to DomainManifest, the byte-identity test alone
// would not catch the semantic gap; this test would.
func TestLoadManifest_CapturesRealFields(t *testing.T) {
	t.Run("hotam-spec-self", func(t *testing.T) {
		m, err := LoadManifest("../../domains/hotam-spec-self/manifest.json")
		if err != nil {
			t.Fatalf("LoadManifest: %v", err)
		}
		if !m.SelfHosting {
			t.Errorf("SelfHosting = false, want true")
		}
		if m.Purpose == "" {
			t.Error("Purpose is empty, want the domain's authored purpose")
		}
		if len(m.Goals) == 0 {
			t.Error("Goals is empty, want the authored goal list")
		}
		if m.Director == "" {
			t.Error("Director is empty")
		}
		if m.Parent != nil {
			t.Errorf("Parent = %v, want nil (hotam-spec-self is a root domain)", *m.Parent)
		}
		if len(m.OrientationFAQ) == 0 {
			t.Error("OrientationFAQ is empty, want the authored FAQ list")
		}
		// sanity: at least one entry exercises the keywords-omitted shape
		// (question + link, no keywords), which is the exact case the
		// Keywords omitempty tag exists to reproduce.
		var sawKeywordsOmitted bool
		for _, e := range m.OrientationFAQ {
			if e.Question != "" && len(e.Keywords) == 0 && e.Link != "" {
				sawKeywordsOmitted = true
			}
		}
		if !sawKeywordsOmitted {
			t.Error("no orientation_faq entry with keywords-omitted shape found — the Keywords omitempty byte-identity case is not exercised by this real manifest")
		}
	})

	t.Run("hotam-dev", func(t *testing.T) {
		m, err := LoadManifest("../../domains/hotam-dev/manifest.json")
		if err != nil {
			t.Fatalf("LoadManifest: %v", err)
		}
		if m.SelfHosting {
			t.Errorf("SelfHosting = true, want false")
		}
		if m.Purpose == "" {
			t.Error("Purpose is empty")
		}
		if len(m.Goals) == 0 {
			t.Error("Goals is empty")
		}
		if m.Director == "" {
			t.Error("Director is empty")
		}
		if m.Parent == nil {
			t.Fatal("Parent is nil, want a string naming hotam-spec-self")
		}
		if *m.Parent != "hotam-spec-self" {
			t.Errorf("Parent = %q, want %q", *m.Parent, "hotam-spec-self")
		}
		if len(m.OrientationFAQ) != 0 {
			t.Errorf("OrientationFAQ has %d entries, want 0 (hotam-dev declares none)", len(m.OrientationFAQ))
		}
	})
}

// TestWriteManifest_NilIsError is the boundary-condition control.
func TestWriteManifest_NilIsError(t *testing.T) {
	if err := WriteManifest(filepath.Join(t.TempDir(), "manifest.json"), nil); err == nil {
		t.Fatal("WriteManifest(nil): want error, got nil")
	}
}

// manifestDiffReport asserts got == want, and on failure prints byte counts plus
// the first differing line (with surrounding context) so a real fidelity
// mismatch (a field-order difference, a nil-vs-[] slice, a null-vs-omitted key)
// is immediately locatable. Mirrors merge_test.go's diffReport shape.
func manifestDiffReport(t *testing.T, name, got, want string) {
	t.Helper()
	if got == want {
		return
	}
	gotLines := strings.Split(got, "\n")
	wantLines := strings.Split(want, "\n")
	max := len(gotLines)
	if len(wantLines) < max {
		max = len(wantLines)
	}
	first := max
	for i := 0; i < max; i++ {
		if gotLines[i] != wantLines[i] {
			first = i
			break
		}
	}
	start := first - 3
	if start < 0 {
		start = 0
	}
	end := first + 5

	var b strings.Builder
	b.WriteString("\n=== byte-identity FAILED for " + name + " ===\n")
	b.WriteString("got bytes=" + strconv.Itoa(len(got)) + " want bytes=" + strconv.Itoa(len(want)) + "\n")
	b.WriteString("got lines=" + strconv.Itoa(len(gotLines)) + " want lines=" + strconv.Itoa(len(wantLines)) + "\n")
	b.WriteString("first differing line index: " + strconv.Itoa(first) + "\n")
	for i := start; i < end; i++ {
		gotLine := ""
		if i < len(gotLines) {
			gotLine = gotLines[i]
		}
		wantLine := ""
		if i < len(wantLines) {
			wantLine = wantLines[i]
		}
		marker := "  "
		switch {
		case i >= len(wantLines):
			marker = "G>"
		case i >= len(gotLines):
			marker = "W<"
		case gotLine != wantLine:
			marker = "* "
		}
		b.WriteString(marker + " line[" + strconv.Itoa(i) + "]\n    got:  " + truncateManifestLine(gotLine) + "\n    want: " + truncateManifestLine(wantLine) + "\n")
	}
	t.Error(b.String())
}

func truncateManifestLine(s string) string {
	const n = 200
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
