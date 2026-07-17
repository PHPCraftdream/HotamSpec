package invariants

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	recordervendor "github.com/PHPCraftdream/HotamSpec/internal/recorder/vendor"
)

// writeVendoredRecorderFixture writes content at
// <tmp>/spec/hotamspec/hotamspec.go and returns tmp as the domain directory
// (g.DomainDir) -- mirrors writeAuthoredSpecFixture's shape
// (authored_links_test.go) but targets the fixed recorder path
// check_recorder_current reads.
func writeVendoredRecorderFixture(t *testing.T, content string) string {
	t.Helper()
	tmp := t.TempDir()
	full := filepath.Join(tmp, "spec", "hotamspec", "hotamspec.go")
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return tmp
}

func TestCheckRecorderCurrent_NoOpWhenNeverVendored(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{DomainDir: t.TempDir()}
	if vs := runCheck(t, "check_recorder_current", g); len(vs) != 0 {
		t.Fatalf("expected no violations for a domain that never vendored the recorder, got %v", vs)
	}
}

func TestCheckRecorderCurrent_NoOpWhenDomainDirEmpty(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{}
	if vs := runCheck(t, "check_recorder_current", g); len(vs) != 0 {
		t.Fatalf("expected no violations for a graph with no DomainDir, got %v", vs)
	}
}

func TestCheckRecorderCurrent_OK_WhenIdenticalToCanon(t *testing.T) {
	t.Parallel()
	domainDir := writeVendoredRecorderFixture(t, recordervendor.Source())
	g := &ontology.Graph{DomainDir: domainDir}
	if vs := runCheck(t, "check_recorder_current", g); len(vs) != 0 {
		t.Fatalf("expected no violations for a freshly vendored, unmodified copy, got %v", vs)
	}
}

// TestCheckRecorderCurrent_MUTATION_TamperedBodyFires is the mutation probe
// the task's own verification step calls for: corrupt the vendored copy's
// BODY (after the banner, exactly the shape a hand-edit -- or a forged
// "always pass" Then() -- would take), confirm the check goes red, then
// restore byte-identical content and confirm it goes green again.
func TestCheckRecorderCurrent_MUTATION_TamperedBodyFires(t *testing.T) {
	t.Parallel()
	genuine := recordervendor.Source()
	tampered := genuine + "\n// hand-edited: this line was never vendored\n"

	domainDir := writeVendoredRecorderFixture(t, tampered)
	g := &ontology.Graph{DomainDir: domainDir}
	vs := runCheck(t, "check_recorder_current", g)
	if len(vs) == 0 {
		t.Fatalf("expected a violation for a tampered vendored recorder, got none")
	}
	for _, v := range vs {
		if v.Check != "check_recorder_current" {
			t.Errorf("violation Check = %q, want check_recorder_current", v.Check)
		}
	}

	// Restore byte-identical content -- the check must go back to green,
	// proving this is a live content comparison, not a one-shot flag.
	target := filepath.Join(domainDir, "spec", "hotamspec", "hotamspec.go")
	if err := os.WriteFile(target, []byte(genuine), 0o644); err != nil {
		t.Fatalf("restore fixture: %v", err)
	}
	if vs := runCheck(t, "check_recorder_current", g); len(vs) != 0 {
		t.Fatalf("expected no violations after restoring byte-identical content, got %v", vs)
	}
}

func TestCheckRecorderCurrent_FiresWhenBannerMissing(t *testing.T) {
	t.Parallel()
	// The exact canon body, but with NO banner at all -- a file that was
	// hand-created rather than produced by `hotam vendor-recorder`.
	domainDir := writeVendoredRecorderFixture(t, recordervendor.BodyForHash())
	g := &ontology.Graph{DomainDir: domainDir}
	vs := runCheck(t, "check_recorder_current", g)
	if len(vs) == 0 {
		t.Fatalf("expected a violation for a vendored recorder with no do-not-edit banner, got none")
	}
}

func TestCheckRecorderCurrent_FiresWhenStale(t *testing.T) {
	t.Parallel()
	// Simulate an OLDER canon: banner intact, but the body differs (as if an
	// engine upgrade changed the recorder API and this domain never
	// re-vendored).
	stale := recordervendor.Banner + "package hotamspec\n\n// an older, different recorder body\n"
	domainDir := writeVendoredRecorderFixture(t, stale)
	g := &ontology.Graph{DomainDir: domainDir}
	vs := runCheck(t, "check_recorder_current", g)
	if len(vs) == 0 {
		t.Fatalf("expected a violation for a stale vendored recorder, got none")
	}
}
