package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/PHPCraftdream/HotamSpec/internal/invariants"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func consumerTestGraph(n int, claimPad string) *ontology.Graph {
	g := &ontology.Graph{}
	for i := 0; i < n; i++ {
		enf := ontology.EnforcementENFORCED
		status := ontology.StatusSETTLED
		switch i % 3 {
		case 1:
			enf = ontology.EnforcementSTRUCTURAL
		case 2:
			enf = ontology.EnforcementPROSE
		}
		if i == 4 {
			status = ontology.StatusDRAFT
		}
		g.Requirements = append(g.Requirements, ontology.Requirement{
			ID:          fmt.Sprintf("R-item-%03d", i),
			Claim:       fmt.Sprintf("Item %d shall hold%s", i, claimPad),
			Status:      status,
			Enforcement: enf,
			VerifiedBy:  []string{fmt.Sprintf("spec/tests/item_test.go:TestItem%03d", i)},
			DeclOrder:   i,
		})
	}
	return g
}

func renderConsumerForTest(t *testing.T, g *ontology.Graph) string {
	t.Helper()
	ov := &ViolationsOverride{For: g, Violations: []invariants.Violation{}}
	return RenderClaudeMDFromTemplateWithViolations(g, "acme", t.TempDir(), 0, nil, "2026-10-01", true, ov, "")
}

// The consumer crystal inlines one line per requirement while the list fits
// the budget: id, claim, flag, verified_by test.
func TestConsumerCrystal_InlinesClaimsUnderBudget(t *testing.T) {
	t.Parallel()
	g := consumerTestGraph(6, "")
	out := renderConsumerForTest(t, g)

	for _, want := range []string{
		"- R-item-000 — Item 0 shall hold [E] ← TestItem000",
		"- R-item-001 — Item 1 shall hold [S] ← TestItem001",
		"- R-item-004 — Item 4 shall hold [S] DRAFT ← TestItem004",
		"- R-item-005 — Item 5 shall hold [P] ← TestItem005",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("consumer crystal missing inline line %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Inline list omitted") {
		t.Errorf("under budget the crystal must not switch to counts:\n%s", out)
	}
	if n := utf8.RuneCountInString(out); n > 6000 {
		t.Errorf("small consumer crystal is %d chars, want a lightweight crystal", n)
	}
}

// Above the budget the list is dropped ALL OR NOTHING: counts per flag, every
// DRAFT or not-ENFORCED id, and a pointer to the full text.
func TestConsumerCrystal_SwitchesToCountsAboveBudget(t *testing.T) {
	t.Parallel()
	g := consumerTestGraph(60, strings.Repeat(" long claim padding words", 8))
	out := renderConsumerForTest(t, g)

	if strings.Contains(out, "- R-item-000 —") {
		t.Errorf("above budget no requirement line may be inlined (all-or-nothing):\n%s", out)
	}
	for _, want := range []string{
		"E (enforced) 20 · S (structural) 20 · P (prose) 20",
		"R-item-004[S,DRAFT]",
		"R-item-002[P]",
		"hotam req list --domain domains/acme",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("counts form missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "R-item-000[") {
		t.Errorf("ENFORCED non-draft R-item-000 must not be listed among exceptions:\n%s", out)
	}
}

func TestConsumerCrystal_OverBudgetIndexesAtomPackages(t *testing.T) {
	t.Parallel()
	g := consumerTestGraph(60, strings.Repeat(" long claim padding words", 8))
	g.SelfExecutingAtoms = true
	for i := range g.Requirements {
		g.Requirements[i].ImplementedBy = []string{"model/people/person.go:Person.Name"}
		if i >= 30 {
			g.Requirements[i].ImplementedBy = []string{"model/roles/role.go:Role.Name"}
		}
	}
	out := renderConsumerRequirements(g, "acme")
	for _, pkg := range []string{"model/people", "model/roles"} {
		want := "| [" + pkg + "](domains/acme/docs/gen/spec/" + pkg + ".md) | 30 | 10 | 10 | 10 |"
		if !strings.Contains(out, want) {
			t.Errorf("package counts or shard link missing: %s", want)
		}
	}
	if strings.Contains(out, "- R-item-000 —") {
		t.Error("oversized requirement list remains inline")
	}
}

func TestConsumerCrystalCompressedLinksSelectLanguageShardDirectory(t *testing.T) {
	t.Parallel()
	g := consumerTestGraph(60, strings.Repeat(" long claim padding words", 8))
	g.SelfExecutingAtoms = true
	g.Languages = []string{"en", "ru", "zh"}
	g.DefaultLanguage = "ru"
	paddingRU := strings.Repeat(" подробное нормативное условие", 8)
	paddingZH := strings.Repeat(" 明确的规范状态约束", 20)
	for index := range g.Requirements {
		g.Requirements[index].ImplementedBy = []string{"spec/model/people/person.go:Person.Name"}
		g.Requirements[index].ClaimTexts = ontology.LocalizedText{
			"en": g.Requirements[index].Claim,
			"ru": fmt.Sprintf("Элемент %d должен сохранять состояние%s", index, paddingRU),
			"zh": fmt.Sprintf("条目%d必须保持状态%s", index, paddingZH),
		}
	}
	for _, language := range g.Languages {
		t.Run(language, func(t *testing.T) {
			g.RenderLanguage = language
			output := renderConsumerRequirements(g, "acme")
			target := "domains/acme/docs/gen/spec/" + language + "/model/people.md"
			if !strings.Contains(output, "[spec/model/people]("+target+")") {
				t.Fatalf("compressed crystal does not link to its language-local SPEC shard %q:\n%s", target, output)
			}
			legacy := "domains/acme/docs/gen/spec/model/people." + language + ".md"
			if strings.Contains(output, legacy) {
				t.Fatalf("compressed crystal links to nonexistent filename-localized shard %q", legacy)
			}
		})
	}
}

// A code-authority domain's crystal routes requirement changes through
// sync-domain, never through ProposedRequirement JSON (apply.go refuses it).
func TestConsumerCrystal_CodeAuthoritySaysSyncDomain(t *testing.T) {
	t.Parallel()
	g := consumerTestGraph(2, "")
	g.RequirementsAuthorityCode = true
	g.Discipline = "full"
	out := renderConsumerForTest(t, g)

	for _, want := range []string{
		"hotam sync-domain --domain domains/acme",
		"--confirm-hash <hex>",
		"domains/acme/spec/requirements.go",
		"hotam all-violations --domain domains/acme",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("code-authority crystal missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "ProposedRequirement") {
		t.Errorf("code-authority crystal must not tell agents to write ProposedRequirement JSON:\n%s", out)
	}

	plain := renderConsumerForTest(t, consumerTestGraph(2, ""))
	if strings.Contains(plain, "sync-domain") || !strings.Contains(plain, "hotam land") {
		t.Errorf("non-code-authority crystal must use hotam land, not sync-domain:\n%s", plain)
	}
}

// The consumer crystal keeps the four rules (incl. the phrase "resolver
// decides") and the pointers, and drops the heavy full-profile blocks.
func TestConsumerCrystal_RulesPointersAndNoHeavyBlocks(t *testing.T) {
	t.Parallel()
	out := renderConsumerForTest(t, consumerTestGraph(1, ""))

	for _, want := range []string{
		"resolver decides", "Cite R-ids", "domain's language", "Never hand-edit",
		"ALWAYS, NEVER, MUST, MUST NOT, ONLY, ANY",
		"`hotam what-now`", "`hotam req show <id>`", "`hotam -h`", "domains/acme/README.md",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("consumer crystal missing %q:\n%s", want, out)
		}
	}
	for _, banned := range []string{
		"MEDIATION-LOOP", "EMBEDDED-THINKING", "OPERATOR-RECURSION", "OPERATOR-ROLE",
		"CONSTITUTION", "CONCEPT-MAP", "AGENT-MAP", "LIVE-STATE", "DOMAIN-MAP",
		"framework/tools", "internal/",
	} {
		if strings.Contains(out, banned) {
			t.Errorf("consumer crystal must not carry %q:\n%s", banned, out)
		}
	}
	if !strings.HasSuffix(out, DurableNotesMarkerLine+"\n") {
		t.Errorf("consumer crystal must end with the durable-notes marker")
	}
}

// Optional blocks (stakeholders, domain map, parent, rejected) appear only
// when non-empty.
func TestConsumerCrystal_OptionalBlocksOnlyWhenNonEmpty(t *testing.T) {
	t.Parallel()
	g := consumerTestGraph(1, "")
	empty := renderConsumerForTest(t, g)
	for _, name := range []string{"STAKEHOLDERS", "DOMAIN-MAP", "PARENT-PROJECT", "RECENTLY-REJECTED"} {
		if strings.Contains(empty, BeginSentinel(name)) {
			t.Errorf("empty %s block must not render", name)
		}
	}

	g.Stakeholders = []ontology.Stakeholder{{ID: "owner", Name: "Domain Owner", Domain: "acme"}}
	g.ParentDeclared, g.Parent = true, "umbrella"
	g.Requirements = append(g.Requirements,
		ontology.Requirement{ID: "R-old", Claim: "Old.", Status: ontology.StatusREJECTED, Why: "REJECTED — REPLACES by R-item-000", DeclOrder: 9},
	)
	full := renderConsumerForTest(t, g)
	for _, name := range []string{"STAKEHOLDERS", "PARENT-PROJECT", "RECENTLY-REJECTED"} {
		if !strings.Contains(full, BeginSentinel(name)) {
			t.Errorf("non-empty %s block must render:\n%s", name, full)
		}
	}
	if strings.Contains(full, "- R-old —") {
		t.Errorf("REJECTED requirement must not be in the live list")
	}

	repoRoot := t.TempDir()
	for _, d := range []string{"acme", "other"} {
		if err := os.MkdirAll(filepath.Join(repoRoot, "domains", d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	ov := &ViolationsOverride{For: g, Violations: []invariants.Violation{}}
	multi := RenderClaudeMDFromTemplateWithViolations(g, "acme", repoRoot, 0, map[string]*ontology.Graph{"acme": g}, "2026-10-01", true, ov, "")
	if !strings.Contains(multi, BeginSentinel("DOMAIN-MAP")) {
		t.Errorf("a project with more than one domain must render DOMAIN-MAP:\n%s", multi)
	}
}

// REQUIREMENTS.md is dropped only for consumer + code authority + discipline
// full; the full profile always writes it for a non-empty graph.
func TestRequirementsMDWritten_Matrix(t *testing.T) {
	t.Parallel()
	g := consumerTestGraph(1, "")
	if !RequirementsMDWritten(g, true) || !RequirementsMDWritten(g, false) {
		t.Fatal("plain domain: REQUIREMENTS.md must be written under both profiles")
	}
	g.RequirementsAuthorityCode, g.Discipline = true, "full"
	if RequirementsMDWritten(g, true) {
		t.Error("consumer + code authority + discipline full must drop REQUIREMENTS.md")
	}
	if !RequirementsMDWritten(g, false) {
		t.Error("full profile must still write REQUIREMENTS.md")
	}
	g.Discipline = ""
	if !RequirementsMDWritten(g, true) {
		t.Error("code authority without discipline full keeps REQUIREMENTS.md")
	}
}

// HISTORY/OPEN/UNENFORCED have "their own data" only when something feeds them.
func TestConsumerDocPredicates_OwnData(t *testing.T) {
	t.Parallel()
	all := consumerTestGraph(3, "")
	for i := range all.Requirements {
		all.Requirements[i].Enforcement = ontology.EnforcementENFORCED
		all.Requirements[i].Status = ontology.StatusSETTLED
	}
	if ConsumerHistoryMDHasContent(all) || ConsumerOpenMDHasContent(all) || ConsumerUnenforcedMDHasContent(all) {
		t.Error("all-ENFORCED settled graph has no history/open/unenforced data")
	}
	all.Requirements[0].Enforcement = ontology.EnforcementPROSE
	if !ConsumerUnenforcedMDHasContent(all) {
		t.Error("a PROSE settled requirement feeds UNENFORCED.md")
	}
	all.Requirements[1].Status = ontology.StatusREJECTED
	if !ConsumerHistoryMDHasContent(all) {
		t.Error("a REJECTED requirement feeds HISTORY.md")
	}
}
