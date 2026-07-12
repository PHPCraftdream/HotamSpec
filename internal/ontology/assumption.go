package ontology

const (
	AssumptionHOLDS      = "HOLDS"
	AssumptionUNCERTAIN  = "UNCERTAIN"
	AssumptionDEAD       = "DEAD"
	AssumptionIMPLEMENTS = "IMPLEMENTS"
)

var AssumptionStates = map[string]struct{}{
	AssumptionHOLDS:      {},
	AssumptionUNCERTAIN:  {},
	AssumptionDEAD:       {},
	AssumptionIMPLEMENTS: {},
}

type Assumption struct {
	ID           string   `json:"id"`
	Statement    string   `json:"statement"`
	Status       string   `json:"status"`
	Owner        string   `json:"owner"`
	MachineCheck *string  `json:"machine_check"`
	Signoff      *Signoff `json:"signoff"`
	CreatedAt    string   `json:"created_at"`
	DecidedAt    string   `json:"decided_at"`
	DeclOrder    int      `json:"decl_order"`
}
