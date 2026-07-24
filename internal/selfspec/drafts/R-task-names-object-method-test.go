//go:build ignore

// DRAFT — NOT LANDED (task #342, R5-generate-dont-lint). Ready-to-paste
// registry entry for R-task-names-object-method-test, the self-referential
// methodology requirement that turns an invisible task-composition convention
// ("a TaskList implementation task must name the object/method/test triple
// before start") into a typed anchored node under a named resolver, per the
// Generative law. Excluded from compilation (//go:build ignore) so it never
// enters the Requirements registry and never touches graph.json — landing it
// is a separate, resolver-approved step (same precedent as
// internal/selfspec/drafts/R-debt-ratchet-no-growth.go, task #340).
//
// WHY A GO LITERAL (not a ProposedRequirement JSON): RAC-B flipped Requirement
// authority on the self-hosting domain so the Go registry in internal/selfspec
// is canonical and apply-proposal/land REFUSE a Requirement edit here (RAC-B3).
// The sanctioned landing path is: paste this literal into a topic file under
// internal/selfspec/, then `hotam sync-self` (appends to graph.json + History),
// then `hotam gen-spec`. This draft stays //go:build ignore so it does none of
// that — the implementing session is forbidden from landing it per task #342's
// "НЕ реализуй ничего ... только анализ и отчёт".
//
// LANDING CHECKLIST (for the landing session, NOT this consult):
//   1. Remove the //go:build ignore line above.
//   2. Move this MustRegister block into a topic file under internal/selfspec/
//      (candidate: requirements_authoredspec.go, which already hosts
//      R-authored-spec-layer-progression — the sibling authoring-order
//      discipline this refines; or a new requirements_task.go for the
//      §Ticket/§Loop topic).
//   3. Delete this standalone draft file.
//   4. `hotam sync-self --domain domains/hotam-spec-self --today <YYYY-MM-DD>`
//      (review the dry-run diff first, then --confirm-hash).
//   5. `hotam gen-spec` + `go test ./...` (full suite, mandatory at wave
//      boundary per R-verify-closure-per-action).
//   6. DeclOrder 276 is the next-free slot after the current max (274) and the
//      not-yet-landed R-debt-ratchet-no-growth draft (275); bump only on
//      collision with a concurrent landing.
package selfspec

import "github.com/PHPCraftdream/HotamSpec/internal/ontology"

var _ = Requirements.MustRegister("R-task-names-object-method-test", ontology.Requirement{
	ID:    "R-task-names-object-method-test",
	Claim: "Every implementation task in this repository's task surface (an orchestration TaskList item, or a tickets/*.md work item) SHALL name, before work starts, the triple it will produce or change: the OBJECT (the Go type/struct or file:symbol to be touched), the METHOD (the function/method to add or change), and the TEST (the Test* that will prove the change holds). A task that cannot name the triple is a RESEARCH task, not an implementation task, and MUST be split or relabeled before execution; a research task is a valid outcome, but it MUST NOT silently carry an implementation task's authority.",
	Owner: "framework-reviewer",
	Status: "DRAFT",
	Why: "The Generative law (generated into the operator crystal's Role block) states that important-yet-invisible things become typed anchored nodes under a named resolver. A task-composition convention that currently lives only in orchestration prompts ('name the object/method/test triple before start') is exactly that — important (it prevents scope-ambiguous tasks that start coding before the verification path is known, the same class of failure R-authored-spec-layer-progression prevents for code AUTHORING order), yet invisible (no graph node, no check, no citation). Anchoring it as a Requirement turns the invisible convention into a visible, citable, resolver-owned discipline node, mirroring how R-authored-spec-layer-progression anchored the code-authoring layer order and R-debt-ratchet-no-growth anchored the debt-direction ratchet. It is honest-PROSE today: no check exists that parses the task surface and verifies each in_progress task names the triple — but the structural floor for such a check is present (the task surface is machine-readable when serialized to disk), so the requirement is ENFORCEABLE in principle, not INHERENTLY_PROSE. Refines R-authored-spec-layer-progression (same authoring-discipline family; that requirement governs the ORDER of authoring code within a domain, this one governs the COMPLETENESS of a task's plan before authoring starts).",
	Assumptions: []string{"A-bootstrap-self-applies"},
	Relations: []ontology.Relation{
		{Kind: "refines", Target: "R-authored-spec-layer-progression"},
	},
	Enforcement:    "PROSE",
	EnforcedBy:     []string{},
	Enforceability: "ENFORCEABLE",
	CreatedAt:      "2026-07-24",
	SourceRefs:     []string{},
	DeclOrder:      276,
})
