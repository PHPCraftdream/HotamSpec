// Origin: one-time bulk extraction by .scratch/selfspec-codegen (task #345, RAC-A) from
// domains/hotam-spec-self/graph.json. That program was a THROWAWAY (run once, never
// committed to git) and is NOT a regeneration path -- it went graph->Go, the OPPOSITE of
// today's sync direction. These literals are HAND-MAINTAINED Go source: edit them directly,
// then mirror Go->graph via `hotam sync-self` (dry-run by default; --confirm-hash to write).
// The "generated" framing describes this file's ORIGIN (task #345's one-time bulk pull from
// the pre-existing JSON graph), not an ongoing regeneration workflow.
//
// Topic: Boot and Orientation.
//
// STRUCTURAL fields only: Claim, Why, Owner, Status, Relations, Assumptions,
// Enforcement, EnforcedBy, Enforceability, MTag, Summary, CreatedAt, SettledAt,
// BlockedOn, ImplementedBy, VerifiedBy, SourceRefs, DeclOrder. EVENT fields
// (History, GateSignoffs, LastReviewedAt, ReviewAfter, Evidence) are
// deliberately absent — MergeIntoGraph passes them through from the graph's
// existing node untouched.
package selfspec

import "github.com/PHPCraftdream/HotamSpec/internal/ontology"

var _ = Requirements.MustRegister("R-boot-cite-in-first-sentence", ontology.Requirement{
	ID:             "R-boot-cite-in-first-sentence",
	Claim:          "The operator shall cite at least one of the three substrate facts in the first sentence of any substantive reply.",
	Owner:          "ai-agent",
	Status:         "SETTLED",
	Why:            "Atom of R-boot-from-substrate (WHEN to cite). Citing anchors the reply in the live substrate, proving the operator actually loaded it and is not parroting from memory.",
	Assumptions:    []string{"A-compaction-loses-working", "A-bootstrap-self-applies"},
	Relations:      []ontology.Relation{{Kind: "replaces", Target: "R-boot-from-substrate"}},
	Enforcement:    "PROSE",
	EnforcedBy:     []string{},
	MTag:           "",
	Enforceability: "INHERENTLY_PROSE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-06-30",
	SourceRefs:     []string{},
	DeclOrder:      111,
})

var _ = Requirements.MustRegister("R-boot-cite-measured", ontology.Requirement{
	ID:             "R-boot-cite-measured",
	Claim:          "A Stop hook shall lexically check whether the first sentence of the operator's last reply in the transcript contains a typed anchor (R-/C-/A-/OP-/GOAL-/section-sign), logging the result to spec/.runtime/boot-cite-log.jsonl, checked as a form-level (not substance-level) signal.",
	Owner:          "ai-agent",
	Status:         "REJECTED",
	Why:            "REJECTED — REPLACES by R-agent-conduct-is-rules-not-tests: измеритель мерил форму (лексический чек первого предложения), не суть заземления; 10-13% \"соблюдения\" писались в журнал, который ничто не читало — form-metric theatre. Правило R-boot-cite-in-first-sentence остаётся честным PROSE. — (was: R-boot-cite-in-first-sentence (PROSE) has never had any mechanical trace of compliance. tools/boot_cite_status.py's writer half reads the Stop hook's transcript_path payload, extracts the last assistant text block's first sentence, and lexically tests it for an anchor token; the reader half (compute_boot_cite_status) answers what fraction of the last N logged replies complied. HONESTY BOUNDARY, explicit in the tool docstring: this measures the citation RITUAL (a token-shaped string appears), never the citation's TRUTH (that the anchor is relevant or that graph reality was actually confronted) -- R-boot-cite-in-first-sentence itself stays PROSE/STRUCTURAL; this atom only claims the measurable slice exists and is tested.)",
	Assumptions:    []string{"A-finite-context-operators"},
	Relations:      []ontology.Relation{},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"test_tool_boot_cite_status.py::test_write_from_payload_cited_true", "test_tool_boot_cite_status.py::test_compute_status_mixed_and_windowed"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-07-02",
	SettledAt:      "",
	SourceRefs:     []string{},
	DeclOrder:      231,
})

var _ = Requirements.MustRegister("R-boot-from-substrate", ontology.Requirement{
	ID:             "R-boot-from-substrate",
	Claim:          "The operator shall begin every new turn by re-loading three facts from the substrate — current context %, the top what_now action, and the SETTLED-DRAFT-UNENFORCED ratio — and cite at least one of them in the first sentence of any substantive reply.",
	Owner:          "ai-agent",
	Status:         "REJECTED",
	Why:            "REJECTED — REPLACES split into R-boot-reload-three-facts + R-boot-cite-in-first-sentence (wave 2, decided by framework-author 2026-06-30) — (was: REJECTED — REPLACES split into R-boot-reload-three-facts + R-boot-cite-in-first-sentence (wave 2, decided by framework-author 2026-06-30) — (was: Without this, the operator knows the spec but lives in session memory; CLAUDE.md is the only file the harness auto-loads, so the boot ritual MUST live there (not in CONSTITUTION.md, which is referenceable but not auto-loaded). This is the structural fix for 'knows the spec vs lives by it'.))",
	Assumptions:    []string{"A-compaction-loses-working", "A-bootstrap-self-applies"},
	Relations:      []ontology.Relation{},
	Enforcement:    "STRUCTURAL",
	EnforcedBy:     []string{},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "",
	SourceRefs:     []string{},
	DeclOrder:      13,
})

var _ = Requirements.MustRegister("R-boot-reload-three-facts", ontology.Requirement{
	ID:             "R-boot-reload-three-facts",
	Claim:          "The operator shall begin every new turn by re-loading three facts from the substrate: current context %, the top what_now action, and the SETTLED-DRAFT-UNENFORCED ratio.",
	Owner:          "ai-agent",
	Status:         "SETTLED",
	Why:            "Atom of R-boot-from-substrate (WHAT to load). Without re-loading from the substrate, the operator lives in session memory and drifts from the graph's live state.",
	Assumptions:    []string{"A-compaction-loses-working", "A-bootstrap-self-applies"},
	Relations:      []ontology.Relation{{Kind: "replaces", Target: "R-boot-from-substrate"}},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"TestBuildLiveState_RendersOnFixture"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{},
	DeclOrder:      110,
})

var _ = Requirements.MustRegister("R-orientation-faq-answerable", ontology.Requirement{
	ID:             "R-orientation-faq-answerable",
	Claim:          "An AI agent (or a new human reader) orients in this project fast: for every basic onboarding question a domain DECLARES in its own manifest.json \"orientation_faq\" opt-in list (e.g. what is this project / what is the requirement lifecycle / who decides what / what is currently blocked / where is the full requirements list), the answer MUST be reachable from the domain's generated crystal in at most ONE hop -- NEVER buried behind a chain of pointers the agent would have to follow blind -- and, when the entry declares an \"assert\", the answer MUST ALSO be LIVE-TRUE against the domain's current graph state, not merely a lexical leftover from when the entry was authored. An answer is reachable in <=1 hop when EITHER (a) the manifest entry's declared keywords ALL appear inline in the crystal's text (the answer is present with ZERO hops), OR (b) the manifest entry's declared link (a repo-root-relative path, the SAME convention the crystal's own cross-references use) appears in the crystal's text (as a markdown link OR a bare path) AND resolves to a REAL EXISTING FILE on disk; (a)/(b) are ALTERNATIVES, either satisfies the keywords/link contract. When the entry ALSO declares (c) an \"assert\" (kind: gate_signoff_count | conflict_count_by_lifecycle | requirement_count_by_status, backed by internal/graphfacts's live graph-fact tallies), the assert's live (count, total) pair must satisfy the entry's declared \"expect\" (all / none / {op, value}) and/or its declared \"phrase\" ({count}/{total}-templated, live-substituted, required present in the crystal or linked file) -- this third signal is ADDITIVE, not an alternative: it must pass ALONGSIDE whatever keywords/link contract the entry carries, never substitute for it. A domain that has NOT declared an orientation_faq list makes NO orientation promise and this requirement is an honest no-op for it -- exactly the same opt-in boundary discipline:\"full\" / the committed SPEC.md / the vendored recorder already draw between the engine's structural floor and a domain's own stricter, explicitly-declared contract.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "SETTLED: making 'an agent orients fast' a MECHANICALLY CHECKABLE property (not a hope or a demo-session feeling) is the orientation showcase the framework sells to a customer -- a measurable, drift-proof guarantee that declared onboarding answers cannot silently disappear when a crystal is rewritten, a doc is moved, or a section is reworded. The structural half is check_orientation_faq_answered (internal/invariants/orientation_faq.go), which reads each declared manifest entry via loader.ResolveOrientationFAQ (internal/loader/orientation_faq.go -- the SAME opt-in reader shape ResolveDiscipline/ResolveGenProfile already establish: honest no-op when the field is absent, real per-entry gate when present) and proves each declared question is answerable from the crystal in <=1 hop. The check mirrors the established filesystem-aware, honest-no-op-when-absent shape of check_recorder_current / check_spec_md_current / check_settled_requires_scenario: a domain without an orientation_faq list contributes ZERO violations regardless of how sparse its crystal is ('no committed opt-in = no lie'), and a domain WITH the list is held to its own declared contract. The crystal-path resolution (resolveCrystalPath: local <domainDir>/CLAUDE.md for a consumer domain, else repo-root CLAUDE.md for the active domain) mirrors cmd/hotam.resolveClaudeMDPath's intent via a robust existing-file priority. This domain (hotam-spec-self) carries the real self-example: 5 questions covering purpose / lifecycle / decision-authority / current-blockers / full-requirements-list, each answerable inline (keywords) or via a one-hop link to a real docs/gen file. TASK #321 (R3-semantic-faq) EXTENSION: the original two-signal (keywords/link) rule proves a phrase is lexically PRESENT in the crystal, never that the phrase is still semantically TRUE relative to the graph's current state -- this session hit exactly this bug (a manifest FAQ entry claimed '27 of 32 requirements' and kept passing the keyword check long after the graph reached 32/32, fixed by hand in tasks #318/#322 without closing the underlying design gap). The new optional \"assert\" field (loader.OrientationFAQAssert, internal/loader/orientation_faq.go) ties an entry to a LIVE query over the graph (internal/graphfacts/facts.go: GateSignoffTally/GateFrontier/ConflictLifecycleTally/RequirementStatusTally) instead of, or alongside, the static keyword/link signals -- evalOrientationAssert (internal/invariants/orientation_faq_assert.go) evaluates it and fails closed on any unrecognized kind, undeclared gate stage, malformed expect, or an assert declaring neither expect nor phrase. This claim text was updated (task #321) to describe the new three-signal rule the code now implements; it was NOT applied unilaterally -- per R-decided-needs-human-signoff/R-ai-presents-not-decides this UPDATE is drafted as proposals/draft-R-orientation-faq-answerable-assert.json for human review and separate landing, not applied by the same task that authored the code. The self-hosting hotam-spec-self domain's own orientation_faq entries remain keyword/link-only (zero assert entries) -- migrating them to use assert is explicitly a separate follow-up task, not part of this engine-portion change.",
	Assumptions:    []string{"A-bootstrap-self-applies"},
	Relations:      []ontology.Relation{},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"check_orientation_faq_answered"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-07-21",
	SettledAt:      "2026-07-21",
	SourceRefs:     []string{"internal/invariants/orientation_faq.go", "internal/invariants/orientation_faq_assert.go", "internal/loader/orientation_faq.go", "internal/graphfacts/facts.go", "domains/hotam-spec-self/manifest.json"},
	DeclOrder:      0,
})

var _ = Requirements.MustRegister("R-speak-by-reference", ontology.Requirement{
	ID:             "R-speak-by-reference",
	Claim:          "An operator shall communicate by reference, ensuring every assertion cites at least one concrete anchor in the info-space.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "SETTLED (P5): the references-not-content discipline is now structurally bound. The structural half is enforced by check_section_anchors_known (internal/invariants/self_reference.go): it ranges over every registered invariant in the All registry and fires a STRUCTURE violation whenever an invariant's Canon is nil -- i.e. an invariant that fails to reference a registered methodology.Section. Every Invariant.Canon is a TYPED *methodology.Section pointer obtained at registration time from methodology.Sections.MustRegister -- a typed pointer into the registry cannot dangle, so the only remaining failure mode (a nil Canon) is what this check catches directly. (Historical note: the original mechanism walked every source file with ast, extracted section-sign tokens from docstrings via regex, and checked each against the glossary; that AST+regex walk is structurally unnecessary once the Canon is a typed pointer.) The §Glossary itself lives as DATA in methodology.Sections (internal/methodology/sections_data.go), so an invented §-token cannot compile. The advisory what-now/inspect output cites anchor ids in every action (the Finding/Signal Target fields). Together these make reference-not-content structurally visible and machine-checked. (Wave 1 seed-coherence pass: enforced_by entries that named bare doc paths or since-renamed tests were corrected to the enforcer that actually covers this claim, under the check_enforced_by_resolvable discipline.)",
	Assumptions:    []string{"A-bootstrap-self-applies"},
	Relations:      []ontology.Relation{},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"check_section_anchors_known"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{"internal/invariants/self_reference.go", "internal/methodology/section.go", "internal/methodology/sections_data.go"},
	DeclOrder:      52,
})

var _ = Requirements.MustRegister("R-speak-domain-register-by-default", ontology.Requirement{
	ID:             "R-speak-domain-register-by-default",
	Claim:          "An operator ALWAYS speaks in the language of the LAYER it is currently working in, and AVOIDS the language of any HIGHER layer whenever the same thing can be said in the current layer's own terms. The layers, bottom-up, are: (1) the active CONSUMER domain's own language -- its business concepts, names, stages, and roles (e.g. for gsm: FT numbers, P-G0..P-G4 gates, PM/BA/TechLead roles, the essence of a conflict or stage); (2) the methodology-constitution's own terms (the prat layer); (3) the Hotam engine's internals (the graph, R-/C-/A-/OP- anchors, SETTLED/ENFORCED/DRAFT statuses, check_* invariants, `hotam` commands). Escalation to a higher layer's language is permitted ONLY when: (a) the human explicitly asks to switch to that layer, OR (b) the human's own message already uses that higher layer's terms. Absent both conditions, the agent stays at the current layer's language even when it could formally reference something higher; the first answer leads with the essence of the current layer, NEVER with the name of the Hotam framework. R-speak-by-reference stays in force for the TRANSLATE/PRESENT/LAND steps of the mediation loop, where anchor-citation is mandatory to the act itself; this requirement governs the DEFAULT register of every other answer.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "SETTLED: the demo-session defect was a pure default-register bug -- on an ambiguous question the agent dived into a HIGHER layer's language (Hotam engine internals: Lifecycle, SETTLED/DRAFT, check_*) instead of staying in the CURRENT layer's language (the consumer domain's own terms: FT numbers, P-G0..P-G4 gates, roles, conflict essence); after a manual re-prompt the same model answered correctly in the current layer's terms, proving the data was good and only the default register set by the Role block was wrong. The general principle is a LAYER HIERARCHY (current consumer domain < methodology-constitution < Hotam engine): escalate to a higher layer ONLY on an explicit user request, or when the user's own message already uses higher-layer terms -- never unprompted, even when a higher-layer reference is formally available. The structural half is RenderOperatorRoleBlock (internal/generator/claudemd.go), which renders this layer-hierarchy guidance into the Role block of EVERY generated CLAUDE.md/AGENTS.md/GEMINI.md so the default register is fixed at boot, before any input is processed. The live-conversation half is inherently prose -- no invariant can mechanically verify an LLM's spoken register -- so the requirement is classified INHERENTLY_PROSE, like R-ai-presents-not-decides, with the structural rendering documented here rather than in enforced_by.",
	Assumptions:    []string{"A-bootstrap-self-applies"},
	Relations:      []ontology.Relation{},
	Enforcement:    "STRUCTURAL",
	EnforcedBy:     []string{},
	MTag:           "",
	Enforceability: "INHERENTLY_PROSE",
	Summary:        "",
	CreatedAt:      "2026-07-21",
	SettledAt:      "2026-07-21",
	SourceRefs:     []string{"internal/generator/claudemd.go"},
	DeclOrder:      0,
})

var _ = Requirements.MustRegister("R-status-single-command-summary", ontology.Requirement{
	ID:             "R-status-single-command-summary",
	Claim:          "The hotam CLI shall provide a `status` command (`hotam status [--domain <path>] [--today YYYY-MM-DD] [--json]`) that composes what-now's top action + debt counts (internal/diagnose), due's freshness counts (internal/freshness), and all-violations' violation count (internal/invariants) into a single compact summary, so an agent does not need to run those three commands separately to reconstruct the same picture. It reuses the same underlying functions those commands call rather than reimplementing their logic, never gates (exit code always 0), and its --json output uses flat, explicitly named fields (top_action, settled_count, enforced_count, closeable_debt_count, overdue_count, never_reviewed_count, violation_count, node_count) parseable in one shot.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "External review (TaskList P2 item #80) observed that an agent orienting itself has to run `what-now`, `due`, and `all-violations` separately and mentally merge the results, burning context on three graph loads and three renderings to reconstruct one picture. This mirrors the existing three-cipher pulse the root CLAUDE.md's LIVE-STATE block already carries (R-three-cipher-pulse-structurally-injected) but as an ad hoc, freshly-runnable CLI command rather than a static crystal block — useful mid-session when the crystal is stale or when a different domain than the resident one needs a quick pulse. Composition, not duplication: buildStatusReport (cmd/hotam/status.go) calls diagnose.DiagnoseSignals (the same call formatSignals/BuildLiveState make for what-now's top action and the LIVE-STATE debt line), freshness.ClassifyGraph (the same call buildDueReport makes), and invariants.AllViolations (the same call cmdAllViolations makes) on one already-loaded graph — it introduces no new definition of 'closeable debt', 'overdue', or 'violation'. Human-readable output is a handful of labeled lines (top action / debt / freshness / violations / graph), not a full report, matching the point of the command: a single-shot, low-context pulse. Exit code is always 0 like due/inspect/confront — status is advisory, not a gate; all-violations remains the hard gate command for invariant violations. ENFORCED 2026-07-13: TestStatusToolRegisteredImplemented (internal/methodology/methodology_test.go) enforces the existence half — the `status` tool is registered Implemented with a real Purpose, so the registry entry read by cmd/hotam/tool_wiring.go and the generated EMBEDDED-TOOLS crystal block can never silently regress. The behavioral half — buildStatusReport's aggregated numbers actually agreeing with what-now/due/all-violations — is proven by TestBuildStatusReport_MatchesWhatNowDueAllViolations (cmd/hotam/status_test.go), which independently recomputes every field via diagnose.DiagnoseSignals / freshness.ClassifyGraph / invariants.AllViolations on the same fixture graph; it cannot be named in enforced_by directly because internal/gate's Test*-name resolver (check_enforced_by_resolvable) is scoped to internal/**/*_test.go and cmd/hotam tests use unexported cmd/hotam helpers unreachable from internal/, but it is the test that actually guards against drift and is cited here for the audit trail. UPDATE 2026-07-13 (TaskList P2 item #81, external review): 'parseable in one shot' is refined to also mean array-typed fields are never JSON `null` for the empty case — a nil Go slice (`var x []T`) and an omitted/`[]` field are semantically identical ('no rows') but encoding/json distinguishes them, and a `null` where an agent expects an array is a real footgun (`for x of null` throws in JS; OpenAPI/JSON-Schema generators typed the field as an array). This was not hypothetical: domains/hotam-spec-self/graph.json itself carried `\"enforced_by\": null` on 2 SETTLED requirements and `\"source_refs\": null` on 3, which flowed unchanged through `hotam req show --json` into the CLI's machine-readable output before this fix. Every array-typed field across every `--json` command (`due`, `confront`, `inspect`, `req show/list/search/context/related`) now normalizes a nil slice to `[]` at the boundary where it is marshaled — either at construction (`due.go`'s DueReport, list.go's Search) or at the CLI-facing card-building step (query/show.go's requirementToCard/conflictToCard, which is also where a domain's own possibly-null persisted graph.json array field is normalized on its way OUT to an agent, without touching the loader or the persisted format itself). Regression-pinned at the byte level (asserting the literal marshaled JSON text, not just a Go-level len()==0 check) in cmd/hotam/due_test.go, internal/diagnose/confront_test.go, and internal/query/query_test.go. This is a refinement of an existing claim, not a new architectural commitment — no new node was created for it.",
	Assumptions:    nil,
	Relations:      []ontology.Relation{{Kind: "refines", Target: "R-three-cipher-pulse-structurally-injected"}},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"TestStatusToolRegisteredImplemented"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-07-13",
	SettledAt:      "2026-07-13",
	SourceRefs:     []string{"cmd/hotam/status.go", "cmd/hotam/status_test.go", "cmd/hotam/due.go", "internal/diagnose/confront.go", "internal/query/show.go", "internal/query/list.go", "internal/query/context.go"},
	DeclOrder:      0,
})
