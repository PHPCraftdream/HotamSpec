package methodology

func init() {
	// --- Implemented: real `hotam` CLI subcommands (cmd/hotam/main.go) ---

	Tools.MustRegister("gen_spec", Tool{
		Command:  "gen_spec",
		Canon:    "§Generator",
		Purpose:  "Usage: hotam gen-spec [--domain <path>] [--today YYYY-MM-DD] [--claude-md <path>]. Regenerates docs/gen/*.md + graph.json for a domain graph from the executable model (internal/generator + internal/ontology), making drift structurally impossible.",
		Status:   Implemented,
		Claim:    "regenerates docs/gen/ from the executable model (methodology + graph), making drift structurally impossible.",
		Enforcer: "",
		Run:      nil,
	})
	Tools.MustRegister("what_now", Tool{
		Command:  "what_now",
		Canon:    "§Harness",
		Purpose:  "Usage: hotam what-now [--domain <path>] [--limit N] [--today YYYY-MM-DD]. Derives the prioritized next correct action from any graph state (internal/diagnose), making being-lost structurally impossible.",
		Status:   Implemented,
		Claim:    "derives the prioritized next correct action from any graph state, making being-lost structurally impossible.",
		Enforcer: "",
		Run:      nil,
	})
	Tools.MustRegister("apply_proposal", Tool{
		Command:  "apply_proposal",
		Canon:    "§Proposal",
		Purpose:  "Usage: hotam apply-proposal <proposal.json> --domain <path> --today YYYY-MM-DD [--batch <dir>]. Mechanical writer for steward-approved JSON proposals (internal/proposal): consumes an approved Proposed* JSON and applies the change to a domain graph.json.",
		Status:   Implemented,
		Claim:    "mechanical writer for steward-approved JSON proposals.",
		Enforcer: "",
		Run:      nil,
	})
	Tools.MustRegister("gate", Tool{
		Command:  "gate",
		Canon:    "§Closure",
		Purpose:  "Usage: hotam gate <target-anchor> [--domain <path>]. T1 tiered LAND gate (internal/gate): selects a targeted enforcer subset for a target node instead of the full test suite.",
		Status:   Implemented,
		Claim:    "T1 tiered LAND gate: select a targeted enforcer subset instead of the full suite.",
		Enforcer: "test_tool_gate",
		Run:      nil,
	})
	Tools.MustRegister("all_violations", Tool{
		Command:  "all_violations",
		Canon:    "§Invariants",
		Purpose:  "Usage: hotam all-violations [--domain <path>]. Prints all invariant violations for a domain graph (internal/invariants); exits 1 if any are found.",
		Status:   Implemented,
		Claim:    "prints all invariant violations for a domain graph, exiting 1 if any are found.",
		Enforcer: "",
		Run:      nil,
	})
	Tools.MustRegister("req", Tool{
		Command:  "req",
		Canon:    "§Requirement",
		Purpose:  "Usage: hotam req <show|list|search|context|related> [args] [--domain <path>] [--json]. Compact agentic read interface over the domain graph (internal/query): answers 'what is R-x' / 'what touches R-x' without loading the full graph.json or a generated doc.",
		Status:   Implemented,
		Claim:    "compact agentic read interface over the domain graph, answering 'what is R-x' / 'what touches R-x' without loading the full graph.json or a generated doc.",
		Enforcer: "",
		Run:      nil,
	})
	Tools.MustRegister("due", Tool{
		Command:  "due",
		Canon:    "§Requirement",
		Purpose:  "Usage: hotam due [--domain <path>] [--today YYYY-MM-DD] [--json]. Advisory report of OVERDUE and NEVER-REVIEWED SETTLED requirements (internal/freshness); never gates, exit code always 0.",
		Status:   Implemented,
		Claim:    "advisory report of OVERDUE and NEVER-REVIEWED SETTLED requirements; never gates, exit code always 0.",
		Enforcer: "",
		Run:      nil,
	})
	Tools.MustRegister("status", Tool{
		Command:  "status",
		Canon:    "§Operator",
		Purpose:  "Usage: hotam status [--domain <path>] [--today YYYY-MM-DD] [--json]. Single-shot compact summary combining what-now's top action + debt (internal/diagnose), due's freshness counts (internal/freshness), and all-violations' violation count (internal/invariants), so an agent doesn't need to run all three separately. Never gates; exit code always 0.",
		Status:   Implemented,
		Claim:    "single-shot compact summary combining what-now's top action + debt, due's freshness counts, and all-violations' violation count, so an agent doesn't need to run all three separately.",
		Enforcer: "",
		Run:      nil,
	})
	Tools.MustRegister("inspect", Tool{
		Command:  "inspect",
		Canon:    "§Conflict",
		Purpose:  "Usage: hotam inspect [--domain <path>] [--json] [--limit N] [--min-score N]. Advisory listing of semantic conflict candidates with evidence (internal/diagnose): shared-assumption clusters, entity-state suspects, lexical claim overlap, axis co-reference. --min-score (default 5) suppresses low-signal candidates; 0 shows all. Never gates; exit code always 0.",
		Status:   Implemented,
		Claim:    "advisory listing of semantic conflict candidates with evidence: shared-assumption clusters, entity-state suspects, lexical claim overlap, axis co-reference.",
		Enforcer: "",
		Run:      nil,
	})
	Tools.MustRegister("confront", Tool{
		Command:  "confront",
		Canon:    "§Loop",
		Purpose:  "Usage: hotam confront <text> [--domain <path>] [--file <path>] [--json]. CONFRONT step of the mediation loop (internal/diagnose): checks a candidate claim for lexical overlap with SETTLED requirements (duplicate guard) and REJECTED history (anti-relitigation) before anything is written. <text> is a quoted positional; --file <path> reads a long draft. Reuses the inspect overlap engine. Never gates; exit code always 0.",
		Status:   Implemented,
		Claim:    "the CONFRONT step's tool: ranks a candidate claim's lexical overlap against SETTLED reality and REJECTED history before anything is written.",
		Enforcer: "test_tool_confront",
		Run:      nil,
	})
	Tools.MustRegister("land", Tool{
		Command:  "land",
		Canon:    "§Closure",
		Purpose:  "Usage: hotam land <proposal.json> --domain <path> --today YYYY-MM-DD [--batch <dir>] [--claude-md <path>]. Single CLI entry point over apply-proposal -> gen-spec -> all-violations (internal/proposal + internal/generator + internal/invariants).",
		Status:   Implemented,
		Claim:    "single CLI entry point over gate/gate_status/closure.",
		Enforcer: "test_tool_land",
		Run:      nil,
	})
	Tools.MustRegister("init", Tool{
		Command:  "init",
		Canon:    "§Domain",
		Purpose:  "Usage: hotam init <dir> [--name <domain-name>]. Scaffolds a new domain: minimal graph.json (seed Stakeholder + seed SETTLED Requirement, all-violations=0 immediately), manifest.json, docs/gen/, and a README.md pointing at the next commands to run. <dir> may be anywhere on disk.",
		Status:   Implemented,
		Claim:    "scaffolds a new domain with a minimal graph.json (seed Stakeholder + seed SETTLED Requirement, all-violations=0 immediately), manifest.json, docs/gen/, and a README.md.",
		Enforcer: "",
		Run:      nil,
	})
	Tools.MustRegister("init_project", Tool{
		Command:  "init_project",
		Canon:    "§Domain",
		Purpose:  "Usage: hotam init-project <dir> [--domain <name>] [--today YYYY-MM-DD]. Bootstraps an external business project's full Hotam-Spec layout in one call: scaffolds a base domain under <dir>/domains/<name> (default main) via init, writes the project-root marker (.hotam-spec-project), and renders the root crystal (CLAUDE.md/AGENTS.md/GEMINI.md) + all docs/gen/* via gen-spec. Refuses to overwrite an existing project marker or CLAUDE.md.",
		Status:   Implemented,
		Claim:    "bootstraps an external business project's full Hotam-Spec layout in one call: scaffolds a base domain, writes the project-root marker, and renders the root crystal + all docs/gen/* via gen-spec.",
		Enforcer: "",
		Run:      nil,
	})
	Tools.MustRegister("version", Tool{
		Command:  "version",
		Canon:    "§Operator",
		Purpose:  "Usage: hotam version, hotam --version. Prints the hotam binary's version, commit, and build date (build-time defaults \"dev\"/\"unknown\"/\"unknown\", overridable via -ldflags -X main.version/commit/buildDate).",
		Status:   Implemented,
		Claim:    "prints the hotam binary's version, commit, and build date.",
		Enforcer: "",
		Run:      nil,
	})

	// --- Planned: methodology surface not yet implemented as a
	// Go command. Command below is the historical tool name, not a
	// runnable invocation. See P1-6. ---

	Tools.MustRegister("attention", Tool{
		Command:  "attention",
		Canon:    "§Attention",
		Purpose:  "Not implemented. Historically: the agent-agnostic CLI over the attention core.",
		Status:   Planned,
		Claim:    "the agent-agnostic CLI over the attention core.",
		Enforcer: "",
		Run:      nil,
	})
	Tools.MustRegister("attention_hook", Tool{
		Command:  "attention_hook",
		Canon:    "§Attention",
		Purpose:  "Not implemented. Historically: the Claude adapter that injects the attention list into context.",
		Status:   Planned,
		Claim:    "the Claude adapter: inject the attention list into context.",
		Enforcer: "",
		Run:      nil,
	})
	Tools.MustRegister("audit_atomicity", Tool{
		Command:  "audit_atomicity",
		Canon:    "§Invariants",
		Purpose:  "Not implemented. Historically: surfaces Requirements with compound claims and check_* functions with compound conditions, both structural signals for decomposition.",
		Status:   Planned,
		Claim:    "surfaces Requirements with compound claims and check_* functions with compound conditions, both structural signals for decomposition.",
		Enforcer: "test_tool_audit_atomicity",
		Run:      nil,
	})
	Tools.MustRegister("audit_tensions", Tool{
		Command:  "audit_tensions",
		Canon:    "§Loop",
		Purpose:  "Not implemented. Historically: the generative-audit tool, a deterministic, LLM-free shortlist of latent-connector suspects.",
		Status:   Planned,
		Claim:    "the generative-audit tool: a deterministic, LLM-free shortlist of",
		Enforcer: "test_tool_audit_tensions",
		Run:      nil,
	})
	Tools.MustRegister("claude_md_diff_watch", Tool{
		Command:  "claude_md_diff_watch",
		Canon:    "§Operator",
		Purpose:  "Not implemented. Historically: auto-injects the diff of CLAUDE.md since the operator's last turn into session context via a UserPromptSubmit hook.",
		Status:   Planned,
		Claim:    "auto-injects the diff of CLAUDE.md since the operator's last turn into session context via a UserPromptSubmit hook.",
		Enforcer: "",
		Run:      nil,
	})
	Tools.MustRegister("closure", Tool{
		Command:  "closure",
		Canon:    "§Closure",
		Purpose:  "Not implemented. Historically: per-action verify — did the proposal remove its diagnosis?",
		Status:   Planned,
		Claim:    "per-action verify: did the proposal remove its diagnosis?",
		Enforcer: "",
		Run:      nil,
	})
	Tools.MustRegister("context", Tool{
		Command:  "context",
		Canon:    "§Context",
		Purpose:  "Not implemented. Historically: the operator's working-context measurement (reader + CLI dispatcher).",
		Status:   Planned,
		Claim:    "the operator's working-context measurement (reader + CLI dispatcher).",
		Enforcer: "test_tool_context",
		Run:      nil,
	})
	Tools.MustRegister("context_producer", Tool{
		Command:  "context_producer",
		Canon:    "§Context",
		Purpose:  "Not implemented. Historically: the producer half of the context cipher, writing a runtime context snapshot.",
		Status:   Planned,
		Claim:    "the producer half of the context cipher, writing a runtime context.json snapshot.",
		Enforcer: "",
		Run:      nil,
	})
	Tools.MustRegister("create_agent", Tool{
		Command:  "create_agent",
		Canon:    "§Agent",
		Purpose:  "Not implemented. Historically: scaffolds domains/<domain>/agents/<name>/ as a self-contained sub-operator directory with its own CLAUDE.md, scope, tools/, agents/, and README.md.",
		Status:   Planned,
		Claim:    "scaffolds domains/<name>/agents/<name>/ as a self-contained sub-operator directory with its own CLAUDE.md, scope definition, tools/, agents/, and README.md.",
		Enforcer: "test_tool_create_agent",
		Run:      nil,
	})
	Tools.MustRegister("create_axis", Tool{
		Command:  "create_axis",
		Canon:    "§Axis",
		Purpose:  "Not implemented. Historically: scaffolds a new Axis into the active domain's controlled-vocabulary.",
		Status:   Planned,
		Claim:    "scaffolds a new Axis into the active domain's controlled-vocabulary",
		Enforcer: "test_tool_create_axis",
		Run:      nil,
	})
	Tools.MustRegister("create_domain", Tool{
		Command:  "create_domain",
		Canon:    "§Domain",
		Purpose:  "Not implemented. Historically: scaffolds domains/<name>/ as a self-contained business domain with a manifest, graph.json, tools/, agents/director/, docs/gen/, and CLAUDE.md.",
		Status:   Planned,
		Claim:    "scaffolds domains/<name>/ as a self-contained business domain with manifest.json, graph.json, tools/, agents/director/, docs/gen/, and CLAUDE.md.",
		Enforcer: "test_tool_create_domain",
		Run:      nil,
	})
	Tools.MustRegister("create_entity_type", Tool{
		Command:  "create_entity_type",
		Canon:    "§Entity",
		Purpose:  "Not implemented. Historically: scaffolds an EntityType declaration into the active domain's graph via apply-proposal.",
		Status:   Planned,
		Claim:    "scaffolds an EntityType declaration into the active domain's graph via apply_proposal.",
		Enforcer: "test_tool_create_entity_type",
		Run:      nil,
	})
	Tools.MustRegister("emit_cipher", Tool{
		Command:  "emit_cipher",
		Canon:    "§Operator",
		Purpose:  "Not implemented. Historically: emits the three-cipher pulse (top action / debt / context) directly from the active domain's graph.",
		Status:   Planned,
		Claim:    "emits the three-cipher pulse (top action / debt / context) directly from the active domain's graph.",
		Enforcer: "",
		Run:      nil,
	})
	Tools.MustRegister("gate_status", Tool{
		Command:  "gate_status",
		Canon:    "§Closure",
		Purpose:  "Not implemented. Historically: reads the runtime land-log and answers the commit-boundary question.",
		Status:   Planned,
		Claim:    "read the runtime land-log.jsonl and answer the commit-boundary question.",
		Enforcer: "test_tool_gate_status",
		Run:      nil,
	})
	Tools.MustRegister("invoke_agent", Tool{
		Command:  "invoke_agent",
		Canon:    "§Agent",
		Purpose:  "Not implemented. Historically: invokes a sub-agent by loading its CLAUDE.md as the operator-prompt and printing it to stdout.",
		Status:   Planned,
		Claim:    "invokes a sub-agent by loading its CLAUDE.md as the operator-prompt and printing it to stdout.",
		Enforcer: "test_tool_invoke_agent",
		Run:      nil,
	})
	Tools.MustRegister("mark_revisit_evaluated", Tool{
		Command:  "mark_revisit_evaluated",
		Canon:    "§Conflict",
		Purpose:  "Not implemented. Historically: records that a DECIDED conflict's revisit_marker was evaluated.",
		Status:   Planned,
		Claim:    "record that a DECIDED conflict's revisit_marker was evaluated.",
		Enforcer: "test_tool_mark_revisit_evaluated",
		Run:      nil,
	})
	Tools.MustRegister("review", Tool{
		Command:  "review",
		Canon:    "§Closure",
		Purpose:  "Not implemented. Historically: single CLI entry point over the low-traffic review tools.",
		Status:   Planned,
		Claim:    "single CLI entry point over the low-traffic review tools.",
		Enforcer: "test_tool_review",
		Run:      nil,
	})
	Tools.MustRegister("setup_context_hook", Tool{
		Command:  "setup_context_hook",
		Canon:    "§Context",
		Purpose:  "Not implemented. Historically: installs/removes the project-local hook that feeds the context producer.",
		Status:   Planned,
		Claim:    "installs/removes the project-local hook that feeds the context producer.",
		Enforcer: "test_tool_setup_context_hook",
		Run:      nil,
	})
	Tools.MustRegister("setup_hooks", Tool{
		Command:  "setup_hooks",
		Canon:    "§Operator",
		Purpose:  "Not implemented. Historically: generates the committable, portable project sensorium.",
		Status:   Planned,
		Claim:    "generate the committable, portable project sensorium.",
		Enforcer: "test_tool_setup_hooks",
		Run:      nil,
	})
	Tools.MustRegister("spawn_agent", Tool{
		Command:  "spawn_agent",
		Canon:    "§Agent",
		Purpose:  "Not implemented. Historically: composes a sub-agent's task prompt by prepending the agent's CLAUDE.md crystal, so the subagent boots from substrate (not from raw text).",
		Status:   Planned,
		Claim:    "composes a sub-agent's task prompt by prepending the agent's CLAUDE.md crystal, so the subagent boots from substrate (not from raw text).",
		Enforcer: "test_tool_spawn_agent",
		Run:      nil,
	})
	Tools.MustRegister("spawn_log_isolation_status", Tool{
		Command:  "spawn_log_isolation_status",
		Canon:    "§Agent",
		Purpose:  "Not implemented. Historically: reads the runtime spawn-log and flags mutating agents recorded without worktree isolation.",
		Status:   Planned,
		Claim:    "reads the runtime spawn-log.jsonl and flags mutating agents recorded without worktree isolation.",
		Enforcer: "test_tool_spawn_log_isolation_status",
		Run:      nil,
	})
	Tools.MustRegister("ticket_comment", Tool{
		Command:  "ticket_comment",
		Canon:    "§Ticket",
		Purpose:  "Not implemented. Historically: appends a stamped comment to a ticket (and a History 'commented' entry).",
		Status:   Planned,
		Claim:    "append a stamped comment to a ticket (and a History \"commented\" entry).",
		Enforcer: "test_tool_ticket_comment",
		Run:      nil,
	})
	Tools.MustRegister("ticket_create", Tool{
		Command:  "ticket_create",
		Canon:    "§Ticket",
		Purpose:  "Not implemented. Historically: creates a new on-disk ticket (auto-id, initial status, first History entry).",
		Status:   Planned,
		Claim:    "create a new on-disk ticket (auto-id, initial status, first History entry).",
		Enforcer: "test_tool_ticket_create",
		Run:      nil,
	})
	Tools.MustRegister("ticket_edit", Tool{
		Command:  "ticket_edit",
		Canon:    "§Ticket",
		Purpose:  "Not implemented. Historically: edits a ticket's title/body, snapshotting the prior text into History.",
		Status:   Planned,
		Claim:    "edit a ticket's title/body, snapshotting the prior text into History.",
		Enforcer: "test_tool_ticket_edit",
		Run:      nil,
	})
	Tools.MustRegister("ticket_list", Tool{
		Command:  "ticket_list",
		Canon:    "§Ticket",
		Purpose:  "Not implemented. Historically: lists tickets, optionally filtered by status or assignee (read-only).",
		Status:   Planned,
		Claim:    "list tickets, optionally filtered by status or assignee (read-only).",
		Enforcer: "",
		Run:      nil,
	})
	Tools.MustRegister("ticket_move", Tool{
		Command:  "ticket_move",
		Canon:    "§Ticket",
		Purpose:  "Not implemented. Historically: moves a ticket to a new status (relocates the file + records the transition in History).",
		Status:   Planned,
		Claim:    "move a ticket to a new status (relocates the file + records the transition in History).",
		Enforcer: "test_tool_ticket_move",
		Run:      nil,
	})
	Tools.MustRegister("ticket_show", Tool{
		Command:  "ticket_show",
		Canon:    "§Ticket",
		Purpose:  "Not implemented. Historically: prints one ticket's header, body, comments and full History (read-only).",
		Status:   Planned,
		Claim:    "print one ticket's header, body, comments and full History (read-only).",
		Enforcer: "",
		Run:      nil,
	})
}
