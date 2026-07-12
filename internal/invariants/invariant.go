package invariants

import (
	"github.com/PHPCraftdream/HotamSpecGo/internal/methodology"
	"github.com/PHPCraftdream/HotamSpecGo/internal/ontology"
	"github.com/PHPCraftdream/HotamSpecGo/internal/registry"
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
