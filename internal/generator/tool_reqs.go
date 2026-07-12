package generator

import (
	"sort"
	"strings"
)

type ToolRequirement struct {
	ID           string
	Basename     string
	CanonSection string
	Claim        string
	Enforcer     string
}

var toolRequirementData = []ToolRequirement{
	{Basename: "apply_proposal", CanonSection: "Proposal", Claim: "mechanical writer for steward-approved JSON proposals.", Enforcer: ""},
	{Basename: "attention", CanonSection: "Attention", Claim: "the agent-agnostic CLI over the attention core.", Enforcer: ""},
	{Basename: "attention_hook", CanonSection: "Attention", Claim: "the Claude adapter: inject the attention list into context.", Enforcer: ""},
	{Basename: "audit_atomicity", CanonSection: "Invariants", Claim: "surfaces Requirements with compound claims and check_* functions with compound conditions, both structural signals for decomposition.", Enforcer: "test_tool_audit_atomicity"},
	{Basename: "audit_tensions", CanonSection: "Loop", Claim: "the generative-audit tool: a deterministic, LLM-free shortlist of", Enforcer: "test_tool_audit_tensions"},
	{Basename: "claude_md_diff_watch", CanonSection: "Operator", Claim: "auto-injects the diff of CLAUDE.md since the operator's last turn into session context via a UserPromptSubmit hook.", Enforcer: ""},
	{Basename: "closure", CanonSection: "Closure", Claim: "per-action verify: did the proposal remove its diagnosis?", Enforcer: ""},
	{Basename: "confront", CanonSection: "Loop", Claim: "the CONFRONT step's tool: ranks a candidate claim's lexical overlap against SETTLED reality and REJECTED history before anything is written.", Enforcer: "test_tool_confront"},
	{Basename: "context", CanonSection: "Context", Claim: "the operator's working-context measurement (reader + CLI dispatcher).", Enforcer: "test_tool_context"},
	{Basename: "context_producer", CanonSection: "Context", Claim: "the producer half of the context cipher, writing a runtime context.json snapshot.", Enforcer: ""},
	{Basename: "create_agent", CanonSection: "Agent", Claim: "scaffolds domains/<name>/agents/<name>/ as a self-contained sub-operator directory with its own CLAUDE.md, scope definition, tools/, agents/, and README.md.", Enforcer: "test_tool_create_agent"},
	{Basename: "create_axis", CanonSection: "Axis", Claim: "scaffolds a new Axis into the active domain's controlled-vocabulary", Enforcer: "test_tool_create_axis"},
	{Basename: "create_domain", CanonSection: "Domain", Claim: "scaffolds domains/<name>/ as a self-contained business domain with manifest.json, graph.json, tools/, agents/director/, docs/gen/, and CLAUDE.md.", Enforcer: "test_tool_create_domain"},
	{Basename: "create_entity_type", CanonSection: "Entity", Claim: "scaffolds an EntityType declaration into the active domain's graph via apply_proposal.", Enforcer: "test_tool_create_entity_type"},
	{Basename: "emit_cipher", CanonSection: "Operator", Claim: "emits the three-cipher pulse (top action / debt / context) directly from the active domain's graph.", Enforcer: ""},
	{Basename: "gate", CanonSection: "Closure", Claim: "T1 tiered LAND gate: select a targeted enforcer subset instead of the full suite.", Enforcer: "test_tool_gate"},
	{Basename: "gate_status", CanonSection: "Closure", Claim: "read the runtime land-log.jsonl and answer the commit-boundary question.", Enforcer: "test_tool_gate_status"},
	{Basename: "gen_spec", CanonSection: "Generator", Claim: "regenerates docs/gen/ from the executable model (methodology + graph), making drift structurally impossible.", Enforcer: ""},
	{Basename: "hotam_req", CanonSection: "Requirement", Claim: "CLI for browsing, searching, patching and contextualizing Requirements.", Enforcer: "test_tool_hotam_req"},
	{Basename: "invoke_agent", CanonSection: "Agent", Claim: "invokes a sub-agent by loading its CLAUDE.md as the operator-prompt and printing it to stdout.", Enforcer: "test_tool_invoke_agent"},
	{Basename: "land", CanonSection: "Closure", Claim: "single CLI entry point over gate/gate_status/closure.", Enforcer: "test_tool_land"},
	{Basename: "mark_revisit_evaluated", CanonSection: "Conflict", Claim: "record that a DECIDED conflict's revisit_marker was evaluated.", Enforcer: "test_tool_mark_revisit_evaluated"},
	{Basename: "review", CanonSection: "Closure", Claim: "single CLI entry point over the low-traffic review tools.", Enforcer: "test_tool_review"},
	{Basename: "setup_context_hook", CanonSection: "Context", Claim: "installs/removes the project-local hook that feeds the context producer.", Enforcer: "test_tool_setup_context_hook"},
	{Basename: "setup_hooks", CanonSection: "Operator", Claim: "generate the committable, portable project sensorium.", Enforcer: "test_tool_setup_hooks"},
	{Basename: "spawn_agent", CanonSection: "Agent", Claim: "composes a sub-agent's task prompt by prepending the agent's CLAUDE.md crystal, so the subagent boots from substrate (not from raw text).", Enforcer: "test_tool_spawn_agent"},
	{Basename: "spawn_log_isolation_status", CanonSection: "Agent", Claim: "reads the runtime spawn-log.jsonl and flags mutating agents recorded without worktree isolation.", Enforcer: "test_tool_spawn_log_isolation_status"},
	{Basename: "ticket_comment", CanonSection: "Ticket", Claim: "append a stamped comment to a ticket (and a History \"commented\" entry).", Enforcer: "test_tool_ticket_comment"},
	{Basename: "ticket_create", CanonSection: "Ticket", Claim: "create a new on-disk ticket (auto-id, initial status, first History entry).", Enforcer: "test_tool_ticket_create"},
	{Basename: "ticket_edit", CanonSection: "Ticket", Claim: "edit a ticket's title/body, snapshotting the prior text into History.", Enforcer: "test_tool_ticket_edit"},
	{Basename: "ticket_list", CanonSection: "Ticket", Claim: "list tickets, optionally filtered by status or assignee (read-only).", Enforcer: ""},
	{Basename: "ticket_move", CanonSection: "Ticket", Claim: "move a ticket to a new status (relocates the file + records the transition in History).", Enforcer: "test_tool_ticket_move"},
	{Basename: "ticket_show", CanonSection: "Ticket", Claim: "print one ticket's header, body, comments and full History (read-only).", Enforcer: ""},
	{Basename: "what_now", CanonSection: "Harness", Claim: "derives the prioritized next correct action from any graph state, making being-lost structurally impossible.", Enforcer: ""},
}

func ScanToolRequirements() []ToolRequirement {
	out := make([]ToolRequirement, len(toolRequirementData))
	for i, tr := range toolRequirementData {
		out[i] = ToolRequirement{
			ID:           "R-tool-" + strings.ReplaceAll(tr.Basename, "_", "-"),
			Basename:     tr.Basename,
			CanonSection: tr.CanonSection,
			Claim:        tr.Claim,
			Enforcer:     tr.Enforcer,
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
