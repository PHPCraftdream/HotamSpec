package gocode

import (
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// TestBuildCoverageReport_Deterministic asserts two independent
// BuildCoverageReport calls on the SAME requirementModel/entityModels input
// produce an identical candidate list, in identical order — contract §5's
// determinism invariant applied to the extraction mechanism itself (not just
// the rendered file downstream), and contract §3.1's explicit instruction
// that extraction is "детерминированно (без LLM-угадывания)".
func TestBuildCoverageReport_Deterministic(t *testing.T) {
	ets, reqs := syntheticRequirementFixtures()
	models := buildSyntheticEntityModels(t, ets)
	reqModels, err := BuildRequirementModels(reqs, models, nil)
	if err != nil {
		t.Fatalf("BuildRequirementModels: %v", err)
	}
	settled := settledOnly(reqs)

	for _, rm := range reqModels {
		first := BuildCoverageReport(rm, models, settled)
		second := BuildCoverageReport(rm, models, settled)

		if len(first.candidates) != len(second.candidates) {
			t.Fatalf("%s: candidate count differs across repeated calls: %d vs %d", rm.src.ID, len(first.candidates), len(second.candidates))
		}
		for i := range first.candidates {
			a, b := first.candidates[i], second.candidates[i]
			if a.text != b.text || a.kind != b.kind || a.resolution != b.resolution {
				t.Errorf("%s: candidate[%d] differs across repeated calls: %+v vs %+v", rm.src.ID, i, a, b)
			}
		}
	}
}

// TestBuildCoverageReport_Deterministic_RealPratDomain runs the same
// repeated-call determinism check against the real prat domain's full
// SETTLED corpus, so the determinism guarantee is pinned against real claim
// text shapes, not only the small synthetic fixture set.
func TestBuildCoverageReport_Deterministic_RealPratDomain(t *testing.T) {
	ets, reqs := pratRequirements(t)
	models := buildSyntheticEntityModels(t, ets)
	reqModels, err := BuildRequirementModels(reqs, models, nil)
	if err != nil {
		t.Fatalf("BuildRequirementModels: %v", err)
	}
	settled := settledOnly(reqs)

	for _, rm := range reqModels {
		first := BuildCoverageReport(rm, models, settled)
		second := BuildCoverageReport(rm, models, settled)
		if len(first.candidates) != len(second.candidates) {
			t.Fatalf("%s: candidate count differs across repeated calls: %d vs %d", rm.src.ID, len(first.candidates), len(second.candidates))
		}
		for i := range first.candidates {
			a, b := first.candidates[i], second.candidates[i]
			if a.text != b.text || a.kind != b.kind || a.resolution != b.resolution {
				t.Errorf("%s: candidate[%d] differs across repeated calls: %+v vs %+v", rm.src.ID, i, a, b)
			}
		}
	}
}

// partialCoverageFixture builds a synthetic domain engineered for contract
// §3.1's own worked example shape: a requirement whose claim names a
// required field that DOES get captured as a field atom (so the requirement
// is not a bare "no structural atom" case), PLUS a second required field of
// the SAME EntityType that the claim ALSO names but that the classification
// heuristic does not turn into an atom (a single-field-atom requirement only
// ever gets ONE atom row today - BuildRequirementModel's row-1 walk collects
// every matching field, so to engineer a genuine "field the claim names but
// is not in this requirement's own atoms" gap without fighting that
// behavior, the second field is deliberately given a name the claim does NOT
// literally contain via termMatch, but DOES contain via the coverage
// module's own broader capitalized-token extraction - i.e. a capitalized
// abbreviation the claim uses that also happens to equal another EntityType
// field's name/translated identifier elsewhere in the domain, structurally
// unrelated to this requirement (contract §3.1's actual prat-shaped
// "feature_lead not in atoms" gap, generalized).
func partialCoverageFixture() ([]ontology.EntityType, []ontology.Requirement) {
	covered := ontology.EntityType{
		Slug:        "widget",
		Description: "synthetic entity whose текст field IS captured as this requirement's atom",
		Fields: []ontology.EntityField{
			{Name: "текст", Kind: "string", Required: true},
		},
		Lifecycle: ontology.Lifecycle{
			Slug: "widget-lifecycle",
			States: []ontology.State{
				{Name: "черновик", Kind: ontology.StateKindInitial},
				{Name: "утверждён", Kind: ontology.StateKindTerminal},
			},
		},
	}
	// A SECOND, structurally unrelated EntityType whose own field the claim
	// ALSO happens to name (by exact translated identifier, "Owner") - since
	// BuildRequirementModel's row-1 field scan (resolveScopedFieldMatches)
	// is unconditional for unambiguous non-shared translated words, this
	// second field WOULD also become one of this requirement's own atoms if
	// termMatch found it in the claim text. To get a genuine "resolves
	// elsewhere, not in this requirement's atoms" gap instead, the claim
	// names the OTHER entity's field only via a capitalized-token spelling
	// that termMatch's word-sequence rule does not recognize as the same
	// graph name (a plausible-looking abbreviation, not the literal
	// underscore/translated form) - "OWNR" is deliberately NOT
	// "owner"/"Owner" so termMatch never fires, while still being close
	// enough conceptually for a human/steward reading the audit file
	// (contract §3.1's own purpose): this fixture instead uses the
	// EntityType's own SLUG as the capitalized token, which resolveCapitalizedCandidate
	// resolves via entity-slug match but resolveScopedFieldMatches/termMatch (row 1)
	// never considers a field-name hit at all - a clean, unambiguous
	// "resolves to a real graph element (an EntityType), not part of this
	// requirement's own atoms" case.
	other := ontology.EntityType{
		Slug:        "gizmo",
		Description: "second synthetic entity, named by capitalized token in the claim but never becoming this requirement's own atom",
		Fields: []ontology.EntityField{
			{Name: "риск", Kind: "string", Required: true},
		},
		Lifecycle: ontology.Lifecycle{
			Slug: "gizmo-lifecycle",
			States: []ontology.State{
				{Name: "черновик", Kind: ontology.StateKindInitial},
				{Name: "утверждён", Kind: ontology.StateKindTerminal},
			},
		},
	}

	reqs := []ontology.Requirement{
		{
			ID:     "R-partial-coverage",
			Claim:  "Поле текст widget MUST быть заполнено; GIZMO упомянут в claim, но не входит в атомы этого требования.",
			Status: ontology.StatusSETTLED,
			Owner:  "test-owner",
		},
	}
	return []ontology.EntityType{covered, other}, reqs
}

// TestBuildCoverageReport_PartialCoverageGap is the synthetic
// partially-covered-requirement case the task brief asks for: a requirement
// that DOES get a real field atom (so it is not a bare "no structural atom"
// case) but whose claim ALSO names a second graph element (here: the
// "gizmo" EntityType, via its slug spelled as a capitalized token) that
// never became one of this requirement's own atoms - contract §3.1's
// "partial coverage gap": resolves to a real graph concept, but not mirrored
// into this requirement's atoms.
func TestBuildCoverageReport_PartialCoverageGap(t *testing.T) {
	ets, reqs := partialCoverageFixture()
	models := buildSyntheticEntityModels(t, ets)
	reqModels, err := BuildRequirementModels(reqs, models, nil)
	if err != nil {
		t.Fatalf("BuildRequirementModels: %v", err)
	}
	if len(reqModels) != 1 {
		t.Fatalf("expected exactly 1 requirement model, got %d", len(reqModels))
	}
	rm := reqModels[0]
	if rm.kind != atomKindField {
		t.Fatalf("expected atomKindField (the widget.текст atom), got %v", rm.kind)
	}

	cov := BuildCoverageReport(rm, models, settledOnly(reqs))

	gaps := cov.gaps()
	var found *candidateTerm
	for i := range gaps {
		if gaps[i].text == "GIZMO" {
			found = &gaps[i]
		}
	}
	if found == nil {
		t.Fatalf("expected a partial-coverage-gap candidate for %q, got gaps: %+v", "GIZMO", gaps)
	}
	if found.resolution != candidateResolvedElsewhere {
		t.Errorf("GIZMO candidate resolution = %v, want candidateResolvedElsewhere (resolves to a real EntityType, not this requirement's own atom)", found.resolution)
	}
	if found.resolvedEntity == nil || found.resolvedEntity.structName != "Gizmo" {
		t.Errorf("GIZMO candidate resolvedEntity = %+v, want the Gizmo entityModel", found.resolvedEntity)
	}

	// The requirement's own captured atom (widget's текст field, translated
	// identifier "Text") must NOT appear in the gap list - it is fully
	// covered, contract §3.1's "resolves into this requirement's own atoms"
	// case, not a gap.
	for _, g := range gaps {
		if g.resolvedField != nil && g.resolvedField.fieldName == "Text" && g.resolvedEntity.structName == "Widget" {
			t.Errorf("Widget.Text is this requirement's own atom - must not be reported as a coverage gap, got %+v", g)
		}
	}

	if cov.resolvedCount() == 0 {
		t.Error("expected at least one candidate to resolve (the widget slug/field itself)")
	}
	// gaps() includes every candidate that is NOT one of this requirement's
	// own atoms - GIZMO resolves to a real graph element (candidateResolvedElsewhere)
	// but is still a gap in that sense, distinct from resolvedCount() (which
	// counts BOTH candidateResolvedAtom and candidateResolvedElsewhere as
	// "resolved somewhere in the graph"). The requirement's own atom
	// (Widget.Text) must be absent from gaps(); GIZMO must be present.
	if len(gaps) == 0 {
		t.Error("expected at least one partial-coverage-gap candidate (GIZMO), got none")
	}
}

// TestExtractCapitalizedCandidates_ExcludesMetaTokensAndSingleLetters pins
// the extraction rule's exclusion list: reserved meta-language tokens
// (MUST/ALWAYS/NEVER/ONLY/ANY/MUST NOT) are never graph terms (contract
// §3.1's own explicit instruction to exclude them), and lone single-letter
// capitalized tokens (contract §3.1's own worked examples - SA, DoR, COSMIC,
// US, AC, UC - are all 2+ characters) never appear as candidates either,
// since a single letter cannot honestly resolve against the graph without
// spurious accidental-substring hits.
func TestExtractCapitalizedCandidates_ExcludesMetaTokensAndSingleLetters(t *testing.T) {
	claim := "Gate P-G3 (BRD Approved) MUST NOT skip ALWAYS NEVER ONLY ANY review; N:M mapping."
	got := extractCapitalizedCandidates(claim)

	for _, tok := range []string{"MUST", "ALWAYS", "NEVER", "ONLY", "ANY"} {
		for _, g := range got {
			if g == tok {
				t.Errorf("extractCapitalizedCandidates(%q) unexpectedly included reserved meta-token %q: %v", claim, tok, got)
			}
		}
	}
	for _, tok := range []string{"P", "N", "M"} {
		for _, g := range got {
			if g == tok {
				t.Errorf("extractCapitalizedCandidates(%q) unexpectedly included single-letter token %q: %v", claim, tok, got)
			}
		}
	}
	foundBRD, foundGate, foundApproved := false, false, false
	for _, g := range got {
		switch g {
		case "BRD":
			foundBRD = true
		case "Gate":
			foundGate = true
		case "Approved":
			foundApproved = true
		}
	}
	if !foundBRD || !foundGate || !foundApproved {
		t.Errorf("extractCapitalizedCandidates(%q) = %v, expected to include BRD, Gate, Approved", claim, got)
	}
}

// TestExtractCapitalizedCandidates_Deterministic asserts repeated calls on
// the same claim text produce the identical candidate list (same elements,
// same order) - the extraction step's own determinism, independent of
// BuildCoverageReport's higher-level determinism test above.
func TestExtractCapitalizedCandidates_Deterministic(t *testing.T) {
	claim := "Feature Lead MUST быть назначенным SA — Ready for Development до P-G4."
	first := extractCapitalizedCandidates(claim)
	second := extractCapitalizedCandidates(claim)
	if len(first) != len(second) {
		t.Fatalf("candidate count differs across repeated calls: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Errorf("candidate[%d] differs across repeated calls: %q vs %q", i, first[i], second[i])
		}
	}
}

// TestExtractQuotedCandidates_ExtractsBothQuoteStyles asserts the Rule-3
// mechanism (contract §3.1: "квотированные/бэктик-выделенные термины")
// extracts both double-quoted and backtick-delimited spans, even though (per
// the task brief's own observation) no real prat claim currently uses this
// syntax - the mechanism must exist and be exercised directly.
func TestExtractQuotedCandidates_ExtractsBothQuoteStyles(t *testing.T) {
	claim := `Поле "forecast_v2" MUST совпадать с состоянием ` + "`v2`" + ` графа.`
	got := extractQuotedCandidates(claim)
	want := map[string]bool{"forecast_v2": true, "v2": true}
	if len(got) != 2 {
		t.Fatalf("extractQuotedCandidates(%q) = %v, want 2 entries", claim, got)
	}
	for _, g := range got {
		if !want[g] {
			t.Errorf("extractQuotedCandidates(%q) included unexpected term %q", claim, g)
		}
	}
}

// TestRenderAuditFile_CoverageSection asserts RenderAuditFile's per-
// requirement entries carry the contract §3.1 "Coverage: N/M candidate
// terms resolved" line, and that a requirement with a genuine partial-
// coverage gap lists it explicitly (never silently only showing "atom
// found" completeness).
func TestRenderAuditFile_CoverageSection(t *testing.T) {
	ets, reqs := partialCoverageFixture()
	models := buildSyntheticEntityModels(t, ets)
	reqModels, err := BuildRequirementModels(reqs, models, nil)
	if err != nil {
		t.Fatalf("BuildRequirementModels: %v", err)
	}
	audit, err := RenderAuditFile(models, reqModels, nil)
	if err != nil {
		t.Fatalf("RenderAuditFile: %v", err)
	}
	text := string(audit)

	if !strings.Contains(text, "Coverage:") {
		t.Fatalf("expected a 'Coverage:' line in the audit file, got:\n%s", text)
	}
	if !strings.Contains(text, "candidate terms resolved") {
		t.Errorf("expected the exact 'candidate terms resolved' phrasing, got:\n%s", text)
	}
	if !strings.Contains(text, "`GIZMO`") {
		t.Errorf("expected the unresolved-as-this-requirement's-atom GIZMO candidate to be listed, got:\n%s", text)
	}
	if !strings.Contains(text, "Unresolved / partial-coverage-gap candidate terms:") {
		t.Errorf("expected the partial-coverage-gap section header, got:\n%s", text)
	}
}

// TestRenderAuditFile_CoverageSection_Deterministic asserts two renders of
// the same input produce byte-identical output (contract §5), now that the
// coverage section is part of every rendered requirement entry.
func TestRenderAuditFile_CoverageSection_Deterministic(t *testing.T) {
	ets, reqs := partialCoverageFixture()
	models := buildSyntheticEntityModels(t, ets)
	reqModels, err := BuildRequirementModels(reqs, models, nil)
	if err != nil {
		t.Fatalf("BuildRequirementModels: %v", err)
	}
	a, err := RenderAuditFile(models, reqModels, nil)
	if err != nil {
		t.Fatalf("RenderAuditFile (first): %v", err)
	}
	b, err := RenderAuditFile(models, reqModels, nil)
	if err != nil {
		t.Fatalf("RenderAuditFile (second): %v", err)
	}
	if string(a) != string(b) {
		t.Fatal("RenderAuditFile is not byte-identical across repeated calls on the same input")
	}
}

// TestResolveCapitalizedCandidate_AmbiguousSingleLetter_NeverProduced is a
// regression guard for the exact noise pattern found (and removed) during
// this feature's own development on the real prat domain: before the 2+
// character length floor, single letters like "P" (from "Gate P-G3") were
// extracted and then spuriously "resolved" via substring containment
// against unrelated requirement ids. This test pins that the floor holds via
// the public extraction entrypoint, not just the regex in isolation.
func TestResolveCapitalizedCandidate_AmbiguousSingleLetter_NeverProduced(t *testing.T) {
	claim := "Ось A, Ось B и Ось C НЕЛЬЗЯ смешивать; X и Y стадии релиза."
	got := extractCapitalizedCandidates(claim)
	for _, g := range got {
		if len([]rune(g)) < 2 {
			t.Errorf("extractCapitalizedCandidates(%q) produced a single-letter candidate %q, want none: %v", claim, g, got)
		}
	}
}
