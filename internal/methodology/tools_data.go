package methodology

func init() {
	// --- Implemented: real `hotam` CLI subcommands (cmd/hotam/main.go) ---

	Tools.MustRegister("gen_spec", Tool{
		Command: "gen_spec",
		Canon:   "§Generator",
		Purpose: "Usage: hotam gen-spec [--domain <path>]. Regenerates docs/gen/*.md + graph.json for a domain graph from the executable model (internal/generator + internal/ontology), making drift structurally impossible.",
		Status:  Implemented,
		Run:     nil,
	})
	Tools.MustRegister("what_now", Tool{
		Command: "what_now",
		Canon:   "§Harness",
		Purpose: "Usage: hotam what-now [--domain <path>] [--limit N]. Derives the prioritized next correct action from any graph state (internal/diagnose), making being-lost structurally impossible.",
		Status:  Implemented,
		Run:     nil,
	})
	Tools.MustRegister("apply_proposal", Tool{
		Command: "apply_proposal",
		Canon:   "§Proposal",
		Purpose: "Usage: hotam apply-proposal <proposal.json> --domain <path> --today YYYY-MM-DD. Mechanical writer for steward-approved JSON proposals (internal/proposal): consumes an approved Proposed* JSON and applies the change to a domain graph.json.",
		Status:  Implemented,
		Run:     nil,
	})
	Tools.MustRegister("gate", Tool{
		Command: "gate",
		Canon:   "§Closure",
		Purpose: "Usage: hotam gate <target-anchor> [--domain <path>]. T1 tiered LAND gate (internal/gate): selects a targeted enforcer subset for a target node instead of the full test suite.",
		Status:  Implemented,
		Run:     nil,
	})
	Tools.MustRegister("all_violations", Tool{
		Command: "all_violations",
		Canon:   "§Invariants",
		Purpose: "Usage: hotam all-violations [--domain <path>]. Prints all invariant violations for a domain graph (internal/invariants); exits 1 if any are found.",
		Status:  Implemented,
		Run:     nil,
	})
	Tools.MustRegister("req", Tool{
		Command: "req",
		Canon:   "§Requirement",
		Purpose: "Usage: hotam req <show|list|search|context|related> [args] [--domain <path>] [--json]. Compact agentic read interface over the domain graph (internal/query): answers 'what is R-x' / 'what touches R-x' without loading the full graph.json or a generated doc.",
		Status:  Implemented,
		Run:     nil,
	})
	Tools.MustRegister("due", Tool{
		Command: "due",
		Canon:   "§Requirement",
		Purpose: "Usage: hotam due [--domain <path>] [--today YYYY-MM-DD] [--json]. Advisory report of OVERDUE and NEVER-REVIEWED SETTLED requirements (internal/freshness); never gates, exit code always 0.",
		Status:  Implemented,
		Run:     nil,
	})
	Tools.MustRegister("status", Tool{
		Command: "status",
		Canon:   "§Operator",
		Purpose: "Usage: hotam status [--domain <path>] [--today YYYY-MM-DD] [--json]. Single-shot compact summary combining what-now's top action + debt (internal/diagnose), due's freshness counts (internal/freshness), and all-violations' violation count (internal/invariants), so an agent doesn't need to run all three separately. Never gates; exit code always 0.",
		Status:  Implemented,
		Run:     nil,
	})
	Tools.MustRegister("inspect", Tool{
		Command: "inspect",
		Canon:   "§Conflict",
		Purpose: "Usage: hotam inspect [--domain <path>] [--json] [--limit N] [--min-score N]. Advisory listing of semantic conflict candidates with evidence (internal/diagnose): shared-assumption clusters, entity-state suspects, lexical claim overlap, axis co-reference. --min-score (default 5) suppresses low-signal candidates; 0 shows all. Never gates; exit code always 0.",
		Status:  Implemented,
		Run:     nil,
	})
	Tools.MustRegister("confront", Tool{
		Command: "confront",
		Canon:   "§Loop",
		Purpose: "Usage: hotam confront <text> [--domain <path>] [--file <path>] [--json]. CONFRONT step of the mediation loop (internal/diagnose): checks a candidate claim for lexical overlap with SETTLED requirements (duplicate guard) and REJECTED history (anti-relitigation) before anything is written. <text> is a quoted positional; --file <path> reads a long draft. Reuses the inspect overlap engine. Never gates; exit code always 0.",
		Status:  Implemented,
		Run:     nil,
	})
	Tools.MustRegister("land", Tool{
		Command: "land",
		Canon:   "§Closure",
		Purpose: "Usage: hotam land <proposal.json> --domain <path> --today YYYY-MM-DD [--claude-md <path>]. Single CLI entry point over apply-proposal -> gen-spec -> all-violations (internal/proposal + internal/generator + internal/invariants).",
		Status:  Implemented,
		Run:     nil,
	})

	// --- Planned: methodology surface not yet implemented as a
	// Go command. Command below is the historical tool name, not a
	// runnable invocation. See P1-6. ---

	Tools.MustRegister("attention", Tool{
		Command: "attention",
		Canon:   "§Attention",
		Purpose: "Not implemented. Historically: the agent-agnostic CLI over the attention core.",
		Status:  Planned,
		Run:     nil,
	})
	Tools.MustRegister("attention_hook", Tool{
		Command: "attention_hook",
		Canon:   "§Attention",
		Purpose: "Not implemented. Historically: the Claude adapter that injects the attention list into context.",
		Status:  Planned,
		Run:     nil,
	})
	Tools.MustRegister("audit_atomicity", Tool{
		Command: "audit_atomicity",
		Canon:   "§Invariants",
		Purpose: "Not implemented. Historically: surfaces Requirements with compound claims and check_* functions with compound conditions, both structural signals for decomposition.",
		Status:  Planned,
		Run:     nil,
	})
	Tools.MustRegister("audit_tensions", Tool{
		Command: "audit_tensions",
		Canon:   "§Loop",
		Purpose: "Not implemented. Historically: the generative-audit tool, a deterministic, LLM-free shortlist of latent-connector suspects.",
		Status:  Planned,
		Run:     nil,
	})
	Tools.MustRegister("claude_md_diff_watch", Tool{
		Command: "claude_md_diff_watch",
		Canon:   "§Operator",
		Purpose: "Not implemented. Historically: auto-injects the diff of CLAUDE.md since the operator's last turn into session context via a UserPromptSubmit hook.",
		Status:  Planned,
		Run:     nil,
	})
	Tools.MustRegister("closure", Tool{
		Command: "closure",
		Canon:   "§Closure",
		Purpose: "Not implemented. Historically: per-action verify — did the proposal remove its diagnosis?",
		Status:  Planned,
		Run:     nil,
	})
	Tools.MustRegister("context", Tool{
		Command: "context",
		Canon:   "§Context",
		Purpose: "Not implemented. Historically: the operator's working-context measurement (reader + CLI dispatcher).",
		Status:  Planned,
		Run:     nil,
	})
	Tools.MustRegister("context_producer", Tool{
		Command: "context_producer",
		Canon:   "§Context",
		Purpose: "Not implemented. Historically: the producer half of the context cipher, writing a runtime context snapshot.",
		Status:  Planned,
		Run:     nil,
	})
	Tools.MustRegister("create_agent", Tool{
		Command: "create_agent",
		Canon:   "§Agent",
		Purpose: "Not implemented. Historically: scaffolds domains/<domain>/agents/<name>/ as a self-contained sub-operator directory with its own CLAUDE.md, scope, tools/, agents/, and README.md.",
		Status:  Planned,
		Run:     nil,
	})
	Tools.MustRegister("create_axis", Tool{
		Command: "create_axis",
		Canon:   "§Axis",
		Purpose: "Not implemented. Historically: scaffolds a new Axis into the active domain's controlled-vocabulary.",
		Status:  Planned,
		Run:     nil,
	})
	Tools.MustRegister("create_domain", Tool{
		Command: "create_domain",
		Canon:   "§Domain",
		Purpose: "Not implemented. Historically: scaffolds domains/<name>/ as a self-contained business domain with a manifest, graph.json, tools/, agents/director/, docs/gen/, and CLAUDE.md.",
		Status:  Planned,
		Run:     nil,
	})
	Tools.MustRegister("create_entity_type", Tool{
		Command: "create_entity_type",
		Canon:   "§Entity",
		Purpose: "Not implemented. Historically: scaffolds an EntityType declaration into the active domain's graph via apply-proposal.",
		Status:  Planned,
		Run:     nil,
	})
	Tools.MustRegister("emit_cipher", Tool{
		Command: "emit_cipher",
		Canon:   "§Operator",
		Purpose: "Not implemented. Historically: emits the three-cipher pulse (top action / debt / context) directly from the active domain's graph.",
		Status:  Planned,
		Run:     nil,
	})
	Tools.MustRegister("gate_status", Tool{
		Command: "gate_status",
		Canon:   "§Closure",
		Purpose: "Not implemented. Historically: reads the runtime land-log and answers the commit-boundary question.",
		Status:  Planned,
		Run:     nil,
	})
	Tools.MustRegister("invoke_agent", Tool{
		Command: "invoke_agent",
		Canon:   "§Agent",
		Purpose: "Not implemented. Historically: invokes a sub-agent by loading its CLAUDE.md as the operator-prompt and printing it to stdout.",
		Status:  Planned,
		Run:     nil,
	})
	Tools.MustRegister("mark_revisit_evaluated", Tool{
		Command: "mark_revisit_evaluated",
		Canon:   "§Conflict",
		Purpose: "Not implemented. Historically: records that a DECIDED conflict's revisit_marker was evaluated.",
		Status:  Planned,
		Run:     nil,
	})
	Tools.MustRegister("review", Tool{
		Command: "review",
		Canon:   "§Closure",
		Purpose: "Not implemented. Historically: single CLI entry point over the low-traffic review tools.",
		Status:  Planned,
		Run:     nil,
	})
	Tools.MustRegister("setup_context_hook", Tool{
		Command: "setup_context_hook",
		Canon:   "§Context",
		Purpose: "Not implemented. Historically: installs/removes the project-local hook that feeds the context producer.",
		Status:  Planned,
		Run:     nil,
	})
	Tools.MustRegister("setup_hooks", Tool{
		Command: "setup_hooks",
		Canon:   "§Operator",
		Purpose: "Not implemented. Historically: generates the committable, portable project sensorium.",
		Status:  Planned,
		Run:     nil,
	})
	Tools.MustRegister("spawn_agent", Tool{
		Command: "spawn_agent",
		Canon:   "§Agent",
		Purpose: "Not implemented. Historically: composes a sub-agent's task prompt by prepending the agent's CLAUDE.md crystal, so the subagent boots from substrate (not from raw text).",
		Status:  Planned,
		Run:     nil,
	})
	Tools.MustRegister("spawn_log_isolation_status", Tool{
		Command: "spawn_log_isolation_status",
		Canon:   "§Agent",
		Purpose: "Not implemented. Historically: reads the runtime spawn-log and flags mutating agents recorded without worktree isolation.",
		Status:  Planned,
		Run:     nil,
	})
	Tools.MustRegister("ticket_comment", Tool{
		Command: "ticket_comment",
		Canon:   "§Ticket",
		Purpose: "Not implemented. Historically: appends a stamped comment to a ticket (and a History 'commented' entry).",
		Status:  Planned,
		Run:     nil,
	})
	Tools.MustRegister("ticket_create", Tool{
		Command: "ticket_create",
		Canon:   "§Ticket",
		Purpose: "Not implemented. Historically: creates a new on-disk ticket (auto-id, initial status, first History entry).",
		Status:  Planned,
		Run:     nil,
	})
	Tools.MustRegister("ticket_edit", Tool{
		Command: "ticket_edit",
		Canon:   "§Ticket",
		Purpose: "Not implemented. Historically: edits a ticket's title/body, snapshotting the prior text into History.",
		Status:  Planned,
		Run:     nil,
	})
	Tools.MustRegister("ticket_list", Tool{
		Command: "ticket_list",
		Canon:   "§Ticket",
		Purpose: "Not implemented. Historically: lists tickets, optionally filtered by status or assignee (read-only).",
		Status:  Planned,
		Run:     nil,
	})
	Tools.MustRegister("ticket_move", Tool{
		Command: "ticket_move",
		Canon:   "§Ticket",
		Purpose: "Not implemented. Historically: moves a ticket to a new status (relocates the file + records the transition in History).",
		Status:  Planned,
		Run:     nil,
	})
	Tools.MustRegister("ticket_show", Tool{
		Command: "ticket_show",
		Canon:   "§Ticket",
		Purpose: "Not implemented. Historically: prints one ticket's header, body, comments and full History (read-only).",
		Status:  Planned,
		Run:     nil,
	})
}
