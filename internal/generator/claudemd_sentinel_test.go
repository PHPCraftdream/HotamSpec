package generator

import (
	"regexp"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
)

// TestSentinelOps_WrapExtract round-trips a block through WrapBlock/ExtractBlock
// and asserts ExtractBlock returns the inner content with surrounding newlines
// stripped, plus its not-found and malformed-ordering guards.
func TestSentinelOps_WrapExtract(t *testing.T) {
	t.Parallel()
	name := "LIVE-STATE"
	wrapped := WrapBlock(name, "inner line 1\ninner line 2")

	got, ok := ExtractBlock(wrapped, name)
	if !ok {
		t.Fatalf("ExtractBlock: expected ok=true for a wrapped block")
	}
	if got != "inner line 1\ninner line 2" {
		t.Errorf("ExtractBlock content = %q, want the inner text only", got)
	}
	// sentinels themselves must not appear in the extracted inner text
	if strings.Contains(got, ":BEGIN") || strings.Contains(got, ":END") {
		t.Errorf("ExtractBlock leaked sentinel markers: %q", got)
	}
}

func TestSentinelOps_ExtractMissingSentinels(t *testing.T) {
	t.Parallel()
	// neither sentinel present
	if _, ok := ExtractBlock("plain text with no markers", "ABSENT"); ok {
		t.Errorf("ExtractBlock must return ok=false when sentinels are absent")
	}
	// END precedes BEGIN → malformed ordering
	reversed := EndSentinel("X") + " junk " + BeginSentinel("X")
	if _, ok := ExtractBlock(reversed, "X"); ok {
		t.Errorf("ExtractBlock must return ok=false when END precedes BEGIN")
	}
}

func TestSentinelOps_ReplacePreservesSurroundings(t *testing.T) {
	t.Parallel()
	name := "BLOCK"
	src := "header line\n" + WrapBlock(name, "old") + "\nfooter line"
	out, err := ReplaceBlock(src, name, "new content")
	if err != nil {
		t.Fatalf("ReplaceBlock: %v", err)
	}
	if !strings.HasPrefix(out, "header line") {
		t.Errorf("ReplaceBlock must preserve text before BEGIN: %q", out)
	}
	if !strings.HasSuffix(out, "footer line") {
		t.Errorf("ReplaceBlock must preserve text after END: %q", out)
	}
	extracted, ok := ExtractBlock(out, name)
	if !ok || extracted != "new content" {
		t.Errorf("ReplaceBlock inner = %q (ok=%v), want %q", extracted, ok, "new content")
	}
}

func TestSentinelOps_ReplaceMissingSentinelsErrors(t *testing.T) {
	t.Parallel()
	_, err := ReplaceBlock("no markers here", "MISSING", "x")
	if err == nil {
		t.Fatalf("ReplaceBlock must error when a sentinel is absent")
	}
	msg := err.Error()
	if !strings.Contains(msg, "MISSING") || !strings.Contains(msg, "not found") {
		t.Errorf("ReplaceBlock error should name the block + corruption hint, got: %s", msg)
	}
}

func TestSentinelOps_InsertBlockAfter(t *testing.T) {
	t.Parallel()
	anchor := "ANCHOR"
	src := "pre\n" + WrapBlock(anchor, "a") + "\npost"
	out, err := InsertBlockAfter(src, anchor, "NEW", "n")
	if err != nil {
		t.Fatalf("InsertBlockAfter: %v", err)
	}
	// the NEW block must sit immediately after the ANCHOR END sentinel
	idxAnchor := strings.Index(out, EndSentinel(anchor))
	idxNew := strings.Index(out, BeginSentinel("NEW"))
	if idxAnchor == -1 || idxNew == -1 || idxNew < idxAnchor {
		t.Errorf("NEW block must follow the ANCHOR END sentinel:\n%s", out)
	}
	// the trailing "post" text must still be present after the NEW block
	if !strings.HasSuffix(out, "\npost") {
		t.Errorf("InsertBlockAfter must preserve trailing text, got tail: %q", out[len(out)-40:])
	}
}

func TestSentinelOps_InsertBlockAfterMissingAnchorErrors(t *testing.T) {
	t.Parallel()
	_, err := InsertBlockAfter("no anchor here", "NOPE", "NEW", "n")
	if err == nil {
		t.Fatalf("InsertBlockAfter must error when the anchor END sentinel is absent")
	}
	msg := err.Error()
	if !strings.Contains(msg, "NOPE") || !strings.Contains(msg, "Cannot insert") {
		t.Errorf("InsertBlockAfter error should name anchor + the block being inserted, got: %s", msg)
	}
}

// TestRenderOperatorRoleBlock_CarriesScopeGuardianLaw enforces
// R-crystal-carries-role-seed: the OPERATOR-ROLE block must state the
// operator's scope (identity + domain + SETTLED count), the guardian-of-
// consistency triad across spec/tests/business, and the single generative
// law. Vacuity guard: dropping any of the three from the renderer fails it.
func TestRenderOperatorRoleBlock_CarriesScopeGuardianLaw(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	inner := RenderOperatorRoleBlock(g, "fixture")
	wrapped := WrapBlock("OPERATOR-ROLE", inner)

	// it is a real OPERATOR-ROLE sentinel block, not loose prose
	if !strings.Contains(wrapped, BeginSentinel("OPERATOR-ROLE")) || !strings.Contains(wrapped, EndSentinel("OPERATOR-ROLE")) {
		t.Errorf("OPERATOR-ROLE block must carry its BEGIN/END sentinels:\n%s", wrapped)
	}

	// scope line: operator identity bound to the active domain + SETTLED count
	if !strings.Contains(inner, "Operator of `fixture`") {
		t.Errorf("OPERATOR-ROLE missing the scope line (Operator of `<domain>` … SETTLED):\n%s", inner)
	}
	if !strings.Contains(inner, "SETTLED") {
		t.Errorf("OPERATOR-ROLE scope line missing the SETTLED-atom count:\n%s", inner)
	}

	// guardian-of-consistency role across spec, tests, business intent
	if !strings.Contains(inner, "Guardian:") {
		t.Errorf("OPERATOR-ROLE missing the Guardian role statement:\n%s", inner)
	}
	for _, want := range []string{"**spec**", "**tests**", "**business**"} {
		if !strings.Contains(inner, want) {
			t.Errorf("OPERATOR-ROLE missing guardian-triad leg %q:\n%s", want, inner)
		}
	}

	// the single generative law
	if !strings.Contains(inner, "Generative law") {
		t.Errorf("OPERATOR-ROLE missing the generative-law statement:\n%s", inner)
	}
}

// TestRenderMediationLoopBlock_NamesSixStepsAndRealTools enforces
// R-crystal-carries-mediation-loop: the MEDIATION-LOOP block must name all
// six steps AND every `hotam <cmd>` it cites must be a real entry in the
// methodology.Tools registry — the block references live commands, not
// aspirational prose. This is the exact drift class the old static tools
// block fell into (P1-6): if a command is renamed/removed without updating
// the baked loop text, this test fails.
func TestRenderMediationLoopBlock_NamesSixStepsAndRealTools(t *testing.T) {
	t.Parallel()
	inner := RenderMediationLoopBlock()

	// all six steps named
	for _, step := range []string{"ORIENT", "LOCATE", "CONFRONT", "TRANSLATE", "PRESENT", "LAND"} {
		if !strings.Contains(inner, "**"+step+"**") {
			t.Errorf("MEDIATION-LOOP missing step %q:\n%s", step, inner)
		}
	}

	// every `hotam <cmd>` referenced must resolve to a registered tool.
	// The backtick span may carry args (e.g. `hotam confront <text>`), so we
	// capture only the command token after "hotam ".
	cmdRE := regexp.MustCompile("`hotam ([a-z][a-z-]+)")
	matches := cmdRE.FindAllStringSubmatch(inner, -1)
	if len(matches) == 0 {
		t.Fatalf("MEDIATION-LOOP cites no `hotam <cmd>` — drifted to pure prose (claim requires real tool commands):\n%s", inner)
	}
	seen := map[string]bool{}
	for _, m := range matches {
		seen[strings.ReplaceAll(m[1], "-", "_")] = true
	}
	for cmd := range seen {
		if _, ok := methodology.Tools.Get(cmd); !ok {
			t.Errorf("MEDIATION-LOOP cites `hotam %s` but %q is NOT in methodology.Tools (phantom/stale command — drift):\n%s", strings.ReplaceAll(cmd, "_", "-"), cmd, inner)
		}
	}
}

// TestRenderOperatorRecursionBlock_SameSeedNarrowed enforces
// R-crystal-carries-recursion-seed: the OPERATOR-RECURSION block must be
// present and state the "same seed narrowed" semantics — a sub-operator is
// THIS SAME seed with a narrower scope, not a different prompt.
func TestRenderOperatorRecursionBlock_SameSeedNarrowed(t *testing.T) {
	t.Parallel()
	inner := RenderOperatorRecursionBlock("fixture")
	wrapped := WrapBlock("OPERATOR-RECURSION", inner)

	if !strings.Contains(wrapped, BeginSentinel("OPERATOR-RECURSION")) || !strings.Contains(wrapped, EndSentinel("OPERATOR-RECURSION")) {
		t.Errorf("OPERATOR-RECURSION block must carry its BEGIN/END sentinels:\n%s", wrapped)
	}
	if !strings.Contains(inner, "Recursion") {
		t.Errorf("OPERATOR-RECURSION missing its section header:\n%s", inner)
	}
	// "same seed narrowed" semantics
	if !strings.Contains(inner, "THIS SAME seed") {
		t.Errorf("OPERATOR-RECURSION missing the 'same seed' identity statement:\n%s", inner)
	}
	if !strings.Contains(inner, "narrower scope") {
		t.Errorf("OPERATOR-RECURSION missing the 'narrower scope' narrowing statement:\n%s", inner)
	}
}
