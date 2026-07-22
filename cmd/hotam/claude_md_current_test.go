package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PHPCraftdream/HotamSpec/internal/generator"
	"github.com/PHPCraftdream/HotamSpec/internal/invariants"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// claudeMDCurrentTestToday is used as the --today value everywhere this test
// file calls genSpec/cmdLand, kept equal to time.Now() (NOT a fixed pinned
// date like most other cmd/hotam fixtures use) for exactly one reason:
// checkDomainClaudeMDCurrentReal itself has no --today input at all (it is
// invoked generically via Invariant.PostProcessCheck, a signature with no
// date parameter) and therefore always sources today from time.Now()
// internally (see claude_md_current_wiring.go's own doc comment for why
// this is a deliberate, accepted design choice: freshness/OVERDUE lines
// embedded in LIVE-STATE are supposed to reflect real calendar time, exactly
// like a stale "last generated N days ago" banner would be). A test that
// pins genSpec's OWN --today to a fixed past date while
// checkDomainClaudeMDCurrentReal computes its comparison target against the
// REAL current date would introduce a spurious day-boundary mismatch
// unrelated to what each test actually means to prove — using time.Now()
// consistently here keeps every fixture's render and the check's own render
// on the same calendar day, matching how a real operator would actually use
// `hotam gen-spec --claude-md` followed shortly by `hotam all-violations`.
var claudeMDCurrentTestToday = time.Now().Format("2006-01-02")

// findClaudeMDCurrentViolations filters vs down to just
// check_domain_claude_md_current entries, so assertions read as "does THIS
// check fire" rather than "are there any violations at all" (a fixture
// copied from the real hotam-spec-self domain may carry unrelated debt that
// is not this test's concern).
func findClaudeMDCurrentViolations(vs []invariants.Violation) []invariants.Violation {
	var out []invariants.Violation
	for _, v := range vs {
		if v.Check == "check_domain_claude_md_current" {
			out = append(out, v)
		}
	}
	return out
}

// TestCheckDomainClaudeMDCurrent_FreshCrystalPasses proves the positive
// control: a CLAUDE.md that IS exactly what a fresh `hotam gen-spec
// --claude-md` run currently produces must pass clean — the direct
// regression test for the false-positive class this task exists to fix (a
// committed, genuinely fresh crystal must never trip this check, unlike the
// old apply-proposal pre/post-mutation diff bug this same task's item #3
// separately fixes for a different call path).
func TestCheckDomainClaudeMDCurrent_FreshCrystalPasses(t *testing.T) {
	t.Parallel()
	projectRoot, domainDir := copySelfDomainUnderRoot(t)
	// Adopt the crystal convention (a marker file is enough — see
	// crystalConventionExists) and generate a REAL, current crystal.
	if err := os.WriteFile(filepath.Join(projectRoot, ".hotam-spec-project"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	claudeMDPath := filepath.Join(projectRoot, "CLAUDE.md")
	if _, _, err := genSpec(domainDir, claudeMDPath, claudeMDCurrentTestToday, "", false); err != nil {
		t.Fatalf("genSpec: %v", err)
	}
	if _, err := os.Stat(claudeMDPath); err != nil {
		t.Fatalf("precondition: CLAUDE.md must exist after genSpec: %v", err)
	}

	vs, err := allViolations(domainDir)
	if err != nil {
		t.Fatalf("allViolations: %v", err)
	}
	if got := findClaudeMDCurrentViolations(vs); len(got) != 0 {
		t.Fatalf("expected 0 check_domain_claude_md_current violations for a freshly generated crystal, got %d: %v", len(got), got)
	}
}

// TestCheckDomainClaudeMDCurrent_StaleGeneratedPartFires is the mutation
// probe: start from a genuinely fresh crystal (green), hand-edit ONE line
// INSIDE the generated span (before the durable-notes marker), confirm the
// check goes red, then restore byte-identical content and confirm it goes
// green again — proving this is a live content comparison, not a one-shot
// flag, mirroring TestCheckSpecMDCurrent_MUTATION_HandEditedFiresThenClears's
// shape for SPEC.md.
func TestCheckDomainClaudeMDCurrent_StaleGeneratedPartFires(t *testing.T) {
	t.Parallel()
	projectRoot, domainDir := copySelfDomainUnderRoot(t)
	if err := os.WriteFile(filepath.Join(projectRoot, ".hotam-spec-project"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	claudeMDPath := filepath.Join(projectRoot, "CLAUDE.md")
	if _, _, err := genSpec(domainDir, claudeMDPath, claudeMDCurrentTestToday, "", false); err != nil {
		t.Fatalf("genSpec: %v", err)
	}

	genuine, err := os.ReadFile(claudeMDPath)
	if err != nil {
		t.Fatalf("read genuine CLAUDE.md: %v", err)
	}
	generatedPart, _, ok := generator.SplitAtDurableNotesMarker(string(genuine))
	if !ok {
		t.Fatalf("precondition: a freshly generated CLAUDE.md must carry the durable-notes marker")
	}
	// Corrupt one line strictly INSIDE the generated span (never touching the
	// marker line itself or anything after it) — replace the Role block's
	// operator-identity sentence with an obviously wrong claim.
	const anchor = "Guardian: **spec**"
	if !strings.Contains(generatedPart, anchor) {
		t.Fatalf("precondition: generated part must contain %q to corrupt", anchor)
	}
	tampered := strings.Replace(string(genuine), anchor, "Guardian: **HAND-EDITED-STALE-TEXT**", 1)
	if tampered == string(genuine) {
		t.Fatalf("precondition: tamper must actually change the file")
	}
	if err := os.WriteFile(claudeMDPath, []byte(tampered), 0o644); err != nil {
		t.Fatalf("write tampered CLAUDE.md: %v", err)
	}

	vs, err := allViolations(domainDir)
	if err != nil {
		t.Fatalf("allViolations: %v", err)
	}
	got := findClaudeMDCurrentViolations(vs)
	if len(got) == 0 {
		t.Fatalf("expected a check_domain_claude_md_current violation for a hand-edited/stale CLAUDE.md, got none")
	}
	if got[0].ID != domainDir {
		t.Errorf("violation ID = %q, want %q", got[0].ID, domainDir)
	}

	// Restore byte-identical content — must go back to green.
	if err := os.WriteFile(claudeMDPath, genuine, 0o644); err != nil {
		t.Fatalf("restore genuine CLAUDE.md: %v", err)
	}
	vs2, err := allViolations(domainDir)
	if err != nil {
		t.Fatalf("allViolations (restored): %v", err)
	}
	if got2 := findClaudeMDCurrentViolations(vs2); len(got2) != 0 {
		t.Fatalf("expected 0 violations after restoring byte-identical current content, got %d: %v", len(got2), got2)
	}
}

// TestCheckDomainClaudeMDCurrent_DurableNotesTailIgnored is the key
// regression test against a naive whole-file comparison: appending
// arbitrary author text AFTER the durable-notes marker line must NEVER
// trigger a violation, since the template's own documented contract
// (generator.DurableNotesMarkerLine) invites exactly that.
func TestCheckDomainClaudeMDCurrent_DurableNotesTailIgnored(t *testing.T) {
	t.Parallel()
	projectRoot, domainDir := copySelfDomainUnderRoot(t)
	if err := os.WriteFile(filepath.Join(projectRoot, ".hotam-spec-project"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	claudeMDPath := filepath.Join(projectRoot, "CLAUDE.md")
	if _, _, err := genSpec(domainDir, claudeMDPath, claudeMDCurrentTestToday, "", false); err != nil {
		t.Fatalf("genSpec: %v", err)
	}

	genuine, err := os.ReadFile(claudeMDPath)
	if err != nil {
		t.Fatalf("read genuine CLAUDE.md: %v", err)
	}
	withNotes := string(genuine) + "\nMy own durable note: remember to check the widget queue.\nAnother line of notes.\n"
	if err := os.WriteFile(claudeMDPath, []byte(withNotes), 0o644); err != nil {
		t.Fatalf("write CLAUDE.md with durable notes: %v", err)
	}

	vs, err := allViolations(domainDir)
	if err != nil {
		t.Fatalf("allViolations: %v", err)
	}
	if got := findClaudeMDCurrentViolations(vs); len(got) != 0 {
		t.Fatalf("expected 0 violations when only the durable-notes tail was edited, got %d: %v", len(got), got)
	}
}

// TestCheckDomainClaudeMDCurrent_NoOpWhenDomainDirEmpty mirrors
// TestCheckSpecMDCurrent_NoOpWhenDomainDirEmpty's shape for the sibling
// check: an in-memory fixture graph with no DomainDir has nothing on disk to
// compare against.
func TestCheckDomainClaudeMDCurrent_NoOpWhenDomainDirEmpty(t *testing.T) {
	t.Parallel()
	// checkDomainClaudeMDCurrentReal itself early-returns on g.DomainDir ==
	// "" before touching the filesystem at all — exercised directly (not via
	// allViolations, which needs a real *ontology.Graph the loader
	// populated) since this package already imports both invariants and the
	// check function.
	if got := checkDomainClaudeMDCurrentReal(&ontology.Graph{}, nil); len(got) != 0 {
		t.Fatalf("expected no violations for a graph with no DomainDir, got %v", got)
	}
}

// TestCheckDomainClaudeMDCurrent_NoOpWhenCrystalConventionAbsent proves a
// domain whose project root carries NEITHER a CLAUDE.md NOR a
// .hotam-spec-project marker is an honest no-op — resolveClaudeMDPath
// returns "" in that state (crystalConventionExists is false), so there is
// nothing this check could even compare against.
func TestCheckDomainClaudeMDCurrent_NoOpWhenCrystalConventionAbsent(t *testing.T) {
	t.Parallel()
	_, domainDir := copySelfDomainUnderRoot(t)
	// Deliberately create NO CLAUDE.md and NO marker at the project root.

	vs, err := allViolations(domainDir)
	if err != nil {
		t.Fatalf("allViolations: %v", err)
	}
	if got := findClaudeMDCurrentViolations(vs); len(got) != 0 {
		t.Fatalf("expected 0 violations when the project has not adopted the crystal convention at all, got %d: %v", len(got), got)
	}
}

// TestCheckDomainClaudeMDCurrent_NoOpWhenCrystalNeverGenerated proves that a
// project WITH the crystal convention (a marker present) but a domain that
// has never had `hotam gen-spec --claude-md` run for it yet (no CLAUDE.md on
// disk at all) is ALSO an honest no-op — nothing has gone stale, the crystal
// simply does not exist yet.
func TestCheckDomainClaudeMDCurrent_NoOpWhenCrystalNeverGenerated(t *testing.T) {
	t.Parallel()
	projectRoot, domainDir := copySelfDomainUnderRoot(t)
	if err := os.WriteFile(filepath.Join(projectRoot, ".hotam-spec-project"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	// Deliberately never call genSpec with a claudeMDPath — no CLAUDE.md
	// exists anywhere yet.
	if _, err := os.Stat(filepath.Join(projectRoot, "CLAUDE.md")); !os.IsNotExist(err) {
		t.Fatalf("precondition: CLAUDE.md must not exist")
	}

	vs, err := allViolations(domainDir)
	if err != nil {
		t.Fatalf("allViolations: %v", err)
	}
	if got := findClaudeMDCurrentViolations(vs); len(got) != 0 {
		t.Fatalf("expected 0 violations when no crystal has ever been generated, got %d: %v", len(got), got)
	}
}

// TestApplyProposal_FreshCrystalAndSpecMDNoLongerFalselyBlocked is the
// regression test for item #3 of this task: BEFORE
// ComparesOnDiskProjection/AllViolationsForProposalGate existed,
// internal/proposal/apply.go's applyToGraph computed a pre/post-mutation
// violation diff using plain invariants.AllViolations — which ALWAYS ran
// check_spec_md_current/check_domain_claude_md_current too. Landing ANY
// substantive proposal against a domain with a genuinely fresh, committed
// SPEC.md and/or CLAUDE.md would false-block: the "after" graph is the
// in-memory mutated graph, never regenerated through `hotam gen-spec`, so
// the committed projection is guaranteed to disagree with a fresh render of
// the POST-mutation graph — a new violation both checks correctly report
// against the mutated state, which the old pre/post diff misread as "this
// proposal introduced a NEW violation" and refused to land. This test proves
// `hotam land` on such a domain now succeeds.
func TestApplyProposal_FreshCrystalAndSpecMDNoLongerFalselyBlocked(t *testing.T) {
	t.Parallel()
	projectRoot, domainDir := copySelfDomainUnderRoot(t)
	if err := os.WriteFile(filepath.Join(projectRoot, ".hotam-spec-project"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	claudeMDPath := filepath.Join(projectRoot, "CLAUDE.md")
	// Generate a genuinely fresh, committed CLAUDE.md BEFORE landing anything
	// — the exact precondition that used to false-block.
	if _, _, err := genSpec(domainDir, claudeMDPath, claudeMDCurrentTestToday, "", false); err != nil {
		t.Fatalf("baseline genSpec: %v", err)
	}
	if vs, err := allViolations(domainDir); err != nil {
		t.Fatalf("allViolations (baseline): %v", err)
	} else if got := findClaudeMDCurrentViolations(vs); len(got) != 0 {
		t.Fatalf("precondition: baseline crystal must be clean, got %v", got)
	}

	proposalPath := filepath.Join(t.TempDir(), "proposal.json")
	proposalJSON := `{
		"kind": "Requirement",
		"id": "R-land-no-false-block-e4",
		"claim": "hotam land succeeds against a domain with a fresh committed crystal on disk",
		"owner": "framework-author",
		"status": "DRAFT",
		"why": "E4 regression coverage for the apply-proposal pre/post-mutation diff false-positive"
	}`
	if err := os.WriteFile(proposalPath, []byte(proposalJSON), 0o644); err != nil {
		t.Fatalf("write proposal: %v", err)
	}

	if err := cmdLand([]string{
		"--domain", domainDir,
		"--today", claudeMDCurrentTestToday,
		proposalPath,
	}); err != nil {
		t.Fatalf("cmdLand unexpectedly failed (false-block regression): %v", err)
	}
}
