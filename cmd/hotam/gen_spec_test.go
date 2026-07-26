package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGenSpec_MissingGraphRendersCalmNotice enforces R-empty-content-gen-notice:
// when the active domain has NO graph.json at all (a freshly cloned framework
// with no domain populated yet), gen-spec must NOT fail. Task #364 changed
// WHAT the calm behavior is: rather than rendering a "no content yet" notice
// into an otherwise-unconditional docs/gen/*.md set, gen-spec now writes
// ZERO files under docs/gen/ — the two situations (missing graph.json vs. an
// empty-but-present one) are still indistinguishable to an adopter with
// nothing modeled yet (a missing graph.json is substituted with an empty
// graph, loadGraphOrEmpty), so they still produce IDENTICAL output: nothing.
//
// EXACT RULE (mechanically checked): genSpec against a temp dir containing NO
// graph.json returns no error, and docs/gen/ either does not exist at all or
// contains 0 files — not a directory full of calm-but-empty placeholders.
// The PROJECT-shared framework/ output (GLOSSARY.md, tools/*.md/INDEX.md) is
// UNAFFECTED by this gate — it is written unconditionally, independent of
// domain content — so `written` is still non-empty overall.
//
// Profile: this test passes the CONSUMER profile explicitly — the same
// default a real `hotam init`/`hotam init-project` domain gets (initDomain
// writes gen_profile: consumer, R8-e). Under consumer, atoms-*.md and
// docs/gen/thinking/*.md are ALSO withheld for an empty/framework-internal-
// free graph (a PRE-EXISTING, unrelated gate — shouldWriteAtoms's own
// empty-notice check, and thinking/*.md's `if !consumer` gate), so "0 files"
// is exactly true end to end. Under the FULL profile (this function's own
// empty-string-profile default, which resolves to "full" for a manifest-less
// dir), atoms-*.md/thinking/*.md remain unconditional regardless of domain
// content — an intentionally separate, PRE-EXISTING design this task does not
// change (see shouldWriteAtoms's own doc comment in gen_spec.go) — so
// asserting "0 files" there would conflate two independent gates.
//
// Discrimination: see TestGenSpec_MissingGraph_MalformedStillErrors — a
// graph.json that EXISTS but is malformed (a decode error, not IsNotExist)
// must still surface as a real error, proving errors.Is(err, os.ErrNotExist)
// is the discrimination rather than a blanket error-swallow.
func TestGenSpec_MissingGraphRendersCalmNotice(t *testing.T) {
	t.Parallel()
	// A genuinely empty domain dir: exists, but NO graph.json. Placed under a
	// domains/ parent so repoRootForDomain's tier-1 resolves the project root to
	// the temp root (task #357: genSpec writes GLOSSARY.md + tools/*.md to
	// <repoRoot>/framework/ — a bare temp dir would leak through tier-2 to the
	// real repo's framework/).
	projectRoot := t.TempDir()
	domainDir := filepath.Join(projectRoot, "domains", "empty")
	if _, err := os.Stat(filepath.Join(domainDir, "graph.json")); !os.IsNotExist(err) {
		t.Fatalf("precondition: graph.json must not exist in the temp domain dir")
	}

	written, _, err := genSpec(domainDir, "", "2026-07-12", "consumer", false)
	if err != nil {
		t.Fatalf("R-empty-content-gen-notice: genSpec on missing graph.json must not fail, got: %v", err)
	}
	if len(written) == 0 {
		t.Fatal("R-empty-content-gen-notice: genSpec wrote no files at all (expected the project-shared framework/ files at minimum)")
	}

	// docs/gen/ must either not exist, or exist with 0 files — no
	// REQUIREMENTS.md, no docs/gen/graph.json, nothing.
	genDir := filepath.Join(domainDir, "docs", "gen")
	if st, statErr := os.Stat(genDir); statErr == nil {
		if !st.IsDir() {
			t.Fatalf("%s exists but is not a directory", genDir)
		}
		entries, rdErr := os.ReadDir(genDir)
		if rdErr != nil {
			t.Fatalf("ReadDir %s: %v", genDir, rdErr)
		}
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		if len(entries) != 0 {
			t.Errorf("R-empty-content-gen-notice: docs/gen/ for a missing/empty graph must be empty, got %d entr(y/ies): %v", len(entries), names)
		}
	} else if !os.IsNotExist(statErr) {
		t.Fatalf("stat %s: %v", genDir, statErr)
	}

	// written must not list any docs/gen/ path for this run.
	for _, p := range written {
		if strings.Contains(filepath.ToSlash(p), "/docs/gen/") {
			t.Errorf("R-empty-content-gen-notice: written unexpectedly includes a docs/gen/ file for an empty domain: %s", p)
		}
	}
}

// TestGenSpec_MissingGraph_MalformedStillErrors is the non-vacuity control: the
// calm missing-file path must NOT swallow genuine errors. A graph.json that
// EXISTS but is malformed (a decode error, which is NOT os.IsNotExist) must
// still propagate as a real error — proving the IsNotExist check is the
// discrimination, not a blanket error-swallow.
func TestGenSpec_MissingGraph_MalformedStillErrors(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	domainDir := filepath.Join(projectRoot, "domains", "malformed")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("mkdir domain: %v", err)
	}
	garbage := []byte("{ this is not valid json")
	if err := os.WriteFile(filepath.Join(domainDir, "graph.json"), garbage, 0o644); err != nil {
		t.Fatalf("write malformed graph.json: %v", err)
	}
	if _, _, err := genSpec(domainDir, "", "2026-07-12", "", false); err == nil {
		t.Fatal("R-empty-content-gen-notice non-vacuity: a malformed graph.json must still produce a real decode error, got nil")
	}
}

// TestGenSpec_SharedProjectionsModeIndependent enforces
// R-shared-projections-mode-independent: TRACEABILITY.md/COVERAGE.md/
// REPO-MAP.md render byte-identically whether or not the SAME domain's most
// recent gen-spec state includes a --spec run, EXCEPT SPEC.md itself (the
// sole --spec-shaped artifact, separately enforced by
// check_spec_md_current). BuildTraceability/BuildCoverage are pure functions
// of g alone (no specRows dependency) so they are trivially mode-independent
// by construction; REPO-MAP.md is the interesting case — its own "Generated
// docs" listing must still name SPEC.md on a PLAIN run that follows an
// earlier --spec run (the file already exists on disk), via genSpec's own
// "SPEC.md acknowledgment on a plain run" logic (gen_spec.go) — otherwise a
// plain `hotam land` immediately after a `hotam gen-spec --spec` would make
// REPO-MAP.md regress to a stale, self-contradictory listing.
//
// Task #364: a bare initDomain now scaffolds a genuinely EMPTY graph, under
// which TRACEABILITY.md/COVERAGE.md/REPO-MAP.md are ALL withheld regardless
// of --spec (this same task) — proving nothing about mode-independence. This
// test seeds minimal real content first (seedMinimalRequirement) so all
// three files actually render under both modes.
func TestGenSpec_SharedProjectionsModeIndependent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	domainDir := filepath.Join(root, "domains", "shared-mode-test")
	if _, err := initDomain(domainDir, "shared-mode-test", "2026-07-23"); err != nil {
		t.Fatalf("initDomain: %v", err)
	}
	seedMinimalRequirement(t, domainDir, "shared-mode-test", "2026-07-23")

	names := []string{"TRACEABILITY.md", "COVERAGE.md", "REPO-MAP.md"}
	readAll := func(t *testing.T) map[string]string {
		t.Helper()
		out := map[string]string{}
		for _, n := range names {
			data, err := os.ReadFile(filepath.Join(domainDir, "docs", "gen", n))
			if err != nil {
				t.Fatalf("read %s: %v", n, err)
			}
			out[n] = string(data)
		}
		return out
	}

	// (1) A REAL --spec run first — this domain's one seeded Requirement is
	// INHERENTLY_PROSE with no verified_by, so CollectSpecRows finds nothing
	// to execute (no real `go test` subprocess spawned), keeping this fast
	// while still exercising includeSpec=true's code path and establishing
	// SPEC.md on disk.
	if _, _, err := genSpec(domainDir, "", "2026-07-23", "", true); err != nil {
		t.Fatalf("genSpec --spec: %v", err)
	}
	afterSpec := readAll(t)

	// (2) A subsequent PLAIN run (includeSpec=false) on the SAME,
	// already-spec'd domain: SPEC.md already exists on disk, so REPO-MAP.md's
	// own acknowledgment logic still lists it — all three files must render
	// byte-identically to the --spec run.
	if _, _, err := genSpec(domainDir, "", "2026-07-23", "", false); err != nil {
		t.Fatalf("genSpec plain (after --spec): %v", err)
	}
	afterPlain := readAll(t)
	for _, n := range names {
		if afterSpec[n] != afterPlain[n] {
			t.Errorf("R-shared-projections-mode-independent violated: %s differs between a --spec run and a subsequent plain run:\n--spec:\n%s\n\nplain:\n%s", n, afterSpec[n], afterPlain[n])
		}
	}

	// (3) Round-trip back to --spec once more: still byte-identical, proving
	// this is a stable fixpoint, not an artifact of write order.
	if _, _, err := genSpec(domainDir, "", "2026-07-23", "", true); err != nil {
		t.Fatalf("genSpec --spec (round-trip): %v", err)
	}
	afterSpec2 := readAll(t)
	for _, n := range names {
		if afterSpec[n] != afterSpec2[n] {
			t.Errorf("R-shared-projections-mode-independent violated: %s not stable across a --spec -> plain -> --spec round-trip", n)
		}
	}
}
