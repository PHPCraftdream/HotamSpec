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
	// Claim is the one-sentence requirement-claim text projected into the
	// STRUCTURAL R-tool-<basename> requirement (id = "R-tool-" + Command with
	// underscores → hyphens) at render time. It is the sole source of the
	// tool-derived requirement claim text (R-tool-is-its-own-requirement).
	Claim string
	// Enforcer is the OPTIONAL historical enforcer-test name carried for
	// documentary continuity from the Python era. It is purely descriptive —
	// it has never resolved to an actually-running Go test, and an empty
	// string means "no historical enforcer name". Do not infer a stronger
	// contract than that.
	Enforcer string
	Run      func(args []string) error
}

var Tools = registry.New[Tool]()
