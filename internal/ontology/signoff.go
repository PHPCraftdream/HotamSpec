package ontology

const (
	SignoffInstrumentPersonal = "personal"
	SignoffInstrumentDEL      = "DEL"
)

var SignoffInstruments = map[string]struct{}{
	SignoffInstrumentPersonal: {},
	SignoffInstrumentDEL:      {},
}

type Signoff struct {
	DecidedBy     string `json:"decided_by"`
	Date          string `json:"date"`
	Verbatim      string `json:"verbatim"`
	Instrument    string `json:"instrument"`
	ChosenVariant string `json:"chosen_variant"`
}
