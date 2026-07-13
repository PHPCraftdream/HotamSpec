package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/paths"
)

// captureStderr redirects os.Stderr to a pipe for the duration of fn, returning
// whatever fn wrote to stderr. It is process-global (os.Stderr is a single
// *os.File), so tests using it MUST NOT call t.Parallel() — but a non-parallel
// test never runs concurrently with any other test in the binary, so this is
// safe. The reader goroutine drains the pipe so the writer never blocks.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w
	outC := make(chan string)
	go func() {
		var buf strings.Builder
		_, _ = io.Copy(&buf, r)
		outC <- buf.String()
	}()
	fn()
	w.Close()
	os.Stderr = orig
	return <-outC
}

// markerWithActiveDomain writes a marker file carrying active_domain=<name> into
// dir and returns its path, for tier-3 setup in the resolveDomain tests.
func markerWithActiveDomain(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, paths.MarkerFilename)
	if err := paths.WriteActiveDomain(p, name); err != nil {
		t.Fatalf("seed marker: %v", err)
	}
	return p
}

// TestResolveDomain_ExplicitDomainWins is tier 1: a non-empty --domain is
// resolved verbatim via filepath.Abs with NO project-root resolution attempt
// and NO stderr notice — byte-identical to pre-active-domain behavior.
func TestResolveDomain_ExplicitDomainWins(t *testing.T) {
	// Not parallel: captureStderr mutates process-global os.Stderr.
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	rel := filepath.Join("some", "relative", "domain")
	want, err := filepath.Abs(rel)
	if err != nil {
		t.Fatal(err)
	}
	var (
		got    string
		gotErr error
	)
	stderr := captureStderr(t, func() {
		got, gotErr = resolveDomain(rel)
	})
	if gotErr != nil {
		t.Fatalf("resolveDomain(--domain) errored: %v", gotErr)
	}
	if got != want {
		t.Errorf("resolveDomain(--domain) = %q, want abs %q", got, want)
	}
	if stderr != "" {
		t.Errorf("tier 1 must be silent (no stderr), got:\n%s", stderr)
	}
	// Belt-and-suspenders: cwd is unchanged (no side effect).
	if now, _ := os.Getwd(); now != cwd {
		t.Errorf("resolveDomain(--domain) changed CWD: %q -> %q", cwd, now)
	}
}

// TestResolveDomain_FourTierPriority exercises the full tier 2/3/4 chain in
// isolation (no binary spawn): HOTAM_DOMAIN env > marker > legacy default, with
// the legacy default silent and tiers 2-3 emitting a stderr notice. The project
// root is pinned via HOTAM_SPEC_PROJECT_ROOT (R1) so resolution is deterministic.
func TestResolveDomain_FourTierPriority(t *testing.T) {
	// Not parallel: captureStderr mutates process-global os.Stderr; t.Setenv
	// for HOTAM_SPEC_PROJECT_ROOT/HOTAM_DOMAIN steers ProjectRoot resolution.

	// Tier 4: legacy default, silent. No env, no marker in the root.
	t.Run("tier4_legacy_default_silent", func(t *testing.T) {
		root := t.TempDir()
		t.Setenv(paths.EnvProjectRoot, root)
		t.Setenv(paths.EnvActiveDomain, "")
		var got string
		stderr := captureStderr(t, func() {
			g, err := resolveDomain("")
			if err != nil {
				t.Fatalf("resolveDomain: %v", err)
			}
			got = g
		})
		want := filepath.Join(root, "domains", defaultDomainName)
		if got != want {
			t.Errorf("tier4 = %q, want %q", got, want)
		}
		if stderr != "" {
			t.Errorf("tier 4 (legacy default) must be silent, got stderr:\n%s", stderr)
		}
	})

	// Tier 3: marker wins over legacy default, with a stderr notice.
	t.Run("tier3_marker_wins_and_reports", func(t *testing.T) {
		root := t.TempDir()
		t.Setenv(paths.EnvProjectRoot, root)
		t.Setenv(paths.EnvActiveDomain, "")
		markerWithActiveDomain(t, root, "marked")
		var got string
		stderr := captureStderr(t, func() {
			g, err := resolveDomain("")
			if err != nil {
				t.Fatalf("resolveDomain: %v", err)
			}
			got = g
		})
		want := filepath.Join(root, "domains", "marked")
		if got != want {
			t.Errorf("tier3 = %q, want %q", got, want)
		}
		if !strings.Contains(stderr, "resolved domain: marked") || !strings.Contains(stderr, ".hotam-spec-project marker") {
			t.Errorf("tier3 should report marker resolution on stderr, got:\n%s", stderr)
		}
	})

	// Tier 2: HOTAM_DOMAIN env wins, with a stderr notice, even if a marker
	// is ALSO present (env precedence over marker).
	t.Run("tier2_env_wins_over_marker_and_reports", func(t *testing.T) {
		root := t.TempDir()
		t.Setenv(paths.EnvProjectRoot, root)
		t.Setenv(paths.EnvActiveDomain, "envdomain")
		// Marker present but must be overridden by the env var.
		markerWithActiveDomain(t, root, "marked")
		var got string
		stderr := captureStderr(t, func() {
			g, err := resolveDomain("")
			if err != nil {
				t.Fatalf("resolveDomain: %v", err)
			}
			got = g
		})
		want := filepath.Join(root, "domains", "envdomain")
		if got != want {
			t.Errorf("tier2 = %q, want %q (env over marker)", got, want)
		}
		if !strings.Contains(stderr, "resolved domain: envdomain") || !strings.Contains(stderr, "HOTAM_DOMAIN env") {
			t.Errorf("tier2 should report env resolution on stderr, got:\n%s", stderr)
		}
		if strings.Contains(stderr, ".hotam-spec-project marker") {
			t.Errorf("tier2 should NOT report the marker (env won), got:\n%s", stderr)
		}
	})

	// Tier 1: explicit --domain wins over ALL of env + marker + legacy, with no
	// stderr and no project-root resolution (the root env above is irrelevant).
	t.Run("tier1_explicit_wins_over_all", func(t *testing.T) {
		root := t.TempDir()
		t.Setenv(paths.EnvProjectRoot, root)
		t.Setenv(paths.EnvActiveDomain, "envdomain")
		markerWithActiveDomain(t, root, "marked")
		rel := filepath.Join("explicit", "domain")
		want, err := filepath.Abs(rel)
		if err != nil {
			t.Fatal(err)
		}
		var got string
		stderr := captureStderr(t, func() {
			g, err := resolveDomain(rel)
			if err != nil {
				t.Fatalf("resolveDomain: %v", err)
			}
			got = g
		})
		if got != want {
			t.Errorf("tier1 = %q, want abs %q", got, want)
		}
		if stderr != "" {
			t.Errorf("tier 1 must be silent even with env+marker present, got:\n%s", stderr)
		}
	})

	// Empty marker (legacy/empty file) degrades to the silent legacy default.
	t.Run("empty_marker_falls_through_silently", func(t *testing.T) {
		root := t.TempDir()
		t.Setenv(paths.EnvProjectRoot, root)
		t.Setenv(paths.EnvActiveDomain, "")
		emptyMarker := filepath.Join(root, paths.MarkerFilename)
		if err := os.WriteFile(emptyMarker, []byte{}, 0o644); err != nil {
			t.Fatal(err)
		}
		var got string
		stderr := captureStderr(t, func() {
			g, err := resolveDomain("")
			if err != nil {
				t.Fatalf("resolveDomain: %v", err)
			}
			got = g
		})
		want := filepath.Join(root, "domains", defaultDomainName)
		if got != want {
			t.Errorf("empty marker should fall through to legacy default, got %q want %q", got, want)
		}
		if stderr != "" {
			t.Errorf("empty marker fall-through must be silent, got:\n%s", stderr)
		}
	})
}
