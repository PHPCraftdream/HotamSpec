package gocode

import (
	"errors"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
			rm := BuildRequirementModel(byID[tc.id], models, settledOnly(reqs), nil)
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
	rm := BuildRequirementModel(req, models, settledOnly(reqs), nil)
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
	rm := BuildRequirementModel(req, models, settledOnly(reqs), nil)
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
	rm := BuildRequirementModel(req, models, settledOnly(reqs), nil)
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
	rm := BuildRequirementModel(req, models, settledOnly(reqs), nil)
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
	rm := BuildRequirementModel(req, models, settledOnly(reqs), nil)
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

// twoForecastEntityTypes builds a minimal synthetic domain with two
// DIFFERENT EntityTypes ("plan-package" and "design-package") that each
// declare their own "прогноз" (forecast) field — the exact ambiguity shape
// found on the real prat domain (brd-package.прогноз and
// sdr-package.прогноз both translate to the single word "Forecast"). Each
// EntityType's why text quotes a DIFFERENT concrete forecast version token
// ("forecast_v2" for plan-package, "forecast_v3" for design-package), giving
// resolveScopedFieldMatches' why-token signal something to disambiguate
// against when a claim quotes one of those tokens specifically.
func twoForecastEntityTypes() []ontology.EntityType {
	return []ontology.EntityType{
		{
			Slug:        "plan-package",
			Description: "first synthetic entity with a forecast field",
			Why:         "plan-package carries forecast_v2 as its own concrete forecast token.",
			Fields: []ontology.EntityField{
				{Name: "прогноз", Kind: "string", Required: true},
			},
			Lifecycle: ontology.Lifecycle{
				Slug: "plan-package-lifecycle",
				States: []ontology.State{
					{Name: "черновик", Kind: ontology.StateKindInitial},
					{Name: "утверждён", Kind: ontology.StateKindTerminal},
				},
			},
		},
		{
			Slug:        "design-package",
			Description: "second synthetic entity with a forecast field",
			Why:         "design-package carries forecast_v3 as its own concrete forecast token.",
			Fields: []ontology.EntityField{
				{Name: "прогноз", Kind: "string", Required: true},
			},
			Lifecycle: ontology.Lifecycle{
				Slug: "design-package-lifecycle",
				States: []ontology.State{
					{Name: "черновик", Kind: ontology.StateKindInitial},
					{Name: "утверждён", Kind: ontology.StateKindTerminal},
				},
			},
		},
	}
}

// TestResolveScopedFieldMatches_AmbiguousSingleWord_WhyTokenDisambiguates
// asserts that when a claim quotes a concrete forecast-version token that
// only ONE of the two ambiguous EntityTypes' why text also quotes
// (forecast_v3 - design-package's own token, not plan-package's
// forecast_v2), only THAT EntityType's field atom is kept — the exact fix
// for the real prat over-match bug (R-gate-pg4-solution-ready wrongly
// getting BrdPackage.Forecast alongside the correct SdrPackage.Forecast).
func TestResolveScopedFieldMatches_AmbiguousSingleWord_WhyTokenDisambiguates(t *testing.T) {
	ets := twoForecastEntityTypes()
	models := buildSyntheticEntityModels(t, ets)

	claim := "Design sign-off MUST require forecast_v3 to be locked before dev starts."
	fields := resolveScopedFieldMatches(claim, models, nil)

	if len(fields) != 1 {
		t.Fatalf("expected exactly 1 disambiguated field atom, got %d: %+v", len(fields), fields)
	}
	if fields[0].entity.structName != "DesignPackage" {
		t.Errorf("field atom entity = %s, want DesignPackage (the one whose why quotes forecast_v3)", fields[0].entity.structName)
	}
}

// TestResolveScopedFieldMatches_AmbiguousSingleWord_NoSignalDropsAll asserts
// that when a claim mentions the ambiguous single-word translated term but
// quotes NO concrete token tying it to either EntityType's why text (and
// names neither entity's own slug, nor any sibling field of either entity),
// NO field atom is produced for either EntityType — an honest gap is
// preferred over a false-positive cross-entity atom (GEN-CODE-CONTRACT.md §0
// "explicit refusal over silent guessing"), mirroring the real
// R-gate-pg1-planning-approved case (claim names "forecast_v1", which
// neither brd-package's nor sdr-package's why text quotes).
func TestResolveScopedFieldMatches_AmbiguousSingleWord_NoSignalDropsAll(t *testing.T) {
	ets := twoForecastEntityTypes()
	models := buildSyntheticEntityModels(t, ets)

	claim := "Pilot planning MUST produce an early forecast before Planning Approved."
	fields := resolveScopedFieldMatches(claim, models, nil)

	if len(fields) != 0 {
		t.Fatalf("expected zero field atoms for a genuinely ambiguous, unresolvable match, got %d: %+v", len(fields), fields)
	}
}

// TestResolveScopedFieldMatches_AmbiguousSingleWord_SlugSignalDisambiguates
// asserts the entity-slug signal alone (claim names the EntityType's own
// graph slug directly, with no forecast-version token at all) is enough to
// keep that one EntityType's field atom while still excluding the sibling.
func TestResolveScopedFieldMatches_AmbiguousSingleWord_SlugSignalDisambiguates(t *testing.T) {
	ets := twoForecastEntityTypes()
	models := buildSyntheticEntityModels(t, ets)

	claim := "The plan-package forecast MUST be reviewed before sign-off."
	fields := resolveScopedFieldMatches(claim, models, nil)

	if len(fields) != 1 {
		t.Fatalf("expected exactly 1 disambiguated field atom, got %d: %+v", len(fields), fields)
	}
	if fields[0].entity.structName != "PlanPackage" {
		t.Errorf("field atom entity = %s, want PlanPackage (the one whose slug the claim names)", fields[0].entity.structName)
	}
}

// TestResolveScopedFieldMatches_UnambiguousSingleWord_Unaffected asserts a
// single-word translated field name that only ONE EntityType in the domain
// has at all is entirely unaffected by the scoping guard — the guard only
// engages when 2+ EntityTypes share the same translated word.
func TestResolveScopedFieldMatches_UnambiguousSingleWord_Unaffected(t *testing.T) {
	ets := []ontology.EntityType{syntheticEntityType()}
	models := buildSyntheticEntityModels(t, ets)

	claim := "Поле текст test-card MUST быть заполнено содержательно."
	fields := resolveScopedFieldMatches(claim, models, nil)

	if len(fields) != 1 {
		t.Fatalf("expected exactly 1 field atom (unambiguous, unique translated word), got %d: %+v", len(fields), fields)
	}
	if fields[0].entity.structName != "TestCard" || fields[0].field.fieldName != "Text" {
		t.Errorf("field atom = %s.%s, want TestCard.Text", fields[0].entity.structName, fields[0].field.fieldName)
	}
}

// TestBuildRequirementModels_ExcludesNonSettled asserts DRAFT/REJECTED
// requirements never reach requirements_test.go — only SETTLED ones (contract
// §1 task scope: "для каждого SETTLED-требования домена").
func TestBuildRequirementModels_ExcludesNonSettled(t *testing.T) {
	ets, reqs := syntheticRequirementFixtures()
	models := buildSyntheticEntityModels(t, ets)
	reqModels, err := BuildRequirementModels(reqs, models, nil)
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
	reqModels, err := BuildRequirementModels(reqs, models, nil)
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
	reqModels, err := BuildRequirementModels(reqs, models, nil)
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
	reqModels, err := BuildRequirementModels(reqs, models, nil)
	if err != nil {
		t.Fatalf("BuildRequirementModels: %v", err)
	}
	audit, err := RenderAuditFile(models, reqModels, nil, nil)
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
	reqModels, err := BuildRequirementModels(reqs, models, nil)
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

	// R-brd-integrity-zero-blockers (synced with live prat as of 2026-07-16;
	// will drift as prat grows). This block has re-drifted twice as the
	// live domain grew, each time to a HIGHER-priority contract §3 row:
	//  - originally: P-G3/P-G3-CQA anchors were why-only -> atomKindNone;
	//  - task #202 landed R-gate-pg3-cqa-mandatory -> the P-G3-CQA anchor
	//    became a real requirement-id correlate -> atomKindGate;
	//  - tasks #193-#217 landed risk-registry's kind:reference fields p_g3
	//    and p_g4. termMatch splits "p_g3" into the word sequence [p g3],
	//    which the claim's literal "P-G3" (hyphen-joined, boundary-bounded)
	//    now hits as a RAW field-name match — contract §3 row 1 (field atom)
	//    outranks row 3 (gate atom) by the table's explicit top-to-bottom
	//    priority, so the requirement now classifies as atomKindField and
	//    rm.gate is never populated (row 1 returns before row 3 runs).
	// Semantically sound, not a generator bug: risk-registry.p_g3 IS the
	// domain's structural carrier of the P-G3 gate this claim is about (it
	// references brd-package, whose approval P-G3 gates on).
	if rm, ok := byID["R-brd-integrity-zero-blockers"]; ok {
		if rm.kind != atomKindField {
			t.Errorf("R-brd-integrity-zero-blockers: kind = %v, want atomKindField (claim's literal P-G3 raw-name-matches risk-registry.p_g3; contract §3 row 1 outranks row 3)", rm.kind)
		}
		foundPG3Field := false
		for _, fa := range rm.fields {
			if fa.entity.src.Slug == "risk-registry" && fa.field.src.Name == "p_g3" {
				foundPG3Field = true
			}
		}
		if !foundPG3Field {
			t.Errorf("R-brd-integrity-zero-blockers: expected a field atom on risk-registry.p_g3 (the claim's P-G3 carrier), got %d field atoms", len(rm.fields))
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

	audit, err := RenderAuditFile(models, reqModels, nil, nil)
	if err != nil {
		t.Fatalf("RenderAuditFile: %v", err)
	}
	if !strings.Contains(string(audit), "## Requirements") {
		t.Error("expected a '## Requirements' section in the real-domain audit file")
	}
}

// TestBuildRequirementModel_RealPratDomain_FieldAtomCarriesPipelineGate is
// task #209's focused unit-level proof (Coverage-audit #192 finding): a
// field atom on a kind:reference field whose claim names a concrete
// referenced state must carry that field's already-built precise-state
// pipeline gate, not just the plain "field is not empty" presence check.
// Exercises the real prat domain's three documented cases
// (R-gate-pg1-planning-approved's general-terminal gate,
// R-gate-pg3-brd-approved's precise v2 gate, R-gate-pg4-solution-ready's
// precise v3 gate) end to end: BuildPipelineGateModels -> BuildRequirementModels
// (gates threaded through, unlike the nil-gates call in
// TestGenerateRequirementsFromGraph_RealPratDomain above) -> asserts each
// field atom's fa.pipelineGate is the SAME *pipelineGateModel
// BuildPipelineGateModels built (identity, not merely equal fields) -> AND
// the rendered requirements_test.go source contains the expected
// "<Entity>_<Field>_pipeline_gate" sub-test name for each.
func TestBuildRequirementModel_RealPratDomain_FieldAtomCarriesPipelineGate(t *testing.T) {
	ets, reqs := pratRequirements(t)
	models := buildSyntheticEntityModels(t, ets)

	gates, err := BuildPipelineGateModels(models, reqs)
	if err != nil {
		t.Fatalf("BuildPipelineGateModels: %v", err)
	}
	byFuncName := make(map[string]*pipelineGateModel, len(gates))
	for _, g := range gates {
		byFuncName[g.funcName] = g
	}

	reqModels, err := BuildRequirementModels(reqs, models, gates)
	if err != nil {
		t.Fatalf("BuildRequirementModels: %v", err)
	}
	byID := make(map[string]*requirementModel, len(reqModels))
	for _, rm := range reqModels {
		byID[rm.src.ID] = rm
	}

	cases := []struct {
		reqID       string
		entityField string // "<EntityStructName>.<FieldName>"
		gateFunc    string
	}{
		{"R-gate-pg1-planning-approved", "ImplementationOrder.GraphDependencies", "GateImplementationOrderGraphDependenciesRequiresFrGraphTerminal"},
		{"R-gate-pg3-brd-approved", "BrdPackage.Forecast", "GateBrdPackageForecastRequiresForecast_ForecastStateV2"},
		{"R-gate-pg4-solution-ready", "SdrPackage.Forecast", "GateSdrPackageForecastRequiresForecast_ForecastStateV3"},
	}

	for _, tc := range cases {
		t.Run(tc.reqID, func(t *testing.T) {
			rm, ok := byID[tc.reqID]
			if !ok {
				t.Fatalf("expected requirement %q to be present", tc.reqID)
			}
			wantGate, ok := byFuncName[tc.gateFunc]
			if !ok {
				t.Fatalf("expected pipeline gate function %q to be present among: %v", tc.gateFunc, gateFuncNames(gates))
			}
			var found *fieldAtom
			for i := range rm.fields {
				key := rm.fields[i].entity.structName + "." + rm.fields[i].field.fieldName
				if key == tc.entityField {
					found = &rm.fields[i]
					break
				}
			}
			if found == nil {
				t.Fatalf("%s: expected a field atom for %s, got %v", tc.reqID, tc.entityField, rm.fields)
			}
			if found.pipelineGate != wantGate {
				t.Fatalf("%s: field atom %s.pipelineGate = %p (funcName %s), want the SAME *pipelineGateModel BuildPipelineGateModels built for %q (%p)",
					tc.reqID, tc.entityField, found.pipelineGate, gateFuncNameOrNil(found.pipelineGate), tc.gateFunc, wantGate)
			}
		})
	}

	src, err := RenderRequirementsTestFile("gen", reqModels)
	if err != nil {
		t.Fatalf("RenderRequirementsTestFile: %v", err)
	}
	text := string(src)

	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, "requirements_test.go", text, parser.AllErrors); err != nil {
		t.Fatalf("generated source does not parse as Go: %v\n---\n%s", err, text)
	}

	for _, subName := range []string{
		"ImplementationOrder_GraphDependencies_pipeline_gate",
		"BrdPackage_Forecast_pipeline_gate",
		"SdrPackage_Forecast_pipeline_gate",
	} {
		if !strings.Contains(text, strconv.Quote(subName)) {
			t.Errorf("expected rendered requirements_test.go to contain sub-test %s", strconv.Quote(subName))
		}
	}
}

// gateFuncNameOrNil is a small t.Fatalf formatting helper: g.funcName when g
// is non-nil, "<nil>" otherwise (avoids a nil-pointer panic inside the
// failure message itself).
func gateFuncNameOrNil(g *pipelineGateModel) string {
	if g == nil {
		return "<nil>"
	}
	return g.funcName
}

// TestGenerateRequirementsFromGraph_RealPratDomain_CompilesAndRuns is the
// исполнимый-слой check on the real domain: entities.go + lifecycle.go +
// lifecycle_test.go + requirements_test.go + pipeline_test.go, generated
// together from the real prat graph, compile and `go test` green in a fresh
// temp module — idempotency is checked separately below (two renders,
// byte-identical). pipeline_test.go (GeneratePipelineFromGraph) is included
// here (task #209) because requirements_test.go's own field-atom sub-tests
// now legitimately call pipeline.go's generated Gate<...> functions for
// kind:reference fields with a pipeline gate (see
// renderFieldAtomPipelineGateSubTest, requirements_test_gen.go) — omitting
// pipeline_test.go would leave those calls undefined, exactly the kind of
// cross-stage compile dependency this исполнимый-слой check exists to catch
// (contract §0, "код реально запускается").
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
	pipelineFiles, err := GeneratePipelineFromGraph(ets, reqs)
	if err != nil {
		t.Fatalf("GeneratePipelineFromGraph: %v", err)
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
	for k, v := range pipelineFiles {
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
