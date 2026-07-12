package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpecGo/internal/ontology"
)

// TestRenderClaudeMDFromTemplate_Fixture drives the full root-CLAUDE.md
// renderer end-to-end against the well-formed fixture, which cascades through
// every MIND and BUSINESS sub-block (OPERATOR-ROLE, MEDIATION-LOOP,
// EMBEDDED-THINKING, EMBEDDED-TOOLS, OPERATOR-RECURSION, LIVE-STATE,
// DOMAIN-MAP, CONSTITUTION, AGENT-MAP, CONCEPT-MAP, RECENTLY-REJECTED) plus
// the constitution-index model and the replaces/hasRejectedReplacesMarker
// helpers. Asserting the sentinel-pair markers + a few invariant substrings
// confirms each bucket rendered and the template placeholders were
// substituted (not left as "<!-- mind -->"/"<!-- business -->").
func TestRenderClaudeMDFromTemplate_Fixture(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	repoRoot := t.TempDir() // no domains/ dir → exercises the "absent" branch of DOMAIN-MAP

	out := RenderClaudeMDFromTemplate(g, "hotam-spec-self", repoRoot, 4200, nil)

	// both template placeholders must have been substituted
	if strings.Contains(out, mindPlaceholder) {
		t.Errorf("mind placeholder was not substituted")
	}
	if strings.Contains(out, businessPlaceholder) {
		t.Errorf("business placeholder was not substituted")
	}
	// every MIND + BUSINESS sentinel pair must be present
	for _, block := range []string{
		"OPERATOR-ROLE", "MEDIATION-LOOP", "EMBEDDED-THINKING",
		"EMBEDDED-TOOLS", "OPERATOR-RECURSION",
		"LIVE-STATE", "DOMAIN-MAP", "CONSTITUTION", "AGENT-MAP",
		"CONCEPT-MAP", "RECENTLY-REJECTED",
	} {
		begin := "<!-- " + block + ":BEGIN -->"
		end := "<!-- " + block + ":END -->"
		bi, ei := strings.Index(out, begin), strings.Index(out, end)
		if bi == -1 || ei == -1 || ei < bi {
			t.Errorf("block %s sentinels missing or mis-ordered (begin=%d end=%d)", block, bi, ei)
		}
	}
	// OPERATOR-ROLE must report the fixture's SETTLED atom count and domain
	if !strings.Contains(out, "Operator of `hotam-spec-self`") {
		t.Errorf("OPERATOR-ROLE did not embed the domain name: missing 'Operator of'")
	}
	// EMBEDDED-THINKING must reference the thinking-docs index
	if !strings.Contains(out, "spec/docs/thinking/") {
		t.Errorf("EMBEDDED-THINKING thinking-doc pointer missing")
	}
	// EMBEDDED-TOOLS must list the ported `req` tool
	if !strings.Contains(out, "`hotam req`") {
		t.Errorf("EMBEDDED-TOOLS missing ported req tool reference")
	}
	// trailing human-notes marker must survive byte-for-byte
	if !strings.Contains(out, "survives every regeneration verbatim") {
		t.Errorf("trailing human-notes marker was dropped")
	}
}

// TestBuildAgentContext_Fixture drives the AGENT-CONTEXT.md renderer, which
// reuses the LIVE-STATE, diagnose-signals, status-counter, and constitution-
// index building blocks. Asserts each section header renders.
func TestBuildAgentContext_Fixture(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	out := BuildAgentContext(g, "", 4200)
	for _, want := range []string{
		"AGENT-CONTEXT.md",
		"## Top actions",
		"## Status counters",
		"## Constitution index",
		"hotam req show",
		"FRAMEWORK-INVARIANTS.md",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("BuildAgentContext missing %q", want)
		}
	}
	// empty-domain fallback must qualify pointers with the default name
	if !strings.Contains(out, "domains/hotam-spec-self") {
		t.Errorf("BuildAgentContext empty domainName should fall back to hotam-spec-self")
	}
}

func TestBuildAgentContext_CleanGraphWhatNow(t *testing.T) {
	t.Parallel()
	// a graph with no signals → the "_(none — graph clean)_" what-now branch
	out := BuildAgentContext(&ontology.Graph{}, "hotam-spec-self", 0)
	if !strings.Contains(out, "_(none — graph clean)_") {
		t.Errorf("clean graph should render the none-actions sentinel, got:\n%s", out)
	}
}

func TestRenderDomainMapBlock_NoDomainsDir(t *testing.T) {
	t.Parallel()
	out := RenderDomainMapBlock(t.TempDir(), nil)
	if !strings.Contains(out, "domains/ directory absent") {
		t.Errorf("missing domains/ dir should render the absent notice, got: %s", out)
	}
}

func TestRenderDomainMapBlock_EmptyDomainsDir(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "domains"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := RenderDomainMapBlock(root, nil)
	if !strings.Contains(out, "_(no domains yet)_") {
		t.Errorf("empty domains/ dir should render the no-domains notice, got: %s", out)
	}
}

// TestRenderDomainMapBlock_PopulatedKnown exercises: real domain directory
// enumeration, the "_"-prefix skip, a KNOWN manifest (hotam-dev), a
// pre-supplied graph (avoids disk load), and the atoms/open-actions rendering.
func TestRenderDomainMapBlock_PopulatedKnown(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "domains", "hotam-dev"), 0o755); err != nil {
		t.Fatal(err)
	}
	// underscore-prefixed dir must be skipped by the enumeration
	if err := os.MkdirAll(filepath.Join(root, "domains", "_archive"), 0o755); err != nil {
		t.Fatal(err)
	}
	fixture := loadFixtureGraph(t)
	out := RenderDomainMapBlock(root, map[string]*ontology.Graph{"hotam-dev": fixture})

	if !strings.Contains(out, "### hotam-dev") {
		t.Errorf("known domain hotam-dev should appear, got: %s", out)
	}
	if strings.Contains(out, "_archive") {
		t.Errorf("underscore-prefixed dir must be skipped, but appeared in output")
	}
	// known manifest for hotam-dev carries a Russian purpose string
	if !strings.Contains(out, "модель разработки") {
		t.Errorf("known manifest description not embedded for hotam-dev")
	}
	// the fixture has 3 SETTLED requirements → atoms-count line
	if !strings.Contains(out, "3 SETTLED") {
		t.Errorf("atoms-count should reflect the fixture's 3 SETTLED reqs, got: %s", out)
	}
}

func TestRenderDomainMapBlock_UnknownDomainFallbacks(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "domains", "brand-new"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := RenderDomainMapBlock(root, map[string]*ontology.Graph{"brand-new": &ontology.Graph{}})
	// unknown manifest → em-dash placeholders for purpose/goals/director
	if !strings.Contains(out, "- **purpose** — —") {
		t.Errorf("unknown domain should fall back to em-dash purpose, got: %s", out)
	}
	if !strings.Contains(out, "open actions** — 0 (graph clean)") {
		t.Errorf("empty graph should report 0 open actions / clean, got: %s", out)
	}
}

func TestRenderRecentlyRejectedBlock_None(t *testing.T) {
	t.Parallel()
	out := RenderRecentlyRejectedBlock(&ontology.Graph{})
	if !strings.Contains(out, "nothing recently rejected") {
		t.Errorf("empty graph should render the no-entries notice, got: %s", out)
	}
}

// TestRenderRecentlyRejectedBlock_CapExceeded builds >recentlyRejectedCap
// rejected-with-replacement requirements to drive the "showing N of M"
// compaction line. Each rejected requirement carries the prose
// "REJECTED — REPLACES" marker (covers hasRejectedReplacesMarker) so it counts
// without needing structural replaces edges.
func TestRenderRecentlyRejectedBlock_CapExceeded(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{}
	for i := 0; i < 5; i++ {
		g.Requirements = append(g.Requirements, ontology.Requirement{
			ID:     "R-rej-" + string(rune('a'+i)),
			Status: ontology.StatusREJECTED,
			Why:    "REJECTED — REPLACES R-successor",
		})
	}
	out := RenderRecentlyRejectedBlock(g)
	if !strings.Contains(out, "REJECTED (REPLACES known)** (5)") {
		t.Errorf("should report 5 total rejected-with-replacement, got: %s", out)
	}
	if !strings.Contains(out, "showing 3 of 5") {
		t.Errorf("cap of 3 should trigger the compaction line, got: %s", out)
	}
}

func TestBuildConstitutionBlock_EmptyGraph(t *testing.T) {
	t.Parallel()
	out := BuildConstitutionBlock(&ontology.Graph{}, "hotam-spec-self")
	if !strings.Contains(out, "_No SETTLED requirements yet._") {
		t.Errorf("empty graph should render the no-settled notice, got: %s", out)
	}
}

func TestRenderOperatorRecursionBlock_DefaultDomain(t *testing.T) {
	t.Parallel()
	out := RenderOperatorRecursionBlock("")
	// empty domain falls back to hotam-spec-self in the spawn-path example
	if !strings.Contains(out, "domains/hotam-spec-self/agents") {
		t.Errorf("empty domain should default to hotam-spec-self in the recursion block")
	}
	out2 := RenderOperatorRecursionBlock("hotam-dev")
	if !strings.Contains(out2, "domains/hotam-dev/agents") {
		t.Errorf("named domain should appear in the recursion block")
	}
	if strings.Contains(out2, "domains/hotam-spec-self/agents") {
		t.Errorf("named domain should replace the default in the recursion block")
	}
}

// --- pure helpers ---

func TestRuneTruncate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in           string
		keep         int
		keepEllipsis string // "..."-suffixed expected when truncation occurs
		truncated    bool
	}{
		{"short", 100, "", false},           // well under threshold → unchanged
		{"abcdefghij", 5, "abcde...", true}, // ASCII truncation
		{"aaaaaaaaaaaa", 5, "aaaaa...", true},
	}
	for _, c := range cases {
		got := runeTruncate(c.in, c.keep)
		if c.truncated {
			// runeTruncate appends "..." only when len(runes) > keep; build expectation
			want := string([]rune(c.in)[:c.keep]) + "..."
			if got != want {
				t.Errorf("runeTruncate(%q,%d) = %q, want %q", c.in, c.keep, got, want)
			}
		} else {
			if got != c.in {
				t.Errorf("runeTruncate(%q,%d) = %q, want unchanged", c.in, c.keep, got)
			}
		}
	}
	// Cyrillic must be truncated by rune count, not byte count
	cyr := strings.Repeat("я", 200)
	got := runeTruncate(cyr, 137)
	if !strings.HasSuffix(got, "...") {
		t.Errorf("long Cyrillic string should be truncated with ellipsis")
	}
	// the kept prefix length in runes must be exactly 137
	kept := []rune(strings.TrimSuffix(got, "..."))
	if len(kept) != 137 {
		t.Errorf("runeTruncate kept %d runes, want 137 (must be rune-aware not byte-aware)", len(kept))
	}
}

func TestRuneTruncateEllipsis(t *testing.T) {
	t.Parallel()
	if got := runeTruncateEllipsis("short", 10); got != "short" {
		t.Errorf("short string should be unchanged, got %q", got)
	}
	long := strings.Repeat("x", 50)
	got := runeTruncateEllipsis(long, 10)
	if !strings.HasSuffix(got, "…") {
		t.Errorf("long string should end with the single … rune, got %q", got)
	}
}

func TestHasRejectedReplacesMarker(t *testing.T) {
	t.Parallel()
	// every dash variant the regex accepts must match
	for _, why := range []string{
		"foo REJECTED — REPLACES bar", // em-dash
		"REJECTED – REPLACES",         // en-dash
		"REJECTED--REPLACES",          // double hyphen
		"REJECTED-REPLACES",           // single hyphen
		"REJECTED  —  REPLACES",       // whitespace around dash
	} {
		if !hasRejectedReplacesMarker(why) {
			t.Errorf("expected marker to match %q", why)
		}
	}
	for _, why := range []string{"REJECTED", "REPLACES something", "no marker here"} {
		if hasRejectedReplacesMarker(why) {
			t.Errorf("should not match %q", why)
		}
	}
}

func TestReplacesMap(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{Requirements: []ontology.Requirement{
		{ID: "R-2", DeclOrder: 1, Relations: []ontology.Relation{{Kind: "replaces", Target: "R-1"}}},
		{ID: "R-3", DeclOrder: 0, Relations: []ontology.Relation{{Kind: "refines", Target: "R-1"}}},
	}}
	rmap := replacesMap(g)
	succ, ok := rmap["R-1"]
	if !ok {
		t.Fatalf("replacesMap should record R-1 as a replaces target")
	}
	if len(succ) != 1 || succ[0] != "R-2" {
		t.Errorf("replacesMap[R-1] = %v, want [R-2] (refines must NOT count)", succ)
	}
}

func TestMapKeysSorted(t *testing.T) {
	t.Parallel()
	got := mapKeysSorted(map[string]struct{}{"b": {}, "a": {}, "c": {}})
	want := []string{"a", "b", "c"}
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Errorf("mapKeysSorted = %v, want %v", got, want)
	}
	if got := mapKeysSorted(map[string]struct{}{}); len(got) != 0 {
		t.Errorf("empty map should yield empty slice, got %v", got)
	}
}

func TestDecisionsMDHasContent(t *testing.T) {
	t.Parallel()
	if DecisionsMDHasContent(&ontology.Graph{}) {
		t.Errorf("graph with no M-tagged reqs should report no content")
	}
	g := &ontology.Graph{Requirements: []ontology.Requirement{{ID: "R-1", MTag: "M-1"}}}
	if !DecisionsMDHasContent(g) {
		t.Errorf("graph with an M-tagged req should report content")
	}
}

func TestEntitiesMDHasContent(t *testing.T) {
	t.Parallel()
	if EntitiesMDHasContent(&ontology.Graph{}) {
		t.Errorf("graph with no entity types should report no content")
	}
	g := &ontology.Graph{EntityTypes: []ontology.EntityType{{Slug: "et"}}}
	if !EntitiesMDHasContent(g) {
		t.Errorf("graph with an entity type should report content")
	}
}

func TestExtractRevisitRationale(t *testing.T) {
	t.Parallel()
	if got := extractRevisitRationale("DECIDED(reason)"); got != "" {
		t.Errorf("non-REVISIT prefix should yield empty, got %q", got)
	}
	if got := extractRevisitRationale("REVISIT_WHEN(bare reason)"); got != "bare reason" {
		t.Errorf("parenthesized revisit rationale should be unwrapped, got %q", got)
	}
	if got := extractRevisitRationale("REVISIT_WHEN  spaced  "); got != "spaced" {
		t.Errorf("bare revisit rationale should be trimmed, got %q", got)
	}
}

func TestClusterRepresentative(t *testing.T) {
	t.Parallel()
	if rep, ok := clusterRepresentative("R-attention-foo"); !ok || rep != "R-attention-registry" {
		t.Errorf("R-attention- prefix should collapse to R-attention-registry, got rep=%q ok=%v", rep, ok)
	}
	if _, ok := clusterRepresentative("R-unrelated-thing"); ok {
		t.Errorf("non-clustered id should report ok=false")
	}
}

func TestClusterIndexItems_CollapsesCluster(t *testing.T) {
	t.Parallel()
	// Two requirements sharing the R-attention- cluster prefix collapse into one
	// "+N related" token headed by the cluster representative.
	reqs := []ontology.Requirement{
		{ID: "R-attention-foo", Enforcement: ontology.EnforcementENFORCED},
		{ID: "R-attention-registry", Enforcement: ontology.EnforcementPROSE},
		{ID: "R-plain", Enforcement: ontology.EnforcementENFORCED},
	}
	items := clusterIndexItems(reqs)
	if len(items) != 2 {
		t.Fatalf("expected 2 items (one collapsed cluster + one plain), got %d: %v", len(items), items)
	}
	var cluster, plain string
	for _, it := range items {
		if strings.Contains(it, "R-attention-registry") {
			cluster = it
		} else {
			plain = it
		}
	}
	if !strings.Contains(cluster, "+1 related") {
		t.Errorf("cluster token should report +1 related sibling, got: %s", cluster)
	}
	if !strings.HasPrefix(cluster, "R-attention-registry") {
		t.Errorf("cluster token should be headed by the representative, got: %s", cluster)
	}
	if !strings.HasPrefix(plain, "R-plain [") {
		t.Errorf("plain token should be the bare id with its flag, got: %q", plain)
	}
}

func TestClusterIndexItems_SoloClusterMemberNoCollapse(t *testing.T) {
	t.Parallel()
	// A cluster-prefix id with NO sibling present renders as a plain token —
	// a "(+0 related)" token would be pure noise.
	reqs := []ontology.Requirement{
		{ID: "R-attention-lonely", Enforcement: ontology.EnforcementENFORCED},
	}
	items := clusterIndexItems(reqs)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d: %v", len(items), items)
	}
	if strings.Contains(items[0], "related") {
		t.Errorf("solo cluster member should not collapse, got: %s", items[0])
	}
}

func TestFlagFor(t *testing.T) {
	t.Parallel()
	if got := flagFor(ontology.EnforcementENFORCED); got != enforcementFlag[ontology.EnforcementENFORCED] {
		t.Errorf("flagFor(ENFORCED) = %q, want the known flag %q", got, enforcementFlag[ontology.EnforcementENFORCED])
	}
	if got := flagFor("BOGUS"); got != "?" {
		t.Errorf("flagFor(unknown) = %q, want ?", got)
	}
}
