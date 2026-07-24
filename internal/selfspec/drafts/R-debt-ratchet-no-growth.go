//go:build ignore

// DRAFT — NOT LANDED (task #340, R5-debt-ratchet). This file is the
// ready-to-paste registry entry for R-debt-ratchet-no-growth, the
// self-referential methodology requirement that formalizes the very ratchet
// implemented in internal/selfcheck/debt_ratchet_test.go as a SETTLED graph
// node. It is excluded from compilation (//go:build ignore) so it never
// enters the Requirements registry and therefore never touches graph.json —
// landing it is a separate, resolver-approved step.
//
// WHY A GO LITERAL (not a ProposedRequirement JSON), and WHY NOT SYNC-SELFD:
// RAC-B (task #346/#348-351) flipped Requirement authority on the
// self-hosting domain so that the Go registry in internal/selfspec is the
// canonical authoring surface and apply-proposal/land REFUSE a Requirement
// edit on this domain (RAC-B3). The sanctioned landing path is therefore:
// paste this literal into internal/selfspec/requirements_enforcement.go,
// then run `hotam sync-self` (which appends the node to graph.json and
// records a History entry), then `hotam gen-spec`. That sequence LANDS the
// requirement — which task #340 explicitly forbids the implementing session
// from doing ("НЕ приземляй его сам, оставь как drafted"). So this file
// stays a draft: the literal in the exact Go form RAC-B mandates, with
// build:ignore keeping it out of the live registry, ready for a
// resolver-approved landing wave.
//
// LANDING CHECKLIST (for the landing session, NOT this one):
//   1. Remove the //go:build ignore line above.
//   2. Move this file's MustRegister block into
//      internal/selfspec/requirements_enforcement.go (topic: Enforcement and
//      Check Discipline — the home of R-atomicity-ratchet-no-growth, the
//      sibling ratchet requirement).
//   3. Delete this standalone draft file (it has no other content).
//   4. `hotam sync-self --domain domains/hotam-spec-self --today <YYYY-MM-DD>`
//      (review the dry-run diff first, then --confirm-hash).
//   5. `hotam gen-spec` + `go test ./...` (full suite, mandatory at the wave
//      boundary per R-verify-closure-per-action).
//   6. Bump DeclOrder only if 275 collides with a concurrent landing — it is
//      the next-free slot after the current max (274) at task #340's capture.
package selfspec

import "github.com/PHPCraftdream/HotamSpec/internal/ontology"

var _ = Requirements.MustRegister("R-debt-ratchet-no-growth", ontology.Requirement{
	ID:    "R-debt-ratchet-no-growth",
	Claim: "The count of closeable-debt requirements (IsCloseableDebt: Enforcement != ENFORCED AND Enforceability == ENFORCEABLE) and the count of PROSE-enforcement ENFORCEABLE requirements SHALL NEVER grow beyond a frozen per-domain pin, failing CI RED the instant either grows; INHERENTLY_PROSE requirements are excluded from both counts (legitimate never-ENFORCED prose, not debt).",
	Owner: "framework-reviewer",
	Status: "DRAFT",
	Why: "R-enforceability-kind-declared split real closeable debt from permanent (INHERENTLY_PROSE) discipline so the debt metric could converge rather than count inherent-prose rules forever — but convergence is a DIRECTION, and a direction with no mechanical guard can silently reverse. A new PROSE/STRUCTURAL-but-ENFORCEABLE requirement lands as +1 debt the moment it is added, regardless of status; without a ratchet that growth is invisible until a human happens to read the pulse. This requirement is the one-way ratchet that makes growth a conscious, commit-time decision: the pin is a numeric constant in internal/selfcheck/debt_ratchet_test.go (closeableDebtPins / proseEnforceablePins, keyed by domain dir name — the same `const expected = N` ceiling pattern internal/invariants/registry_complete_test.go already uses for the invariant count), and the test fails RED on growth, asking the author to either reduce the debt (convert a PROSE/STRUCTURAL requirement to ENFORCED via a real check_*/Test* in the same wave) or bump the pin WITH a one-line justification in the pin comment. Two distinct signals: (a) total closeable debt (the actionable 'claimed but not mechanically guaranteed' set, status-agnostic — DRAFT claims count, because a brand-new unenforced claim is growing debt the moment it lands), and (b) the narrower PROSE-enforcement ENFORCEABLE subset (the softest tier — no engine link at all — so adding a new PROSE claim requires converting an old one to ENFORCED in the same wave, keeping the 'soft claims' volume from silently growing). INHERENTLY_PROSE is excluded from both by construction (IsCloseableDebt ANDs Enforceability == ENFORCEABLE; check (b) ANDs it explicitly) — adding legitimate glossary/semantic-judgment prose must not trip a debt ratchet. Per-domain pins (not one global ceiling), because each domain has its own maturity level and a single engine-wide ceiling would couple a mature domain's discipline to a fresh domain's loose start; only the two domains living in THIS repo are pinned (hotam-spec-self, hotam-dev), and a domain with no pin entry is skipped (new-domain grace: a freshly-founded domain legitimately starts with a wall of DRAFT PROSE and little ENFORCED, and the ratchet must not punish that starting state, only GROWTH relative to an already-recorded pin). Sibling of R-atomicity-ratchet-no-growth (the same growth-direction pattern, applied to compound-claim atomicity rather than enforcement debt) and R-core-periphery-import-ratchet (the same pattern, applied to the import arrow); refines R-enforceability-kind-declared because it is the convergence mechanism that enforceability-kind split exists to enable.",
	Assumptions: []string{"A-bootstrap-self-applies"},
	Relations: []ontology.Relation{
		{Kind: "refines", Target: "R-enforceability-kind-declared"},
	},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"TestDebtRatchet_CloseableDebtNotGrowing", "TestDebtRatchet_ProseEnforceableNotGrowing"},
	Enforceability: "ENFORCEABLE",
	CreatedAt:      "2026-07-24",
	SourceRefs:     []string{},
	DeclOrder:      275,
})
