package generator

import "strings"

// repoMapFrameworkAndToolsContent holds the domain-independent "Framework
// body" and "Tools" sections of REPO-MAP.md — these scan spec/src/hotam_spec/
// and spec/tools/, which are the same regardless of which domain the doc is
// generated for.
var repoMapFrameworkAndToolsContent = strings.Join([]string{
	"### Repository Map",
	"",
	"**Framework body** (`spec/src/hotam_spec/`)",
	"",
	"- `spec/src/hotam_spec/assumption.py` — a claim with its OWN lifecycle (the root of context drift).",
	"- `spec/src/hotam_spec/attention.py` — the agent-agnostic registry of \"attention codes\".",
	"- `spec/src/hotam_spec/axis.py` — controlled vocabulary of tension dimensions.",
	"- `spec/src/hotam_spec/claude_md.py` — reusable CLAUDE.md sentinel-block operations.",
	"- `spec/src/hotam_spec/conflict.py` — the first-class connector NODE, a held property of the discipline (not its headline; J1, commit b2c58c8).",
	"- `spec/src/hotam_spec/doc_readers.py` — every generated doc names its reader (R-doc-names-reader).",
	"- `spec/src/hotam_spec/domain_resolution.py` — the single active-domain resolver (env → pin → alphabetical).",
	"- `spec/src/hotam_spec/enforcer_resolution.py` — shared enforcer-name -> pytest node-id resolution logic.",
	"- `spec/src/hotam_spec/entity.py` — domain-declared business concept with its own lifecycle.",
	"- `spec/src/hotam_spec/glossary.py` — the methodology's controlled vocabulary (framework-side).",
	"- `spec/src/hotam_spec/graph.py` — the tension graph store and its traversal helpers.",
	"- `spec/src/hotam_spec/invariants.py` — structural form of the tension graph (the check_* layer).",
	"- `spec/src/hotam_spec/invariants_table_engine.py` — declarative table engine for TABLE_DRIVEN check_*.",
	"- `spec/src/hotam_spec/lifecycle.py` — the generic state-machine value-type (framework keystone).",
	"- `spec/src/hotam_spec/node_schemas.py` — registry of every typed-anchor node kind in the framework.",
	"- `spec/src/hotam_spec/operator.py` — the acting facet of a Stakeholder (M20: NEW TYPE).",
	"- `spec/src/hotam_spec/process.py` — opt-in behavioral aspect (M12).",
	"- `spec/src/hotam_spec/project_paths.py` — project-root resolution for the consumer's data directory.",
	"- `spec/src/hotam_spec/proposal.py` — structured operator-→-steward change proposals.",
	"- `spec/src/hotam_spec/reflection.py` — the operator's P0 self-diagnosis conditions as named predicates.",
	"- `spec/src/hotam_spec/repo_paths.py` — centralized repository path roots.",
	"- `spec/src/hotam_spec/requirement.py` — a business requirement as a node in the tension graph.",
	"- `spec/src/hotam_spec/runtime_paths.py` — runtime-directory resolution for consumer-side ephemera.",
	"- `spec/src/hotam_spec/scope_projection.py` — an operator's sub-domain as a PROJECTION, not a copy (design B).",
	"- `spec/src/hotam_spec/signoff.py` — the frozen provenance record of a human steward decision.",
	"- `spec/src/hotam_spec/stakeholder.py` — who owns requirements and stewards conflicts.",
	"- `spec/src/hotam_spec/template_loader.py` — template loader via importlib.resources (PEP 391).",
	"- `spec/src/hotam_spec/text.py` — text helpers for crystal rendering (stdlib-only).",
	"",
	"**Tools** (`spec/tools/`)",
	"",
	"- `spec/tools/apply_proposal.py` — mechanical writer for steward-approved JSON proposals.  →  R-tool-apply-proposal",
	"- `spec/tools/attention.py` — the agent-agnostic CLI over the attention core.  →  R-tool-attention",
	"- `spec/tools/attention_hook.py` — the Claude adapter: inject the attention list into context.  →  R-tool-attention-hook",
	"- `spec/tools/audit_atomicity.py` — surfaces Requirements with compound claims and check_* functions with compound conditions, both structural signals for decomposition.  →  R-tool-audit-atomicity",
	"- `spec/tools/audit_tensions.py` — the generative-audit tool: a deterministic, LLM-free shortlist of  →  R-tool-audit-tensions",
	"- `spec/tools/claude_md_diff_watch.py` — auto-injects the diff of CLAUDE.md since the operator's last turn into session context via a UserPromptSubmit hook.  →  R-tool-claude-md-diff-watch",
	"- `spec/tools/closure.py` — per-action verify: did the proposal remove its diagnosis?  →  R-tool-closure",
	"- `spec/tools/confront.py` — the CONFRONT step's tool: ranks a candidate claim's lexical overlap against SETTLED reality and REJECTED history before anything is written.  →  R-tool-confront",
	"- `spec/tools/context.py` — the operator's working-context measurement (reader + CLI dispatcher).  →  R-tool-context",
	"- `spec/tools/context_producer.py` — the producer half of the context cipher, writing spec/.runtime/context.json.  →  R-tool-context-producer",
	"- `spec/tools/create_agent.py` — scaffolds spec/agents/<name>/ as a self-contained sub-operator directory with its own CLAUDE.md, scope.py, tools/, agents/, and README.md.  →  R-tool-create-agent",
	"- `spec/tools/create_axis.py` — scaffolds a new Axis into the active domain's controlled-vocabulary  →  R-tool-create-axis",
	"- `spec/tools/create_domain.py` — scaffolds domains/<name>/ as a self-contained business domain with manifest.py, graph.py, tools/, agents/director/, docs/gen/, and CLAUDE.md.  →  R-tool-create-domain",
	"- `spec/tools/create_entity_type.py` — scaffolds an EntityType declaration into the active domain's graph via apply_proposal.  →  R-tool-create-entity-type",
	"- `spec/tools/delegate.py` — Canon: §Ticket (sibling) -- file-based delegation tool (create / close / show / list).",
	"- `spec/tools/emit_cipher.py` — emits the three-cipher pulse (top action / debt / context) directly from the active domain's graph.  →  R-tool-emit-cipher",
	"- `spec/tools/gate.py` — T1 tiered LAND gate: select a targeted enforcer subset instead of the full suite.  →  R-tool-gate",
	"- `spec/tools/gate_status.py` — read spec/.runtime/land-log.jsonl and answer the commit-boundary question.  →  R-tool-gate-status",
	"- `spec/tools/gen_enforcer_map.py` — Build-time snapshot generator for the enforcer-name -> pytest node-id map.",
	"- `spec/tools/gen_spec.py` — regenerates docs/gen/ from the executable model (docstrings + graph), making drift structurally impossible.  →  R-tool-gen-spec",
	"- `spec/tools/hotam_req.py` — CLI for browsing, searching, patching and contextualizing Requirements.  →  R-tool-hotam-req",
	"- `spec/tools/invoke_agent.py` — invokes a sub-agent by loading its spec/agents/<name>/CLAUDE.md as the operator-prompt and printing it to stdout.  →  R-tool-invoke-agent",
	"- `spec/tools/land.py` — single CLI entry point over gate.py/gate_status.py/closure.py.  →  R-tool-land",
	"- `spec/tools/mark_revisit_evaluated.py` — record that a DECIDED conflict's revisit_marker was evaluated.  →  R-tool-mark-revisit-evaluated",
	"- `spec/tools/review.py` — single CLI entry point over the low-traffic review tools.  →  R-tool-review",
	"- `spec/tools/setup_context_hook.py` — installs/removes the project-local hook that feeds tools/context_producer.py.  →  R-tool-setup-context-hook",
	"- `spec/tools/setup_hooks.py` — generate the committable, portable project sensorium.  →  R-tool-setup-hooks",
	"- `spec/tools/spawn_agent.py` — composes a sub-agent's task prompt by prepending the agent's CLAUDE.md crystal, so the subagent boots from substrate (not from raw text).  →  R-tool-spawn-agent",
	"- `spec/tools/spawn_log_isolation_status.py` — reads spec/.runtime/spawn-log.jsonl and flags mutating agents recorded without worktree isolation.  →  R-tool-spawn-log-isolation-status",
	"- `spec/tools/ticket_comment.py` — append a stamped comment to a ticket (and a History \"commented\" entry).  →  R-tool-ticket-comment",
	"- `spec/tools/ticket_create.py` — create a new on-disk ticket (auto-id, initial status, first History entry).  →  R-tool-ticket-create",
	"- `spec/tools/ticket_edit.py` — edit a ticket's title/body, snapshotting the prior text into History.  →  R-tool-ticket-edit",
	"- `spec/tools/ticket_list.py` — list tickets, optionally filtered by status or assignee (read-only).  →  R-tool-ticket-list",
	"- `spec/tools/ticket_move.py` — move a ticket to a new status (relocates the file + records the transition in History).  →  R-tool-ticket-move",
	"- `spec/tools/ticket_show.py` — print one ticket's header, body, comments and full History (read-only).  →  R-tool-ticket-show",
	"- `spec/tools/update_baseline.py` — Canon: §Invariants -- sanctioned baseline updater for protected hash baselines.",
	"- `spec/tools/what_now.py` — derives the prioritized next correct action from any graph state, making being-lost structurally impossible.  →  R-tool-what-now",
}, "\n")

// domainGraphPyRole returns the one-line role text for domains/<name>/graph.py
// in the "Domain content" section — mirrors Python's _docstring_role(), which
// reads the first line of that domain's graph.py module docstring (with any
// "Canon: §X — " prefix stripped). The Go port has no per-domain graph.py
// source file to introspect (domains carry graph.json instead), so the
// known domains' role text is captured here verbatim from the Python
// reference; any other domain falls back to the generic phrasing Python's
// create_domain.py scaffold uses for a fresh domain's graph.py docstring.
func domainGraphPyRole(domainName string) string {
	switch domainName {
	case "hotam-spec-self":
		return "Hotam-Spec modeling itself — the meta-domain (the framework's own design)."
	default:
		return "content graph of domain '" + domainName + "'."
	}
}

// GenDocEntry describes one file written into a domain's docs/gen/ during
// this run, for the REPO-MAP.md "Generated docs" section.
type GenDocEntry struct {
	Filename string
	Content  string
}

// mdTitle extracts the first H1/H2 heading from generated Markdown content as
// a short title — mirrors Python's _md_title(): take the first line starting
// with '#', drop everything up to (and including) the last " — ", then strip
// a trailing " (...)" suffix.
func mdTitle(content string) string {
	for _, line := range strings.Split(content, "\n") {
		stripped := strings.TrimSpace(strings.TrimLeft(line, "#"))
		if strings.HasPrefix(line, "#") && stripped != "" {
			parts := strings.Split(stripped, " — ")
			title := parts[len(parts)-1]
			if idx := strings.Index(title, " ("); idx >= 0 {
				title = title[:idx]
			}
			return strings.TrimSpace(title)
		}
	}
	return ""
}
