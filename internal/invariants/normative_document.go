package invariants

import (
	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func checkNormativeDocumentValid(g *ontology.Graph) (violations []Violation) {
	if g == nil || g.Conformance == nil || len(g.Conformance.DocumentSections) == 0 {
		return nil
	}
	view, index, err := InvocationSourceIndex(g)
	if view != nil && view.InvocationState != g.InvocationState {
		defer func() {
			if closeErr := CloseInvocation(view); closeErr != nil {
				violations = append(violations, Violation{Check: "execution_session_close", ID: g.DomainDir, Message: closeErr.Error()})
			}
		}()
	}
	if err == nil {
		err = gate.ValidateNormativeDocument(view, index)
	}
	if err != nil {
		return []Violation{{Check: "check_normative_document_valid", ID: g.DomainDir, Message: err.Error()}}
	}
	return nil
}

var _ = All.MustRegister("check_normative_document_valid", Invariant{
	Name:  "check_normative_document_valid",
	Canon: methodology.Domain,
	Claim: "an authored normative document has stable unambiguous identities, complete translations, and explicit source and illustrative-case bindings without converting traceability into semantic proof",
	Rule:  "IF conformance.document_sections is declared, every section, block, text reference, clause and selected example MUST resolve uniquely; parent cycles, ambiguous order, stale or duplicate IDs, missing translations and inferred case scope MUST be rejected before publication",
	Why:   "a readable document must fail closed when authored structure or bindings are stale, while informative, recommended, qualified and evidence-only claims remain distinct from executable proof",
	Check: checkNormativeDocumentValid,
})
