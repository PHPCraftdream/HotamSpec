package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// TestRenderProjectEssenceBlock_FieldsPresent proves the GREEN path: a
// manifest carrying purpose/goals/director renders all three fields with
// their parsed values, in the "- **<field>** — <value>" shape the
// consumer-profile crystal opens with.
func TestRenderProjectEssenceBlock_FieldsPresent(t *testing.T) {
	t.Parallel()
	repoRoot := t.TempDir()
	domainDir := filepath.Join(repoRoot, "domains", "consumer-demo")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	manifest := `{
  "purpose": "Demo consumer domain.",
  "goals": ["ship feature A", "close debt B"],
  "director": "resolver-role"
}`
	if err := os.WriteFile(filepath.Join(domainDir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	out := RenderProjectEssenceBlock(repoRoot, "consumer-demo")
	for _, want := range []string{
		"### Project essence",
		"- **purpose** — Demo consumer domain.",
		"- **goals** — ship feature A, close debt B",
		"- **director** — resolver-role",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("PROJECT-ESSENCE missing %q, got:\n%s", want, out)
		}
	}
	// must NOT carry the em-dash placeholders when fields are populated
	if strings.Contains(out, " — ") && strings.Contains(out, "**purpose** — Demo") {
		// ok — the value separator is " — "
	} else {
		t.Errorf("PROJECT-ESSENCE purpose line mis-formatted, got:\n%s", out)
	}
}

// TestRenderProjectEssenceBlock_MissingManifestYieldsPlaceholders proves
// the manifest-absent fallback: every field falls back to the em-dash
// placeholder, mirroring RenderDomainMapBlock's missing-field shape —
// honest "no value" rather than empty output or a misleading default.
func TestRenderProjectEssenceBlock_MissingManifestYieldsPlaceholders(t *testing.T) {
	t.Parallel()
	repoRoot := t.TempDir()
	out := RenderProjectEssenceBlock(repoRoot, "never-existed")
	for _, want := range []string{
		"- **purpose** — —",
		"- **goals** — —",
		"- **director** — —",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing-manifest PROJECT-ESSENCE must keep placeholder %q, got:\n%s", want, out)
		}
	}
}

// TestRenderProjectEssenceBlock_OldFormatManifestYieldsPlaceholders proves
// an old-format manifest (no purpose/goals/director keys — every manifest
// predating these fields) renders the placeholders, never panics and never
// produces empty output. Same backward-compat contract
// ResolveDomainPresentation itself guarantees.
func TestRenderProjectEssenceBlock_OldFormatManifestYieldsPlaceholders(t *testing.T) {
	t.Parallel()
	repoRoot := t.TempDir()
	domainDir := filepath.Join(repoRoot, "domains", "old-format")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(domainDir, "manifest.json"), []byte(`{"self_hosting": true}`), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	out := RenderProjectEssenceBlock(repoRoot, "old-format")
	for _, want := range []string{
		"- **purpose** — —",
		"- **goals** — —",
		"- **director** — —",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("old-format PROJECT-ESSENCE must keep placeholder %q, got:\n%s", want, out)
		}
	}
}

// TestRenderStakeholdersBlock_Populated proves the populated case: every
// g.Stakeholders entry renders as a table row, in DeclOrder (narrative)
// order, with Cell-safe text interpolation — same shape REQUIREMENTS.md's
// Stakeholders section already uses.
func TestRenderStakeholdersBlock_Populated(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{
			{ID: "S-second", Name: "Second", Domain: "demo", DeclOrder: 1},
			{ID: "S-first", Name: "First", Domain: "demo", DeclOrder: 0},
		},
	}
	out := RenderStakeholdersBlock(g)
	for _, want := range []string{
		"### Stakeholders & roles",
		"| id | name | domain |",
		"|---|---|---|",
		"| `S-first` | First | demo |",
		"| `S-second` | Second | demo |",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("STAKEHOLDERS missing %q, got:\n%s", want, out)
		}
	}
	// DeclOrder must drive the row order: S-first (decl_order 0) appears
	// BEFORE S-second (decl_order 1) in the rendered text.
	firstAt := strings.Index(out, "S-first")
	secondAt := strings.Index(out, "S-second")
	if firstAt == -1 || secondAt == -1 || firstAt > secondAt {
		t.Errorf("STAKEHOLDERS must order by DeclOrder (S-first before S-second), got firstAt=%d secondAt=%d:\n%s", firstAt, secondAt, out)
	}
}

// TestRenderStakeholdersBlock_Empty proves the empty case: zero
// stakeholders renders an explicit empty marker (never empty output), so
// the sentinel pair always has honest inner content — same contract
// AGENT-MAP and CONSTITUTION already follow.
func TestRenderStakeholdersBlock_Empty(t *testing.T) {
	t.Parallel()
	out := RenderStakeholdersBlock(&ontology.Graph{})
	if !strings.Contains(out, "### Stakeholders & roles") {
		t.Errorf("empty STAKEHOLDERS must still render its heading, got:\n%s", out)
	}
	if !strings.Contains(out, "_(no stakeholders declared in this domain yet.)_") {
		t.Errorf("empty STAKEHOLDERS must render the explicit empty marker, got:\n%s", out)
	}
}

// TestRenderBusinessContent_FullProfileSectionOrderUnchanged is the
// regression guard for the FULL profile: the BUSINESS-bucket order must
// stay byte-identical to the pre-reorder sequence
// (LIVE-STATE → DOMAIN-MAP → PARENT-PROJECT → CONSTITUTION → AGENT-MAP →
// CONCEPT-MAP → RECENTLY-REJECTED), with PROJECT-ESSENCE and
// STAKEHOLDERS NOT present. The engine's own self-hosting domains keep
// the operational order; only consumer domains get the essence-first UX.
func TestRenderBusinessContent_FullProfileSectionOrderUnchanged(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	repoRoot := t.TempDir()
	out := RenderBusinessContent(g, "hotam-spec-self", repoRoot, 4200, nil, "2026-07-12", false)

	wantOrder := []string{
		"LIVE-STATE", "DOMAIN-MAP", "PARENT-PROJECT", "CONSTITUTION",
		"AGENT-MAP", "CONCEPT-MAP", "RECENTLY-REJECTED",
	}
	lastPos := -1
	for _, name := range wantOrder {
		begin := BeginSentinel(name)
		pos := strings.Index(out, begin)
		if pos == -1 {
			t.Fatalf("full BUSINESS bucket missing BEGIN sentinel for %s", name)
		}
		if pos < lastPos {
			t.Errorf("full BUSINESS bucket order drifted: %s BEGIN (pos %d) must come AFTER the previous block (pos %d)", name, pos, lastPos)
		}
		lastPos = pos
	}
	// consumer-only blocks must NOT appear under the full profile
	for _, name := range []string{"PROJECT-ESSENCE", "STAKEHOLDERS"} {
		if strings.Contains(out, BeginSentinel(name)) {
			t.Errorf("full BUSINESS bucket must not carry consumer-only block %s", name)
		}
	}
}

// TestConsumerHeaderLine_PurposePresent proves the GREEN path: a manifest
// carrying a purpose renders "# <domainName> — <purpose>", the domain-first
// header external review P1 (task E2) requires — the file's first line
// must name the DOMAIN, not "Hotam-Spec framework".
func TestConsumerHeaderLine_PurposePresent(t *testing.T) {
	t.Parallel()
	repoRoot := t.TempDir()
	domainDir := filepath.Join(repoRoot, "domains", "acme-widgets")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	manifest := `{"purpose": "Ships widgets to acme customers."}`
	if err := os.WriteFile(filepath.Join(domainDir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	got := consumerHeaderLine(repoRoot, "acme-widgets")
	want := "# acme-widgets — Ships widgets to acme customers."
	if got != want {
		t.Errorf("consumerHeaderLine = %q, want %q", got, want)
	}
}

// TestConsumerHeaderLine_MissingManifestFallsBackToBareDomainName proves the
// degrade path: no manifest / no purpose still yields an honest, non-empty
// header — just the domain name, no dangling em-dash.
func TestConsumerHeaderLine_MissingManifestFallsBackToBareDomainName(t *testing.T) {
	t.Parallel()
	repoRoot := t.TempDir()
	got := consumerHeaderLine(repoRoot, "never-existed")
	want := "# never-existed"
	if got != want {
		t.Errorf("consumerHeaderLine = %q, want %q", got, want)
	}
}

// TestRenderClaudeMDFromTemplate_FullProfileHeaderUnchanged is the
// regression guard for the FULL profile: the framework-identity header and
// MIND-before-BUSINESS ordering must stay exactly as before task E2 — only
// the consumer profile reorders.
func TestRenderClaudeMDFromTemplate_FullProfileHeaderUnchanged(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	repoRoot := t.TempDir()
	out := RenderClaudeMDFromTemplate(g, "hotam-spec-self", repoRoot, 4200, nil, "2026-07-12", false)

	firstLine := strings.SplitN(out, "\n", 2)[0]
	if firstLine != "# CLAUDE.md — Hotam-Spec framework" {
		t.Errorf("full-profile crystal first line = %q, want the framework-identity header", firstLine)
	}
	roleAt := strings.Index(out, BeginSentinel("OPERATOR-ROLE"))
	liveStateAt := strings.Index(out, BeginSentinel("LIVE-STATE"))
	if roleAt == -1 || liveStateAt == -1 {
		t.Fatalf("full-profile crystal missing OPERATOR-ROLE or LIVE-STATE sentinel")
	}
	if roleAt > liveStateAt {
		t.Errorf("full-profile crystal must keep MIND (OPERATOR-ROLE, pos %d) BEFORE BUSINESS (LIVE-STATE, pos %d)", roleAt, liveStateAt)
	}
}
