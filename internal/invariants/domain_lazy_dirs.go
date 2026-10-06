package invariants

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// checkDomainDirsLazyMaterialized enforces the lazy-materialization half of
// R-domain-owns-tools-and-agents on the filesystem: a domain's agents/ and
// tools/ directories materialize only when a real sub-agent / sub-tool is
// actually created — never as eager empty scaffolds. Absence is the correct
// state for a domain with no sub-tools/sub-agents and never fires.
func checkDomainDirsLazyMaterialized(g *ontology.Graph) []Violation {
	if g.DomainDir == "" {
		return nil
	}
	var out []Violation
	agentsDir := filepath.Join(g.DomainDir, "agents")
	if entries, err := os.ReadDir(agentsDir); err == nil && !hasRealSubAgent(agentsDir, entries) {
		out = append(out, Violation{
			Check: "check_domain_dirs_lazy_materialized",
			ID:    g.DomainDir,
			Message: "agents/ exists but carries no real sub-agent (a subdirectory with its own CLAUDE.md) — " +
				"an eager scaffold; R-domain-owns-tools-and-agents materializes it only when a real sub-agent is created",
		})
	}
	toolsDir := filepath.Join(g.DomainDir, "tools")
	if entries, err := os.ReadDir(toolsDir); err == nil && !hasRealFile(toolsDir, entries) {
		out = append(out, Violation{
			Check: "check_domain_dirs_lazy_materialized",
			ID:    g.DomainDir,
			Message: "tools/ exists but carries no real sub-tool file — an eager scaffold; " +
				"R-domain-owns-tools-and-agents materializes it only when a real sub-tool is created",
		})
	}
	return out
}

func hasRealSubAgent(agentsDir string, entries []os.DirEntry) bool {
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(agentsDir, e.Name(), "CLAUDE.md")); err == nil {
			return true
		}
	}
	return false
}

func hasRealFile(dir string, entries []os.DirEntry) bool {
	for _, e := range entries {
		if e.IsDir() {
			sub, err := os.ReadDir(filepath.Join(dir, e.Name()))
			if err == nil && hasRealFile(filepath.Join(dir, e.Name()), sub) {
				return true
			}
			continue
		}
		if !strings.HasPrefix(e.Name(), ".") {
			return true
		}
	}
	return false
}

var _ = All.MustRegister("check_domain_dirs_lazy_materialized", Invariant{
	Name:  "check_domain_dirs_lazy_materialized",
	Canon: methodology.Domain,
	Claim: "a domain's tools/ and agents/ directories exist only when a real sub-tool or sub-agent exists — never as eager scaffolds.",
	Rule:  "R-domain-owns-tools-and-agents: for the active domain directory (g.DomainDir), the agents/ directory MUST NOT exist unless it contains at least one real sub-agent (a subdirectory carrying its own CLAUDE.md crystal), and the tools/ directory MUST NOT exist unless it contains at least one real file (hidden dotfiles and empty subdirectories do not count). A domain with no sub-tools/sub-agents correctly carries NEITHER directory — absence never fires. Runs only against a self_hosting graph (frameworkScopedInvariantNames).",
	Why:   "task #113 reworded R-domain-owns-tools-and-agents from 'shall contain both, even if empty' to lazy materialization; the sibling check_agent_has_* checks are honest no-ops over the in-memory graph, so the filesystem half stayed unenforced — the requirement's own Why names this exact gap ('a future filesystem-aware check ... is theoretically buildable'). This check closes it from the filesystem side via the same g.DomainDir pattern check_enforced_by_resolvable established. An eagerly scaffolded empty agents/ or tools/ fires; absence never does. References: R-domain-owns-tools-and-agents, R-claude-md-consolidates-when-single-agent.",
	Check: checkDomainDirsLazyMaterialized,
})
