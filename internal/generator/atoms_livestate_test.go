package generator

import (
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// TestBuildAtoms_* are covered by TestGenSpec_DeterministicOnRealDomain and
// TestGenSpec_SmokeOnRealDomain (byteidentical_test.go): BuildAtomsOperator/
// Substrate/Discipline/Check select SETTLED requirements by hardcoded
// real-domain ID prefixes (R-operator-, R-claude-md-, R-anchor-, R-check-,
// etc. — see atoms.go), so the small synthetic fixture (whose ids are all
// R-fixture-*) cannot exercise their non-empty branch; only the real
// hotam-spec-self domain can (P2-2). The empty-selection branch ("_No atomic
// requirements in this topic yet._") IS covered directly below.

func TestBuildAtomsOperator_EmptyOnFixture(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	got := BuildAtomsOperator(g)
	if !strings.Contains(got, "_No atomic requirements in this topic yet._") {
		t.Errorf("expected empty-selection notice for fixture graph (no R-operator-/R-agent-/R-boot-/R-prefer-tool- ids), got:\n%s", got)
	}
}

// TestBuildLiveState_TodayIsInjectable proves BuildLiveState's today
// parameter is truly injectable rather than silently recomputed via
// time.Now(): rendering the SAME graph with two different explicit today
// values must produce output that differs, and — critically — the only
// difference must be date-shaped text (the injected today value itself, or
// a count derived purely from it), never a spurious change in unrelated
// content.
//
// The top-action line only carries a date when the freshness advisory
// (diagnose.FreshnessSignals, gated on today via DiagnoseSignals) is
// actually the highest-priority signal — REFLECTION/STRUCTURE/etc. always
// outrank PAdvisory, so a graph with any higher-priority finding (like the
// package's shared fixture-graph.json, which has an aging IMPLEMENTS
// assumption) would mask the date entirely and make this test vacuous. This
// test therefore builds a minimal graph with exactly one SETTLED, reviewed
// requirement (nothing else diagnosable) so the freshness advisory is
// guaranteed to be the sole/top signal and today's OVERDUE count is
// directly observable.
func TestBuildLiveState_TodayIsInjectable(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{{ID: "S-owner"}},
		Requirements: []ontology.Requirement{
			{
				ID:             "R-freshness-only",
				Status:         ontology.StatusSETTLED,
				Enforcement:    ontology.EnforcementPROSE,
				Enforceability: ontology.EnforceabilityINHERENTLY_PROSE,
				Owner:          "S-owner",
				Claim:          "claim",
				Why:            "why",
				ReviewAfter:    "2026-06-01",
			},
		},
	}

	gotA := BuildLiveState(g, 5000, "2026-01-01") // before review_after: not yet overdue
	gotB := BuildLiveState(g, 5000, "2026-12-31") // after review_after: overdue

	if gotA == gotB {
		t.Fatalf("BuildLiveState with two different today values produced byte-identical output — today is not actually threaded through:\n%s", gotA)
	}
	if strings.Contains(gotA, "OVERDUE") {
		t.Errorf("BuildLiveState(today=2026-01-01, before review_after) should not yet report the requirement OVERDUE:\n%s", gotA)
	}
	if !strings.Contains(gotB, "OVERDUE") {
		t.Errorf("BuildLiveState(today=2026-12-31, after review_after) should report the requirement OVERDUE:\n%s", gotB)
	}
	if !strings.Contains(gotB, "2026-12-31") {
		t.Errorf("BuildLiveState(today=2026-12-31) output does not embed the injected today value:\n%s", gotB)
	}
}

// TestBuildLiveState_SameTodayIsByteIdentical proves the idempotency
// property CI's regen-idempotency check needs: rendering twice with the
// SAME explicit today value produces byte-identical output, independent of
// wall-clock time. This is the property that was structurally impossible
// while BuildLiveState computed today via time.Now() internally.
func TestBuildLiveState_SameTodayIsByteIdentical(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	a := BuildLiveState(g, 5000, "2026-07-12")
	b := BuildLiveState(g, 5000, "2026-07-12")
	if a != b {
		t.Fatalf("BuildLiveState with the same today value produced different output across two calls — not idempotent")
	}
}

func TestBuildLiveState_RendersOnFixture(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	got := BuildLiveState(g, 5000, "2026-07-12")
	if strings.TrimSpace(got) == "" {
		t.Fatal("BuildLiveState: empty output on fixture graph")
	}
	for _, want := range []string{"top action:", "debt:", "graph:", "crystal:"} {
		if !strings.Contains(got, want) {
			t.Errorf("BuildLiveState output missing expected fragment %q:\n%s", want, got)
		}
	}
	// NODE_COUNT budget measure branch: the fixture operator uses
	// NODE_COUNT (not CRYSTAL_CHARS), so the rendered line must report
	// "nodes" and "NODE_COUNT measure", not "chars"/"CRYSTAL_CHARS".
	if !strings.Contains(got, "NODE_COUNT measure") {
		t.Errorf("BuildLiveState: expected NODE_COUNT measure branch for fixture graph, got:\n%s", got)
	}
}

// TestBuildLiveState_ThreeCipherPulsePresent enforces
// R-three-cipher-pulse-structurally-injected: the LIVE-STATE block must
// structurally carry all three pulse ciphers — top action, debt, and context
// — so the operator's ORIENT step reads them by reference. Distinct from
// TestBuildLiveState_RendersOnFixture (which checks graph/crystal budget
// lines): this asserts the context cipher specifically, the one the existing
// test does not cover.
func TestBuildLiveState_ThreeCipherPulsePresent(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	got := BuildLiveState(g, 5000, "2026-07-12")
	for _, want := range []string{"- **top action:**", "- **debt:**", "- context: UNMEASURED"} {
		if !strings.Contains(got, want) {
			t.Errorf("LIVE-STATE three-cipher pulse missing fragment %q (top action / debt / context):\n%s", want, got)
		}
	}
}

// TestBuildLiveState_ContextLineNamesHostBoundaryNoCommand enforces
// R-unmeasured-cipher-names-host-boundary: while context is UNMEASURED, the
// context line must honestly name the host-cooperation boundary and must NOT
// suggest a command-to-call (no backtick command, no hotam invocation, no
// .py script). This guards against a regression to the removed
// setup_context_hook.py --apply "fix".
func TestBuildLiveState_ContextLineNamesHostBoundaryNoCommand(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	got := BuildLiveState(g, 5000, "2026-07-12")

	// extract the context line
	var ctxLine string
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, "- context: ") {
			ctxLine = line
			break
		}
	}
	if ctxLine == "" {
		t.Fatalf("LIVE-STATE has no context line:\n%s", got)
	}

	// must name the host-cooperation boundary explicitly
	if !strings.Contains(ctxLine, "host cooperation") {
		t.Errorf("context line must name the host-cooperation boundary, got:\n%s", ctxLine)
	}
	if !strings.Contains(ctxLine, "will not touch") {
		t.Errorf("context line must state the framework will not touch the host, got:\n%s", ctxLine)
	}

	// must name NO command-to-call
	for _, bad := range []string{"`", "hotam ", ".py", "--apply"} {
		if strings.Contains(ctxLine, bad) {
			t.Errorf("context line must name no command-to-call, but contains %q:\n%s", bad, ctxLine)
		}
	}
}

// TestBuildAtoms_GroupsSettledByTopicAndRendersNonEmpty enforces
// R-docs-generated-from-requirements: the atoms-*.md builders group SETTLED
// requirements by topic-id prefix and render each matched requirement's claim,
// excluding non-SETTLED ones. The small fixture (R-fixture-*) cannot exercise
// the non-empty branch (its ids match no topic prefix — see the note above), so
// a synthetic graph carries one SETTLED + one DRAFT requirement to prove the
// grouping + SETTLED filter + non-empty rendering directly.
func TestBuildAtoms_GroupsSettledByTopicAndRendersNonEmpty(t *testing.T) {
	t.Parallel()

	// BuildAtomsCheck: a SETTLED R-check- req is grouped + rendered; a DRAFT
	// R-check- req with the same prefix is excluded by the SETTLED filter.
	settled := ontology.Requirement{
		ID:     "R-check-settled-atom",
		Status: ontology.StatusSETTLED,
		Claim:  "UNIQUE_SETTLED_CHECK_CLAIM_MARKER",
		Why:    "settled why",
	}
	draft := ontology.Requirement{
		ID:     "R-check-draft-atom",
		Status: ontology.StatusDRAFT,
		Claim:  "UNIQUE_DRAFT_CHECK_CLAIM_MARKER",
		Why:    "draft why",
	}
	g := &ontology.Graph{Requirements: []ontology.Requirement{settled, draft}}

	got := BuildAtomsCheck(g)
	if strings.TrimSpace(got) == "" {
		t.Fatal("BuildAtomsCheck: empty output")
	}
	if strings.Contains(got, "_No atomic requirements in this topic yet._") {
		t.Errorf("BuildAtomsCheck should match the SETTLED R-check- req, got the empty-selection notice:\n%s", got)
	}
	if !strings.Contains(got, "UNIQUE_SETTLED_CHECK_CLAIM_MARKER") {
		t.Errorf("BuildAtomsCheck must render the matched SETTLED requirement's claim:\n%s", got)
	}
	if strings.Contains(got, "UNIQUE_DRAFT_CHECK_CLAIM_MARKER") {
		t.Errorf("BuildAtomsCheck must NOT render a DRAFT requirement (SETTLED filter), but the draft claim appeared:\n%s", got)
	}

	// exercise the other three builders with one SETTLED match each: non-empty
	// + the matched claim renders (grouping by each builder's own prefix).
	cases := []struct {
		name   string
		build  func(*ontology.Graph) string
		prefix string
	}{
		{"Operator", BuildAtomsOperator, "R-operator-"},
		{"Substrate", BuildAtomsSubstrate, "R-claude-md-"},
		{"Discipline", BuildAtomsDiscipline, "R-anchor-"},
	}
	for _, c := range cases {
		marker := "UNIQUE_SETTLED_" + strings.ToUpper(c.name) + "_MARKER"
		gg := &ontology.Graph{Requirements: []ontology.Requirement{
			{ID: c.prefix + "settled-marker", Status: ontology.StatusSETTLED, Claim: marker, Why: "x"},
		}}
		out := c.build(gg)
		if strings.TrimSpace(out) == "" {
			t.Errorf("BuildAtoms%s: empty output", c.name)
		}
		if strings.Contains(out, "_No atomic requirements in this topic yet._") {
			t.Errorf("BuildAtoms%s should match its SETTLED %s req, got empty notice:\n%s", c.name, c.prefix, out)
		}
		if !strings.Contains(out, marker) {
			t.Errorf("BuildAtoms%s must render the matched SETTLED requirement's claim:\n%s", c.name, out)
		}
	}
}
