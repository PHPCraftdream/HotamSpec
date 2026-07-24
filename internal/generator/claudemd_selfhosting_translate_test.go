package generator

import (
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// TestRenderMediationLoopBlock_SelfHostingBranchesTranslateStep is the
// acceptance test for task #347 (RAC-C part 1): the TRANSLATE step of the
// generated MEDIATION-LOOP block must describe the internal/selfspec +
// `hotam sync-self` outcome for a Requirement/Rejection on the self-hosting
// domain (g.SelfHosting == true, e.g. hotam-spec-self) — RAC-B3's
// apply-proposal/land guard actively refuses the plain JSON path for those
// two kinds there — while a consumer (non-self-hosting) domain's crystal
// keeps the single unbranched Proposed*/JSON path for every outcome kind,
// Requirement included, and must NOT mention internal/selfspec or sync-self
// at all (sync-self's own gate 0 refuses to run against a non-self-hosting
// domain, so naming it to a consumer operator would point at a command that
// cannot succeed for them).
func TestRenderMediationLoopBlock_SelfHostingBranchesTranslateStep(t *testing.T) {
	t.Parallel()

	selfHosting := RenderMediationLoopBlock(&ontology.Graph{SelfHosting: true})
	consumer := RenderMediationLoopBlock(&ontology.Graph{SelfHosting: false})
	nilGraph := RenderMediationLoopBlock(nil)

	// self-hosting branch: names the Go-registry + sync-self path.
	for _, want := range []string{"internal/selfspec", "sync-self", "requirements_"} {
		if !strings.Contains(selfHosting, want) {
			t.Errorf("self-hosting MEDIATION-LOOP text missing %q:\n%s", want, selfHosting)
		}
	}
	// The self-hosting note must also name the ordinary JSON kinds that stay
	// on the Proposed*-JSON path (Conflict/Assumption/journal), so the text
	// draws the boundary rather than implying sync-self covers everything.
	for _, want := range []string{"Conflict", "Assumption", "GateSignoffBatch", "ReviewMark"} {
		if !strings.Contains(selfHosting, want) {
			t.Errorf("self-hosting MEDIATION-LOOP text missing the non-Requirement JSON-path kind %q:\n%s", want, selfHosting)
		}
	}

	// consumer (and nil, the "no graph yet"/non-self-hosting default)
	// TRANSLATE step: no mention of the self-hosting-only mechanism. Scoped
	// to the TRANSLATE step's own text (up to the PRESENT heading) rather
	// than the whole block, because founding-canvas step 6 (part 2, below)
	// legitimately names "sync-self" once on EVERY graph, consumer included,
	// to disclaim that no consumer equivalent exists yet.
	consumerTranslate := consumer[:strings.Index(consumer, "**PRESENT**")]
	nilTranslate := nilGraph[:strings.Index(nilGraph, "**PRESENT**")]
	for _, unwanted := range []string{"internal/selfspec", "sync-self"} {
		if strings.Contains(consumerTranslate, unwanted) {
			t.Errorf("consumer MEDIATION-LOOP TRANSLATE step must not mention %q (sync-self is exclusive to the self-hosting domain):\n%s", unwanted, consumerTranslate)
		}
		if strings.Contains(nilTranslate, unwanted) {
			t.Errorf("nil-graph MEDIATION-LOOP TRANSLATE step must not mention %q (nil defaults to non-self-hosting):\n%s", unwanted, nilTranslate)
		}
	}

	// both branches still name all six mediation-loop steps (the branching
	// must not damage the base structure the sentinel test also checks).
	for _, step := range []string{"ORIENT", "LOCATE", "CONFRONT", "TRANSLATE", "PRESENT", "LAND"} {
		if !strings.Contains(selfHosting, "**"+step+"**") {
			t.Errorf("self-hosting MEDIATION-LOOP text missing step %q", step)
		}
		if !strings.Contains(consumer, "**"+step+"**") {
			t.Errorf("consumer MEDIATION-LOOP text missing step %q", step)
		}
	}

	// consumer text is exactly the pre-RAC-C shape for every OTHER step —
	// only the TRANSLATE step's self-hosting sentence should ever differ
	// between the two renders. nilGraph must render byte-identically to the
	// explicit non-self-hosting graph (the SelfHosting zero value IS "not
	// self-hosting").
	if consumer != nilGraph {
		t.Errorf("RenderMediationLoopBlock(nil) must render identically to a graph with SelfHosting: false:\nnil:\n%s\nconsumer:\n%s", nilGraph, consumer)
	}
}

// TestRenderMediationLoopBlock_FoundingCanvasStep6MiddlePath is the
// acceptance test for task #347 (RAC-C part 2): founding-canvas step 6 must
// describe the middle path for a NEW domain (Requirement drafts authored as
// []ontology.Requirement literals in spec/requirements.go, JSON generated
// FROM the code for landing) on BOTH self-hosting and consumer crystals —
// this is general methodology text about founding domains, not gated by
// g.SelfHosting — and must explicitly disclaim that a sync-self-equivalent
// CLI for consumer domains is NOT YET IMPLEMENTED, never promising tooling
// that does not exist.
func TestRenderMediationLoopBlock_FoundingCanvasStep6MiddlePath(t *testing.T) {
	t.Parallel()

	for _, g := range []*ontology.Graph{{SelfHosting: true}, {SelfHosting: false}, nil} {
		inner := RenderMediationLoopBlock(g)
		if !strings.Contains(inner, "spec/requirements.go") {
			t.Errorf("founding-canvas step 6 missing the middle-path spec/requirements.go mention:\n%s", inner)
		}
		if !strings.Contains(inner, "middle path") {
			t.Errorf("founding-canvas step 6 missing the 'middle path' framing:\n%s", inner)
		}
		if !strings.Contains(inner, "NOT-YET-IMPLEMENTED") && !strings.Contains(inner, "not-yet-implemented") && !strings.Contains(inner, "NOT YET IMPLEMENTED") {
			t.Errorf("founding-canvas step 6 middle path must explicitly disclaim the consumer sync-self-equivalent as unimplemented:\n%s", inner)
		}
		if !strings.Contains(inner, "task #323") && !strings.Contains(inner, "life-domain") {
			t.Errorf("founding-canvas step 6 middle path must name the future life-domain intent:\n%s", inner)
		}
	}
}
