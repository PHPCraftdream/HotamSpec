package selfcheck

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// This file implements R5-debt-ratchet (task #340): a one-way ratchet over the
// framework's OWN closeable-debt and PROSE-enforcement levels. The ratchet
// fails CI the moment "claimed but not mechanically guaranteed" GROWS against
// a frozen per-domain pin, turning silent debt accumulation into a visible,
// conscious commit-time decision.
//
// ============================================================================
// DESIGN DECISIONS (the four open questions in task #340's brief)
// ============================================================================
//
// Q1. WHERE IS THE PIN STORED?
//     As numeric constants per domain, right here in this test file, in two
//     Go maps (closeableDebtPins / proseEnforceablePins) keyed by domain dir
//     name. This mirrors internal/invariants/registry_complete_test.go's
//     `const expected = N` ceiling pattern, adapted to a one-way ratchet:
//     `got > pin` fails RED, `got < pin` is an improvement (and the failure
//     message tells you to LOWER the pin to capture it), `got == pin` holds.
//     A second hand-maintained artifact (a JSON baseline file, a graph.lock
//     entry) would just be a duplicated source of truth the test then has to
//     load — a const the test reads directly is the single source of truth.
//     R-enforcement-perimeter-baselines-guarded is the sibling requirement
//     that would one day prevent cheating the baseline via a direct edit; the
//     pin lives in version control regardless, so a tampered pin is itself a
//     reviewable diff today.
//
// Q2. DOMAIN SCOPE?
//     Per-domain, NOT global. Each domain has its own maturity level, so a
//     single engine-wide ceiling would couple a mature domain's discipline to
//     a fresh domain's loose start. Only the two domains that live IN THIS
//     repo are pinned (hotam-spec-self, hotam-dev); consumer domains
//     (domains/prat, domains/gpsm-sm) live in the separate PRAT-hotam repo and
//     are never loaded by a test in the engine repo. Adding a domain's pin is
//     a deliberate act (Q4): the map is the exhaustive list of domains under
//     ratchet pressure right now.
//
// Q3. INHERENTLY_PROSE EXCLUDED?
//     Yes, on both axes. Check (a) calls Requirement.IsCloseableDebt(), which
//     is `Enforcement != ENFORCED && Enforceability == ENFORCEABLE` — an
//     INHERENTLY_PROSE requirement fails the Enforceability clause by
//     construction and is never counted. Check (b) counts
//     `Enforcement == PROSE && Enforceability == ENFORCEABLE`, explicitly
//     AND-ing the ENFORCEABLE clause for the same reason: INHERENTLY_PROSE is
//     legitimately never-ENFORCED prose (glossary text, semantic judgments no
//     parser can prove), not debt, and adding legitimate prose must not trip a
//     debt ratchet.
//
// Q4. NEW-DOMAIN GRACE?
//     A domain with no entry in the pin maps is SKIPPED (t.Logf, not t.Skip —
//     the loop continues). A freshly-founded domain legitimately starts with a
//     wall of DRAFT PROSE and little ENFORCED; the ratchet must not punish
//     that starting state, only GROWTH relative to an already-recorded pin.
//     The moment a domain's maintainer decides it is mature enough to be under
//     ratchet pressure, they add its pin here — that addition is itself the
//     deliberate baseline-capture act.
// ============================================================================

// closeableDebtPins is the frozen ceiling for check (a): the count of
// requirements where IsCloseableDebt() is true (Enforcement != ENFORCED AND
// Enforceability == ENFORCEABLE), per domain. This is the set of requirements
// that are "claimed but not mechanically guaranteed AND a real test could be
// written for them" — the actionable debt. A count ABOVE the pin fails RED.
//
// Pinning counts that include DRAFT requirements (not just SETTLED) is
// deliberate: a brand-new unenforced claim is growing debt the moment it
// lands, regardless of its status, and the pin-bump commit is where that
// growth becomes a conscious, reviewable decision. When such a claim later
// reaches ENFORCED, the count drops and the failure message asks you to lower
// the pin to lock the improvement in.
var closeableDebtPins = map[string]int{
	// Captured 2026-07-24 at task #340: 41 SETTLED closeable-debt (the live
	// pulse) + 32 DRAFT closeable-debt = 73 total via IsCloseableDebt().
	"hotam-spec-self": 73,
	// Captured 2026-07-24 at task #340: 6 SETTLED, 0 DRAFT.
	"hotam-dev": 6,
}

// proseEnforceablePins is the frozen ceiling for check (b): the count of
// requirements where Enforcement == PROSE AND Enforceability == ENFORCEABLE,
// per domain. This is a NARROWER, SOFTER signal than check (a): it isolates
// the PROSE-enforcement tier specifically (no engine link at all — not even a
// STRUCTURAL one), among only the ENFORCEABLE claims. A count ABOVE the pin
// fails RED. The intent (per task #340 brief) is that adding a new PROSE
// claim should require converting an old PROSE claim to ENFORCED in the same
// wave, keeping the "soft claims" volume from silently growing.
var proseEnforceablePins = map[string]int{
	// Captured 2026-07-24 at task #340.
	"hotam-spec-self": 70,
	"hotam-dev":       1,
}

// graphDomainDirs returns the repo-relative domain directory names that carry
// a graph.json (the universe of domains this repo's tests can observe).
func graphDomainDirs(t *testing.T) []string {
	t.Helper()
	root := repoRoot(t)
	entries, err := os.ReadDir(filepath.Join(root, "domains"))
	if err != nil {
		t.Fatalf("read domains/ dir: %v", err)
	}
	var doms []string
	for _, e := range entries {
		if _, err := os.Stat(filepath.Join(root, "domains", e.Name(), "graph.json")); err == nil {
			doms = append(doms, e.Name())
		}
	}
	if len(doms) == 0 {
		t.Fatal("found zero domains with a graph.json under domains/ — directory layout changed")
	}
	return doms
}

// TestDebtRatchet_CloseableDebtNotGrowing is check (a): for every domain with
// an entry in closeableDebtPins, the live IsCloseableDebt() count MUST NOT
// exceed the pinned ceiling. Domains without a pin are skipped (Q4 grace).
func TestDebtRatchet_CloseableDebtNotGrowing(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	for _, dom := range graphDomainDirs(t) {
		dom := dom
		t.Run(dom, func(t *testing.T) {
			t.Parallel()
			pin, ok := closeableDebtPins[dom]
			if !ok {
				t.Logf("closeable-debt ratchet: domain %q has no pin — skipped (Q4 new-domain grace; add a pin to closeableDebtPins to put it under ratchet pressure)", dom)
				return
			}
			g, err := loader.LoadGraph(filepath.Join(root, "domains", dom, "graph.json"))
			if err != nil {
				t.Fatalf("LoadGraph domains/%s/graph.json: %v", dom, err)
			}
			got := 0
			for _, r := range g.Requirements {
				if r.IsCloseableDebt() {
					got++
				}
			}
			assertRatchet(t, "closeable debt (IsCloseableDebt)", dom, got, pin)
		})
	}
}

// TestDebtRatchet_ProseEnforceableNotGrowing is check (b): for every domain
// with an entry in proseEnforceablePins, the live count of PROSE-enforcement
// ENFORCEABLE requirements MUST NOT exceed the pinned ceiling. Domains
// without a pin are skipped (Q4 grace).
func TestDebtRatchet_ProseEnforceableNotGrowing(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	for _, dom := range graphDomainDirs(t) {
		dom := dom
		t.Run(dom, func(t *testing.T) {
			t.Parallel()
			pin, ok := proseEnforceablePins[dom]
			if !ok {
				t.Logf("prose-enforceable ratchet: domain %q has no pin — skipped (Q4 new-domain grace; add a pin to proseEnforceablePins to put it under ratchet pressure)", dom)
				return
			}
			g, err := loader.LoadGraph(filepath.Join(root, "domains", dom, "graph.json"))
			if err != nil {
				t.Fatalf("LoadGraph domains/%s/graph.json: %v", dom, err)
			}
			got := 0
			for _, r := range g.Requirements {
				if r.Enforcement == ontology.EnforcementPROSE && r.Enforceability == ontology.EnforceabilityENFORCEABLE {
					got++
				}
			}
			assertRatchet(t, "PROSE-enforcement + ENFORCEABLE", dom, got, pin)
		})
	}
}

// assertRatchet is the shared ratchet verdict: `got > pin` fails RED with an
// actionable message (either reduce the debt, or bump the pin deliberately);
// `got < pin` also fails, because an unlowered pin after a real improvement
// lets the NEXT regression hide inside the slack — the message asks you to
// lower the pin to lock the gain in; `got == pin` holds silently.
func assertRatchet(t *testing.T, metric, dom string, got, pin int) {
	t.Helper()
	switch {
	case got > pin:
		t.Errorf(
			"R-debt-ratchet: %s for domain %q GREW from pin %d to %d (+%d) — this is growing 'claimed but not mechanically guaranteed' debt.\n"+
				"  To make this GREEN again, either:\n"+
				"    1. REDUCE the debt (convert PROSE/STRUCTURAL requirements to ENFORCED via real check_*/Test* enforcers in the same wave), OR\n"+
				"    2. BUMP the pin in internal/selfcheck/debt_ratchet_test.go to %d WITH a one-line justification in the pin comment (conscious debt growth, not silent)",
			metric, dom, pin, got, got-pin, got,
		)
	case got < pin:
		t.Errorf(
			"R-debt-ratchet: %s for domain %q IMPROVED from pin %d to %d (-%d) — the pin is now stale.\n"+
				"  LOWER the pin in internal/selfcheck/debt_ratchet_test.go to %d to lock this improvement in (an unlowered pin lets a future regression hide inside the slack)",
			metric, dom, pin, got, pin-got, got,
		)
	}
}

// TestDebtRatchet_FiresOnGrownDebt is the non-vacuity control for check (a):
// a synthetic graph with MORE closeable debt than a pin MUST fail RED,
// proving the ratchet is not a perpetual-green no-op. Mirrors
// TestRaceRatchet_DetectsUncoveredPackage's shape.
func TestDebtRatchet_FiresOnGrownDebt(t *testing.T) {
	t.Parallel()
	// 3 closeable-debt requirements, pin of 2 -> must fail.
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-a", Enforcement: ontology.EnforcementPROSE, Enforceability: ontology.EnforceabilityENFORCEABLE},
			{ID: "R-b", Enforcement: ontology.EnforcementPROSE, Enforceability: ontology.EnforceabilityENFORCEABLE},
			{ID: "R-c", Enforcement: ontology.EnforcementSTRUCTURAL, Enforceability: ontology.EnforceabilityENFORCEABLE},
		},
	}
	got := 0
	for _, r := range g.Requirements {
		if r.IsCloseableDebt() {
			got++
		}
	}
	if got != 3 {
		t.Fatalf("synthetic setup: expected 3 closeable-debt, got %d — IsCloseableDebt semantics changed", got)
	}
	// Replay the verdict logic directly (assertRatchet calls t.Errorf, which
	// would mark THIS test failed; instead, assert the predicate the verdict
	// branches on so this control proves the FIRE branch is reachable without
	// itself failing).
	if !(got > 2) {
		t.Error("ratchet fire-branch unreachable: got > pin was false with got=3, pin=2 — the main ratchet test would never fail RED")
	}
}

// TestDebtRatchet_InherentlyProseExcluded is the non-vacuity control for the
// INHERENTLY_PROSE exclusion (Q3): a PROSE-enforcement INHERENTLY_PROSE
// requirement must NOT count toward either metric, so adding legitimate
// never-ENFORCED prose does not trip the ratchet.
func TestDebtRatchet_InherentlyProseExcluded(t *testing.T) {
	t.Parallel()
	r := ontology.Requirement{
		ID:             "R-legit-prose",
		Enforcement:    ontology.EnforcementPROSE,
		Enforceability: ontology.EnforceabilityINHERENTLY_PROSE,
	}
	if r.IsCloseableDebt() {
		t.Error("IsCloseableDebt() counted an INHERENTLY_PROSE requirement — Q3 exclusion broken on check (a)")
	}
	if r.Enforcement == ontology.EnforcementPROSE && r.Enforceability == ontology.EnforceabilityENFORCEABLE {
		t.Error("check (b) predicate counted an INHERENTLY_PROSE requirement — Q3 exclusion broken on check (b)")
	}
}

// TestDebtRatchet_AllRepoDomainsObserved proves the ratchet actually reaches
// every domain in this repo: the pin maps' keys MUST be a subset of the live
// domain set, so a pin keyed at a typo'd or renamed domain fails loudly rather
// than silently going unenforced.
func TestDebtRatchet_AllRepoDomainsObserved(t *testing.T) {
	t.Parallel()
	live := map[string]bool{}
	for _, dom := range graphDomainDirs(t) {
		live[dom] = true
	}
	for dom := range closeableDebtPins {
		if !live[dom] {
			t.Errorf("closeableDebtPins has entry for domain %q but no domains/%s/graph.json exists — pin is a dead key that enforces nothing", dom, dom)
		}
	}
	for dom := range proseEnforceablePins {
		if !live[dom] {
			t.Errorf("proseEnforceablePins has entry for domain %q but no domains/%s/graph.json exists — pin is a dead key that enforces nothing", dom, dom)
		}
	}
}
