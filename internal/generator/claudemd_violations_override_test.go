package generator

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PHPCraftdream/HotamSpec/internal/invariants"
	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// reentrancyProbeName/reentrancyProbeCandidate/registerReentrancyProbeOnce:
// invariants.All is a PACKAGE-LEVEL, process-global registry with no
// Unregister — registering a probe invariant inside a test body would leak
// into every OTHER test in this package that calls invariants.AllViolations
// on an unrelated graph (observed directly: TestBuildLiveState_TodayIsInjectable
// failed because this probe's zero-Canon entry started showing up as a
// STRUCTURE-priority top-action signal for a completely different fixture
// graph, since invariants.All.All() has no test-scoping). registerReentrancyProbeOnce
// registers the probe EXACTLY ONCE for the whole test binary (sync.Once), with
// a real, non-nil Canon (methodology.Domain, satisfying
// TestRegisteredInvariantsHaveCanon-equivalent expectations for this package's
// own byte-identical/fixture tests that iterate invariants.All.All()), and its
// Check function is a no-op for every graph EXCEPT the one specific pointer
// reentrancyProbeCandidate names for the CURRENT test — set/cleared per test
// via a mutex-guarded package variable, so parallel tests in this package
// never see each other's probe state.
var (
	reentrancyProbeOnce sync.Once
	reentrancyProbeMu   sync.Mutex
	reentrancyProbeFor  *ontology.Graph
	reentrancyProbeHit  bool
)

const reentrancyProbeName = "zz_render_reentrancy_probe"

func registerReentrancyProbeOnce() {
	reentrancyProbeOnce.Do(func() {
		invariants.All.MustRegister(reentrancyProbeName, invariants.Invariant{
			Name:  reentrancyProbeName,
			Canon: methodology.Domain,
			Check: func(candidate *ontology.Graph) []invariants.Violation {
				reentrancyProbeMu.Lock()
				defer reentrancyProbeMu.Unlock()
				if reentrancyProbeFor != nil && candidate == reentrancyProbeFor {
					reentrancyProbeHit = true
				}
				return nil
			},
		})
	})
}

// TestRenderClaudeMDFromTemplateWithViolations_NeverCallsPlainBuildLiveState
// is the regression test for a real bug caught during E4's implementation:
// renderBusinessContentWithViolations originally computed BuildLiveState/
// RenderDomainMapBlock (the plain, non-override variants, which call
// diagnose.DiagnoseSignals -> invariants.AllViolations(g) internally)
// UNCONDITIONALLY before branching on whether violations was non-nil — Go
// evaluates a function call immediately regardless of whether the result is
// later discarded, so supplying a ViolationsOverride did NOT actually skip
// the internal AllViolations(g) call it exists to avoid. When this render
// path is reached FROM INSIDE invariants.AllViolations's own post-process
// phase (check_domain_claude_md_current, wired from cmd/hotam), that stray
// call recurses back into AllViolations for the SAME graph, which is still
// mid-flight — unbounded same-process recursion, observed in practice as
// `go test ./cmd/hotam/...` hanging until its own goroutine-leak/deadlock
// detection killed the run (300s+ before this fix).
//
// This test proves the fix holds WITHOUT needing the full cmd/hotam wiring
// (which cannot be exercised from this package — see
// ViolationsOverride's own doc comment for why check_domain_claude_md_current
// itself must live in cmd/hotam): it registers a synthetic invariant whose
// Check function panics if called a SECOND time for the same graph pointer
// while a render (invoked with a non-nil ViolationsOverride) is in flight,
// then renders through RenderClaudeMDFromTemplateWithViolations with a
// supplied override and asserts (a) no panic occurred (proving
// invariants.AllViolations was never re-entered for g) and (b) the render
// completes well within a generous timeout (a deadlocked WaitGroup, the
// actual failure mode observed, would otherwise hang the test process).
func TestRenderClaudeMDFromTemplateWithViolations_NeverCallsPlainBuildLiveState(t *testing.T) {
	// NOT t.Parallel(): this test claims exclusive use of the shared
	// reentrancyProbeFor/reentrancyProbeHit package variables (guarded by
	// reentrancyProbeMu) for its duration — running in parallel with a
	// hypothetical future test doing the same would race on which graph
	// pointer the probe is watching for.
	registerReentrancyProbeOnce()

	g := &ontology.Graph{
		DomainDir: t.TempDir(),
		Requirements: []ontology.Requirement{
			{ID: "R-probe", Claim: "probe", Owner: "x", Status: ontology.StatusSETTLED, Enforcement: ontology.EnforcementPROSE, Enforceability: ontology.EnforceabilityENFORCEABLE},
		},
	}

	reentrancyProbeMu.Lock()
	reentrancyProbeFor = g
	reentrancyProbeHit = false
	reentrancyProbeMu.Unlock()
	t.Cleanup(func() {
		reentrancyProbeMu.Lock()
		reentrancyProbeFor = nil
		reentrancyProbeMu.Unlock()
	})

	override := &ViolationsOverride{For: g, Violations: []invariants.Violation{
		{Check: "zz_injected_violation", ID: g.DomainDir, Message: "INJECTED_MARKER_FOR_TEST"},
	}}

	done := make(chan string, 1)
	go func() {
		done <- RenderClaudeMDFromTemplateWithViolations(g, "probe-domain", t.TempDir(), 4200, nil, "2026-07-19", false, override, "")
	}()

	select {
	case rendered := <-done:
		reentrancyProbeMu.Lock()
		hit := reentrancyProbeHit
		reentrancyProbeMu.Unlock()
		if hit {
			// If RenderClaudeMDFromTemplateWithViolations's LIVE-STATE/
			// DOMAIN-MAP path calls invariants.AllViolations(g) again while
			// we are inside a render invoked WITH a ViolationsOverride, the
			// probe's Check func (registered above, run as part of that
			// stray AllViolations(g) call) would have observed candidate ==
			// g and set reentrancyProbeHit — exactly the recursion bug this
			// test guards against.
			t.Fatalf("RenderClaudeMDFromTemplateWithViolations re-entered invariants.AllViolations(g) for the SAME graph despite a non-nil ViolationsOverride — the exact recursion bug this test guards against")
		}
		if !strings.Contains(rendered, "INJECTED_MARKER_FOR_TEST") {
			t.Errorf("rendered crystal does not embed the supplied override's violation message — LIVE-STATE did not actually use the override")
		}
	case <-time.After(20 * time.Second):
		t.Fatal("RenderClaudeMDFromTemplateWithViolations did not return within 20s — almost certainly the same deadlock (AllViolations waiting on a WaitGroup that includes a goroutine blocked re-entering AllViolations) this test exists to catch")
	}
}
