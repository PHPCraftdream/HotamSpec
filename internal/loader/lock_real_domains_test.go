package loader

import "testing"

// realDomains are the on-disk domain directories whose source graph.json MUST be
// governed by the proposal writer (R-no-hand-edit-graph). Each carries a sibling
// graph.lock whose sha256 pin is produced/verified by this package (lock.go); a
// pin that no longer matches the current graph.json is the structural signal that
// the graph was edited outside hotam apply-proposal / hotam land.
//
// Paths are relative to the internal/loader package directory (the test working
// dir), matching the convention used by loader_test.go's fixturePath.
var realDomains = []string{
	"../../domains/hotam-spec-self",
	"../../domains/hotam-dev",
}

// TestNoHandEditGraph_RealDomainLocksPinCurrentGraph enforces R-no-hand-edit-graph:
// for every real domain, the sha256 recorded in graph.lock MUST equal the sha256
// of the current graph.json. A hand-edit that bypassed hotam land / apply-proposal
// (which always writes graph.json + graph.lock together via WriteGraph/WriteLock)
// leaves the lock stale, which VerifyLock detects here. This test fails the moment
// any domain's graph.json is mutated without refreshing its sibling lock.
func TestNoHandEditGraph_RealDomainLocksPinCurrentGraph(t *testing.T) {
	t.Parallel()
	for _, dir := range realDomains {
		dir := dir
		t.Run(dir, func(t *testing.T) {
			t.Parallel()
			graphPath := dir + "/graph.json"
			ok, err := VerifyLock(graphPath)
			if err != nil {
				t.Fatalf("VerifyLock(%s): %v", graphPath, err)
			}
			if !ok {
				t.Errorf("graph.lock pin does not match graph.json for %s -- "+
					"the graph was changed outside hotam apply-proposal/land (R-no-hand-edit-graph)", dir)
			}
		})
	}
}
