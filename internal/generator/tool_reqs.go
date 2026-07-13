package generator

import (
	"sort"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
)

// ToolRequirement is the render-time projection of a methodology.Tools registry
// entry into a STRUCTURAL R-tool-<basename> requirement. Its values come solely
// from methodology.Tool's Claim/Enforcer/Canon fields — there is no second
// hand-maintained table (R-tool-is-its-own-requirement).
type ToolRequirement struct {
	ID           string
	Basename     string
	CanonSection string
	Claim        string
	Enforcer     string
}

// ScanToolRequirements projects the methodology.Tools registry into one
// ToolRequirement per tool, making methodology.Tools the single source of truth
// for the tool SET (R-tool-is-its-own-requirement). ID = "R-tool-" + Command
// with underscores → hyphens; CanonSection = Tool.Canon with the leading §
// stripped. Output is sorted by basename.
func ScanToolRequirements() []ToolRequirement {
	tools := methodology.Tools.All()
	out := make([]ToolRequirement, len(tools))
	for i, t := range tools {
		basename := t.Command
		out[i] = ToolRequirement{
			ID:           "R-tool-" + strings.ReplaceAll(basename, "_", "-"),
			Basename:     basename,
			CanonSection: strings.TrimPrefix(t.Canon, "§"),
			Claim:        t.Claim,
			Enforcer:     t.Enforcer,
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Basename < out[j].Basename })
	return out
}

func BuildToolDerivedSection() string {
	toolReqs := ScanToolRequirements()
	lines := []string{}
	lines = append(lines, "## Tool-derived requirements")
	lines = append(lines, "")
	lines = append(lines, "Projected from the tool registry, one entry per tool whose first doc line matches `Canon: §<topic> — <claim>` (R-tool-is-its-own-requirement). Tools without a Go CLI port yet are tracked here as not-yet-enforced. The doc line IS the claim; the body IS the check; the test IS the enforcer. Deleting the tool deletes the R.")
	lines = append(lines, "")
	if len(toolReqs) == 0 {
		lines = append(lines, "_No tools carry a Canon: §... marker yet._")
		lines = append(lines, "")
	} else {
		for _, tr := range toolReqs {
			enforcerStr := "enforcer: (none)"
			if tr.Enforcer != "" {
				enforcerStr = "enforcer: `" + tr.Enforcer + "`"
			}
			lines = append(lines, "- **"+tr.ID+"** — *"+tr.Claim+"* [STRUCTURAL·tool · §"+tr.CanonSection+"] ["+enforcerStr+"]")
		}
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}
