package methodology

import "github.com/PHPCraftdream/HotamSpec/internal/registry"

type SectionKind string

const (
	ONTOLOGY   SectionKind = "ONTOLOGY"
	DISCIPLINE SectionKind = "DISCIPLINE"
	PROCESS    SectionKind = "PROCESS"
	PLUMBING   SectionKind = "PLUMBING"
)

type Section struct {
	Slug      string
	Kind      SectionKind
	Canon     string
	Narrative string
	Why       string
}

var Sections = registry.New[Section]()
