package methodology

import "github.com/PHPCraftdream/HotamSpec/internal/registry"

// Status classifies whether a Tool entry names a working `hotam` CLI
// subcommand or is merely a carried-over description from the pre-port
// Python methodology with no Go implementation yet.
type Status string

const (
	// Ported means Command is a real `hotam <command>` invocation
	// wired in cmd/hotam/main.go — running it does something.
	Ported Status = "ported"
	// Declared means the tool is documented methodology surface only;
	// no Go command exists for it yet. Command below is NOT a runnable
	// invocation — it is the historical tool name, kept for continuity
	// with the pre-port methodology. See ticket P1-6 for the future
	// work of either porting these or removing them from the registry.
	Declared Status = "declared"
)

type Tool struct {
	Command string
	Canon   string
	Purpose string
	Status  Status
	Run     func(args []string) error
}

var Tools = registry.New[Tool]()
