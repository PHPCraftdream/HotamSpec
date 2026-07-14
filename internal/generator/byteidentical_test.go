package generator

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// fixtureGraphPath is the small, synthetic, hand-curated graph used for the
// package's byte-identity contract (P2-2). It is deliberately tiny (~20
// nodes) but exercises every template branch: SETTLED/DRAFT/REJECTED/OPEN
// requirement statuses, HELD and DECIDED conflicts (plus a bare DETECTED
// one), all four Assumption statuses (HOLDS/UNCERTAIN/DEAD/IMPLEMENTS), an
// entity type with mermaid+fields+instances, a Process, a Goal, an Operator,
// freshness fields (last_reviewed_at/review_after/evidence/source_refs), and
// relations of every kind (refines/depends_on/replaces). It is fully
// well-formed (0 invariant violations — see TestFixtureGraphHasNoViolations
// in fixture_test.go) so it is also safe to feed through internal/invariants
// without special-casing.
const fixtureGraphPath = "testdata/fixture-graph.json"

// domainGraphPath is the full, real hotam-spec-self domain graph. It is used
// ONLY by the determinism test (determinism_test.go), the smoke test below,
// and internal/invariants' own all-violations coverage — NOT by the
// byte-identity template tests, which run against the small fixture instead
// (P2-2: a full-domain byte-identity contract required regenerating ~734KB
// of golden fixtures on every content-bearing change to the real domain).
const domainGraphPath = "../../domains/hotam-spec-self/graph.json"

func loadFixtureGraph(t *testing.T) *ontology.Graph {
	t.Helper()
	g, err := loader.LoadGraph(fixtureGraphPath)
	if err != nil {
		t.Fatalf("LoadGraph(%s): %v", fixtureGraphPath, err)
	}
	return g
}

func loadDomainGraph(t *testing.T) *ontology.Graph {
	t.Helper()
	g, err := loader.LoadGraph(domainGraphPath)
	if err != nil {
		t.Fatalf("LoadGraph(%s): %v", domainGraphPath, err)
	}
	return g
}

// diffReportInto renders the line-level diff summary (byte counts + first
// differing line +/- context) into b, used by diffReport.
func diffReportInto(b *strings.Builder, name, got, want string) {
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
	b.WriteString("got bytes=" + strconv.Itoa(len(got)) + " want bytes=" + strconv.Itoa(len(want)) + "\n")
	b.WriteString("first differing line index: " + strconv.Itoa(first) + "\n")
	for i := start; i < end; i++ {
		gotLine := ""
		if i < len(gotLines) {
			gotLine = gotLines[i]
		}
		marker := "  "
		wantLine := ""
		if i < len(wantLines) {
			wantLine = wantLines[i]
		}
		if i >= len(wantLines) {
			marker = "G>"
		} else if i >= len(gotLines) {
			marker = "W<"
		} else if gotLine != wantLine {
			marker = "* "
		}
		b.WriteString(marker + " line[" + strconv.Itoa(i) + "]\n    got:  " + truncate(gotLine, 160) + "\n    want: " + truncate(wantLine, 160) + "\n")
	}
}

func diffReport(t *testing.T, name, got, want string) {
	t.Helper()
	if got == want {
		return
	}
	var b strings.Builder
	b.WriteString("\n=== byte-identity FAILED for " + name + " ===\n")
	diffReportInto(&b, name, got, want)
	t.Error(b.String())
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func TestBuildRequirements_ByteIdenticalToFixture(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	got := BuildRequirements(g, "hotam-spec-self", false)
	want, err := os.ReadFile("testdata/fixture/REQUIREMENTS.md")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "REQUIREMENTS.md", got, string(want))
}

func TestBuildTensions_ByteIdenticalToFixture(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	got := BuildTensions(g)
	want, err := os.ReadFile("testdata/fixture/TENSIONS.md")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "TENSIONS.md", got, string(want))
}

// TestGenSpec_SmokeOnRealDomain asserts every template produces non-empty
// output when run against the real hotam-spec-self domain graph and does not
// panic. It is the "does not fall over on real data" complement to the
// byte-identity contract, which now lives on the small fixture graph.
func TestGenSpec_SmokeOnRealDomain(t *testing.T) {
	t.Parallel()
	g := loadDomainGraph(t)

	type build struct {
		name string
		fn   func(*ontology.Graph) string
	}
	builds := []build{
		{"REQUIREMENTS.md", func(g *ontology.Graph) string { return BuildRequirements(g, "hotam-spec-self", false) }},
		{"TENSIONS.md", BuildTensions},
		{"OPEN.md", BuildOpen},
		{"UNENFORCED.md", BuildUnenforced},
		{"GLOSSARY.md", func(g *ontology.Graph) string { return BuildGlossary(g, false) }},
		{"HISTORY.md", BuildHistory},
		{"DECISIONS.md", BuildDecisions},
		{"CONSTITUTION.md", func(g *ontology.Graph) string { return BuildConstitution(g, "hotam-spec-self", false) }},
		{"ENTITIES.md", func(g *ontology.Graph) string { return BuildEntities(g, "hotam-spec-self") }},
		{"FRAMEWORK-INVARIANTS.md", func(g *ontology.Graph) string { return BuildFrameworkInvariants(g, "hotam-spec-self") }},
		{"REPO-MAP.md", func(g *ontology.Graph) string {
			return BuildRepoMap(g, "hotam-spec-self", hotamSpecSelfFixtureGenDocs(), false, false, false)
		}},
		{"live-state.md", func(g *ontology.Graph) string { return BuildLiveState(g, 27646, "2026-07-12") }},
	}
	for _, b := range builds {
		got := b.fn(g)
		if strings.TrimSpace(got) == "" {
			t.Errorf("%s: empty output against real domain graph", b.name)
		}
	}

	graphJSON, err := BuildGraphJSON(g)
	if err != nil {
		t.Fatalf("BuildGraphJSON: %v", err)
	}
	if strings.TrimSpace(graphJSON) == "" {
		t.Error("docs-gen-graph.json: empty output against real domain graph")
	}
}

// TestSmoke_EveryBuildTemplateOnRealDomainNoPanicNoEmpty enforces R-smoke-test:
// load the REAL hotam-spec-self domain graph (not the small fixture) and run
// every graph-dependent Build* template — including the atoms builders, the
// root-CLAUDE.md and AGENT-CONTEXT renderers, and the tool-derived section that
// the existing smoke test omits — asserting each returns non-empty output and
// none panics. The explicit per-call recover guard turns a panic (the framework
// "falling over on real data") into a named failure rather than a bare crash.
func TestSmoke_EveryBuildTemplateOnRealDomainNoPanicNoEmpty(t *testing.T) {
	t.Parallel()
	g := loadDomainGraph(t)
	repoRoot := t.TempDir()

	type build struct {
		name string
		fn   func(*ontology.Graph) string
	}
	builds := []build{
		{"REQUIREMENTS.md", func(g *ontology.Graph) string { return BuildRequirements(g, "hotam-spec-self", false) }},
		{"TENSIONS.md", BuildTensions},
		{"OPEN.md", BuildOpen},
		{"UNENFORCED.md", BuildUnenforced},
		{"GLOSSARY.md", func(g *ontology.Graph) string { return BuildGlossary(g, false) }},
		{"HISTORY.md", BuildHistory},
		{"DECISIONS.md", BuildDecisions},
		{"CONSTITUTION.md", func(g *ontology.Graph) string { return BuildConstitution(g, "hotam-spec-self", false) }},
		{"ENTITIES.md", func(g *ontology.Graph) string { return BuildEntities(g, "hotam-spec-self") }},
		{"FRAMEWORK-INVARIANTS.md", func(g *ontology.Graph) string { return BuildFrameworkInvariants(g, "hotam-spec-self") }},
		{"REPO-MAP.md", func(g *ontology.Graph) string {
			return BuildRepoMap(g, "hotam-spec-self", hotamSpecSelfFixtureGenDocs(), false, false, false)
		}},
		{"ATOMS_OPERATOR.md", BuildAtomsOperator},
		{"ATOMS_SUBSTRATE.md", BuildAtomsSubstrate},
		{"ATOMS_DISCIPLINE.md", BuildAtomsDiscipline},
		{"ATOMS_CHECK.md", BuildAtomsCheck},
		{"CLAUDE.md", func(g *ontology.Graph) string {
			return RenderClaudeMDFromTemplate(g, "hotam-spec-self", repoRoot, 27646, nil, "2026-07-12", false)
		}},
		{"AGENT-CONTEXT.md", func(g *ontology.Graph) string { return BuildAgentContext(g, "hotam-spec-self", 27646, "2026-07-12") }},
		{"live-state.md", func(g *ontology.Graph) string { return BuildLiveState(g, 27646, "2026-07-12") }},
	}
	for _, b := range builds {
		var out string
		var panicked bool
		func() {
			defer func() {
				if r := recover(); r != nil {
					panicked = true
					t.Errorf("%s: PANIC running against real domain graph: %v", b.name, r)
				}
			}()
			out = b.fn(g)
		}()
		if panicked {
			continue
		}
		if strings.TrimSpace(out) == "" {
			t.Errorf("%s: empty output against real domain graph", b.name)
		}
	}

	// graph-json build (returns error, not string)
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("docs-gen-graph.json: PANIC: %v", r)
			}
		}()
		graphJSON, err := BuildGraphJSON(g)
		if err != nil {
			t.Fatalf("BuildGraphJSON: %v", err)
		}
		if strings.TrimSpace(graphJSON) == "" {
			t.Error("docs-gen-graph.json: empty output against real domain graph")
		}
	}()

	// registry-derived builder (no graph dependency, but part of the gen pipeline)
	var derived string
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("tool-derived-section.md: PANIC: %v", r)
			}
		}()
		derived = BuildToolDerivedSection()
	}()
	if strings.TrimSpace(derived) == "" {
		t.Error("tool-derived-section.md: empty output")
	}
}
