package invariants

import (
	"os"
	"path/filepath"
	"testing"
	"unicode/utf8"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// TestCheckOperatorWithinBudget_CrystalCharsExternalDomainNotThisRepo is the
// exact false-positive an independent review reported (task #108): an
// external domain whose Operator uses CRYSTAL_CHARS with a small limit must
// be measured against the EXTERNAL domain's own resident crystal, never
// against THIS framework's own root CLAUDE.md. Before the fix, the check
// consulted paths.ProjectRoot() (a CWD-based marker search) which, during a
// `go test` run from internal/invariants, resolves THIS repository's root
// and reads its ~16k-rune CLAUDE.md -- so any small limit would falsely fire.
//
// Layout simulated: <tmp>/domains/external/ (domainDir, parent == "domains")
// with the crystal at <tmp>/CLAUDE.md. limit is set to the external crystal's
// own rune count, so a correct (domain-relative) read never fires; a
// CWD-based read of this repo's crystal would.
func TestCheckOperatorWithinBudget_CrystalCharsExternalDomainNotThisRepo(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	domainDir := filepath.Join(tmp, "domains", "external")
	// A small external crystal (distinctly smaller than this repo's ~16k).
	extCrystal := "# external resident crystal\nsmall working set\n"
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("mkdir domainDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "CLAUDE.md"), []byte(extCrystal), 0o644); err != nil {
		t.Fatalf("write external crystal: %v", err)
	}
	crystalRunes := utf8.RuneCountInString(extCrystal)

	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{sA},
		Operators: []ontology.Operator{
			{ID: "OP-1", Stakeholder: "sa", Lifecycle: "ACTIVE", ContextBudget: ontology.ContextBudget{
				Limit:   crystalRunes, // exactly at limit -> NOT over
				Measure: ontology.BudgetMeasureCRYSTAL_CHARS,
			}},
		},
		DomainDir: domainDir,
	}
	// This repo's own CLAUDE.md is thousands of runes; if the check read it,
	// size(~16k) > limit(crystalRunes) would fire. Assert no fire.
	if vs := runCheck(t, "check_operator_within_budget", g); len(vs) != 0 {
		t.Fatalf("external domain must be measured against its own small crystal, not this repo's; got %v", vs)
	}
	// Non-vacuous boundary: a limit ONE below the external crystal's rune
	// count MUST fire, proving the external crystal is the one being measured.
	g.Operators[0].ContextBudget.Limit = crystalRunes - 1
	if vs := runCheck(t, "check_operator_within_budget", g); !hasViolationFor(vs, "OP-1") {
		t.Fatalf("limit just under the external crystal's rune count must fire; got %v", vs)
	}
}

// TestCheckOperatorWithinBudget_CrystalCharsCountsRunesNotBytes proves the
// measurement convention matches the generator's post-#102 fixpoint
// (utf8.RuneCountInString, see internal/generator/claudemd.go): a crystal
// with multibyte content is measured in runes, not bytes. The fixture is
// asserted to actually have rune count != byte count, so the test cannot
// pass vacuously on ASCII-only content.
func TestCheckOperatorWithinBudget_CrystalCharsCountsRunesNotBytes(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	domainDir := filepath.Join(tmp, "domains", "rune-test")
	// em-dash (U+2014, 3 bytes) + Cyrillic capital Zhe (U+0416, 2 bytes) = 2 runes / 5 bytes.
	content := "—Ж"
	if utf8.RuneCountInString(content) == len(content) {
		t.Fatalf("fixture broken: rune count (%d) must differ from byte count (%d)",
			utf8.RuneCountInString(content), len(content))
	}
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("mkdir domainDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "CLAUDE.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write crystal: %v", err)
	}

	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{sA},
		Operators: []ontology.Operator{
			{ID: "OP-1", Stakeholder: "sa", Lifecycle: "ACTIVE", ContextBudget: ontology.ContextBudget{
				Limit:   2, // == rune count; would fire if measured as 5 bytes
				Measure: ontology.BudgetMeasureCRYSTAL_CHARS,
			}},
		},
		DomainDir: domainDir,
	}
	if vs := runCheck(t, "check_operator_within_budget", g); len(vs) != 0 {
		t.Fatalf("2-rune crystal at limit 2 must not fire (runes, not bytes); got %v", vs)
	}
	g.Operators[0].ContextBudget.Limit = 1 // below rune count -> must fire
	if vs := runCheck(t, "check_operator_within_budget", g); !hasViolationFor(vs, "OP-1") {
		t.Fatalf("2-rune crystal at limit 1 must fire; got %v", vs)
	}
}

// TestCheckOperatorWithinBudget_CrystalCharsSelfHostedReadsRealCrystal is the
// in-repo regression guard: after making resolution domain-relative, the
// self-hosted domain (domains/hotam-spec-self) must STILL resolve to the real
// root CLAUDE.md and measure it. Loading via LoadGraph populates DomainDir
// (../../domains/hotam-spec-self), whose parent is "domains" -> root ../.. =
// this repo's root. A tiny limit MUST fire against the real multi-thousand-rune
// crystal, proving the file is actually read (size != 0), while the real
// budget stays OK.
func TestCheckOperatorWithinBudget_CrystalCharsSelfHostedReadsRealCrystal(t *testing.T) {
	t.Parallel()
	g, err := loader.LoadGraph(domainGraphPath)
	if err != nil {
		t.Fatalf("LoadGraph(%s): %v", domainGraphPath, err)
	}
	if g.DomainDir == "" {
		t.Fatalf("LoadGraph must populate DomainDir (got empty)")
	}

	// (1) The real operator's budget stays under its crystal -> no violation.
	if vs := runCheck(t, "check_operator_within_budget", g); len(vs) != 0 {
		t.Fatalf("self-hosted crystal within its real budget must not fire; got %v", vs)
	}

	// (2) A tiny limit against the real root CLAUDE.md (thousands of runes)
	// MUST fire -- proves the in-repo crystal is actually being read, not 0.
	g.Operators = []ontology.Operator{
		{ID: "OP-probe", Stakeholder: g.Stakeholders[0].ID, Lifecycle: "ACTIVE", ContextBudget: ontology.ContextBudget{
			Limit:   10,
			Measure: ontology.BudgetMeasureCRYSTAL_CHARS,
		}},
	}
	if vs := runCheck(t, "check_operator_within_budget", g); !hasViolationFor(vs, "OP-probe") {
		t.Fatalf("in-repo crystal (thousands of runes) at limit 10 must fire, proving the real CLAUDE.md is read; got %v", vs)
	}
}
