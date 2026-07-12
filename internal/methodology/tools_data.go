package methodology

func init() {
	Tools.MustRegister("apply_proposal", Tool{
		Command: "apply_proposal",
		Canon:   "§Proposal",
		Purpose: "Mechanical writer for steward-approved JSON proposals.",
		Run:     nil,
	})
	Tools.MustRegister("attention", Tool{
		Command: "attention",
		Canon:   "§Attention",
		Purpose: "The agent-agnostic CLI over the attention core.",
		Run:     nil,
	})
	Tools.MustRegister("attention_hook", Tool{
		Command: "attention_hook",
		Canon:   "§Attention",
		Purpose: "The Claude adapter: inject the attention list into context.",
		Run:     nil,
	})
	Tools.MustRegister("audit_atomicity", Tool{
		Command: "audit_atomicity",
		Canon:   "§Invariants",
		Purpose: "Surfaces Requirements with compound claims and check_* functions with compound conditions, both structural signals for decomposition.",
		Run:     nil,
	})
	Tools.MustRegister("audit_tensions", Tool{
		Command: "audit_tensions",
		Canon:   "§Loop",
		Purpose: "The generative-audit tool: a deterministic, LLM-free shortlist of latent-connector suspects.",
		Run:     nil,
	})
	Tools.MustRegister("claude_md_diff_watch", Tool{
		Command: "claude_md_diff_watch",
		Canon:   "§Operator",
		Purpose: "Auto-injects the diff of CLAUDE.md since the operator's last turn into session context via a UserPromptSubmit hook.",
		Run:     nil,
	})
	Tools.MustRegister("closure", Tool{
		Command: "closure",
		Canon:   "§Closure",
		Purpose: "Per-action verify: did the proposal remove its diagnosis?",
		Run:     nil,
	})
	Tools.MustRegister("confront", Tool{
		Command: "confront",
		Canon:   "§Loop",
		Purpose: "The CONFRONT step's tool: ranks a candidate claim's lexical overlap against SETTLED reality and REJECTED history before anything is written.",
		Run:     nil,
	})
	Tools.MustRegister("context", Tool{
		Command: "context",
		Canon:   "§Context",
		Purpose: "The operator's working-context measurement (reader + CLI dispatcher).",
		Run:     nil,
	})
	Tools.MustRegister("context_producer", Tool{
		Command: "context_producer",
		Canon:   "§Context",
		Purpose: "The producer half of the context cipher, writing spec/.runtime/context.json.",
		Run:     nil,
	})
	Tools.MustRegister("create_agent", Tool{
		Command: "create_agent",
		Canon:   "§Agent",
		Purpose: "Scaffolds spec/agents/<name>/ as a self-contained sub-operator directory with its own CLAUDE.md, scope.py, tools/, agents/, and README.md.",
		Run:     nil,
	})
	Tools.MustRegister("create_axis", Tool{
		Command: "create_axis",
		Canon:   "§Axis",
		Purpose: "Scaffolds a new Axis into the active domain's controlled-vocabulary.",
		Run:     nil,
	})
	Tools.MustRegister("create_domain", Tool{
		Command: "create_domain",
		Canon:   "§Domain",
		Purpose: "Scaffolds domains/<name>/ as a self-contained business domain with manifest.py, graph.py, tools/, agents/director/, docs/gen/, and CLAUDE.md.",
		Run:     nil,
	})
	Tools.MustRegister("create_entity_type", Tool{
		Command: "create_entity_type",
		Canon:   "§Entity",
		Purpose: "Scaffolds an EntityType declaration into the active domain's graph via apply_proposal.",
		Run:     nil,
	})
	Tools.MustRegister("emit_cipher", Tool{
		Command: "emit_cipher",
		Canon:   "§Operator",
		Purpose: "Emits the three-cipher pulse (top action / debt / context) directly from the active domain's graph.",
		Run:     nil,
	})
	Tools.MustRegister("gate", Tool{
		Command: "gate",
		Canon:   "§Closure",
		Purpose: "T1 tiered LAND gate: select a targeted enforcer subset instead of the full suite.",
		Run:     nil,
	})
	Tools.MustRegister("gate_status", Tool{
		Command: "gate_status",
		Canon:   "§Closure",
		Purpose: "Read spec/.runtime/land-log.jsonl and answer the commit-boundary question.",
		Run:     nil,
	})
	Tools.MustRegister("gen_spec", Tool{
		Command: "gen_spec",
		Canon:   "§Generator",
		Purpose: "Regenerates docs/gen/ from the executable model (docstrings + graph), making drift structurally impossible.",
		Run:     nil,
	})
	Tools.MustRegister("hotam_req", Tool{
		Command: "hotam_req",
		Canon:   "§Requirement",
		Purpose: "CLI for browsing, searching, patching and contextualizing Requirements.",
		Run:     nil,
	})
	Tools.MustRegister("invoke_agent", Tool{
		Command: "invoke_agent",
		Canon:   "§Agent",
		Purpose: "Invokes a sub-agent by loading its spec/agents/<name>/CLAUDE.md as the operator-prompt and printing it to stdout.",
		Run:     nil,
	})
	Tools.MustRegister("land", Tool{
		Command: "land",
		Canon:   "§Closure",
		Purpose: "Single CLI entry point over gate.py/gate_status.py/closure.py.",
		Run:     nil,
	})
	Tools.MustRegister("mark_revisit_evaluated", Tool{
		Command: "mark_revisit_evaluated",
		Canon:   "§Conflict",
		Purpose: "Record that a DECIDED conflict's revisit_marker was evaluated.",
		Run:     nil,
	})
	Tools.MustRegister("review", Tool{
		Command: "review",
		Canon:   "§Closure",
		Purpose: "Single CLI entry point over the low-traffic review tools.",
		Run:     nil,
	})
	Tools.MustRegister("setup_context_hook", Tool{
		Command: "setup_context_hook",
		Canon:   "§Context",
		Purpose: "Installs/removes the project-local hook that feeds tools/context_producer.py.",
		Run:     nil,
	})
	Tools.MustRegister("setup_hooks", Tool{
		Command: "setup_hooks",
		Canon:   "§Operator",
		Purpose: "Generate the committable, portable project sensorium.",
		Run:     nil,
	})
	Tools.MustRegister("spawn_agent", Tool{
		Command: "spawn_agent",
		Canon:   "§Agent",
		Purpose: "Composes a sub-agent's task prompt by prepending the agent's CLAUDE.md crystal, so the subagent boots from substrate (not from raw text).",
		Run:     nil,
	})
	Tools.MustRegister("spawn_log_isolation_status", Tool{
		Command: "spawn_log_isolation_status",
		Canon:   "§Agent",
		Purpose: "Reads spec/.runtime/spawn-log.jsonl and flags mutating agents recorded without worktree isolation.",
		Run:     nil,
	})
	Tools.MustRegister("ticket_comment", Tool{
		Command: "ticket_comment",
		Canon:   "§Ticket",
		Purpose: "Append a stamped comment to a ticket (and a History 'commented' entry).",
		Run:     nil,
	})
	Tools.MustRegister("ticket_create", Tool{
		Command: "ticket_create",
		Canon:   "§Ticket",
		Purpose: "Create a new on-disk ticket (auto-id, initial status, first History entry).",
		Run:     nil,
	})
	Tools.MustRegister("ticket_edit", Tool{
		Command: "ticket_edit",
		Canon:   "§Ticket",
		Purpose: "Edit a ticket's title/body, snapshotting the prior text into History.",
		Run:     nil,
	})
	Tools.MustRegister("ticket_list", Tool{
		Command: "ticket_list",
		Canon:   "§Ticket",
		Purpose: "List tickets, optionally filtered by status or assignee (read-only).",
		Run:     nil,
	})
	Tools.MustRegister("ticket_move", Tool{
		Command: "ticket_move",
		Canon:   "§Ticket",
		Purpose: "Move a ticket to a new status (relocates the file + records the transition in History).",
		Run:     nil,
	})
	Tools.MustRegister("ticket_show", Tool{
		Command: "ticket_show",
		Canon:   "§Ticket",
		Purpose: "Print one ticket's header, body, comments and full History (read-only).",
		Run:     nil,
	})
	Tools.MustRegister("what_now", Tool{
		Command: "what_now",
		Canon:   "§Harness",
		Purpose: "Derives the prioritized next correct action from any graph state, making being-lost structurally impossible.",
		Run:     nil,
	})
}
