package invariants

import (
	"sync"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func TestGraphInvocationStateIsLocalAndNestedCallsShareOnlyTheirView(t *testing.T) {
	callerGraph := &ontology.Graph{}
	var firstState any
	withGraphInvocation(callerGraph, func(firstView *ontology.Graph) []Violation {
		firstState = firstView.InvocationState
		if firstState == nil {
			t.Fatal("first invocation did not receive local state")
		}
		if callerGraph.InvocationState != nil {
			t.Fatal("invariant invocation mutated the caller's graph")
		}
		withGraphInvocation(firstView, func(nestedView *ontology.Graph) []Violation {
			if nestedView.InvocationState != firstState {
				t.Fatal("nested check did not share its parent invocation state")
			}
			return nil
		})
		withGraphInvocation(callerGraph, func(independentView *ontology.Graph) []Violation {
			if independentView.InvocationState == firstState {
				t.Fatal("independent run on the same caller graph reused another run's state")
			}
			return nil
		})
		return nil
	})
	if callerGraph.InvocationState != nil {
		t.Fatal("invocation state escaped into the caller's graph")
	}
}

func TestConcurrentGraphInvocationsOnSameCallerAreIsolated(t *testing.T) {
	callerGraph := &ontology.Graph{}
	states := make(chan any, 2)
	release := make(chan struct{})
	var workers sync.WaitGroup
	for i := 0; i < 2; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			withGraphInvocation(callerGraph, func(view *ontology.Graph) []Violation {
				states <- view.InvocationState
				<-release
				return nil
			})
		}()
	}
	first := <-states
	second := <-states
	if first == nil || second == nil || first == second {
		close(release)
		workers.Wait()
		t.Fatalf("overlapping invocations shared local state: first=%p second=%p", first, second)
	}
	close(release)
	workers.Wait()
	if callerGraph.InvocationState != nil {
		t.Fatal("concurrent invocations mutated the caller's graph")
	}
}
