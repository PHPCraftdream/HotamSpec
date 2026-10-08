package invariants

import (
	"fmt"

	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func checkLanguageBundleComplete(g *ontology.Graph) (violations []Violation) {
	if g == nil || len(g.Languages) <= 1 {
		return nil
	}
	view, _, err := InvocationSourceIndex(g)
	if view != nil && view.InvocationState != g.InvocationState {
		defer func() {
			if closeErr := CloseInvocation(view); closeErr != nil {
				violations = append(violations, Violation{Check: "execution_session_close", ID: g.DomainDir, Message: closeErr.Error()})
			}
		}()
	}
	if err != nil {
		return []Violation{{
			Check:   "check_language_bundle_complete",
			ID:      g.DomainDir,
			Message: fmt.Sprintf("multilingual atom source is incomplete or invalid: %v", err),
		}}
	}
	return nil
}

var _ = All.MustRegister("check_language_bundle_complete", Invariant{
	Name:  "check_language_bundle_complete",
	Canon: methodology.Domain,
	Claim: "every explicitly multilingual domain has one valid authored atom phrase for every declared language, with no source-indexing errors or fallback.",
	Rule:  "IF Graph.Languages contains more than one locale, gate.NewAtomSourceIndexForGraph MUST parse the shared atom source index and reject unknown, duplicate, empty, malformed, or missing language blocks with source positions; legacy/single-language domains acquire no translation-tag obligation.",
	Why:   "Translation parity is a structural completeness signal, not semantic proof; it is triggered by the explicit multilingual field rather than older atom or discipline opt-ins.",
	Check: checkLanguageBundleComplete,
})
