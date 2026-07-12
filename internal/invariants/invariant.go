package invariants

import (
	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/registry"
)

type Violation struct {
	Check   string
	ID      string
	Message string
}

type Invariant struct {
	Name        string
	Canon       *methodology.Section
	Claim       string
	Rule        string
	Why         string
	Check       func(*ontology.Graph) []Violation
	IsDelegator bool
}

var All = registry.New[Invariant]()
