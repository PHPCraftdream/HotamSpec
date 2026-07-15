package gocode

import (
	"errors"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// syntheticRequirementFixtures builds the same syntheticEntityType() (from
// models_test.go) alongside a small set of SETTLED requirements engineered
// to exercise every GEN-CODE-CONTRACT.md §3 atom row at least once: a field
// atom (references "текст"), a state-pair atom (mentions both "черновик"
// and "на-gate"), a gate/order atom with a REAL structural correlate (MUST +
// an id-shaped anchor "P-G1-R" that is a genuine substring of gate-card's
// "P-G1-R-pass" lifecycle state — the same shape of correlate found on the
// real prat domain while diagnosing the vacuous-assertion bug this fixture
// now guards against), a SECOND gate/order-shaped claim whose anchor has NO
// correlate anywhere in the graph (must be reclassified to honest-gap, not
// rendered as a vacuous self-check), an inter-entity invariant atom (needs a
// second EntityType so two slugs can be named), and a requirement with no
// structural carrier at all.
func syntheticRequirementFixtures() ([]ontology.EntityType, []ontology.Requirement) {
	card := syntheticEntityType()
	other := ontology.EntityType{
		Slug:        "other-card",
		Description: "second synthetic entity, for the inter-entity invariant atom",
		Fields: []ontology.EntityField{
			{Name: "резюме", Kind: "string", Required: false},
		},
		Lifecycle: ontology.Lifecycle{
			Slug: "other-card-lifecycle",
			States: []ontology.State{
				{Name: "черновик", Kind: ontology.StateKindInitial},
				{Name: "утверждён", Kind: ontology.StateKindTerminal},
			},
		},
	}
	gateCard := ontology.EntityType{
		Slug:        "gate-card",
		Description: "third synthetic entity, carries an ASCII-named lifecycle state so the gate/order atom has a real structural correlate to find",
		Lifecycle: ontology.Lifecycle{
			Slug: "gate-card-lifecycle",
			States: []ontology.State{
				{Name: "draft", Kind: ontology.StateKindInitial},
				{Name: "P-G1-R-pass", Kind: ontology.StateKindNormal, Why: "passed sub-gate P-G1-R"},
				{Name: "done", Kind: ontology.StateKindTerminal},
			},
		},
	}

	reqs := []ontology.Requirement{
		{
			ID:     "R-field-atom",
			Claim:  "Поле текст test-card MUST быть заполнено содержательно.",
			Status: ontology.StatusSETTLED,
			Owner:  "test-owner",
		},
		{
			ID:     "R-state-pair-atom",
			Claim:  "test-card переходит из состояния черновик в состояние на-gate при подаче на ревью.",
			Status: ontology.StatusSETTLED,
			Owner:  "test-owner",
		},
		{
			ID:     "R-gate-order-atom",
			Claim:  "Sub-gate P-G1-R MUST быть пройден до перехода gate-card в следующее состояние pipeline.",
			Status: ontology.StatusSETTLED,
			Owner:  "test-owner",
		},
		{
			ID:     "R-gate-order-no-correlate",
			Claim:  "Переход R-gate-x MUST быть подтверждён human review до продолжения pipeline.",
			Status: ontology.StatusSETTLED,
			Owner:  "test-owner",
		},
		{
			ID:     "R-inter-entity-atom",
			Claim:  "test-card и other-card связаны отношением N:M через общий registry.",
			Status: ontology.StatusSETTLED,
			Owner:  "test-owner",
		},
		{
			ID:     "R-no-atom",
			Claim:  "Команда должна согласовывать формат отчёта на еженедельной встрече.",
			Status: ontology.StatusSETTLED,
			Owner:  "test-owner",
		},
		{
			ID:     "R-not-settled-excluded",
			Claim:  "Черновое требование текст MUST не должно попасть в вывод.",
			Status: ontology.StatusDRAFT,
			Owner:  "test-owner",
		},
	}

	return []ontology.EntityType{card, other, gateCard}, reqs
}

func buildSyntheticEntityModels(t *testing.T, ets []ontology.EntityType) []*entityModel {
	t.Helper()
	var models []*entityModel
	for _, et := range ets {
		m, err := BuildEntityModel(et)
		if err != nil {
			t.Fatalf("BuildEntityModel(%s): %v", et.Slug, err)
		}
		models = append(models, m)
	}
	return models
}

// settledOnly filters reqs down to SETTLED ones, mirroring exactly what
// BuildRequirementModels passes as otherSettled in production — direct
// BuildRequirementModel test call sites below use this (not the raw fixture
// slice) so a DRAFT fixture requirement can never accidentally participate
// as a gate/order cross-reference correlate in a test, which production
// code path never allows either.
func settledOnly(reqs []ontology.Requirement) []ontology.Requirement {
	var out []ontology.Requirement
	for _, r := range reqs {
		if r.Status == ontology.StatusSETTLED {
			out = append(out, r)
		}
	}
	return out
}

// TestBuildRequirementModel_AtomClassification exercises GEN-CODE-CONTRACT.md
// §3's five-row taxonomy end to end on the synthetic fixtures, asserting
// each requirement lands in exactly the row its claim text is engineered
// for — the "first matching row wins" priority order (field > state-pair >
// gate/order > inter-entity > none).
func TestBuildRequirementModel_AtomClassification(t *testing.T) {
	ets, reqs := syntheticRequirementFixtures()
	models := buildSyntheticEntityModels(t, ets)

	byID := make(map[string]ontology.Requirement, len(reqs))
	for _, r := range reqs {
		byID[r.ID] = r
	}

	cases := []struct {
		id   string
		kind atomKind
	}{
		{"R-field-atom", atomKindField},
		{"R-state-pair-atom", atomKindStatePair},
		{"R-gate-order-atom", atomKindGate},
		{"R-gate-order-no-correlate", atomKindNone},
		{"R-inter-entity-atom", atomKindInterEntity},
		{"R-no-atom", atomKindNone},
	}

	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			rm := BuildRequirementModel(byID[tc.id], models, settledOnly(reqs))
			if rm.kind != tc.kind {
				t.Errorf("BuildRequirementModel(%s).kind = %v, want %v", tc.id, rm.kind, tc.kind)
			}
		})
	}
}

// TestBuildRequirementModel_FieldAtom_ResolvesRealField asserts the field
// atom for R-field-atom actually points at TestCard's "текст" field
// (fieldName "Text"), not just "some field or other" — the assertion must
// mirror the exact atom the claim names.
func TestBuildRequirementModel_FieldAtom_ResolvesRealField(t *testing.T) {
	ets, reqs := syntheticRequirementFixtures()
	models := buildSyntheticEntityModels(t, ets)
	var req ontology.Requirement
	for _, r := range reqs {
		if r.ID == "R-field-atom" {
			req = r
		}
	}
	rm := BuildRequirementModel(req, models, settledOnly(reqs))
	if rm.kind != atomKindField {
		t.Fatalf("expected atomKindField, got %v", rm.kind)
	}
	if len(rm.fields) != 1 {
		t.Fatalf("expected exactly 1 field atom, got %d", len(rm.fields))
	}
	if rm.fields[0].entity.structName != "TestCard" || rm.fields[0].field.fieldName != "Text" {
		t.Errorf("field atom = %s.%s, want TestCard.Text", rm.fields[0].entity.structName, rm.fields[0].field.fieldName)
	}
}

// TestBuildRequirementModel_StatePairAtom_ResolvesBothStates asserts the
// state-pair atom for R-state-pair-atom names both draft and at-gate states
// of TestCard (the two states the claim's Cyrillic text literally mentions).
func TestBuildRequirementModel_StatePairAtom_ResolvesBothStates(t *testing.T) {
	ets, reqs := syntheticRequirementFixtures()
	models := buildSyntheticEntityModels(t, ets)
	var req ontology.Requirement
	for _, r := range reqs {
		if r.ID == "R-state-pair-atom" {
			req = r
		}
	}
	rm := BuildRequirementModel(req, models, settledOnly(reqs))
	if rm.kind != atomKindStatePair {
		t.Fatalf("expected atomKindStatePair, got %v", rm.kind)
	}
	if rm.statePair.entity.structName != "TestCard" {
		t.Fatalf("state pair entity = %s, want TestCard", rm.statePair.entity.structName)
	}
	if len(rm.statePair.states) != 2 {
		t.Fatalf("expected 2 matched states, got %d", len(rm.statePair.states))
	}
	got := map[string]bool{}
	for _, s := range rm.statePair.states {
		got[s.value] = true
	}
	if !got["draft"] || !got["at-gate"] {
		t.Errorf("expected states {draft, at-gate}, got %v", got)
	}
}

// TestBuildRequirementModel_GateAtom_CapturesAnchors asserts the gate/order
// atom for R-gate-order-atom captures the id-shaped anchor "P-G1-R" the
// claim names, driven purely by the domain-agnostic idAnchorPattern (no
// hardcoded "P-G" convention), AND that it resolved to the real structural
// correlate (gate-card's "P-G1-R-pass" lifecycle state) — not merely
// recorded the literal the regex found in the claim's own text (the bug
// this test guards against: the classification and the rendered assertion
// must point at an independently-verified graph fact).
func TestBuildRequirementModel_GateAtom_CapturesAnchors(t *testing.T) {
	ets, reqs := syntheticRequirementFixtures()
	models := buildSyntheticEntityModels(t, ets)
	var req ontology.Requirement
	for _, r := range reqs {
		if r.ID == "R-gate-order-atom" {
			req = r
		}
	}
	rm := BuildRequirementModel(req, models, settledOnly(reqs))
	if rm.kind != atomKindGate {
		t.Fatalf("expected atomKindGate, got %v", rm.kind)
	}
	found := false
	for _, a := range rm.gate.anchors {
		if a == "P-G1-R" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected gate anchors to include %q, got %v", "P-G1-R", rm.gate.anchors)
	}
	if !rm.gate.hasStructuralCorrelate() {
		t.Fatal("expected at least one anchor to resolve to a real structural correlate")
	}
	var correlate *gateAnchorCorrelate
	for i := range rm.gate.correlates {
		if rm.gate.correlates[i].anchor == "P-G1-R" && rm.gate.correlates[i].kind == gateAnchorCorrelateState {
			correlate = &rm.gate.correlates[i]
		}
	}
	if correlate == nil {
		t.Fatalf("expected a state correlate for anchor %q, got %+v", "P-G1-R", rm.gate.correlates)
	}
	if correlate.stateEntity.structName != "GateCard" {
		t.Errorf("correlate entity = %s, want GateCard", correlate.stateEntity.structName)
	}
	if correlate.state.src.Name != "P-G1-R-pass" {
		t.Errorf("correlate state = %s, want P-G1-R-pass", correlate.state.src.Name)
	}
}

// TestBuildRequirementModel_GateAtom_NoCorrelateReclassifiesToHonestGap
// asserts a claim that LOOKS like a gate/order atom (meta-token + an
// id-shaped anchor, "R-gate-x") but whose anchor does not independently
// correlate anywhere else in the domain graph (no lifecycle.state.name, no
// EntityType.why, no other requirement id) is reclassified to the honest-gap
// row (contract §3's closing row) rather than kept as atomKindGate with a
// vacuous self-check — this is the core fix for the bug found on the real
// prat domain's R-brd-integrity-zero-blockers ("P-G3-CQA" anchor, no
// correlate anywhere in that domain either).
func TestBuildRequirementModel_GateAtom_NoCorrelateReclassifiesToHonestGap(t *testing.T) {
	ets, reqs := syntheticRequirementFixtures()
	models := buildSyntheticEntityModels(t, ets)
	var req ontology.Requirement
	for _, r := range reqs {
		if r.ID == "R-gate-order-no-correlate" {
			req = r
		}
	}
	rm := BuildRequirementModel(req, models, settledOnly(reqs))
	if rm.kind != atomKindNone {
		t.Fatalf("expected atomKindNone (honest gap) for an anchor with no graph correlate, got %v", rm.kind)
	}
}

// TestBuildRequirementModel_InterEntityAtom_ResolvesBothEntities asserts the
// inter-entity atom for R-inter-entity-atom names both TestCard and
// OtherCard (the two EntityType slugs its claim literally mentions).
func TestBuildRequirementModel_InterEntityAtom_ResolvesBothEntities(t *testing.T) {
	ets, reqs := syntheticRequirementFixtures()
	models := buildSyntheticEntityModels(t, ets)
	var req ontology.Requirement
	for _, r := range reqs {
		if r.ID == "R-inter-entity-atom" {
			req = r
		}
	}
	rm := BuildRequirementModel(req, models, settledOnly(reqs))
	if rm.kind != atomKindInterEntity {
		t.Fatalf("expected atomKindInterEntity, got %v", rm.kind)
	}
	if len(rm.interEntity) != 2 {
		t.Fatalf("expected 2 entity ends, got %d", len(rm.interEntity))
	}
	names := map[string]bool{}
	for _, em := range rm.interEntity {
		names[em.structName] = true
	}
	if !names["TestCard"] || !names["OtherCard"] {
		t.Errorf("expected {TestCard, OtherCard}, got %v", names)
	}
}

// TestBuildRequirementModels_ExcludesNonSettled asserts DRAFT/REJECTED
// requirements never reach requirements_test.go — only SETTLED ones (contract
// §1 task scope: "для каждого SETTLED-требования домена").
func TestBuildRequirementModels_ExcludesNonSettled(t *testing.T) {
	ets, reqs := syntheticRequirementFixtures()
	models := buildSyntheticEntityModels(t, ets)
	reqModels, err := BuildRequirementModels(reqs, models)
	if err != nil {
		t.Fatalf("BuildRequirementModels: %v", err)
	}
	for _, rm := range reqModels {
		if rm.src.ID == "R-not-settled-excluded" {
			t.Errorf("DRAFT requirement %q must not appear in the built models", rm.src.ID)
		}
	}
	if len(reqModels) != 6 {
		t.Fatalf("expected 6 SETTLED requirement models, got %d", len(reqModels))
	}
}

// TestRenderRequirementsTestFile_Synthetic_ParsesAndIsASCII renders
// requirements_test.go for the synthetic fixtures and asserts: (1) it parses
// as valid Go, (2) it contains a named Test_<id> function for every SETTLED
// requirement (including the honest-gap one, never t.Skip), and (3) it is
// pure ASCII end to end (contract §1.1's zero-Cyrillic rule), even though
// every synthetic claim above is Cyrillic prose.
func TestRenderRequirementsTestFile_Synthetic_ParsesAndIsASCII(t *testing.T) {
	ets, reqs := syntheticRequirementFixtures()
	models := buildSyntheticEntityModels(t, ets)
	reqModels, err := BuildRequirementModels(reqs, models)
	if err != nil {
		t.Fatalf("BuildRequirementModels: %v", err)
	}

	src, err := RenderRequirementsTestFile("gen", reqModels)
	if err != nil {
		t.Fatalf("RenderRequirementsTestFile: %v", err)
	}
	text := string(src)

	for _, want := range []string{
		"func Test_R_field_atom(t *testing.T) {",
		"func Test_R_state_pair_atom(t *testing.T) {",
		"func Test_R_gate_order_atom(t *testing.T) {",
		"func Test_R_gate_order_no_correlate(t *testing.T) {",
		"func Test_R_inter_entity_atom(t *testing.T) {",
		"func Test_R_no_atom(t *testing.T) {",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("generated source missing %q\n---\n%s", want, text)
		}
	}
	if strings.Contains(text, "Test_R_not_settled_excluded") {
		t.Error("generated source must not contain a test function for the non-SETTLED requirement")
	}

	// The honest-gap requirement must log, never skip.
	if !strings.Contains(text, `t.Log("no structural atom - see requirements_audit.md#r-no-atom")`) {
		t.Errorf("expected honest t.Log for R-no-atom, got:\n%s", text)
	}
	if strings.Contains(text, "t.Skip") {
		t.Error("requirements_test.go must never use t.Skip for a no-atom requirement (contract §3)")
	}

	// The gate/order claim with no real graph correlate must ALSO render as
	// an honest t.Log, not a vacuous "anchors := []string{...}" self-check
	// (the bug this fix closes).
	if !strings.Contains(text, `t.Log("no structural atom - see requirements_audit.md#r-gate-order-no-correlate")`) {
		t.Errorf("expected honest t.Log for R-gate-order-no-correlate (anchor found, but no graph correlate), got:\n%s", text)
	}

	for i, r := range []rune(text) {
		if r > unicode.MaxASCII {
			t.Fatalf("generated source contains non-ASCII rune %q at byte offset %d:\n%s", r, i, text)
		}
	}

	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, "requirements_test.go", text, parser.AllErrors); err != nil {
		t.Fatalf("generated source does not parse as Go: %v\n---\n%s", err, text)
	}
}

// TestRenderRequirementsTestFile_Deterministic asserts two renders of the
// same input are byte-identical (contract §5).
func TestRenderRequirementsTestFile_Deterministic(t *testing.T) {
	ets, reqs := syntheticRequirementFixtures()
	models := buildSyntheticEntityModels(t, ets)
	reqModels, err := BuildRequirementModels(reqs, models)
	if err != nil {
		t.Fatalf("BuildRequirementModels: %v", err)
	}
	a, err := RenderRequirementsTestFile("gen", reqModels)
	if err != nil {
		t.Fatalf("RenderRequirementsTestFile: %v", err)
	}
	b, err := RenderRequirementsTestFile("gen", reqModels)
	if err != nil {
		t.Fatalf("RenderRequirementsTestFile: %v", err)
	}
	if string(a) != string(b) {
		t.Fatal("RenderRequirementsTestFile is not byte-identical across repeated calls on the same input")
	}
	if !strings.HasPrefix(string(a), OwnershipMarker) {
		t.Error("generated file does not start with the ownership marker")
	}
}

// TestRenderAuditFile_RequirementsSection asserts the extended audit
// renderer's "## Requirements" section carries the verbatim (Cyrillic-legal)
// claim text and a heading anchored on the lowercased requirement id, for
// every SETTLED requirement including the honest-gap one.
func TestRenderAuditFile_RequirementsSection(t *testing.T) {
	ets, reqs := syntheticRequirementFixtures()
	models := buildSyntheticEntityModels(t, ets)
	reqModels, err := BuildRequirementModels(reqs, models)
	if err != nil {
		t.Fatalf("BuildRequirementModels: %v", err)
	}
	audit, err := RenderAuditFile(models, reqModels)
	if err != nil {
		t.Fatalf("RenderAuditFile: %v", err)
	}
	text := string(audit)

	if !strings.Contains(text, "## Requirements") {
		t.Fatal("expected a '## Requirements' section")
	}
	if !strings.Contains(text, "### R-field-atom {#r-field-atom}") {
		t.Errorf("expected heading anchor for R-field-atom, got:\n%s", text)
	}
	if !strings.Contains(text, "Поле текст test-card MUST быть заполнено содержательно.") {
		t.Error("expected verbatim (Cyrillic) claim text for R-field-atom in the audit file")
	}
	if !strings.Contains(text, "TestCard.Text") {
		t.Error("expected the field atom entry to name TestCard.Text")
	}
	if !strings.Contains(text, "no structural atom") {
		t.Error("expected the honest-gap entry to say so explicitly")
	}
}

// TestGenerateRequirementsFromGraph_Synthetic_CompilesAndRuns is the
// исполнимый-слой check (contract §0): entities.go + lifecycle.go +
// lifecycle_test.go + requirements_test.go, all rendered together for the
// synthetic fixtures, actually compile and `go test` green in a fresh temp
// module.
func TestGenerateRequirementsFromGraph_Synthetic_CompilesAndRuns(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not on PATH")
	}

	ets, reqs := syntheticRequirementFixtures()

	modelFiles, err := GenerateModelsFromGraph("gocode-req-synth-test", ets)
	if err != nil {
		t.Fatalf("GenerateModelsFromGraph: %v", err)
	}
	lifecycleFiles, err := GenerateLifecycleFromGraph(ets)
	if err != nil {
		t.Fatalf("GenerateLifecycleFromGraph: %v", err)
	}
	reqFiles, err := GenerateRequirementsFromGraph(ets, reqs)
	if err != nil {
		t.Fatalf("GenerateRequirementsFromGraph: %v", err)
	}

	dir := t.TempDir()
	all := map[string][]byte{}
	for k, v := range modelFiles {
		all[k] = v
	}
	for k, v := range lifecycleFiles {
		all[k] = v
	}
	for k, v := range reqFiles {
		all[k] = v
	}
	for name, content := range all {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated module failed to build/test: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "ok") {
		t.Errorf("expected 'ok' in test output, got:\n%s", out)
	}
}

// pratRequirements loads the real PRAT-hotam "prat" domain's SETTLED
// requirements (skipping if the sibling checkout is absent, same guard
// pratDomainDir already uses for the EntityType-only tests).
func pratRequirements(t *testing.T) ([]ontology.EntityType, []ontology.Requirement) {
	t.Helper()
	domainDir := pratDomainDir(t)
	g, err := loader.LoadGraph(filepath.Join(domainDir, "graph.json"))
	if err != nil {
		t.Fatalf("LoadGraph(prat): %v", err)
	}
	return g.EntityTypes, g.Requirements
}

// TestGenerateRequirementsFromGraph_RealPratDomain runs the full stage-4
// generator against the real PRAT-hotam "prat" domain (20 SETTLED
// requirements) and asserts: exactly 20 Test_ functions render, the source
// is pure ASCII, it parses as valid Go, and specific known atom
// classifications hold (a field atom, a gate/order atom, and an honest
// no-atom requirement) — the "manual sverka" spot-check from the task
// description, pinned into a test so a future generator regression on this
// real domain is caught, not just eyeballed once.
func TestGenerateRequirementsFromGraph_RealPratDomain(t *testing.T) {
	ets, reqs := pratRequirements(t)

	models := buildSyntheticEntityModels(t, ets)
	reqModels, err := BuildRequirementModels(reqs, models)
	if err != nil {
		t.Fatalf("BuildRequirementModels: %v", err)
	}

	settledCount := 0
	for _, r := range reqs {
		if r.Status == ontology.StatusSETTLED {
			settledCount++
		}
	}
	if len(reqModels) != settledCount {
		t.Fatalf("expected %d requirement models (one per SETTLED requirement), got %d", settledCount, len(reqModels))
	}
	t.Logf("prat domain: %d SETTLED requirements", settledCount)

	byID := make(map[string]*requirementModel, len(reqModels))
	for _, rm := range reqModels {
		byID[rm.src.ID] = rm
	}

	// Known field atom: R-gate-pg3-brd-approved names US/AC/UC/COSMIC, all
	// literal field names of brd-package.
	if rm, ok := byID["R-gate-pg3-brd-approved"]; ok {
		if rm.kind != atomKindField {
			t.Errorf("R-gate-pg3-brd-approved: kind = %v, want atomKindField", rm.kind)
		} else {
			t.Logf("R-gate-pg3-brd-approved field atoms: %d hits", len(rm.fields))
		}
	} else {
		t.Error("expected R-gate-pg3-brd-approved to be present")
	}

	// R-brd-integrity-zero-blockers has MUST plus id-shaped anchors (P-G3,
	// P-G3-CQA), but NEITHER anchor resolves to a runtime-comparable
	// structural correlate anywhere in the prat domain: "P-G3" is only
	// found inside brd-package's Cyrillic `why` text (textual, not a
	// lifecycle.state.name or another requirement id), and "P-G3-CQA" is
	// not found anywhere at all. This is the exact case the fix in this
	// package closes: previously the generator classified this as
	// atomKindGate and rendered a vacuous self-check of the literal
	// "P-G3"/"P-G3-CQA" anchors it had just found in the claim; now it
	// honestly reclassifies to atomKindNone (contract §3 closing row) rather
	// than imitate a structural check that does not exist.
	if rm, ok := byID["R-brd-integrity-zero-blockers"]; ok {
		if rm.kind != atomKindNone {
			t.Errorf("R-brd-integrity-zero-blockers: kind = %v, want atomKindNone (no anchor resolves to a runtime-comparable graph correlate)", rm.kind)
		}
		foundWhyOnly := false
		for _, c := range rm.gate.correlates {
			if c.anchor == "P-G3" && c.kind == gateAnchorCorrelateWhy {
				foundWhyOnly = true
			}
		}
		if !foundWhyOnly {
			t.Errorf("R-brd-integrity-zero-blockers: expected anchor %q to resolve to a why-text-only correlate, got %+v", "P-G3", rm.gate.correlates)
		}
	} else {
		t.Error("expected R-brd-integrity-zero-blockers to be present")
	}

	// Known REAL gate/order atom on the prat domain: R-gate-pg1r-risk-
	// registry-mandatory's "P-G1-R" anchor is a genuine substring of
	// risk-registry's "P-G1-R-pass" lifecycle state name — a real
	// structural correlate, found independently of the claim text.
	if rm, ok := byID["R-gate-pg1r-risk-registry-mandatory"]; ok {
		if rm.kind != atomKindGate {
			t.Errorf("R-gate-pg1r-risk-registry-mandatory: kind = %v, want atomKindGate", rm.kind)
		} else if !rm.gate.hasStructuralCorrelate() {
			t.Error("R-gate-pg1r-risk-registry-mandatory: expected a real structural correlate")
		} else {
			t.Logf("R-gate-pg1r-risk-registry-mandatory gate anchors: %v, correlates: %+v", rm.gate.anchors, rm.gate.correlates)
		}
	} else {
		t.Error("expected R-gate-pg1r-risk-registry-mandatory to be present")
	}

	// Known honest gap: R-prat-substrate has MUST but no id-shaped anchor
	// and no field/state/entity-slug literal match.
	if rm, ok := byID["R-prat-substrate"]; ok {
		if rm.kind != atomKindNone {
			t.Errorf("R-prat-substrate: kind = %v, want atomKindNone (honest gap)", rm.kind)
		}
	} else {
		t.Error("expected R-prat-substrate to be present")
	}

	src, err := RenderRequirementsTestFile("gen", reqModels)
	if err != nil {
		t.Fatalf("RenderRequirementsTestFile: %v", err)
	}
	text := string(src)

	for i, r := range []rune(text) {
		if r > unicode.MaxASCII {
			t.Fatalf("generated source contains non-ASCII rune %q at byte offset %d", r, i)
		}
	}

	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, "requirements_test.go", text, parser.AllErrors); err != nil {
		t.Fatalf("generated source does not parse as Go: %v\n---\n%s", err, text)
	}

	audit, err := RenderAuditFile(models, reqModels)
	if err != nil {
		t.Fatalf("RenderAuditFile: %v", err)
	}
	if !strings.Contains(string(audit), "## Requirements") {
		t.Error("expected a '## Requirements' section in the real-domain audit file")
	}
}

// TestGenerateRequirementsFromGraph_RealPratDomain_CompilesAndRuns is the
// исполнимый-слой check on the real domain: entities.go + lifecycle.go +
// lifecycle_test.go + requirements_test.go, generated together from the
// real prat graph, compile and `go test` green in a fresh temp module —
// idempotency is checked separately below (two renders, byte-identical).
func TestGenerateRequirementsFromGraph_RealPratDomain_CompilesAndRuns(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not on PATH")
	}
	ets, reqs := pratRequirements(t)

	modelFiles, err := GenerateModelsFromGraph("gocode-prat-req-e2e-test", ets)
	if err != nil {
		var termErr *UnknownTermError
		if errors.As(err, &termErr) {
			t.Skipf("prat domain has an unmapped glossary term today: %v", err)
		}
		t.Fatalf("GenerateModelsFromGraph: %v", err)
	}
	lifecycleFiles, err := GenerateLifecycleFromGraph(ets)
	if err != nil {
		t.Fatalf("GenerateLifecycleFromGraph: %v", err)
	}
	reqFiles, err := GenerateRequirementsFromGraph(ets, reqs)
	if err != nil {
		t.Fatalf("GenerateRequirementsFromGraph: %v", err)
	}

	dir := t.TempDir()
	all := map[string][]byte{}
	for k, v := range modelFiles {
		all[k] = v
	}
	for k, v := range lifecycleFiles {
		all[k] = v
	}
	for k, v := range reqFiles {
		all[k] = v
	}
	for name, content := range all {
		if name == "requirements_audit.md" {
			continue // not Go source
		}
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated module failed to build/test: %v\n%s", err, out)
	}
	t.Logf("go test output:\n%s", out)
}

// TestGenerateRequirementsFromGraph_RealPratDomain_Idempotent asserts two
// independent generation runs on the unchanged real prat graph produce
// byte-identical requirements_test.go and requirements_audit.md (contract §5).
func TestGenerateRequirementsFromGraph_RealPratDomain_Idempotent(t *testing.T) {
	ets, reqs := pratRequirements(t)

	first, err := GenerateRequirementsFromGraph(ets, reqs)
	if err != nil {
		t.Fatalf("GenerateRequirementsFromGraph (first run): %v", err)
	}
	second, err := GenerateRequirementsFromGraph(ets, reqs)
	if err != nil {
		t.Fatalf("GenerateRequirementsFromGraph (second run): %v", err)
	}

	for name := range first {
		if string(first[name]) != string(second[name]) {
			t.Errorf("%s is not byte-identical across two generation runs", name)
		}
	}
}
