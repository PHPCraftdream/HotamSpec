package invariants

import (
	"sync"

	"github.com/PHPCraftdream/HotamSpec/internal/evidence"
	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

type graphInvocation struct {
	executionOnce sync.Once
	execution     *gate.AtomExecutionSnapshot
	executionErr  error
	evidenceOnce  sync.Once
	evidence      evidence.Report
	evidenceErr   error
}

// withGraphInvocation gives one invariant run a shallow graph view carrying
// its own execution/evidence snapshot state. The caller's Graph is never
// mutated or serialized with this runtime-only state; independent calls always
// get new state, while nested helpers passed the view share its sync.Once.
func withGraphInvocation(g *ontology.Graph, run func(*ontology.Graph) []Violation) []Violation {
	if g == nil {
		return run(nil)
	}
	view := *g
	invocation, ok := g.InvocationState.(*graphInvocation)
	if !ok {
		invocation = &graphInvocation{}
	}
	view.InvocationState = invocation
	return run(&view)
}

// InvocationExecutionSnapshot returns the one recording snapshot associated
// with this graph invocation. It covers declared atom packages; manual
// verified_by entries are proven through legacy real-execution paths and must
// not consult this snapshot. It uses a shallow Graph view so the caller's
// persistent graph is never mutated.
func InvocationExecutionSnapshot(g *ontology.Graph) (*ontology.Graph, *gate.AtomExecutionSnapshot, error) {
	if g == nil {
		return nil, nil, nil
	}
	view := *g
	invocation, ok := g.InvocationState.(*graphInvocation)
	if !ok {
		invocation = &graphInvocation{}
	}
	view.InvocationState = invocation
	invocation.executionOnce.Do(func() {
		invocation.execution, invocation.executionErr = gate.CollectAtomExecutionSnapshotForPackages(&view, view.SelfExecutingAtomPackages)
	})
	return &view, invocation.execution, invocation.executionErr
}

// collectInvocationEvidence projects the same raw package runs into the
// evidence report. Concurrent conformance/freshness checks share both stages
// of this one per-invocation snapshot; there is no package-global cache.
func collectInvocationEvidence(g *ontology.Graph) (evidence.Report, error) {
	view, snapshot, err := InvocationExecutionSnapshot(g)
	if err != nil || view == nil || snapshot == nil {
		return evidence.Report{}, err
	}
	invocation := view.InvocationState.(*graphInvocation)
	invocation.evidenceOnce.Do(func() {
		invocation.evidence, invocation.evidenceErr = evidence.CollectFromSnapshot(view, snapshot)
	})
	return invocation.evidence, invocation.evidenceErr
}

// InvocationEvidenceSnapshot returns the evidence projection of the current
// graph invocation's one execution snapshot. The state dies with its shallow
// view; it is never a persisted or cross-command verdict cache.
func InvocationEvidenceSnapshot(g *ontology.Graph) (evidence.Report, error) {
	return collectInvocationEvidence(g)
}
