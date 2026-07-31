// Origin: one-time bulk extraction by .scratch/selfspec-codegen (task #345, RAC-A) from
// domains/hotam-spec-self/graph.json. That program was a THROWAWAY (run once, never
// committed to git) and is NOT a regeneration path -- it went graph->Go, the OPPOSITE of
// today's sync direction. These literals are HAND-MAINTAINED Go source: edit them directly,
// then mirror Go->graph via `hotam sync-self` (dry-run by default; --confirm-hash to write).
// The "generated" framing describes this file's ORIGIN (task #345's one-time bulk pull from
// the pre-existing JSON graph), not an ongoing regeneration workflow.
//
// Topic: Ticket (durable on-disk work items).
//
// STRUCTURAL fields only: Claim, Why, Owner, Status, Relations, Assumptions,
// Enforcement, EnforcedBy, Enforceability, MTag, Summary, CreatedAt, SettledAt,
// BlockedOn, ImplementedBy, VerifiedBy, SourceRefs, DeclOrder. EVENT fields
// (History, GateSignoffs, LastReviewedAt, ReviewAfter, Evidence) are
// deliberately absent — MergeIntoGraph passes them through from the graph's
// existing node untouched.
package selfspec

import "github.com/PHPCraftdream/HotamSpec/internal/ontology"

var _ = Requirements.MustRegister("R-open-tickets-visible", ontology.Requirement{
	ID:             "R-open-tickets-visible",
	Claim:          "The what-now harness shall surface a CLI-only band summarising open (non-done) on-disk tickets broken down by status, read from the filesystem and never fed into DiagnoseSignals.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "Not yet implemented: no open-tickets band exists. The resolver's backlog moved onto disk; without a pulse band the queue is invisible until someone remembers to ls tickets/ (R-agent-never-lost). Like the pending-proposal / revisit-marker bands it would read the filesystem, so it stays OUT of DiagnoseSignals (internal/diagnose/signal.go) to keep generated docs byte-stable (R-deterministic-generation). what-now (cmd/hotam/what_now.go) renders only graph-derived signals from DiagnoseSignals -- there is no open-tickets band, and the on-disk ticket tools (ticket-create/list/show/move/edit/comment) are all Planned (not yet implemented in methodology.Tools). (Historical Python enforcement: tests/test_open_tickets_band.py reported open tickets by status, silent when none, excluded from diagnose.) PROSE (not ENFORCED): no enforcer exists. settled_at RESTORED 2026-07-13 from git HEAD 9af0176 (pre-corruption value) after a mutate.go bug this session silently reset it on content-only updates; see task #84.",
	Assumptions:    []string{"A-text-grounded-in-models"},
	Relations:      []ontology.Relation{},
	Enforcement:    "PROSE",
	EnforcedBy:     []string{},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-07-03",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{},
	DeclOrder:      250,
	BlockedOn:      "blocked on the ticket_* tool family + the what-now open_tickets signal producer, Planned in internal/methodology/tools_data.go",
})

var _ = Requirements.MustRegister("R-spawn-log-carries-isolation", ontology.Requirement{
	ID:             "R-spawn-log-carries-isolation",
	Claim:          "Every spawn-log entry shall carry isolation (worktree|shared) and mutating (bool) fields, defaulting to shared/false when the caller omits them.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "Not yet implemented: spawn-agent is Planned in methodology.Tools -- there is no spawn-log writer, no spawn-log.jsonl, and no isolation/mutating fields, because no sub-agent spawning machinery exists at all (R-claude-md-consolidates-when-single-agent: one operator, zero active sub-agents). Wave 5 (measurable slices of discipline): the spawn-log records WHO/WHAT/WHEN (R-task-spawn-log-runtime) but the spec calls for it to also record the isolation posture under which a sub-agent ran, so a parallel-mutating-agent hazard has a trace -- two additive fields (isolation=worktree|shared, mutating=bool), defaulting to shared/false, backward-compatible with every pre-existing caller. (Historical note: the original writer was tools/spawn_agent.py, whose freeze under R-speculative-aspects-frozen was partially lifted by explicit resolver act -- GO given for Wave 5 -- to add the two CLI flags and matching log fields.) The writer half (this atom) and the honest reader half (R-parallel-mutating-agents-use-worktree, which can only check log-internal consistency, not runtime concurrency) are both unimplemented together, since both depend on the unimplemented spawn machinery. PROSE (not ENFORCED): no enforcer exists.",
	Assumptions:    []string{"A-finite-context-operators"},
	Relations:      []ontology.Relation{},
	Enforcement:    "PROSE",
	EnforcedBy:     []string{},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-07-02",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{},
	DeclOrder:      230,
	BlockedOn:      "blocked on the spawn_agent tool (Planned) + the .runtime/spawn-log.jsonl infrastructure (absent)",
})

var _ = Requirements.MustRegister("R-task-spawn-is-a-hand", ontology.Requirement{
	ID:             "R-task-spawn-is-a-hand",
	Claim:          "A task-agent invocation (a sh/Agent-tool call) is a hand -- a one-shot delegated act, not a standing sub-operator.",
	Owner:          "ai-agent",
	Status:         "SETTLED",
	Why:            "First half of the split R-task-spawn-is-ephemeral (R-requirement-claim-is-atomic). The user's distinction (hands vs agents): a hand executes one task and reports back, distinct from a domain-delegation sub-operator (R-context-bounded-delegation) which owns a persistent sub-domain. This is a naming/classification discipline -- no check_* can verify 'this call was conceptually a hand', so it stays STRUCTURAL, carried by the spawn-log's own shape (one entry per invocation, no operator-id field implying persistence) rather than a dedicated enforcer.",
	Assumptions:    []string{"A-finite-context-operators"},
	Relations:      []ontology.Relation{{Kind: "replaces", Target: "R-task-spawn-is-ephemeral"}},
	Enforcement:    "STRUCTURAL",
	EnforcedBy:     []string{},
	MTag:           "",
	Enforceability: "INHERENTLY_PROSE",
	Summary:        "",
	CreatedAt:      "2026-07-02",
	SettledAt:      "2026-07-02",
	SourceRefs:     []string{},
	DeclOrder:      220,
})

var _ = Requirements.MustRegister("R-task-spawn-is-ephemeral", ontology.Requirement{
	ID:             "R-task-spawn-is-ephemeral",
	Claim:          "A task-agent invocation (a sh/Agent-tool call) is a hand: it returns conclusions and does not persist between invocations.",
	Owner:          "ai-agent",
	Status:         "REJECTED",
	Why:            "REJECTED -- REPLACES split into R-task-spawn-is-a-hand + R-task-spawn-no-cross-invocation-persistence (Wave 2 burn-down, decided by ai-agent 2026-07-02) per atomicity discipline (R-requirement-claim-is-atomic). audit_atomicity.py flagged this claim COMPOUND ('and' connects clause with verb, 'and does'): (1) a task-agent invocation IS a hand (a classification claim), and (2) it returns conclusions and does not persist between invocations (a behavioral claim). Splitting separates the naming/classification half from the persistence-behavior half so each can be honestly graded on its own enforceability -- the classification is discipline/naming, the non-persistence half is closer to (but not fully) checkable via the spawn-log's structure. — (was: The user's distinction today: hands vs agents. BUILD-TRIGGER: D3's spawn-log writer exists — the log is the structural recording of this ephemeral act.)",
	Assumptions:    []string{"A-finite-context-operators"},
	Relations:      []ontology.Relation{},
	Enforcement:    "PROSE",
	EnforcedBy:     []string{},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "",
	SourceRefs:     []string{},
	DeclOrder:      91,
})

var _ = Requirements.MustRegister("R-task-spawn-log-runtime", ontology.Requirement{
	ID:             "R-task-spawn-log-runtime",
	Claim:          "The spawn_agent tool shall append a spawn-log entry to .runtime/spawn-log.jsonl -- with parent, child kind, task subject, and stamp -- on every invocation.",
	Owner:          "ai-agent",
	Status:         "SETTLED",
	Why:            "Not yet implemented: a dedicated spawn-log mechanism. The `spawn_agent` tool is Planned (not Implemented) in methodology.Tools (internal/methodology/tools_data.go), so neither the prompt-composition nor the spawn-log append exists as a hotam CLI command today. (Historical note: the original mechanism was tools/spawn_agent.py, which composed a sub-agent's task prompt by prepending the agent's CLAUDE.md crystal (R-agent-is-recursive-director, R-sub-agent-crystal-triad) and appended a spawn-log entry to spec/.runtime/spawn-log.jsonl, proven by test_spawn_log_written and test_log_only_writes_row_without_composing_prompt.) The 2026-07-10 re-audit's category distinction still holds: 'spawn' here means an explicit spawn_agent invocation composing a domain sub-operator's CLAUDE.md crystal -- there are currently ZERO such sub-operators in this domain (Agent Map: '(no sub-operators yet)'), so zero real invocations is the EXPECTED, honest state, not a broken rule. A host-level Agent-tool spawn (a platform primitive, external to the framework's own code) is category-distinct, covered by R-host-spawn-leaves-trace (hotam-dev). The claim itself stays a single atomic assertion (R-requirement-claim-is-atomic). PROSE (not ENFORCED): no enforcer exists. The original WHY stands: every explicit spawn must leave a durable runtime trace so the isolation discipline (R-parallel-mutating-agents-use-worktree) is auditable.",
	Assumptions:    []string{"A-finite-context-operators"},
	Relations:      []ontology.Relation{},
	Enforcement:    "PROSE",
	EnforcedBy:     []string{},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{},
	DeclOrder:      93,
	BlockedOn:      "blocked on the spawn_agent tool (Planned) + the .runtime/spawn-log.jsonl infrastructure (absent)",
})

var _ = Requirements.MustRegister("R-task-vs-action-distinct-altitudes", ontology.Requirement{
	ID:             "R-task-vs-action-distinct-altitudes",
	Claim:          "The methodology's Task node type (a modeled work item) and the harness's Action (a fix-the-graph instruction) shall remain distinct types at distinct altitudes — never merged.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "SETTLED (P9): the discipline is structural by omission. Hotam-Spec's framework has NO Task type — only the harness Finding (internal/diagnose/finding.go: the typed instruction a what-now Action carries, with Condition/Target/Imperative/Advisory fields). Process.steps carry a forward-compat prose `invokes` field (not a Task type) so the behavioral altitude stays separable from the harness altitude. The two are typed differently by construction: Finding is the harness's typed instruction emitted by diagnose.AllFindings / DiagnoseSignals; any future Task would be a domain-modeled work item under the §Process aspect (internal/ontology/process.go). The altitudes cannot collapse because they live in different namespaces. Implementation: internal/diagnose/finding.go (Finding) + internal/ontology/process.go (Process.steps.invokes) + docs/gen/CONSTITUTION.md.",
	Assumptions:    []string{"A-bootstrap-self-applies"},
	Relations:      []ontology.Relation{},
	Enforcement:    "STRUCTURAL",
	EnforcedBy:     []string{},
	MTag:           "",
	Enforceability: "INHERENTLY_PROSE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{"internal/diagnose/finding.go", "internal/ontology/process.go"},
	DeclOrder:      28,
})

var _ = Requirements.MustRegister("R-ticket-carries-history", ontology.Requirement{
	ID:             "R-ticket-carries-history",
	Claim:          "Every ticket shall carry an append-only ## History section in which each mutation (create, status move, comment, text change) records one machine-recognisable entry, with a text change snapshotting the prior text.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "Not yet implemented: the ticket engine and its History-append mechanism. All ticket-* tools (ticket_create, ticket_move, ticket_comment, ticket_edit) are Planned (not Implemented) in methodology.Tools (internal/methodology/tools_data.go), so neither the on-disk ticket format nor the History-append mechanism exists as a hotam CLI command today. (Historical note: the original mechanism was the ticket_* tool family — tools/ticket_create.py, ticket_move.py, ticket_comment.py, ticket_edit.py — where every ticket_* mutator appended exactly one History line, and ticket_edit snapshotted the OLD title/body into that line so the edit trail survived the edit; proven by tests/test_tool_ticket_create.py, test_tool_ticket_move.py, test_tool_ticket_comment.py, test_tool_ticket_edit.py.) The resolver's original intent still stands: status history AND text-change history should be kept automatically so a ticket's audit trail is durable across mutations. When the ticket engine is implemented, each mutator (create/move/comment/edit) must append exactly one History entry, with ticket_edit snapshotting the prior title/body. PROSE (not ENFORCED): no enforcer exists.",
	Assumptions:    []string{"A-text-grounded-in-models"},
	Relations:      []ontology.Relation{},
	Enforcement:    "PROSE",
	EnforcedBy:     []string{},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-07-03",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{},
	DeclOrder:      248,
	BlockedOn:      "blocked on the ticket_* tool family (the History appender is part of the engine), Planned in internal/methodology/tools_data.go",
})

var _ = Requirements.MustRegister("R-ticket-engine-on-disk", ontology.Requirement{
	ID:             "R-ticket-engine-on-disk",
	Claim:          "Work items shall be tracked as durable on-disk tickets under tickets/<status>/T-<n>.md, each a JSON-frontmatter header plus a Markdown body, created and moved between status folders by the ticket_* tools.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "Not yet implemented: the on-disk ticket engine. All ticket-* tools (ticket_create, ticket_move, ticket_edit, ticket_comment, ticket_list, ticket_show) are Planned (not Implemented) in methodology.Tools (internal/methodology/tools_data.go), so neither the on-disk ticket format nor the folder-move lifecycle exists as a hotam CLI command today. (Historical note: the original mechanism was the ticket_* tool family — tools/ticket_create.py, ticket_move.py — where a ticket was a committed file whose status was its folder, so a status change was a visible file move with a git footprint; JSON-between-sentinels frontmatter kept the machine header exact under a stdlib-only parser (R-core-imports-stdlib-or-hotam-spec-only).) The resolver's original WHY stands: a chat-based queue evaporates between sessions; a committed file whose status is its folder makes a status change a visible file move with a git footprint. When the ticket engine is implemented, the format (tickets/<status>/T-<n>.md with JSON-frontmatter header + Markdown body) and the folder-as-status lifecycle should be preserved. PROSE (not ENFORCED): no enforcer exists.",
	Assumptions:    []string{"A-text-grounded-in-models"},
	Relations:      []ontology.Relation{},
	Enforcement:    "PROSE",
	EnforcedBy:     []string{},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-07-03",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{},
	DeclOrder:      247,
	BlockedOn:      "blocked on the ticket_* tool family (create/edit/move/comment/list/show), Planned in internal/methodology/tools_data.go",
})

var _ = Requirements.MustRegister("R-ticket-mutation-via-tools-only", ontology.Requirement{
	ID:             "R-ticket-mutation-via-tools-only",
	Claim:          "A ticket's frontmatter header and History shall be changed only through the ticket_* tools, never by hand-editing the file.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "The encapsulation the resolver demanded: creation, status movement and auto-history must be owned by the tools so the History trail is never skipped. This is INHERENTLY_PROSE discipline, not closeable debt: a filesystem can always be hand-edited and no test can prove authorship of a line. Its enforceable teeth live in R-ticket-carries-history (each mutation demonstrably appends History); this atom names the discipline that keeps hands off the header.",
	Assumptions:    []string{"A-text-grounded-in-models"},
	Relations:      []ontology.Relation{},
	Enforcement:    "STRUCTURAL",
	EnforcedBy:     []string{},
	MTag:           "",
	Enforceability: "INHERENTLY_PROSE",
	Summary:        "",
	CreatedAt:      "2026-07-03",
	SettledAt:      "2026-07-03",
	SourceRefs:     []string{},
	DeclOrder:      249,
})

var _ = Requirements.MustRegister("R-user-request-decomposed-to-tickets", ontology.Requirement{
	ID:             "R-user-request-decomposed-to-tickets",
	Claim:          "The operator shall, immediately on receiving a user request, decompose it into tickets in the dialogue and ask the addressee -- session tasks or the ticket engine -- before beginning any work.",
	Owner:          "ai-agent",
	Status:         "SETTLED",
	Why:            "Resolver verdict 2026-07-03 (verbatim): 'всё, что просит пользователь - сразу декомпоизровать на тикеты в чате и справшить куда их оптравить - в таски сессии или в движок тикетов'. Decomposition-before-work is a behavioral discipline of the operator: it makes the unit of work explicit and routable BEFORE effort is spent, so a multi-part request is never silently collapsed into one undifferentiated action (R-anchor-everything at the work-item altitude; R-ai-presents-not-decides -- the routing choice is the resolver's, presented not assumed). It is machine-unverifiable (INHERENTLY_PROSE): whether the operator actually paused to decompose and ask lives in the dialogue, not the graph.",
	Assumptions:    []string{},
	Relations:      []ontology.Relation{},
	Enforcement:    "STRUCTURAL",
	EnforcedBy:     []string{},
	MTag:           "",
	Enforceability: "INHERENTLY_PROSE",
	Summary:        "",
	CreatedAt:      "2026-07-03",
	SettledAt:      "2026-07-03",
	SourceRefs:     []string{},
	DeclOrder:      246,
})
