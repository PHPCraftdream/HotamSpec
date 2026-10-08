package invariants

import (
	"errors"
	"sync"

	"github.com/PHPCraftdream/HotamSpec/internal/evidence"
	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

type graphInvocation struct {
	session          *gate.ExecutionSession
	evidenceMu       sync.Mutex
	evidenceSnapshot *gate.AtomExecutionSnapshot
	evidence         evidence.Report
	evidenceErr      error
}

func (invocation *graphInvocation) ExecutionSession() *gate.ExecutionSession {
	return invocation.session
}

// withGraphInvocation gives one invariant run a shallow graph view carrying
// its own execution/evidence snapshot state. The caller's Graph is never
// mutated or serialized with this runtime-only state; independent calls always
// get new state, while nested helpers passed the view share its explicit owner.
func withGraphInvocation(g *ontology.Graph, run func(*ontology.Graph) []Violation) (violations []Violation) {
	if g == nil {
		return run(nil)
	}
	view := *g
	invocation, ok := g.InvocationState.(*graphInvocation)
	if !ok {
		invocation = &graphInvocation{session: gate.NewExecutionSession()}
		defer func() {
			if err := CloseInvocation(&view); err != nil {
				violations = append(violations, Violation{Check: "execution_session_close", ID: g.DomainDir, Message: err.Error()})
			}
		}()
	}
	view.InvocationState = invocation
	return run(&view)
}

// InvocationExecutionSnapshot requests a live, content/profile-current capture
// from this invocation's explicit owner. All declared test/case references share
// its package results and source AST; a closed owner cannot replay cached PASS.
// The caller's persistent Graph is never mutated.
func InvocationExecutionSnapshot(g *ontology.Graph) (*ontology.Graph, *gate.AtomExecutionSnapshot, error) {
	if g == nil {
		return nil, nil, nil
	}
	view := *g
	invocation, ok := g.InvocationState.(*graphInvocation)
	if !ok {
		invocation = &graphInvocation{session: gate.NewExecutionSession()}
	}
	view.InvocationState = invocation
	snapshot, err := invocation.session.CollectAtomExecutionSnapshotForPackages(&view, nil)
	return &view, snapshot, err
}

// collectInvocationEvidence validates ownership and current execution on every
// request. Evidence is shared only while the actual acquired snapshot identity
// remains unchanged; projecting an already acquired snapshot stays pure.
func collectInvocationEvidence(g *ontology.Graph) (evidence.Report, error) {
	view, snapshot, err := InvocationExecutionSnapshot(g)
	if err != nil || view == nil || snapshot == nil {
		return evidence.Report{}, err
	}
	invocation := view.InvocationState.(*graphInvocation)
	invocation.evidenceMu.Lock()
	defer invocation.evidenceMu.Unlock()
	if invocation.evidenceSnapshot != snapshot {
		invocation.evidence, invocation.evidenceErr = evidence.CollectFromSnapshot(view, snapshot)
		invocation.evidenceSnapshot = snapshot
	}
	return invocation.evidence, invocation.evidenceErr
}

// InvocationEvidenceSnapshot returns the evidence projection of the current
// graph invocation's one execution snapshot. The state dies with its shallow
// view; it is never a persisted or cross-command verdict cache.
func InvocationEvidenceSnapshot(g *ontology.Graph) (report evidence.Report, err error) {
	if g == nil {
		return evidence.Report{}, nil
	}
	view := *g
	if _, ok := g.InvocationState.(*graphInvocation); !ok {
		view.InvocationState = &graphInvocation{session: gate.NewExecutionSession()}
		defer func() { err = errors.Join(err, CloseInvocation(&view)) }()
	}
	return collectInvocationEvidence(&view)
}

// CloseInvocation closes the owner carried by an invocation view. It must run
// after all consumers of the shared source/execution/evidence snapshot finish.
func CloseInvocation(g *ontology.Graph) error {
	if g == nil {
		return nil
	}
	if invocation, ok := g.InvocationState.(*graphInvocation); ok {
		err := invocation.session.Close()
		invocation.evidenceMu.Lock()
		invocation.evidenceSnapshot = nil
		invocation.evidence = evidence.Report{}
		invocation.evidenceErr = nil
		invocation.evidenceMu.Unlock()
		return err
	}
	return nil
}

func invocationSession(g *ontology.Graph) (*ontology.Graph, *gate.ExecutionSession, bool) {
	view := *g
	invocation, ok := g.InvocationState.(*graphInvocation)
	if !ok {
		invocation = &graphInvocation{session: gate.NewExecutionSession()}
	}
	view.InvocationState = invocation
	return &view, invocation.session, !ok
}

// InvocationSourceIndex acquires the shared AST without executing tests. The
// returned graph view owns the session when the input did not already carry one.
func InvocationSourceIndex(g *ontology.Graph) (*ontology.Graph, *gate.AtomSourceIndex, error) {
	if g == nil {
		return nil, nil, nil
	}
	view, session, _ := invocationSession(g)
	index, err := session.SourceIndexForGraph(view)
	return view, index, err
}
