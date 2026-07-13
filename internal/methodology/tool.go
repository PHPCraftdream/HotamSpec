package methodology

import "github.com/PHPCraftdream/HotamSpec/internal/registry"

// Status classifies whether a Tool entry names a working `hotam` CLI
// subcommand (Implemented) or is methodology-surface description only,
// with no Go implementation yet (Planned).
type Status string

const (
	// Implemented means Command is a real `hotam <command>` invocation
	// wired in cmd/hotam/main.go — running it does something.
	Implemented Status = "implemented"
	// Planned means the tool is documented methodology surface only;
	// no Go command exists for it yet. Command below is NOT a runnable
	// invocation — it is the historical tool name, kept for continuity
	// with the methodology. See ticket P1-6 for the future work of
	// either implementing these or removing them from the registry.
	Planned Status = "planned"
)

type Tool struct {
	Command string
	Canon   string
	Purpose string
	Status  Status
	Run     func(args []string) error
}

var Tools = registry.New[Tool]()
