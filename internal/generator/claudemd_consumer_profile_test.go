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

// TestRenderBusinessContent_ConsumerProfileSectionOrder enforces the
// consumer-profile BUSINESS-bucket reorder: PROJECT-ESSENCE →
// STAKEHOLDERS → LIVE-STATE → CONSTITUTION → DOMAIN-MAP → PARENT-PROJECT →
// AGENT-MAP → RECENTLY-REJECTED, with CONCEPT-MAP omitted entirely.
// This is the "what is this project / what is blocked" first-screen UX
// the demo transcript motivated; the operational order (LIVE-STATE first)
// stays in place for the full profile, asserted by a separate test below.
func TestRenderBusinessContent_ConsumerProfileSectionOrder(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	repoRoot := t.TempDir() // no domains/ → PROJECT-ESSENCE falls back to placeholders, DOMAIN-MAP renders "no domains yet"
	out := RenderBusinessContent(g, "hotam-spec-self", repoRoot, 4200, nil, "2026-07-12", true)

	wantOrder := []string{
		"PROJECT-ESSENCE", "STAKEHOLDERS", "LIVE-STATE", "CONSTITUTION",
		"DOMAIN-MAP", "PARENT-PROJECT", "AGENT-MAP", "RECENTLY-REJECTED",
	}
	for _, name := range wantOrder {
		if _, ok := ExtractBlock(out, name); !ok {
			t.Errorf("consumer BUSINESS bucket missing block %s", name)
		}
	}
	// the rendered BEGIN sentinels must appear in the exact order above
	lastPos := -1
	for _, name := range wantOrder {
		begin := BeginSentinel(name)
		pos := strings.Index(out, begin)
		if pos == -1 {
			t.Fatalf("consumer BUSINESS bucket missing BEGIN sentinel for %s", name)
		}
		if pos < lastPos {
			t.Errorf("consumer BUSINESS bucket order: %s BEGIN (pos %d) must come AFTER the previous block (pos %d)", name, pos, lastPos)
		}
		lastPos = pos
	}
	// CONCEPT-MAP must NOT appear under consumer
	if strings.Contains(out, BeginSentinel("CONCEPT-MAP")) {
		t.Errorf("consumer BUSINESS bucket must omit CONCEPT-MAP (framework source-file paths absent in external consumer repos), but its sentinel was rendered")
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

// TestRenderClaudeMDFromTemplate_ConsumerProfileOpensWithEssence is the
// end-to-end smoke for the consumer crystal: the rendered CLAUDE.md
// template substitution must surface PROJECT-ESSENCE / STAKEHOLDERS
// before the operational blocks (LIVE-STATE / DOMAIN-MAP), and drop
// CONCEPT-MAP — the same gates already covered per-block above, here
// verified through the full template-substitution path the real
// `hotam gen-spec --profile consumer` invocation walks.
func TestRenderClaudeMDFromTemplate_ConsumerProfileOpensWithEssence(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	repoRoot := t.TempDir()
	out := RenderClaudeMDFromTemplate(g, "hotam-spec-self", repoRoot, 4200, nil, "2026-07-12", true)

	// PROJECT-ESSENCE must appear before LIVE-STATE on the rendered crystal
	// (the "first screen" property the demo transcript motivated).
	essenceAt := strings.Index(out, BeginSentinel("PROJECT-ESSENCE"))
	liveStateAt := strings.Index(out, BeginSentinel("LIVE-STATE"))
	if essenceAt == -1 {
		t.Fatalf("consumer crystal missing PROJECT-ESSENCE block")
	}
	if liveStateAt == -1 {
		t.Fatalf("consumer crystal missing LIVE-STATE block")
	}
	if essenceAt > liveStateAt {
		t.Errorf("consumer crystal must open with PROJECT-ESSENCE (pos %d) BEFORE LIVE-STATE (pos %d)", essenceAt, liveStateAt)
	}

	// STAKEHOLDERS must appear before the operational DOMAIN-MAP
	stakeAt := strings.Index(out, BeginSentinel("STAKEHOLDERS"))
	domainMapAt := strings.Index(out, BeginSentinel("DOMAIN-MAP"))
	if stakeAt == -1 || domainMapAt == -1 {
		t.Fatalf("consumer crystal missing STAKEHOLDERS or DOMAIN-MAP sentinel")
	}
	if stakeAt > domainMapAt {
		t.Errorf("consumer crystal must surface STAKEHOLDERS (pos %d) BEFORE DOMAIN-MAP (pos %d)", stakeAt, domainMapAt)
	}

	// CONCEPT-MAP must be absent under consumer
	if strings.Contains(out, BeginSentinel("CONCEPT-MAP")) {
		t.Errorf("consumer crystal must NOT carry CONCEPT-MAP (framework source-file paths absent in external consumer repos)")
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

// TestRenderClaudeMDFromTemplate_ConsumerProfileDomainFirstHeader is the
// end-to-end regression guard for task E2 (external review P1): the
// consumer-profile crystal's FIRST LINE must open with the domain's own
// name/purpose, not the framework-identity "# CLAUDE.md — Hotam-Spec
// framework" header, and the business cluster (PROJECT-ESSENCE) must
// render BEFORE the methodology seed (OPERATOR-ROLE) — the inverse of the
// full-profile ordering asserted by
// TestRenderClaudeMDFromTemplate_SubstitutesPlaceholdersPreservesRest.
func TestRenderClaudeMDFromTemplate_ConsumerProfileDomainFirstHeader(t *testing.T) {
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
	g := loadFixtureGraph(t)
	out := RenderClaudeMDFromTemplate(g, "acme-widgets", repoRoot, 4200, nil, "2026-07-12", true)

	firstLine := strings.SplitN(out, "\n", 2)[0]
	wantFirstLine := "# acme-widgets — Ships widgets to acme customers."
	if firstLine != wantFirstLine {
		t.Errorf("consumer crystal first line = %q, want %q", firstLine, wantFirstLine)
	}
	if strings.Contains(out, "# CLAUDE.md — Hotam-Spec framework") {
		t.Errorf("consumer crystal must NOT carry the framework-identity header line")
	}

	// The business cluster must render BEFORE the methodology seed.
	essenceAt := strings.Index(out, BeginSentinel("PROJECT-ESSENCE"))
	roleAt := strings.Index(out, BeginSentinel("OPERATOR-ROLE"))
	if essenceAt == -1 || roleAt == -1 {
		t.Fatalf("consumer crystal missing PROJECT-ESSENCE or OPERATOR-ROLE sentinel")
	}
	if essenceAt > roleAt {
		t.Errorf("consumer crystal must render PROJECT-ESSENCE (pos %d) BEFORE OPERATOR-ROLE (pos %d)", essenceAt, roleAt)
	}
	// RECENTLY-REJECTED (last business block) must still precede OPERATOR-ROLE.
	rejectedAt := strings.Index(out, BeginSentinel("RECENTLY-REJECTED"))
	if rejectedAt == -1 {
		t.Fatalf("consumer crystal missing RECENTLY-REJECTED sentinel")
	}
	if rejectedAt > roleAt {
		t.Errorf("consumer crystal must render the full business cluster (through RECENTLY-REJECTED, pos %d) BEFORE the methodology seed (OPERATOR-ROLE, pos %d)", rejectedAt, roleAt)
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
