package methodology

var Requirement = Sections.MustRegister("§Requirement", Section{
	Slug:  "§Requirement",
	Kind:  ONTOLOGY,
	Canon: "A business requirement as a node in the tension graph — a claim the system shall satisfy.",
	Narrative: "A Requirement is a claim the system shall satisfy, written machine-checkable where possible and otherwise " +
		"EARS-style ('the system shall ...'). It is NOT a truth: it changes, it contradicts its siblings, and it rests on " +
		"assumptions that can die. The contradiction itself never lives here; it lives on the Conflict connector node (§Conflict).",
	Why: "Relations are typed tuple-of-id fields (not a generic graph): refines and depends_on are the SUPPORTIVE " +
		"non-adversarial edges. A contradiction is deliberately NOT among them — you cannot express a conflict as a " +
		"Requirement field, because a conflict belongs to neither requirement. This is the structural enforcement of " +
		"'conflict is a node, not an edge'.",
})

var Conflict = Sections.MustRegister("§Conflict", Section{
	Slug:  "§Conflict",
	Kind:  ONTOLOGY,
	Canon: "The first-class connector NODE between requirements, carrying axis+context+resolver.",
	Narrative: "A Conflict is NOT an edge between requirements. It is a mediator node through which two otherwise-" +
		"unconnectable requirements first come to lie in one structure (R-87 -> C-12 <- R-203). The node carries knowledge " +
		"belonging to NEITHER member: axis (the tension dimension), context (the colliding scenario), and shared_assumption " +
		"(the assumption they interpret differently — often the real root).",
	Why: "WHY a node, not an edge: an edge conflicts_with holds nothing — remove it and the requirements fall back into " +
		"isolation. The node holds the axis, context and shared assumption, which exist nowhere else. Conflicts CLUSTER by " +
		"axis, SPAWN requirements (the parent of a derived requirement), and INHERIT drift (if shared_assumption dies, the " +
		"whole cluster revives at once).",
})

var Assumption = Sections.MustRegister("§Assumption", Section{
	Slug:  "§Assumption",
	Kind:  ONTOLOGY,
	Canon: "A claim with its OWN lifecycle (the root of context drift).",
	Narrative: "An Assumption is a statement the requirements rest on but that the world can falsify independently " +
		"('there is a single customer per account'). It is the mechanism behind the third invisibility — context drift: a " +
		"requirement was meaningful under assumption X, X is long false, nobody revisited it. That is only catchable because " +
		"the assumption carries its own status.",
	Why: "WHY assumptions are first-class (not prose inside a requirement): conflicts and requirements INHERIT drift. " +
		"When an Assumption flips to DEAD, every Conflict and Requirement resting on it must light up at once — one trigger " +
		"re-opens a whole semantic cluster. A shared assumption interpreted two different ways is also frequently the REAL " +
		"root of a Conflict (Conflict.shared_assumption).",
})

var Axis = Sections.MustRegister("§Axis", Section{
	Slug:  "§Axis",
	Kind:  ONTOLOGY,
	Canon: "Controlled vocabulary of tension dimensions.",
	Narrative: "An Axis names the DIMENSION along which two requirements diverge inside a Conflict (latency vs " +
		"completeness; cost vs flexibility; privacy vs analytics). The axis is not a property of either requirement — it is " +
		"born only from their meeting and lives on the connector node (§Conflict).",
	Why: "WHY a controlled vocabulary: conflicts CLUSTER by axis — many C-nodes on one axis are one unresolved " +
		"ARCHITECTURAL choice, not ten local disputes. Clustering only works if the axis is a normalized, shared slug rather " +
		"than ad-hoc prose. The vocabulary lives on TensionGraph.axes (not a module constant) because Hotam-Spec is a " +
		"CONTENT-FREE framework: the framework ships zero axes; each domain owns its own.",
})

var Stakeholder = Sections.MustRegister("§Stakeholder", Section{
	Slug:  "§Stakeholder",
	Kind:  ONTOLOGY,
	Canon: "Who owns requirements and resolvers conflicts.",
	Narrative: "A Stakeholder is the human (or role) accountable for a piece of the requirement graph. Two distinct " +
		"accountabilities exist and MUST stay separate: OWNER of a Requirement (defends that requirement's claim) and " +
		"RESOLVER of a Conflict (holds the tension between requirements; by construction the resolver is NOT the owner of any " +
		"member requirement, because a conflict lives BETWEEN stakeholders).",
	Why: "WHY a first-class node (not a free-text 'owner: Finance'): accountability is the external anchor of the whole " +
		"internal loop. Every OPEN question, every undecided conflict, every dead-assumption fallout resolves to a named " +
		"stakeholder the harness can point at. Without a stable id there is nobody to escalate to and the loop floats free " +
		"of reality.",
})

var Invariants = Sections.MustRegister("§Invariants", Section{
	Slug:  "§Invariants",
	Kind:  DISCIPLINE,
	Canon: "Structural form of the tension graph (the check_* layer).",
	Narrative: "These are the spec-stack layer-2 invariants: the SHAPE the graph must always hold, regardless of how " +
		"many requirements contradict each other. A green run does NOT mean 'no contradictions'; contradictions are expected " +
		"and welcome. Green means the contradictions are WELL-FORMED: every conflict has an axis, a context and a resolver; " +
		"no edge dangles; every open hole states its question; every decision justifies itself. A conflict that is invisible " +
		"(resolverless, axis-less) is the one thing forbidden.",
	Why: "WHY return violations, not bool: dev-coin's check_* return bool because there the goal is a single pass/fail " +
		"gate. Here the SAME functions feed the 'what now' diagnosis, which needs the offending id and a human imperative — " +
		"so the richer return type (Violation) is load-bearing, and holds() recovers the boolean when a test just wants " +
		"pass/fail.",
})

var Graph = Sections.MustRegister("§Graph", Section{
	Slug:  "§Graph",
	Kind:  PLUMBING,
	Canon: "The tension graph store and its traversal helpers.",
	Narrative: "The store IS the code: a frozen TensionGraph holding tuples of Axes, Stakeholders, Assumptions, " +
		"Requirements and Conflicts; edges are tuple-of-id fields on those objects; traversal is the plain functions " +
		"below. No database, no RDF — the graph instance the invariants, the generator and the harness all read is the one " +
		"assembled by the loader. CONTENT-FREE FRAMEWORK: this module ships ZERO business data; real domains populate " +
		"domains/<name>/graph.json, loaded by internal/loader into an internal/ontology.Graph.",
	Why: "WHY traversal lives here as functions (not methods on a graph class doing logic): keeps the ontology " +
		"dataclasses pure data and the queries in one auditable place, mirroring dev-coin where chain logic is module " +
		"functions over frozen dataclasses.",
})

var Lifecycle = Sections.MustRegister("§Lifecycle", Section{
	Slug:  "§Lifecycle",
	Kind:  PLUMBING,
	Canon: "The generic state-machine value-type (framework keystone).",
	Narrative: "Every modeled state machine (Requirement.status, Conflict.lifecycle, and future Operator/Process " +
		"lifecycles) MUST validate against a framework-supplied Lifecycle constant. This module ships the SHAPE " +
		"(State / Transition / Lifecycle) plus the canonical framework instances (REQUIREMENT_STATUS_LIFECYCLE and " +
		"CONFLICT_LIFECYCLE). It is content-free: no business data lives here.",
	Why: "WHY one module, two constants: Requirement.status and Conflict.lifecycle are BOTH hand-rolled prefix-test " +
		"state machines with the same shape. Generalizing them into Lifecycle makes the stored strings the single source of " +
		"truth, adds a structural invariant that validates stored values against canonical states, and establishes the " +
		"keystone so Operator.lifecycle / Goal.status / Process.lifecycle in later phases can reuse it — no parallel " +
		"machinery.",
})

var Operator = Sections.MustRegister("§Operator", Section{
	Slug:  "§Operator",
	Kind:  ONTOLOGY,
	Canon: "The acting facet of a Stakeholder (M20: NEW TYPE).",
	Narrative: "An Operator is the acting facet of a Stakeholder (§Stakeholder). Where a Stakeholder answers 'who is " +
		"accountable', an Operator answers 'who can act, within what context, over which slice of the graph'. The two " +
		"facets MUST stay separate. Every Operator carries a typed anchor starting with 'OP-', references a Stakeholder.id, " +
		"has a lifecycle matched by OPERATOR_LIFECYCLE, carries a ContextBudget bounding its WORKING store, and optionally " +
		"has a parent Operator (the delegation hierarchy).",
	Why: "WHY a new type (M20 = new type, not a Stakeholder facet): separating them (Operator is a NEW TYPE referencing " +
		"Stakeholder) preserves the resolver-distinct boundary at the methodology altitude. A Stakeholder is an " +
		"accountability node; the acting/context/domain facet lives on Operator. Conflating them would merge accountability " +
		"with action and re-introduce the invisibility the boundary forbids.",
})

var ContextBudget = Sections.MustRegister("§ContextBudget", Section{
	Slug:  "§ContextBudget",
	Kind:  PLUMBING,
	Canon: "Bounds the working store only; the crystallized substrate (graph + generated docs) is FREE.",
	Narrative: "The ContextBudget is the working-store ceiling of an Operator: size(domain) <= budget.limit is a " +
		"structural check. It bounds only the WORKING store — the in-context cost an operator pays to hold a slice of the " +
		"graph. The crystallized substrate (graph + invariants + generated docs) is free of context cost because it lives " +
		"outside the working window, addressable on demand.",
	Why: "WHY working-vs-substrate (R-working-vs-substrate-budget): the budget must never tax knowledge that has been " +
		"crystallized into the durable substrate, or the operator is penalized for doing the right thing (crystallize first; " +
		"if still over budget, spawn a sub-operator). Measuring only the working store keeps the budget an honest signal of " +
		"live context pressure.",
})

var Loop = Sections.MustRegister("§Loop", Section{
	Slug:  "§Loop",
	Kind:  PROCESS,
	Canon: "The closed loop: State→Diagnosis→Next-action→Action→regenerate→State.",
	Narrative: "The closed loop is how ANY input is processed: ORIENT (reload pulse: top action, debt, context), " +
		"LOCATE (find what input touches), CONFRONT (check input vs reality), TRANSLATE (outcome -> typed nodes under a " +
		"proposal), PRESENT (show resolver the proposal + anchors), LAND (after approval: apply_proposal -> regen -> tiered " +
		"gate -> closure verifies the triggering diagnosis is gone). Writing nothing is a valid outcome.",
	Why: "WHY a closed loop with a human PRESENT/LAND gate: the loop makes 'being lost' structurally impossible " +
		"(what_now derives the next correct action from any graph state) and keeps the hard boundary — the AI presents and " +
		"proposes, the resolver decides; no conflict is ever closed silently. The regenerate->verify->closure arc guarantees " +
		"each landed action actually removed the diagnosis that triggered it.",
})

var Glossary = Sections.MustRegister("§Glossary", Section{
	Slug:  "§Glossary",
	Kind:  PLUMBING,
	Canon: "The methodology's controlled vocabulary (framework-side).",
	Narrative: "This module IS the authoritative membership list of admitted methodology terms. RULE: every §-token " +
		"used in Hotam-Spec framework module docs MUST appear here as a Term entry, and every Term entry MUST be referenced " +
		"in at least one framework module doc. Domain-side business terms (R-ids, axis slugs, stakeholders) live in the " +
		"domain's graph.json — not here.",
	Why: "WHY a generated controlled vocabulary: terminology drift is its own kind of invisibility — 'axis'/'dimension', " +
		"'resolver'/'owner', 'conflict'/'tension' fragment the methodology language without it. The vocabulary and its mirror " +
		"(docs/gen/GLOSSARY.md) are generated from the same source so they cannot drift from each other.",
})

var Scope = Sections.MustRegister("§Scope", Section{
	Slug:  "§Scope",
	Kind:  DISCIPLINE,
	Canon: "An operator's sub-domain as a PROJECTION (id-set view) over the shared graph, not a copy.",
	Narrative: "A Scope is a VIEW (a filtered set of ids) computed from the single shared TensionGraph by prefix match " +
		"on typed anchors. No node is ever copied into a sub-operator's own storage; two operators' Scopes may name the " +
		"SAME Requirement/Conflict/Assumption id, and that overlap is not an error — it is rendered visibly rather than " +
		"hidden by a hard partition.",
	Why: "WHY prefix-projection over the graph, not a copied sub-graph: a copy forks — the moment two operators each hold " +
		"their OWN Requirement objects, editing one cannot be guaranteed to reach the other, and the single writer " +
		"(internal/proposal, applied via `hotam apply-proposal`) is defeated. A Scope computed as id-sets over the one graph " +
		"can never drift from it: re-run the projection, get the current view, always.",
})

var Ticket = Sections.MustRegister("§Ticket", Section{
	Slug:  "§Ticket",
	Kind:  PLUMBING,
	Canon: "A durable on-disk work item: frontmatter header + Markdown body, mutated only through the ticket_* tools.",
	Narrative: "A ticket lives at tickets/<status>/T-<n>.md with a frontmatter header and a Markdown body. It is mutated " +
		"only through the ticket_* tools (create, edit, move, comment, show, list), which auto-maintain its status-and-text " +
		"History so every transition and edit is recorded for anti-relitigation.",
	Why: "WHY on-disk tickets (not in-graph state): a ticket is operational scratch tracked in the filesystem, " +
		"deliberately outside the tension graph so it cannot pollute the content-free substrate. The auto-maintained " +
		"History makes the work-item's lifecycle auditable without a hand-kept changelog.",
})

var Proposal = Sections.MustRegister("§Proposal", Section{
	Slug:  "§Proposal",
	Kind:  PROCESS,
	Canon: "Structured operator→resolver change proposals.",
	Narrative: "The closed loop's ACT half: the AI operator emits a structured proposal (ProposedRequirement / " +
		"ProposedConflictTransition / ProposedRejection / ProposedConflict), the resolver approves it out-of-band (review + " +
		"greenlight), and `hotam apply-proposal` (internal/proposal) mechanically writes the change to the active domain's " +
		"graph.json + runs the regen+verify pipeline. No free-text AI editing of source.",
	Why: "This honors R-ai-presents-not-decides (the AI never closes a conflict silently) AND R-active-loop-playbooks " +
		"(each what_now band has a playbook + a mechanical apply path). The AI TRANSLATES outcomes into typed proposals but " +
		"never decides; the resolver decides and the mechanical writer lands it deterministically.",
})

var Closure = Sections.MustRegister("§Closure", Section{
	Slug:  "§Closure",
	Kind:  DISCIPLINE,
	Canon: "Per-action verify: did the proposal remove its diagnosis?",
	Narrative: "After apply-proposal writes + regens + the gate greens, closure asserts the triggering diagnosis was " +
		"actually removed. The enforcer-name -> go-test node-id resolution is shared (internal/gate) so both the T1 " +
		"tiered gate and the structural invariant check_enforced_by_resolvable use ONE resolution algorithm.",
	Why: "WHY per-action closure (R-verify-closure-per-action): landing a proposal that greens the suite but does NOT " +
		"remove the triggering diagnosis is a false victory — the gate passes yet nothing advanced. Closure makes 'the " +
		"action actually fixed what it was meant to fix' a structural exit condition, not a hope.",
})

var Tick = Sections.MustRegister("§Tick", Section{
	Slug:  "§Tick",
	Kind:  PROCESS,
	Canon: "The closed-loop diagnostic driver (advisory, M32 conservative): one cycle loads the graph, diagnoses, and emits a TickReport for resolver attention.",
	Narrative: "The Drive organ runs the closed loop one cycle: load the graph, call diagnose() (all_violations + " +
		"reflection findings + attention signals), and emit a TickReport. It is ADVISORY (M32 conservative): it presents the " +
		"prioritized next-action surface to the resolver; it does not itself mutate the graph.",
	Why: "WHY advisory (not auto-acting): the hard boundary forbids the AI from closing anything silently; the Tick's " +
		"job is to make the operator's state and next correct action visible, not to take the resolver's decision. A " +
		"structural failure in all_violations outranks every softer signal because a malformed graph makes all downstream " +
		"diagnosis unreliable.",
})

var Conscience = Sections.MustRegister("§Conscience", Section{
	Slug:  "§Conscience",
	Kind:  DISCIPLINE,
	Canon: "The Hypothesis property-test sweep over the critical-core invariants — does my OWN edit introduce a contradiction?",
	Narrative: "The methodology's critical core is the narrow set of invariants whose violation would silently break " +
		"the hard boundary or anti-drift. The §Conscience property-test sweep (go test ./internal/invariants/...) runs over " +
		"this critical core; `hotam all-violations` runs the full set (both rings — critical core and secondary).",
	Why: "WHY a conscience (narrow scope, M7): property-testing the WHOLE invariant surface is expensive and noisy; " +
		"scoping the conscience to the critical core keeps the property sweep fast and focused on the paths by which a " +
		"contradiction could be INTRODUCED without being seen. Secondary invariants pass the same suite but are not the " +
		"primary conscience boundary.",
})

var Constitution = Sections.MustRegister("§Constitution", Section{
	Slug:  "§Constitution",
	Kind:  PLUMBING,
	Canon: "The operator's boot sequence — the generated reconstitution from the substrate's SETTLED laws.",
	Narrative: "The Constitution is the generated index of SETTLED requirements grouped by topic, rendered into " +
		"CLAUDE.md as the operator's boot-time reconstitution. It is regenerated from the executable model (module docs + " +
		"graph) by `hotam gen-spec` (internal/generator), so the operator re-boots from substrate, not from stale prose.",
	Why: "WHY a generated constitution (R-operator-prompt-from-substrate): the operator's prompt must be a projection " +
		"of the substrate's SETTLED laws, never hand-maintained prose that drifts from the graph. Regenerating it from the " +
		"executable model makes drift structurally impossible — the constitution IS the substrate, rendered.",
})

var Reflection = Sections.MustRegister("§Reflection", Section{
	Slug:  "§Reflection",
	Kind:  DISCIPLINE,
	Canon: "The operator's P0 self-diagnosis band — named predicates that diagnose the operator's OWN readiness.",
	Narrative: "Every P0 REFLECTION condition the harness can raise MUST be a named, pure, graph-only predicate in this " +
		"module — draft-overhang, unenforced-settled, over-budget-operators, dead-assumption-on-enforcer, derived-but-" +
		"unbuilt, implements-decay, replaces-edge-migration, all-members-rejected — composed by what_now via all_findings() " +
		"in REFLECTION_PREDICATES order, never re-inlined in tool code.",
	Why: "WHY a first-class module (mirror of §Invariants): the check_* layer diagnoses the domain graph's structural " +
		"form, but the operator's own readiness lived as tool-inlined code — important-yet-invisible. Named predicates give " +
		"each self-diagnosis condition a stable, testable anchor and keep the harness a thin renderer over substrate. WHY " +
		"ranked P0 (above §Invariants P1 STRUCTURE): an operator that cannot see its own state is worse than a malformed graph.",
})

var Process = Sections.MustRegister("§Process", Section{
	Slug:  "§Process",
	Kind:  ONTOLOGY,
	Canon: "The opt-in behavioral aspect (M12): a Lifecycle + ordered Steps + roles_required + drives_entities.",
	Narrative: "A Process is a Lifecycle + ordered Steps + the roles it requires + the entities it drives. It is the " +
		"richest contradiction surface (M12) because: two processes driving one entity along incompatible state paths is " +
		"the canonical hidden contradiction; a step requiring a role no actor provides is a structural dead-end; a method " +
		"postcondition that violates an entity invariant is a real conflict surfaced as a Conflict node on a behavioral axis.",
	Why: "WHY a behavioral aspect: before §Process the ontology could only express static claims; behavioral " +
		"contradictions (two processes driving one entity to incompatible states) were invisible. Making Process a " +
		"first-class aspect with drives_entities (resolving to declared EntityType slugs) and Step.invokes (resolving to a " +
		"real Lifecycle transition) surfaces those contradictions as addressable Conflicts.",
})

var Goal = Sections.MustRegister("§Goal", Section{
	Slug:  "§Goal",
	Kind:  ONTOLOGY,
	Canon: "A first-class target-state type (M19): distinct from a static Requirement because it carries a MOVING TARGET that yields a Gap driving a Process.",
	Narrative: "A Goal is a target-state predicate (kind in TARGET_KINDS: GRAPH_PROPERTY | BUSINESS_STATE | " +
		"ENTITY_STATE) plus what it targets. It is distinct from a static Requirement claim because it carries a MOVING " +
		"TARGET — the distance between the target_state and the current state is a Gap, and that Gap drives Process " +
		"execution as the measurable work remaining.",
	Why: "WHY a type (M19), not a Requirement facet: a Requirement is a static claim ('the system shall ...'), but a " +
		"target that moves ('reach state X by ...') needs a current-vs-target Gap to drive a Process. Promoting Goal to its " +
		"own type keeps the static-claim semantics of Requirement clean. Goal conflicts reuse the existing Conflict " +
		"connector node on a goal-tension axis — no new GoalConflict type.",
})

var Context = Sections.MustRegister("§Context", Section{
	Slug:  "§Context",
	Kind:  PLUMBING,
	Canon: "The operator's working-context fullness measurement — MEASURED from a runtime stamp, never guessed.",
	Narrative: "§Context is the measurement of how full the operator's working context is, read from a runtime stamp " +
		"(spec/.runtime/context.json written by the context_producer). It is the first cipher of the three-cipher pulse " +
		"(top action / debt / context) re-injected each turn so the operator is never lost. It is MEASURED, never guessed: " +
		"the framework measures only if the local stdin payload honestly carries ctx_pct.",
	Why: "WHY measured, not guessed (R-measure-context-size): a guessed context figure is theater — it lets the operator " +
		"claim 'I have room' without evidence. Measuring from a runtime stamp keeps the cipher an honest vital sign and " +
		"names the host boundary: the framework will not touch host cooperation to measure, so an unmeasured cipher " +
		"explicitly says so rather than fabricating a number.",
})

var Atoms = Sections.MustRegister("§Atoms", Section{
	Slug:  "§Atoms",
	Kind:  PLUMBING,
	Canon: "The atomized methodology docs generated under docs/methodology/atoms/ from SETTLED requirements grouped by topic.",
	Narrative: "Atoms are the atomized methodology documents generated from SETTLED requirements, one coherent topic " +
		"per atom, rendered under docs/methodology/atoms/. They are the reader-facing distillation of the substrate's " +
		"SETTLED laws — the prose mirror of the graph, generated so it cannot drift.",
	Why: "WHY atomized docs (R-atomicity-ratchet-no-growth): a single monolithic methodology doc hides compound claims " +
		"and grows without bound; atomizing by topic keeps each doc atomic (one claim, one WHY) and lets the atomicity " +
		"ratchet ensure the surface never grows a compound claim that should itself be split.",
})

var Agent = Sections.MustRegister("§Agent", Section{
	Slug:  "§Agent",
	Kind:  PLUMBING,
	Canon: "A scoped sub-operator directory (domains/<domain>/agents/<name>/) with a scope declaration, CLAUDE.md, tools/, agents/, and docs/ subdirectories.",
	Narrative: "An Agent is a scoped sub-operator: THIS SAME seed, narrowed — same Role text + narrower scope line, same " +
		"mediation loop, with thinking + constitution filtered by SCOPE prefixes. Every agent directory must contain an " +
		"'agents/' subdirectory because every agent is itself a potential director that can spawn sub-agents (the agents/ " +
		"subdir is the recursion slot).",
	Why: "WHY a directory (not a config record): an agent is recursive — it can itself spawn sub-agents, so it carries " +
		"its own tools/, agents/ and docs/. One domain with zero active sub-agents collapses to exactly one CLAUDE.md; " +
		"sub-agent crystals materialize only at real spawn time. Sub-agents return CONCLUSIONS only — shared objects as " +
		"explicit border, never raw context.",
})

var Domain = Sections.MustRegister("§Domain", Section{
	Slug:  "§Domain",
	Kind:  PROCESS,
	Canon: "A self-contained business domain directory (domains/<name>/) with a manifest, graph.json, tools/, agents/director/, docs/gen/, and CLAUDE.md.",
	Narrative: "Exactly ONE function, resolve_active_domain(domains_root), implements the active-domain resolution " +
		"order shared by every tool: (1) HOTAM_SPEC_ACTIVE_DOMAIN env var, (2) domains/.active-domain pin file (committed, " +
		"version-controlled), (3) first domains/<name>/ alphabetically (deterministic fallback so a fresh repo is never " +
		"'lost'). It returns the domain NAME; callers compose their own paths.",
	Why: "WHY a single resolver (the bug this closes): before this module, three copies of the env->pin->alphabetical " +
		"walk lived in graph, gen_spec and apply_proposal — editing one without synchronizing the others was a real source " +
		"of silent divergence (observed: apply_proposal ignored the env var and always targeted the alphabetical-first " +
		"domain). Centralizing the walk makes divergence structurally impossible — there is only one implementation.",
})

var Entity = Sections.MustRegister("§Entity", Section{
	Slug:  "§Entity",
	Kind:  ONTOLOGY,
	Canon: "Domain-declared business concept with its own lifecycle (M12 opt-in aspect): EntityType + EntityField + EntityInstance.",
	Narrative: "A domain (e.g. a finance app) declares EntityType('customer', lifecycle=…, fields=…) in its " +
		"build_graph(); the framework accepts it as first-class and the check_entity_* family covers every declared type " +
		"by iteration. No new code per entity — coverage is generated by iterating g.entity_types and g.entities. It " +
		"reuses the §Lifecycle keystone: EntityType.lifecycle is a Lifecycle, validated by check_lifecycle_wellformed — no " +
		"parallel state machinery for entity states.",
	Why: "WHY a declarative aspect (not framework code per entity): domain-declared entities keep the framework " +
		"content-free while letting each domain model its own business concepts with real state machines. Iterating " +
		"declared types for coverage means a new EntityType needs no new check code — the invariant surface scales with the " +
		"domain's declarations, not with framework edits.",
})

var Signoff = Sections.MustRegister("§Signoff", Section{
	Slug:  "§Signoff",
	Kind:  ONTOLOGY,
	Canon: "The frozen provenance payload (decided_by + date + verbatim + instrument + chosen_variant) attached to a node a human resolver decided on; not a node type, a payload like Variant.",
	Narrative: "Every resolver-approved transition (Conflict -> DECIDED/HELD, Assumption -> HOLDS/DEAD/IMPLEMENTS) MUST " +
		"be auditable from the substrate. Before this record, decided_by lived only in the gitignored proposal JSON and " +
		"evaporated when the writer landed. The Signoff payload closes that gap — it is NOT a new node type, it is a frozen " +
		"dataclass payload attached to the node the human already governed (the same rationale that keeps Variant a payload " +
		"on Conflict).",
	Why: "WHY instrument is an explicit SEAM (not a hidden assumption): the resolver has not yet decided whether to bind " +
		"provenance to git commit authorship or a cryptographic signature; instrument names HOW this signoff was captured " +
		"today and leaves room for git/crypto without reshaping the record. WHY date is the only (coarse) timestamp: this " +
		"wave fixes provenance, not temporal modeling — ISO YYYY-MM-DD is enough to say 'WHEN did the resolver sign'.",
})

var Attention = Sections.MustRegister("§Attention", Section{
	Slug:  "§Attention",
	Kind:  DISCIPLINE,
	Canon: "The agent-agnostic registry of attention-codes (ATTENTION_SOURCES): collect() runs the sources and returns typed AttentionSignals.",
	Narrative: "Every signal an agent is obliged to notice — 'here is what needs your attention right now' — is " +
		"produced by a NAMED source in a single registry, and collect(g, ...) runs the registry and returns a flat list of " +
		"typed AttentionSignal records. GRAPH sources read only the in-memory TensionGraph (pure, deterministic); " +
		"RUNTIME-FS sources read the filesystem and are injected by the live consumer, never built into the core registry. " +
		"For a live agent, collect(...) is a SUPERSET of diagnose_signals(g); for the substrate, only the deterministic " +
		"diagnose subset is consumed.",
	Why: "WHY a core (not just tool functions): the 'what needs attention' signals were scattered — half in diagnose(g), " +
		"half as CLI-only bands owned by `hotam what-now`. There was no single, agent-agnostic, importable-as-a-library source. " +
		"A core registry means no agent, on any platform, has to remember where the signals live: it runs the core and " +
		"reads the list; the platform adapter is merely one consumer, not the owner.",
})
