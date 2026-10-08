package invariants

import (
	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func graphExecutionSession(g *ontology.Graph) *gate.ExecutionSession {
	if g != nil {
		if invocation, ok := g.InvocationState.(*graphInvocation); ok {
			return invocation.session
		}
	}
	return nil
}

// A real invocation shares its content-exact source AST. Standalone structural
// checks remain genuinely fresh and never populate a process-global resolver cache.
func resolveSpecTestForGraph(g *ontology.Graph, specRoot, file, test string, recorderPaths ...string) (gate.SpecTestResult, error) {
	if session := graphExecutionSession(g); session != nil {
		return session.ResolveSpecTest(specRoot, file, test, recorderPaths...)
	}
	return gate.ResolveSpecTest(specRoot, file, test, recorderPaths...)
}

func resolveSpecSymbolForGraph(g *ontology.Graph, specRoot, file, symbol string) (gate.SpecSymbolResult, error) {
	if session := graphExecutionSession(g); session != nil {
		return session.ResolveSpecSymbol(specRoot, file, symbol)
	}
	return gate.ResolveSpecSymbol(specRoot, file, symbol)
}

func resolveSpecSymbolRangeForGraph(g *ontology.Graph, specRoot, file, symbol string) (gate.SymbolRange, bool, error) {
	if session := graphExecutionSession(g); session != nil {
		return session.ResolveSpecSymbolRange(specRoot, file, symbol)
	}
	return gate.ResolveSpecSymbolRange(specRoot, file, symbol)
}

func testFuncNamesForGraph(g *ontology.Graph, domainDir string) (map[string]struct{}, error) {
	if session := graphExecutionSession(g); session != nil {
		return session.TestFuncNames(domainDir)
	}
	return gate.TestFuncNames(domainDir)
}
