package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/diagnose"
	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
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

	out := RenderClaudeMDFromTemplate(g, "hotam-spec-self", repoRoot, 4200, nil, "2026-07-12", false)

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
		"LIVE-STATE", "DOMAIN-MAP", "PARENT-PROJECT", "CONSTITUTION", "AGENT-MAP",
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
	// EMBEDDED-THINKING must reference the thinking-docs index (real path:
	// domains/<domain>/docs/gen/thinking/, not the nonexistent spec/docs/thinking/
	// this used to assert -- see the external-review path-accuracy fix)
	if !strings.Contains(out, "docs/gen/thinking/") {
		t.Errorf("EMBEDDED-THINKING thinking-doc pointer missing")
	}
	// EMBEDDED-TOOLS must list the implemented `req` tool
	if !strings.Contains(out, "`hotam req`") {
		t.Errorf("EMBEDDED-TOOLS missing implemented req tool reference")
	}
	// trailing human-notes marker must survive byte-for-byte
	if !strings.Contains(out, "survives every regeneration verbatim") {
		t.Errorf("trailing human-notes marker was dropped")
	}
}

// TestRenderEmbeddedThinkingBlock_DistillsCanonPerSection enforces
// R-operator-crystal-embeds-thinking-distilled: the EMBEDDED-THINKING block
// must embed one RULE line per §-section, sourcing its rule text from the
// section's Canon (guarded through shortForm/firstWholeSentence), not a bare
// slug-list. Every registered section's short-formed Canon must appear in the
// rendered block, and the bullet must follow the "- **§X** — <rule>" shape.
func TestRenderEmbeddedThinkingBlock_DistillsCanonPerSection(t *testing.T) {
	t.Parallel()
	out := RenderEmbeddedThinkingBlock("hotam-spec-self", false)
	sections := methodology.Sections.All()
	if len(sections) == 0 {
		t.Fatal("methodology.Sections must not be empty")
	}
	for _, s := range sections {
		want := "- **" + s.Slug + "** — " + shortForm(s.Canon, "")
		if !strings.Contains(out, want) {
			t.Errorf("EMBEDDED-THINKING missing the Canon-distilled RULE line for %s; want line %q", s.Slug, want)
		}
	}
	// the path link to the full thinking-doc must survive (deep-dive pointer)
	if !strings.Contains(out, "docs/gen/thinking/<slug>.md") {
		t.Errorf("EMBEDDED-THINKING must keep the thinking-doc path pointer")
	}
}

// TestRenderEmbeddedThinkingBlock_NoMidWordTruncation mirrors
// TestShortForm_NoMidWordEllipsis's no-mid-word-stub contract
// (R-crystal-carries-short-form) for the EMBEDDED-THINKING block: for every
// current Canon value, the embedded RULE must NOT be truncated mid-word with
// a "..." stub. Today's Canon values are single standalone sentences, so the
// firstWholeSentence guard returns them whole; this test is a regression guard
// against future Canon drift into verbose multi-sentence text.
func TestRenderEmbeddedThinkingBlock_NoMidWordTruncation(t *testing.T) {
	t.Parallel()
	out := RenderEmbeddedThinkingBlock("hotam-spec-self", false)
	if strings.Contains(out, "...") {
		t.Errorf("EMBEDDED-THINKING must not contain a mid-word '...' stub, but got:\n%s", out)
	}
	for _, s := range methodology.Sections.All() {
		rule := shortForm(s.Canon, "")
		if strings.HasSuffix(rule, "...") {
			t.Errorf("RULE for %s was truncated mid-word: %q", s.Slug, rule)
		}
	}
}

// TestRenderEmbeddedThinkingBlock_CrystalWithinBudget is a crystal-size
// regression guard (R-context-budget-rule). The historical failure mode this
// requirement's own `why` documents is the OLD multi-paragraph RULE+WHY era
// that blew the crystal to ~200k chars and breached the 150k host cap. This
// sanity-checks that the full root crystal — which now carries the
// Canon-distilled EMBEDDED-THINKING block — still converges to a fixpoint and
// stays comfortably below the cap. The EMBEDDED-THINKING block is
// registry-driven and identical across domains, so Canon growth would show up
// in the fixture crystal too; the 130000 bound (the documented warn threshold)
// leaves generous headroom under the 150000 hard cap.
func TestRenderEmbeddedThinkingBlock_CrystalWithinBudget(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	repoRoot := t.TempDir()
	count, err := ComputeCrystalCharCountFixpoint(g, "hotam-spec-self", repoRoot, nil, "2026-07-13", false)
	if err != nil {
		t.Fatalf("crystal char-count fixpoint did not converge: %v", err)
	}
	const warnThreshold = 130000
	if count >= warnThreshold {
		t.Errorf("root crystal char count %d exceeds the %d warn threshold (R-context-budget-rule); the EMBEDDED-THINKING Canon distillation likely regressed the budget", count, warnThreshold)
	}
}

// TestBuildAgentContext_Fixture drives the AGENT-CONTEXT.md renderer, which
// reuses the LIVE-STATE, diagnose-signals, status-counter, and constitution-
// index building blocks. Asserts each section header renders.
func TestBuildAgentContext_Fixture(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	out := BuildAgentContext(g, "", 4200, "2026-07-12", false)
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
	out := BuildAgentContext(&ontology.Graph{}, "hotam-spec-self", 0, "2026-07-12", false)
	if !strings.Contains(out, "_(none — graph clean)_") {
		t.Errorf("clean graph should render the none-actions sentinel, got:\n%s", out)
	}
}

// TestBuildAgentContext_TodayIsInjectable proves BuildAgentContext's today
// parameter is truly injectable rather than silently recomputed via
// time.Now(): the "## Status counters" line unconditionally embeds today
// (renderAgentContextCounters's "(as of %s)" suffix), so two renders of the
// same graph with two different explicit today values must produce output
// that differs by exactly that date — nothing else, since the fixture
// graph's diagnosable content (SETTLED/DRAFT/REJECTED counts, top actions)
// does not itself depend on today.
func TestBuildAgentContext_TodayIsInjectable(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	gotA := BuildAgentContext(g, "hotam-spec-self", 4200, "2026-01-01", false)
	gotB := BuildAgentContext(g, "hotam-spec-self", 4200, "2026-12-31", false)

	if gotA == gotB {
		t.Fatalf("BuildAgentContext with two different today values produced byte-identical output — today is not actually threaded through")
	}
	if !strings.Contains(gotA, "as of 2026-01-01") {
		t.Errorf("BuildAgentContext(today=2026-01-01) status counters do not embed the injected date:\n%s", gotA)
	}
	if !strings.Contains(gotB, "as of 2026-12-31") {
		t.Errorf("BuildAgentContext(today=2026-12-31) status counters do not embed the injected date:\n%s", gotB)
	}
	// The only difference between the two renders must be the injected
	// today value itself — normalizing it away must make them identical.
	normalizedA := strings.ReplaceAll(gotA, "2026-01-01", "<TODAY>")
	normalizedB := strings.ReplaceAll(gotB, "2026-12-31", "<TODAY>")
	if normalizedA != normalizedB {
		t.Errorf("BuildAgentContext outputs differ by more than the injected today value")
	}
}

// TestBuildAgentContext_SameTodayIsByteIdentical proves the idempotency
// property CI's regen-idempotency check needs: rendering twice with the
// SAME explicit today value produces byte-identical output.
func TestBuildAgentContext_SameTodayIsByteIdentical(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	a := BuildAgentContext(g, "hotam-spec-self", 4200, "2026-07-12", false)
	b := BuildAgentContext(g, "hotam-spec-self", 4200, "2026-07-12", false)
	if a != b {
		t.Fatalf("BuildAgentContext with the same today value produced different output across two calls — not idempotent")
	}
}

// TestRenderClaudeMDFromTemplate_SameTodayIsByteIdentical is the root-crystal
// counterpart of the byte-identity property CI's regen-idempotency check
// relies on: `hotam gen-spec --claude-md CLAUDE.md --today <date>` run twice
// with the SAME --today value must produce byte-identical CLAUDE.md content,
// independent of wall-clock time. This was structurally impossible while the
// renderer's transitive callees computed today via time.Now() internally.
func TestRenderClaudeMDFromTemplate_SameTodayIsByteIdentical(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	repoRoot := t.TempDir()
	a := RenderClaudeMDFromTemplate(g, "hotam-spec-self", repoRoot, 4200, nil, "2026-07-12", false)
	b := RenderClaudeMDFromTemplate(g, "hotam-spec-self", repoRoot, 4200, nil, "2026-07-12", false)
	if a != b {
		t.Fatalf("RenderClaudeMDFromTemplate with the same today value produced different output across two calls — not idempotent")
	}
}

func TestRenderDomainMapBlock_NoDomainsDir(t *testing.T) {
	t.Parallel()
	out := RenderDomainMapBlock(t.TempDir(), nil, "2026-07-12")
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
	out := RenderDomainMapBlock(root, nil, "2026-07-12")
	if !strings.Contains(out, "_(no domains yet)_") {
		t.Errorf("empty domains/ dir should render the no-domains notice, got: %s", out)
	}
}

// TestRenderDomainMapBlock_PopulatedKnown exercises: real domain directory
// enumeration, the "_"-prefix skip, an on-disk manifest.json carrying the
// purpose/goals/director presentation fields (task #210 — these used to live
// in an engine-side knownDomainManifests table, now read from the domain's
// own manifest), a pre-supplied graph (avoids disk load), and the
// atoms/open-actions rendering.
func TestRenderDomainMapBlock_PopulatedKnown(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "domains", "hotam-dev"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{
  "self_hosting": false,
  "purpose": "a model of developing the Hotam-Spec repository itself: waves, commits, spawns, verification gates",
  "goals": ["waves land atomically with a green T2 at the boundary", "push only on the resolver's explicit word"],
  "director": "director"
}`
	if err := os.WriteFile(filepath.Join(root, "domains", "hotam-dev", "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	// underscore-prefixed dir must be skipped by the enumeration
	if err := os.MkdirAll(filepath.Join(root, "domains", "_archive"), 0o755); err != nil {
		t.Fatal(err)
	}
	fixture := loadFixtureGraph(t)
	out := RenderDomainMapBlock(root, map[string]*ontology.Graph{"hotam-dev": fixture}, "2026-07-12")

	if !strings.Contains(out, "### hotam-dev") {
		t.Errorf("known domain hotam-dev should appear, got: %s", out)
	}
	if strings.Contains(out, "_archive") {
		t.Errorf("underscore-prefixed dir must be skipped, but appeared in output")
	}
	// the on-disk manifest carries the purpose string
	if !strings.Contains(out, "developing the Hotam-Spec repository itself") {
		t.Errorf("manifest purpose not embedded for hotam-dev, got: %s", out)
	}
	if !strings.Contains(out, "- **goals** — waves land atomically with a green T2 at the boundary, push only on the resolver's explicit word") {
		t.Errorf("manifest goals not embedded for hotam-dev, got: %s", out)
	}
	if !strings.Contains(out, "- **director** — director") {
		t.Errorf("manifest director not embedded for hotam-dev, got: %s", out)
	}
	// the fixture has 3 SETTLED requirements → atoms-count line
	if !strings.Contains(out, "3 SETTLED") {
		t.Errorf("atoms-count should reflect the fixture's 3 SETTLED reqs, got: %s", out)
	}
	// No local CLAUDE.md was placed in the domain dir → no crystal link
	// (honest "no committed file = no lie", task A2 router).
	if strings.Contains(out, "**crystal**") {
		t.Errorf("DOMAIN-MAP should NOT carry a crystal link when no local CLAUDE.md exists, got: %s", out)
	}
}

// TestRenderDomainMapBlock_CrystalLinkWhenLocalCrystalExists is the task A2
// router proof: when a consumer domain carries a LOCAL crystal at
// <domainDir>/CLAUDE.md on disk, the DOMAIN-MAP entry surfaces a direct
// pointer to it so the root crystal routes an agent to that domain's boot
// file. A domain WITHOUT a local crystal (the active domain, whose crystal
// lives at the repo root; or a domain never generated) gets no link.
func TestRenderDomainMapBlock_CrystalLinkWhenLocalCrystalExists(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// "consumer" has a local crystal on disk; "bare" does not.
	consumerDir := filepath.Join(root, "domains", "consumer")
	bareDir := filepath.Join(root, "domains", "bare")
	for _, d := range []string{consumerDir, bareDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(consumerDir, "CLAUDE.md"), []byte("# consumer crystal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := RenderDomainMapBlock(root, map[string]*ontology.Graph{
		"consumer": &ontology.Graph{},
		"bare":     &ontology.Graph{},
	}, "2026-07-12")

	if !strings.Contains(out, "- **crystal** — `domains/consumer/CLAUDE.md`") {
		t.Errorf("DOMAIN-MAP should link the consumer domain's local crystal, got: %s", out)
	}
	// "bare" has no local crystal → it must NOT get a crystal link (and the
	// string "domains/bare/CLAUDE.md" must not appear anywhere for it).
	if strings.Contains(out, "domains/bare/CLAUDE.md") {
		t.Errorf("DOMAIN-MAP should NOT link a crystal for a domain with no local CLAUDE.md, got: %s", out)
	}
}

func TestRenderDomainMapBlock_UnknownDomainFallbacks(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "domains", "brand-new"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := RenderDomainMapBlock(root, map[string]*ontology.Graph{"brand-new": &ontology.Graph{}}, "2026-07-12")
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
	out := BuildConstitutionBlock(&ontology.Graph{}, "hotam-spec-self", false)
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

// TestShortForm_SummaryPriority enforces the summary-priority half of
// R-crystal-carries-short-form: when a non-empty summary is supplied it is
// returned verbatim (trimmed), regardless of the text — the explicit summary
// is the meaningful short form, never a truncation of the text.
func TestShortForm_SummaryPriority(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		text    string
		summary string
		want    string
	}{
		{"summary wins over long text", strings.Repeat("x", 300), "the short version", "the short version"},
		{"summary wins over multi-sentence text", "First sentence. Second sentence.", "use this instead", "use this instead"},
		{"summary is trimmed", "ignored", "  trimmed summary  ", "trimmed summary"},
		{"empty text with summary", "", "summary only", "summary only"},
	}
	for _, c := range cases {
		if got := shortForm(c.text, c.summary); got != c.want {
			t.Errorf("%s: shortForm(%q, %q) = %q, want %q", c.name, c.text, c.summary, got, c.want)
		}
	}
	// empty summary must NOT short-circuit: it falls through to firstWholeSentence.
	if got := shortForm("Only sentence.", ""); got != "Only sentence." {
		t.Errorf("empty summary should fall through to first-whole-sentence, got %q", got)
	}
}

// TestFirstWholeSentence_SentenceBoundary enforces the first-whole-sentence
// fallback half of R-crystal-carries-short-form: text without a summary is
// reduced to its leading whole sentence, splitting on '.', '!', or '?'
// followed by whitespace or end-of-string — never a hard rune cutoff mid-word.
func TestFirstWholeSentence_SentenceBoundary(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			"two sentences, period+space boundary",
			"First whole sentence here. Second sentence follows.",
			"First whole sentence here.",
		},
		{"single sentence ending text", "Only one sentence.", "Only one sentence."},
		{"exclamation boundary", "Stop! Do more.", "Stop!"},
		{"question boundary", "Is this real? Yes.", "Is this real?"},
		{
			"no terminator returns whole text",
			"no sentence terminator at all in this string",
			"no sentence terminator at all in this string",
		},
		{
			"period not followed by whitespace is not a boundary (decimal)",
			"value is 3.14 exactly",
			"value is 3.14 exactly",
		},
		{
			"trailing period at end of string is a boundary",
			"ends with a period.",
			"ends with a period.",
		},
		{"empty string", "   ", ""},
		{"leading whitespace trimmed", "   Trimmed first. Second.", "Trimmed first."},
	}
	for _, c := range cases {
		if got := firstWholeSentence(c.in); got != c.want {
			t.Errorf("%s: firstWholeSentence(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

// TestShortForm_NoMidWordEllipsis enforces the core prohibition of
// R-crystal-carries-short-form: a realistic long imperative message must NOT
// produce a mid-word "..." stub in the rendered short form. The fixture graph's
// real top message ends in "See docs/gen/UNENFORCED.md." which the old
// runeTrunate reduced to the mid-word stub "See doc..."; shortForm must instead
// yield a whole-sentence short form with no "..." suffix.
func TestShortForm_NoMidWordEllipsis(t *testing.T) {
	t.Parallel()
	long := "REFLECTION on enforcement-gradient — 42 SETTLED requirements are " +
		"closeable debt (ENFORCEABLE, still PROSE/STRUCTURAL) — claimed but not " +
		"guaranteed, soft context-debt. See docs/gen/UNENFORCED.md."
	got := shortForm(long, "")
	if strings.HasSuffix(got, "...") {
		t.Errorf("short form must not end with a mid-word '...' stub, got %q", got)
	}
	if !strings.HasSuffix(got, ".") {
		t.Errorf("short form should end at a whole-sentence terminator, got %q", got)
	}
	// the second sentence ("See docs/...") must NOT leak — only the first
	// whole sentence is the short form.
	if strings.Contains(got, "See docs") {
		t.Errorf("short form should be the first whole sentence only, but includes the second: %q", got)
	}
	want := "REFLECTION on enforcement-gradient — 42 SETTLED requirements are " +
		"closeable debt (ENFORCEABLE, still PROSE/STRUCTURAL) — claimed but not " +
		"guaranteed, soft context-debt."
	if got != want {
		t.Errorf("shortForm(long,\"\") = %q, want %q", got, want)
	}
}

// TestSummaryForTarget enforces the Requirement-lookup helper that wires
// summary-priority into domainPulse (R-crystal-carries-short-form): a target
// equal to a Requirement ID returns that requirement's Summary; any other
// target (an axis label, an assumption ID, a composite) returns "" so the
// caller falls back to the first-whole-sentence short form.
func TestSummaryForTarget(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-real-one", Summary: "the explicit summary"},
			{ID: "R-empty-summary", Summary: ""},
		},
	}
	if got := summaryForTarget(g, "R-real-one"); got != "the explicit summary" {
		t.Errorf("summaryForTarget resolving hit = %q, want %q", got, "the explicit summary")
	}
	if got := summaryForTarget(g, "R-empty-summary"); got != "" {
		t.Errorf("summaryForTarget on a Requirement with empty Summary = %q, want \"\" (fall through)", got)
	}
	if got := summaryForTarget(g, "enforcement-gradient"); got != "" {
		t.Errorf("summaryForTarget on a non-Requirement label = %q, want \"\"", got)
	}
	if got := summaryForTarget(g, ""); got != "" {
		t.Errorf("summaryForTarget on empty target = %q, want \"\"", got)
	}
}

// TestDomainPulse_SummaryPreferredWhenTargetResolves enforces the
// summary-priority wiring inside domainPulse (R-crystal-carries-short-form):
// when the TOP signal's Target resolves to a Requirement carrying a non-empty
// Summary, the pulse line carries that summary rather than a sentence of the
// message. Built from a synthetic graph that deterministically yields a P0
// signal whose Target is a Requirement ID (ReflectDeadAssumptionOnEnforcer:
// an ENFORCED requirement resting on a DEAD assumption), so summary-priority is
// the reachable path — not a fixture-dependent guess.
func TestDomainPulse_SummaryPreferredWhenTargetResolves(t *testing.T) {
	t.Parallel()
	today := "2026-07-13"
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{{
			ID:          "R-enforced-target",
			Status:      ontology.StatusSETTLED,
			Enforcement: ontology.EnforcementENFORCED,
			Assumptions: []string{"A-dead-one"},
			Summary:     "INJECTED_SUMMARY_MARKER",
		}},
		Assumptions: []ontology.Assumption{{
			ID:     "A-dead-one",
			Status: ontology.AssumptionDEAD,
		}},
	}
	signals := diagnose.DiagnoseSignals(g, today)
	if len(signals) == 0 {
		t.Fatal("synthetic graph should yield at least one signal")
	}
	if signals[0].Target != "R-enforced-target" {
		t.Fatalf("test setup invariant: top signal target = %q, want R-enforced-target", signals[0].Target)
	}
	_, line := domainPulse(g, today, nil)
	if !strings.Contains(line, "INJECTED_SUMMARY_MARKER") {
		t.Errorf("domainPulse should prefer the resolved Requirement's Summary, got: %s", line)
	}
	if strings.Contains(line, "...") {
		t.Errorf("domainPulse must never emit mid-word '...', got: %s", line)
	}
}

// TestDomainPulse_LongMessageUsesFirstSentence enforces the fallback path of
// domainPulse (R-crystal-carries-short-form): a top signal whose Target does
// NOT resolve to a Requirement (a reflection axis label) is rendered as the
// first whole sentence of its message, never a mid-word truncation. Built from
// a synthetic graph with >5 closeable-debt requirements so
// ReflectUnenforcedSettled fires its two-sentence message (ending
// "See docs/gen/UNENFORCED.md.") whose old runeTruncate form produced the
// mid-word stub "See doc...".
func TestDomainPulse_LongMessageUsesFirstSentence(t *testing.T) {
	t.Parallel()
	today := "2026-07-13"
	reqs := make([]ontology.Requirement, 0, 6)
	for i := 0; i < 6; i++ {
		reqs = append(reqs, ontology.Requirement{
			ID:             fmt.Sprintf("R-debt-%d", i),
			Status:         ontology.StatusSETTLED,
			Enforcement:    ontology.EnforcementSTRUCTURAL,
			Enforceability: ontology.EnforceabilityENFORCEABLE,
		})
	}
	g := &ontology.Graph{Requirements: reqs}
	signals := diagnose.DiagnoseSignals(g, today)
	if len(signals) == 0 {
		t.Fatal("synthetic graph should yield at least one signal")
	}
	if signals[0].Target != "enforcement-gradient" {
		t.Fatalf("test setup invariant: top signal target = %q, want enforcement-gradient", signals[0].Target)
	}
	_, line := domainPulse(g, today, nil)
	if strings.Contains(line, "...") {
		t.Errorf("domainPulse must never emit mid-word '...', got: %s", line)
	}
	// the second sentence ("See docs/gen/UNENFORCED.md.") must be dropped at
	// the first whole-sentence boundary, not leaked/truncated mid-word.
	if strings.Contains(line, "See docs") {
		t.Errorf("short form should be the first whole sentence only, got: %s", line)
	}
	if !strings.Contains(line, "soft context-debt.") {
		t.Errorf("first-whole-sentence short form should carry the first sentence's end, got: %s", line)
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

// === Batch A2 enforcement-debt closure (wave6 category-a) ===
//
// The seven tests below mechanically enforce SETTLED claims that were still
// PROSE/STRUCTURAL + ENFORCEABLE. Each drives a real renderer against the
// fixture graph (or a synthetic graph where the fixture cannot exercise the
// branch) and asserts the claim's observable projection, so a regression in
// the renderer fails the test rather than silently drifting.

// TestRenderRecentlyRejectedBlock_SurfacesEveryMarkerCarrier enforces
// R-recently-rejected-surfaced: the RECENTLY-REJECTED block must list EVERY
// REJECTED requirement whose why carries the "REJECTED — REPLACES" marker
// (completeness), and must EXCLUDE a REJECTED requirement that carries neither
// the prose marker nor a structural replaces edge (discrimination). Stays under
// recentlyRejectedCap so each carrier's id appears verbatim.
func TestRenderRecentlyRejectedBlock_SurfacesEveryMarkerCarrier(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{Requirements: []ontology.Requirement{
		{ID: "R-marker-one", Status: ontology.StatusREJECTED, Why: "REJECTED — REPLACES R-succ-one"},
		{ID: "R-marker-two", Status: ontology.StatusREJECTED, Why: "REJECTED — REPLACES R-succ-two"},
		{ID: "R-rejected-untracked", Status: ontology.StatusREJECTED, Why: "rejected with no marker and no replaces edge"},
	}}
	out := RenderRecentlyRejectedBlock(g)
	// completeness: every marker carrier is listed by id
	for _, id := range []string{"R-marker-one", "R-marker-two"} {
		if !strings.Contains(out, id) {
			t.Errorf("RECENTLY-REJECTED must list marker-carrier %q:\n%s", id, out)
		}
	}
	// discrimination: the untracked rejection (no marker, no edge) must NOT appear
	if strings.Contains(out, "R-rejected-untracked") {
		t.Errorf("RECENTLY-REJECTED must NOT list a rejection lacking both the marker and a replaces edge:\n%s", out)
	}
	// the total reflects exactly the two carriers
	if !strings.Contains(out, "REJECTED (REPLACES known)** (2)") {
		t.Errorf("RECENTLY-REJECTED should report 2 carriers, got:\n%s", out)
	}
}

// TestRenderDomainMapBlock_KnownDomainCarriesAllSixFields enforces
// R-domain-map-generated: each DOMAIN-MAP entry for a known domain must carry
// all six presentation fields — id, purpose/description, goals, director, path,
// atoms-count. The existing DomainMap tests assert only purpose + atoms-count;
// this completes the goals/director/path coverage against the fixture graph.
func TestRenderDomainMapBlock_KnownDomainCarriesAllSixFields(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "domains", "hotam-dev"), 0o755); err != nil {
		t.Fatal(err)
	}
	fixture := loadFixtureGraph(t)
	out := RenderDomainMapBlock(root, map[string]*ontology.Graph{"hotam-dev": fixture}, "2026-07-12")
	for _, want := range []string{
		"### hotam-dev",                     // id
		"- **purpose** — ",                  // description
		"- **goals** — ",                    // goals
		"- **director** — ",                 // director
		"- **path** — `domains/hotam-dev/`", // path
		"- **atoms-count** — 3 SETTLED",     // atoms-count (fixture has 3 SETTLED)
	} {
		if !strings.Contains(out, want) {
			t.Errorf("DOMAIN-MAP hotam-dev entry missing field %q:\n%s", want, out)
		}
	}
}

// TestRenderDomainMapBlock_EntryCarriesOpenActionsPulse enforces the DOMAIN-MAP
// half of R-domain-map-shows-pulse: each entry must carry an "open actions" line
// with the domain's open-action COUNT and its TOP action. The fixture graph has
// a DETECTED conflict, a HELD conflict, and an OPEN requirement, so domainPulse
// yields a non-zero count with a top-action line. (The claim's emit_cipher half
// is separately-tracked stale phrasing from the Python era and is intentionally
// NOT asserted here.)
func TestRenderDomainMapBlock_EntryCarriesOpenActionsPulse(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "domains", "hotam-dev"), 0o755); err != nil {
		t.Fatal(err)
	}
	fixture := loadFixtureGraph(t)
	out := RenderDomainMapBlock(root, map[string]*ontology.Graph{"hotam-dev": fixture}, "2026-07-12")

	var oaLine string
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "**open actions** — ") {
			oaLine = line
			break
		}
	}
	if oaLine == "" {
		t.Fatalf("DOMAIN-MAP hotam-dev entry missing the open-actions line:\n%s", out)
	}
	// the fixture has real signals → the line must NOT be the clean-graph branch
	if strings.Contains(oaLine, "graph clean)") {
		t.Fatalf("fixture graph yields open actions; expected the pulse branch, got the clean branch: %s", oaLine)
	}
	// pulse format: count + top-action carrying a [Pn] priority marker
	if !strings.Contains(oaLine, "(top: [P") {
		t.Errorf("open-actions line must carry the top action with a [Pn] priority marker, got: %s", oaLine)
	}
}

// TestRenderAgentMapBlock_EmptyMarkerWhenNoSubAgents enforces
// R-agent-map-generated: the root crystal must contain an AGENT-MAP sentinel
// block, and in the zero-sub-agent case (today's only reachable state) it must
// render an explicit "no sub-operators yet" marker rather than an empty block —
// so the operator can distinguish "no agents" from "block missing".
func TestRenderAgentMapBlock_EmptyMarkerWhenNoSubAgents(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	repoRoot := t.TempDir()
	out := RenderClaudeMDFromTemplate(g, "hotam-spec-self", repoRoot, 4200, nil, "2026-07-12", false)

	inner, ok := ExtractBlock(out, "AGENT-MAP")
	if !ok {
		t.Fatalf("root crystal missing the AGENT-MAP sentinel block")
	}
	if strings.TrimSpace(inner) == "" {
		t.Fatalf("AGENT-MAP block is empty — must render an explicit empty marker, not nothing")
	}
	if !strings.Contains(inner, "no sub-operators") {
		t.Errorf("zero-sub-agent AGENT-MAP must render an explicit 'no sub-operators' marker, got:\n%s", inner)
	}
}

// TestRenderClaudeMDFromTemplate_NoProseBetweenSentinels enforces
// R-root-claude-md-is-sentinel-only: the regenerated root crystal must be a
// minimal framework-identity header + sentinel-bounded generated blocks + the
// trailing durable-notes marker, with NO stray hand-written prose in the gaps
// between sentinel blocks. Walks every line outside a sentinel span and asserts
// each inter-block gap is whitespace-only.
func TestRenderClaudeMDFromTemplate_NoProseBetweenSentinels(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	repoRoot := t.TempDir()
	out := RenderClaudeMDFromTemplate(g, "hotam-spec-self", repoRoot, 4200, nil, "2026-07-12", false)

	blockNames := []string{
		"OPERATOR-ROLE", "MEDIATION-LOOP", "EMBEDDED-THINKING",
		"EMBEDDED-TOOLS", "OPERATOR-RECURSION",
		"LIVE-STATE", "DOMAIN-MAP", "PARENT-PROJECT", "CONSTITUTION", "AGENT-MAP",
		"CONCEPT-MAP", "RECENTLY-REJECTED",
	}
	begins := make(map[string]bool)
	ends := make(map[string]bool)
	for _, n := range blockNames {
		begins[BeginSentinel(n)] = true
		ends[EndSentinel(n)] = true
	}

	// Walk lines, collecting contiguous regions of text OUTSIDE any sentinel
	// block. A BEGIN flushes the accumulated outside region and enters a block;
	// an END leaves a block; everything else, when outside, accumulates.
	var regions []string
	inside := false
	var cur strings.Builder
	for _, line := range strings.Split(out, "\n") {
		tl := strings.TrimSpace(line)
		switch {
		case begins[tl]:
			regions = append(regions, cur.String())
			cur.Reset()
			inside = true
		case ends[tl]:
			inside = false
		default:
			if !inside {
				cur.WriteString(line)
				cur.WriteString("\n")
			}
		}
	}
	regions = append(regions, cur.String())
	if len(regions) < 3 {
		t.Fatalf("expected header + gaps + trailing regions, got %d region(s)", len(regions))
	}

	// region 0 = the framework-identity header (before the first BEGIN)
	header := regions[0]
	for _, want := range []string{"# CLAUDE.md — Hotam-Spec framework", "**Hotam-Spec**", "Boot:"} {
		if !strings.Contains(header, want) {
			t.Errorf("header region missing %q:\n%s", want, header)
		}
	}
	// regions 1..len-2 = inter-block gaps — the core claim: whitespace-only
	for i := 1; i < len(regions)-1; i++ {
		if strings.TrimSpace(regions[i]) != "" {
			t.Errorf("inter-block gap %d must be whitespace-only (no hand-written prose between sentinels), got:\n%q", i, regions[i])
		}
	}
	// last region = the trailing durable-notes marker
	trailing := regions[len(regions)-1]
	if !strings.Contains(trailing, "survives every regeneration verbatim") {
		t.Errorf("trailing region must carry the durable-notes marker, got:\n%s", trailing)
	}
}

// TestRenderClaudeMDFromTemplate_SubstitutesPlaceholdersPreservesRest enforces
// R-claude-md-template-driven: rendering substitutes the two template
// placeholders (<!-- mind --> / <!-- business -->) with rendered content and
// preserves every other template line verbatim, with the single known exception
// of the boot line's deep-dive pointer (spec/docs/thinking/ → the
// domain-qualified path, a documented targeted replace).
func TestRenderClaudeMDFromTemplate_SubstitutesPlaceholdersPreservesRest(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	repoRoot := t.TempDir()
	const domain = "hotam-spec-self"
	out := RenderClaudeMDFromTemplate(g, domain, repoRoot, 4200, nil, "2026-07-12", false)

	// both placeholders must be substituted away
	if strings.Contains(out, mindPlaceholder) {
		t.Errorf("mind placeholder was not substituted")
	}
	if strings.Contains(out, businessPlaceholder) {
		t.Errorf("business placeholder was not substituted")
	}
	// every other template line preserved verbatim
	for _, want := range []string{
		"# CLAUDE.md — Hotam-Spec framework",
		"**Hotam-Spec** — executable memory and discipline for a human + LLM-agent fleet: understand, evolve, protect, and support a shared model over time. Contradictory requirements are one of its properties — held open as tension-graph nodes, never silently discarded. License: MIT OR Apache-2.0.",
		"<!-- Anything you write below this line survives every regeneration verbatim. Use this space for durable notes, reminders, or context that the generator should never touch. -->",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("a non-placeholder template line was NOT preserved verbatim; missing %q", want)
		}
	}
	// ...except the boot line's deep-dive pointer — the one documented targeted
	// domain-qualified replace: the bare spec/docs/thinking/ form must be GONE
	// and the domain-qualified form must be PRESENT.
	if strings.Contains(out, "`spec/docs/thinking/`") {
		t.Errorf("boot pointer should be domain-qualified, but the bare spec/docs/thinking/ form survived")
	}
	if !strings.Contains(out, "`domains/"+domain+"/docs/gen/thinking/`") {
		t.Errorf("boot pointer should carry the domain-qualified deep-dive path")
	}
}

// TestRenderClaudeMDFromTemplate_SingleDomainConsolidatesToOneCrystal enforces
// R-claude-md-consolidates-when-single-agent: for a single-domain / zero-sub-
// agent fixture, the root renderer emits exactly ONE consolidated crystal
// containing ALL operator-prompt content (the full MIND + BUSINESS bucket), not
// per-agent files. The consolidation condition is rendered into the crystal
// itself: the AGENT-MAP reports "no sub-operators yet" and the
// OPERATOR-RECURSION block states "exactly ONE CLAUDE.md (this file)" under the
// R-claude-md-consolidates-when-single-agent anchor.
func TestRenderClaudeMDFromTemplate_SingleDomainConsolidatesToOneCrystal(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	repoRoot := t.TempDir()
	out := RenderClaudeMDFromTemplate(g, "hotam-spec-self", repoRoot, 4200, nil, "2026-07-12", false)

	// the single render consolidates the FULL operator prompt: all eleven MIND +
	// BUSINESS blocks present in one string (no per-agent split)
	for _, block := range []string{
		"OPERATOR-ROLE", "MEDIATION-LOOP", "EMBEDDED-THINKING",
		"EMBEDDED-TOOLS", "OPERATOR-RECURSION",
		"LIVE-STATE", "DOMAIN-MAP", "PARENT-PROJECT", "CONSTITUTION", "AGENT-MAP",
		"CONCEPT-MAP", "RECENTLY-REJECTED",
	} {
		if _, ok := ExtractBlock(out, block); !ok {
			t.Errorf("consolidated crystal missing block %s (single-domain render must carry the full prompt)", block)
		}
	}
	// the zero-sub-agent consolidation trigger is rendered into the crystal
	agentMap, ok := ExtractBlock(out, "AGENT-MAP")
	if !ok || !strings.Contains(agentMap, "no sub-operators") {
		t.Errorf("zero-sub-agent AGENT-MAP must report 'no sub-operators yet' (the consolidation trigger):\n%s", agentMap)
	}
	// the consolidation rule is stated in the OPERATOR-RECURSION block
	recursion, ok := ExtractBlock(out, "OPERATOR-RECURSION")
	if !ok || !strings.Contains(recursion, "exactly ONE CLAUDE.md (this file)") {
		t.Errorf("OPERATOR-RECURSION must state the 'exactly ONE CLAUDE.md' consolidation rule:\n%s", recursion)
	}
	if !ok || !strings.Contains(recursion, "R-claude-md-consolidates-when-single-agent") {
		t.Errorf("OPERATOR-RECURSION must cite the consolidation anchor:\n%s", recursion)
	}
}

// === Batch A3 enforcement-debt closure (wave6 category-a) ===

// TestRenderEmbeddedToolsBlock_PointerOneLiner enforces
// R-operator-crystal-embeds-tools-distilled: the EMBEDDED-TOOLS block is a
// compact pointer-only reference — it reports the Implemented and Planned tool
// counts (computed from the methodology.Tools registry) and directs the
// operator to `hotam -h` / `hotam status --json` / `hotam req` / `hotam brief`
// / docs/gen/tools/INDEX.md for on-demand detail. It must NOT render one line
// per tool — that earlier distillation shape was superseded (wave-5 review,
// task #128) now that agentic on-demand discovery makes the embedded list
// redundant.
func TestRenderEmbeddedToolsBlock_PointerOneLiner(t *testing.T) {
	t.Parallel()
	out := RenderEmbeddedToolsBlock(false)

	var implemented, planned []methodology.Tool
	for _, tl := range methodology.Tools.All() {
		if tl.Status == methodology.Implemented {
			implemented = append(implemented, tl)
		} else {
			planned = append(planned, tl)
		}
	}
	if len(implemented) == 0 {
		t.Fatal("precondition: registry has no Implemented tools — test is meaningless")
	}

	// The block must report the Implemented count computed from the registry.
	wantImpl := fmt.Sprintf("%d Implemented", len(implemented))
	if !strings.Contains(out, wantImpl) {
		t.Errorf("block should report %q (registry Implemented count), missing:\n%s", wantImpl, out)
	}

	// The block must report the Planned count computed from the registry.
	wantPlanned := fmt.Sprintf("%d Planned", len(planned))
	if !strings.Contains(out, wantPlanned) {
		t.Errorf("block should report %q (registry Planned count), missing:\n%s", wantPlanned, out)
	}

	// The block must point at hotam -h for the full command list.
	if !strings.Contains(out, "`hotam -h`") {
		t.Errorf("block should mention `hotam -h` for the full command list:\n%s", out)
	}

	// The block must point at hotam status --json for structured access.
	if !strings.Contains(out, "hotam status --json") {
		t.Errorf("block should mention `hotam status --json` for structured access:\n%s", out)
	}

	// The block must NOT render one line per Implemented tool (old shape).
	for _, tl := range implemented {
		display := strings.ReplaceAll(tl.Command, "_", "-")
		if strings.Contains(out, "- **"+display+"** — ") {
			t.Errorf("block should NOT have a per-tool bullet for %q (old one-line-per-tool shape):\n%s", tl.Command, out)
		}
	}
}

// TestRenderEmbeddedToolsBlock_IsPureRegistryProjection enforces
// R-tools-registry-generated: the EMBEDDED-TOOLS block is a projection of
// the methodology.Tools registry — the Implemented/Planned counts it reports
// match the registry, and every `hotam <command>` invocation in the output
// names a real Implemented registry tool (drift guard in both directions:
// counts never silently diverge, nothing invented).
func TestRenderEmbeddedToolsBlock_IsPureRegistryProjection(t *testing.T) {
	t.Parallel()
	out := RenderEmbeddedToolsBlock(false)

	implementedDisplays := make(map[string]bool)
	implCount := 0
	plannedCount := 0
	for _, tl := range methodology.Tools.All() {
		display := strings.ReplaceAll(tl.Command, "_", "-")
		if tl.Status == methodology.Implemented {
			implementedDisplays[display] = true
			implCount++
		} else {
			plannedCount++
		}
	}

	// The Implemented count in the output must equal the registry's count.
	if !strings.Contains(out, fmt.Sprintf("%d Implemented", implCount)) {
		t.Errorf("block should report %d Implemented (registry count), missing in:\n%s", implCount, out)
	}

	// The Planned count in the output must equal the registry's count.
	if !strings.Contains(out, fmt.Sprintf("%d Planned", plannedCount)) {
		t.Errorf("block should report %d Planned (registry count), missing in:\n%s", plannedCount, out)
	}

	// Reverse direction: every `hotam <cmd>` invocation (where <cmd> does not
	// start with '-' — that is a flag like -h, not a tool name) names an
	// Implemented registry tool. The regex captures only single-word commands
	// immediately inside backticks.
	cmdRE := regexp.MustCompile("`hotam ([a-z][a-z0-9-]*)`")
	for _, m := range cmdRE.FindAllStringSubmatch(out, -1) {
		cmd := m[1]
		if !implementedDisplays[cmd] {
			t.Errorf("output invokes `hotam %s` but no Implemented registry tool has that display name — drift (invented tool)", cmd)
		}
	}
}

// TestRenderRepoMapToolsSection_ListsEveryImplementedAndPlannedTool guards
// REPO-MAP.md's Tools section (review-6 R6-g) against the same drift class
// its own former doc comment flagged: the section had drifted twice while
// hand-maintained (found 2026-07-13: 5 of the then-10 Implemented commands
// missing). renderRepoMapToolsSection (repomap_data.go) now projects the
// section from methodology.Tools directly, mirroring
// TestRenderEmbeddedToolsBlock_IsPureRegistryProjection's registry-projection
// shape for the EMBEDDED-TOOLS block. This test asserts every Implemented
// tool gets its own `hotam <command>` bullet (with its registry Claim text)
// and every Planned tool's command name appears in the trailing
// not-yet-implemented roster — both directions, so neither an omission nor
// an invented entry can pass silently.
func TestRenderRepoMapToolsSection_ListsEveryImplementedAndPlannedTool(t *testing.T) {
	t.Parallel()
	out := renderRepoMapToolsSection()

	var implemented, planned []methodology.Tool
	for _, tl := range methodology.Tools.All() {
		if tl.Status == methodology.Implemented {
			implemented = append(implemented, tl)
		} else {
			planned = append(planned, tl)
		}
	}
	if len(implemented) == 0 {
		t.Fatal("precondition: registry has no Implemented tools — test is meaningless")
	}
	if len(planned) == 0 {
		t.Fatal("precondition: registry has no Planned tools — test is meaningless")
	}

	// Every Implemented tool must get its own bullet: `hotam <display>` — <Claim>.
	for _, tl := range implemented {
		display := strings.ReplaceAll(tl.Command, "_", "-")
		wantBullet := "- `hotam " + display + "` — " + tl.Claim
		if !strings.Contains(out, wantBullet) {
			t.Errorf("Implemented tool %q: missing bullet %q in Tools section:\n%s", tl.Command, wantBullet, out)
		}
	}

	// Every Planned tool's command name must appear in the trailing roster.
	for _, tl := range planned {
		if !strings.Contains(out, tl.Command) {
			t.Errorf("Planned tool %q: not listed in the not-yet-implemented roster:\n%s", tl.Command, out)
		}
	}

	// Reverse direction: no Planned tool gets an Implemented-style bullet.
	for _, tl := range planned {
		display := strings.ReplaceAll(tl.Command, "_", "-")
		badBullet := "- `hotam " + display + "` — "
		if strings.Contains(out, badBullet) {
			t.Errorf("Planned tool %q should NOT have an Implemented-style bullet (drift: invented command):\n%s", tl.Command, out)
		}
	}
}
