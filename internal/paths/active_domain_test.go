package paths

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReadActiveDomain_MissingFile returns ("", false): a project with no
// marker file (or a domains/-native-marker project) has no recorded preference
// and must fall through gracefully.
func TestReadActiveDomain_MissingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	name, ok := ReadActiveDomain(filepath.Join(dir, MarkerFilename))
	if ok {
		t.Errorf("missing marker should resolve ok=false, got name=%q ok=true", name)
	}
	if name != "" {
		t.Errorf("missing marker should resolve name empty, got %q", name)
	}
}

// TestReadActiveDomain_EmptyFile: a marker written empty (the pre-active-domain
// scaffold behavior, still produced by hand or by an older init-project) must
// resolve ok=false, not error — this is the graceful-degradation guarantee.
func TestReadActiveDomain_EmptyFile(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), MarkerFilename)
	if err := os.WriteFile(p, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	name, ok := ReadActiveDomain(p)
	if ok {
		t.Errorf("empty marker should resolve ok=false, got name=%q ok=true", name)
	}
}

// TestReadActiveDomain_EmptyJSONObject: a marker written as just "{}" carries
// no active_domain field, so it resolves ok=false (degrades to the next tier).
func TestReadActiveDomain_EmptyJSONObject(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), MarkerFilename)
	if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	name, ok := ReadActiveDomain(p)
	if ok {
		t.Errorf("{} marker should resolve ok=false, got name=%q ok=true", name)
	}
}

// TestReadActiveDomain_MalformedJSON degrades gracefully: a marker containing
// garbage must return ok=false rather than panic or error, because the marker
// is advisory for active-domain resolution and must never break project-root
// detection (which only ever checks existence).
func TestReadActiveDomain_MalformedJSON(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), MarkerFilename)
	if err := os.WriteFile(p, []byte("{ this is not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}
	name, ok := ReadActiveDomain(p)
	if ok {
		t.Errorf("malformed marker should resolve ok=false, got name=%q ok=true", name)
	}
}

// TestReadActiveDomain_EmptyField: valid JSON with an empty active_domain
// resolves ok=false (no preference recorded).
func TestReadActiveDomain_EmptyField(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), MarkerFilename)
	if err := os.WriteFile(p, []byte(`{"active_domain": ""}`), 0o644); err != nil {
		t.Fatal(err)
	}
	name, ok := ReadActiveDomain(p)
	if ok {
		t.Errorf("empty active_domain field should resolve ok=false, got name=%q ok=true", name)
	}
}

// TestReadActiveDomain_ValidContent returns the recorded name.
func TestReadActiveDomain_ValidContent(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), MarkerFilename)
	if err := os.WriteFile(p, []byte(`{"active_domain": "main"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	name, ok := ReadActiveDomain(p)
	if !ok {
		t.Fatal("valid marker should resolve ok=true")
	}
	if name != "main" {
		t.Errorf("got name %q, want %q", name, "main")
	}
}

// TestWriteActiveDomain_RoundTripsWithRead writes a name then reads it back,
// asserting exact round-trip fidelity.
func TestWriteActiveDomain_RoundTripsWithRead(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), MarkerFilename)
	if err := WriteActiveDomain(p, "second"); err != nil {
		t.Fatalf("WriteActiveDomain: %v", err)
	}
	name, ok := ReadActiveDomain(p)
	if !ok {
		t.Fatal("after WriteActiveDomain, ReadActiveDomain should resolve ok=true")
	}
	if name != "second" {
		t.Errorf("got name %q, want %q", name, "second")
	}
}

// TestWriteActiveDomain_CreatesParentDirs writes into a marker path whose
// parent does not yet exist, asserting mkdir-then-write works (so `hotam use`
// can promote a marker-less project).
func TestWriteActiveDomain_CreatesParentDirs(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), "nested", "deep", MarkerFilename)
	if err := WriteActiveDomain(p, "main"); err != nil {
		t.Fatalf("WriteActiveDomain should mkdir parents: %v", err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("marker not written: %v", err)
	}
}

// TestWriteActiveDomain_OverwritesExisting rewrites an existing marker (the
// `hotam use` flow on an already-marked project) rather than refusing.
func TestWriteActiveDomain_OverwritesExisting(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), MarkerFilename)
	if err := WriteActiveDomain(p, "main"); err != nil {
		t.Fatal(err)
	}
	if err := WriteActiveDomain(p, "second"); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	name, ok := ReadActiveDomain(p)
	if !ok || name != "second" {
		t.Fatalf("after overwrite, got name=%q ok=%v, want second/true", name, ok)
	}
}

// TestWriteActiveDomain_MatchesJSONConvention asserts the on-disk bytes match
// the repo's JSON-file convention: 2-space-indented JSON with a trailing
// newline (like graph.lock).
func TestWriteActiveDomain_MatchesJSONConvention(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), MarkerFilename)
	if err := WriteActiveDomain(p, "main"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"active_domain\": \"main\"\n}\n"
	if string(data) != want {
		t.Errorf("marker bytes mismatch:\nwant %q\ngot  %q", want, string(data))
	}
	if !strings.HasSuffix(string(data), "\n") {
		t.Error("marker file must end with a trailing newline")
	}
}
