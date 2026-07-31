# Independent result review of the code-authority-completion wave (W0–W4, tasks #388–#402, #404–#405)

Date: 2026-07-31 · Reviewer: `@oh` (Opus, adversarial, read-only) · Scope: commits `3404be0..6fd535b` on `master`.

Baseline documents this review measures against (all read in full):
`../../docs/reviews/2026-07-28-code-only-spec-discipline-review.md`,
`docs/reviews/2026-07-30-information-system-readiness.md`,
`docs/PLAN-code-authority-completion.md` (Часть 4 scoring table),
`CHANGELOG.md` § *Wave summary — code-authority-completion*,
`docs/reviews/2026-07-31-life-methodology-acceptance.md`.

**Method.** I did not re-diagnose the framework. I read the plan, the two prior reviews, the wave
summary and a sample of per-task CHANGELOG entries; then I read the actual diffs for the tasks I judged
most load-bearing (#388, #389, #390, #391, #392, #395, #396, #397, #399, #400) and the current source of
every check they added; then I ran the engine myself against all four real domains and executed two
independent refutation tests of my own construction. Everything marked **[verified]** below I reproduced
personally with a command; everything marked **[read]** I confirmed by reading the code, not the
CHANGELOG's description of it.

**Runs performed** (from a clean `go run ./cmd/hotam`, current `master`):

| target | result |
|---|---|
| `all-violations --domain PRAT-hotam/domains/prat` | 1 violation — `check_domain_claude_md_current` only. **Zero `check_claim_matches_scenario`** (was 19). |
| `all-violations --domain PRAT-hotam/domains/gpsm-sm` | 1 violation — `check_domain_claude_md_current` only. **Zero `check_claim_matches_scenario`** (was 22). |
| `all-violations --domain life/domains/life` | **0 violations — graph clean.** |
| refutation A: temp copy of `life`, `INFRASTRUCTURE:` marker removed from `CalendarService.CheckAvailability` | `check_public_surface_linked_or_marked` fired with the exact expected message. Gate is real. |
| refutation B: temp copy of `life`, scenario title of `TestCommitment_Break_RequiresRationale` blanked | `check_scenario_quality` fired (`rule 1 (non-empty title)`) + `check_spec_md_current` fired. Gate is real. |

---

## 1. Verdict per plan dimension (Часть 4 re-scored)

| измерение | план (до волны) | моя оценка (после) | почему |
|---|---:|---:|---|
| доказательная половина (тест как доказательство) | 8/10 | **8.5/10** | Real movement (`check_scenario_quality`, `RequirementState` surfaced, live-execution provenance in `brief`), but the quality bar itself is thinner than its own doc claims — see §3.3. The half that was already strong stayed strong; it did not become materially stronger. |
| модельная половина (объекты/поля/методы в доках) | 5/10 | **7.5/10** | The wave's biggest genuine win, and I confirmed it on a live domain: interfaces, interface methods, constructors, typed consts, struct tags, doc comments, and `object/value/port/mock/policy` classification all now render. What is still missing is real: no constraints *derived* from tags, and exported fields/types are outside the symmetric gate's scope (§2). |
| единый источник истины | 4/10 | **5.5/10** | `EntityType.ModelSymbol` + its resolvability check is a real link and is genuinely in live use (one EntityType in `life`), and #391 stopped vendored types leaking into the business model. But the link is optional and one-directional, nothing requires an EntityType to have one, and graph / registry / Go AST / test artifacts remain four stores connected by one thin optional pointer. |
| принуждение для нового домена | 5/10 | **6.5/10** | `life` was born fully obligated (three triggers, 0 violations, gates verified firing by me), and `init-project` now defaults to `--discipline full`. But the two escape hatches the plan itself named — `DRAFT` and `INHERENTLY_PROSE` — were not touched by this wave at all, and the ratchet that is supposed to make the new triggers one-way is **not actually latched on `life`** (§3.4). |
| «доки только из тестов» в строгом чтении | 2–4/10 | **3/10 (unchanged, deliberately)** | The plan explicitly redefined this goal (Часть 1.6) and the wave executed that redefinition honestly: it *measured* the number (≈49% authored prose on `hotam-spec-self`, ≈12% on `hotam-dev`) and corrected a false claim in `AUTHORED-SPEC-CONTRACT.md` §8.2 rather than moving the metric. Full derivation exists as `claim_authority:"scenario"` and **no domain in the world uses it — including `life`**. Honesty improved a lot; the metric did not move, and was not supposed to. |
| охват «любой ИС» | 5/10 | **6/10** | The movement here is evidential, not architectural: `life` demonstrates the object/lifecycle/port/mock/decision shape works for a genuinely non-software domain. Per Часть 3 no new first-class IS concepts were added, deliberately. One narrow slice of one domain is real evidence but thin evidence. |

**Aggregate judgment.** The wave did what it said it would do, and the single most important thing it
claimed — the W0.1 live regression — is genuinely and verifiably closed. The plan's own framing is
accurate and, unusually, not inflated: I found no dimension where the CHANGELOG overstates the result
in kind, though I found several places where it overstates it in *reach* (a mechanism that works only
where it is least needed — §3.1).

---

## 2. Genuinely closed vs partially closed vs not closed

### Genuinely closed (evidence-backed)

- **W0.1 — the live regression.** `check_claim_matches_scenario` now requires **both**
  `g.Discipline == DisciplineFull` **and** `g.ClaimAuthorityScenario`
  (`internal/invariants/claim_scenario_current.go:102-115` **[read]**). Both consumer domains dropped
  from 20/23 violations to 1/1, and the remaining one is a different, self-healing check **[verified]**.
  This is the wave's headline claim and it holds.
- **W0.3 — `Why` in the vendored mirror.** `internal/ontology/canon/requirement.go:64` now carries
  `Why string \`json:"why"\`` **[read]**; `MergeIntoGraph` still treats `Why` as structural
  (`internal/selfspec/merge.go:79-86`), so without the field the first `sync-domain` would indeed have
  blanked it. The e2e round-trip tests for both the ADDED and CHANGED path are real.
- **W0.4 — vendored/generated exclusion.** Replaced three ad-hoc cases with one banner registry
  (`knownGeneratedBannerFirstLines` / `IsGeneratedOrVendoredFile`), each entry derived from the
  producer's own `Banner` constant rather than hand-copied **[read]**. The regression test deliberately
  names a *real* domain file `spec/model/registry.go` to prove exclusion is by banner content, not path.
  This is the cleanest fix in the wave.
- **W0.5 — `ProposedGoal` / `ProposedEntityInstance`.** Both kinds now exist with CREATE paths and
  referential validation; the four orphaned `check_entity_instance_*` invariants now have something to
  validate **[read]**.
- **W1.1/W1.2 — AST scan and classification.** Confirmed live in
  `life/domains/life/docs/gen/MODELS.md` **[verified]**: `CalendarService` renders as
  `(interface, port)` with its own `InterfaceMethods`, `mockCalendarService` as `(struct, mock)` with
  its own `Methods`, plus a `Constants:` table, a `Functions:` table, and a `Typed errors:` table. The
  prior review's "самый тяжёлый пробел — методы интерфейсов" is closed.
- **W1.4/W1.5 — the two new gates.** Both are real gates, not decorative. I broke each one myself in a
  temp copy and both fired with precise, actionable messages **[verified]**. `check_public_surface_linked_or_marked`
  gates solely on `g.PublicSurfaceAuthorityLinked` (`internal/invariants/model_complete_symmetric.go:129-134`)
  and is genuinely independent of `discipline:"full"` **[read]**.
- **W2.2 — the `gate.MatchCitedSymbol` promotion is genuinely behavior-preserving.** I read the diff
  and both bodies. `internal/gate/citation_match.go:96-151` is line-for-line the old
  `internal/invariants` body, with one substitution: it calls gate's pre-existing `splitQualifiedSymbol`
  instead of the invariants copy `splitQualifiedCitationSymbol`. I checked those two functions against
  each other — `internal/gate/spec_resolver.go:327-332` and the deleted
  `splitQualifiedCitationSymbol` are byte-identical bodies (`strings.LastIndex(symbol, ".")`) **[read]**.
  The "zero-line diff on the old test files" claim is therefore not merely true, it is *justified*: I
  found no path by which the promotion could change a result. No leftover dead copies of
  `normalizeRelPath`/`splitQualifiedCitationSymbol` remain in `internal/invariants` **[verified via grep]**.
- **W4 — the 289→43 correction.** This is the most intellectually honest item in the whole wave: a
  number the plan itself asserted was measured, found wrong by a factor of ~7, and corrected *against*
  the framework's convenience in a public contract document (`docs/AUTHORED-SPEC-CONTRACT.md` §0).

### Partially closed

- **W1.1's constraint derivation.** The plan asked for "struct tags **и выводимые из них ограничения**
  (обязательность, уникальность, валидация)". Tags are now *captured and rendered* (there is a `tag`
  column in MODELS.md) but nothing is *derived* from them — no required/unique/validation semantics
  anywhere. Half of the sentence was implemented.
- **W1.4's symmetry.** The check covers exported receiver methods, exported interface methods, and
  exported top-level funcs (`model_complete_symmetric.go:161-198`). It does **not** cover exported
  struct fields, exported type declarations themselves, or exported package-level vars other than
  `Err*`. The invariant's own `Claim` states this scope honestly, so it is not a false claim — but
  "каждый публичный authored-символ" from the plan is not what shipped; three of the six categories
  the scan can now see are checked.
- **W1.3's EntityType↔Go link.** `ModelSymbol` exists, resolves, is checked, and is used once. But
  there is no inverse obligation: a domain can still keep an EntityType with no `model_symbol` and a Go
  type with no EntityType, and nothing notices. The two-catalog problem is *addressable* now, not
  *closed*.
- **W2.1's proof visibility.** `RequirementState()` reached `req show`/`brief`/`context` — correctly
  scoped, and the plan's own warning about `TRACEABILITY.md`/`COVERAGE.md` idempotency was respected
  **[read]**. But state is still absent from `req list`/`search`, i.e. from every roster view an agent
  actually starts from. The cost argument for that is sound; the gap is still a gap.
- **W2.3's freshness mechanism.** Built, tested, and **inert exactly where it matters** — see §3.1.

### Not closed (and not claimed to be)

- `INHERENTLY_PROSE` as an enforcement hole (plan §1.5). Untouched. `hotam-spec-self` currently has 36
  requirements sitting in it.
- The Requirement-does-three-jobs problem (plan §1.5: normative claim vs scope element vs decision
  record). Untouched, and correctly deferred — but it is the deepest remaining item.
- `hotam-spec-self` opting into any of the four triggers. It opts into **none** of
  `discipline`/`claim_authority`/`public_surface_authority`/`scenario_authority` **[verified]** — nor
  does `hotam-dev`.

---

## 3. New problems introduced by the fixes themselves

### 3.1 [HIGH] The engine-fingerprint mechanism (#400) is inert for every real cross-repo consumer — the exact case it was built for

`generator.BuildEngineVersionMD(repoRoot)` is called with `repoRoot = repoRootForDomain(domainDir)`
(`cmd/hotam/gen_spec.go:504`), and the write is skipped silently when the fingerprint cannot be
computed. `gate.EngineDocsFingerprint` errors unless `internal/generator`, `internal/ontology` or
`internal/loader` exist **under that root** (`internal/gate/engine_fingerprint.go:109-133`). On the
read side, `checkEngineDocsFingerprintCurrent` returns `nil` on the same error
(`internal/invariants/engine_version_current.go:111-119`).

For a consumer domain in its own repository, `repoRoot` is the **consumer's** repo, which by definition
has no engine source. Empirically **[verified]**:

- `PRAT-hotam/` has no `go.mod` and no `internal/` — `docs/gen/ENGINE-VERSION.md` is absent for **both**
  `domains/prat` and `domains/gpsm-sm`.
- `life/` (the wave's own showcase consumer, whose `docs/gen/` was regenerated at 03:58, i.e. **after**
  #400 landed at 02:41) has **no `docs/gen/ENGINE-VERSION.md`** either.
- Only `domains/hotam-spec-self` and `domains/hotam-dev` — the two domains living *inside* the engine
  repo, where docs are regenerated in the same commit as the engine change anyway — actually carry the
  file.

So the mechanism protects precisely the zero domains that needed protection. The CHANGELOG's own framing
("Consumer-generated documentation … sits in a separate repository, and nothing told the consumer's
resolver …") describes a scenario this implementation cannot reach. The honest-no-op degradation is
*correct* in the sense of not producing false positives; it is *wrong* in the sense that the honest
no-op is the only branch a real consumer will ever take.

Worse, `ENGINE-VERSION.md` is listed in `cleanupStaleGenFiles`'s `topLevelFiles`
(`cmd/hotam/gen_spec.go:918`): if a consumer ever *did* obtain the file and then ran `gen-spec` in a
context where the fingerprint cannot be computed, the file would be **deleted** as stale, silently
disarming the check permanently. Absence is never itself a violation.

### 3.2 [MEDIUM] The fingerprint's package set is wrong in both directions

`EngineDocsFingerprint` hashes exactly `internal/generator`, `internal/ontology`, `internal/loader`
(`internal/gate/engine_fingerprint.go:110-114`).

- **Under-sensitive.** `internal/generator` imports `internal/gate` in eight files, including
  `models.go`, `coverage.go`, `traceability.go`, `spec.go`, `claudemd.go` **[verified via grep]** —
  `MODELS.md` content *is* `gate.ScanAuthoredModels`'s output. Task #393, in this very wave, rewrote
  `internal/gate/model_scan.go` and materially changed every domain's `MODELS.md`. That change would
  **not** have moved the fingerprint. Likewise `cmd/hotam/gen_spec.go` decides *which* files exist at
  all (task #364's conditional `docs/gen/`) and is also outside the set.
- **Over-sensitive.** `hashDirContent` walks every regular file and excludes only build-output
  extensions (`internal/gate/engine_fingerprint.go:44-49`) — `_test.go` files are hashed. Editing a
  test in `internal/loader` invalidates every domain's stamp even though a test file cannot change one
  byte of generated output. The #400 commit message itself records this happening
  ("the check correctly fired on its own mid-task test-file edit (loader_domain_test.go)") and reads it
  as evidence the mechanism works; I read it as evidence of a false-positive class.

Minor: `sort.Strings(digests)` before combination (`:135`) is justified in the doc comment as making the
result "deterministic regardless of which package is hashed first", but `packages` is a fixed-order
slice — there was no non-determinism to remove. The sort's only real effect is to erase which package
produced which digest, which is the one (contrived) way two different engine states could collide.
Practically negligible; the stated reasoning is simply wrong.

### 3.3 [MEDIUM] `check_scenario_quality` does not catch the defect its own doc comment opens with

The file's motivating example is verbatim (`internal/invariants/scenario_quality.go:6-11`):

> A test that does `s := hotamspec.NewScenario(t, "R-x", ""); s.Then("", true)` formally satisfies
> check_settled_requires_scenario's "has a scenario" bar with **an empty title, an empty Then
> description**, and … no guarantee they are recorded in a sensible order.

Rule 3 as implemented only asks whether *any* step has `Kind == "then"`
(`internal/invariants/scenario_quality.go:284-293`) — it never inspects `Desc`. So
`NewScenario(t, "R-x", "a real title"); s.Then("", true)` passes all four rules. Of the three defects
named in the motivating sentence, the implementation catches the title and the ordering, and **misses
the empty step description entirely**. A one-line `strings.TrimSpace(st.Desc) != ""` condition would
close it; as shipped, "содержательность сценария" (plan W1.5) is enforced only at the scenario header,
not in the steps that become SPEC.md's normative body.

### 3.4 [MEDIUM] The one-way ratchet for the three new triggers is not actually latched on `life`

`life/domains/life/manifest.json` declares all three of `discipline:"full"`,
`public_surface_authority:"linked"`, `scenario_authority:"quality"`. Its committed
`life/domains/life/graph.lock` is **[verified]**:

```json
{ "sha256": "057e5f0f…", "updated_at": "2026-07-31T01:48:54Z", "note": "" }
```

— no `discipline_full_observed`, no `public_surface_authority_linked_observed`, no
`scenario_authority_quality_observed`. The pins are written only by `loader.WriteLock`
(`internal/loader/lock.go:106-144`), which runs on the graph-write path (`apply-proposal`/`land`/
`sync-domain`), **not** on `gen-spec`. The manifest was edited after the last graph write, so no pin
exists. Every ratchet check honest-no-ops on `!pinObserved`
(`internal/invariants/public_surface_authority_ratchet.go:74-80`).

Consequence: right now a resolver can delete `"public_surface_authority": "linked"` from `life`'s
manifest and **nothing fires**. The one-way door the wave anchored three requirements around is, for
the only domain that ever walked through it, still open. This is an inherited property of the F2
ratchet design (`discipline_full_observed` has the same latch condition) rather than a bug introduced
here — but the wave replicated it three more times without noticing that the opt-in event
(`manifest.json` edit) and the latch event (graph write) are decoupled.

### 3.5 [LOW] All four ratchet violation messages give remediation advice that does not work

Every ratchet says: *"if the downgrade is intentional, land it explicitly via `hotam apply-proposal`
(which rewrites graph.lock and **resets the pin**)"* —
`internal/invariants/discipline_ratchet.go:95`, `claim_authority_ratchet.go:89`,
`public_surface_authority_ratchet.go:100`, `scenario_authority_ratchet.go:105`.

`WriteLock` computes each pin as `prev || live` (`internal/loader/lock.go:140-143`) and reads `prev`
from the existing lock. `apply-proposal` therefore **preserves** a true pin, never resets it. No code
path anywhere deletes `graph.lock` **[verified via grep on `LockPath`]**. The escape hatch described to
the user does not exist. The wrong sentence originates in the pre-existing `discipline_ratchet.go` and
was copy-propagated into all three new ratchets without being re-checked against `WriteLock`'s actual
body.

### 3.6 [LOW] `claim_authority:"scenario"` alone is a silent no-op with no diagnostic

Unlike its two successors, `claim_authority:"scenario"` requires `discipline:"full"` as a
co-requirement (`internal/invariants/claim_scenario_current.go:102-115`). A domain author who sets only
`claim_authority:"scenario"` gets a manifest flag that reads as a strong opt-in and does absolutely
nothing, with no warning, no `hotam status` note, and no violation. The asymmetry is documented in
three doc comments inside the engine; it is invisible from the outside. Compare 3.7.

### 3.7 [LOW, but consumer-visible] Manifest opt-ins are documented nowhere in a domain's own generated surface

The `life` evaluation's Q9 flagged this and I confirm it **[verified]**: `grep -rl
"public_surface_authority\|scenario_authority" life/domains/life/docs/gen/` matches **nothing**, and
`life/CLAUDE.md` mentions them zero times. A domain declares three obligations in `manifest.json` and
its own generated documentation — the surface this wave spent #398/#399 making authoritative — never
names them, never says what they gate, and never says they are one-way. For a methodology whose central
claim is "the generated surface is enough", this is a structural gap, not a cosmetic one.

### 3.8 [LOW] Both real consumer domains are currently red, from this wave's own edits

`prat` and `gpsm-sm` each report exactly one violation, `check_domain_claude_md_current` **[verified]** —
their `CLAUDE.md` crystals are stale because this wave edited the crystal template four times
(`5597139`, `616e5cc`, `a883611`, `befa504` all touch `AGENTS.md`/`CLAUDE.md`/`GEMINI.md`). This is a
*much* milder instance of the same pattern W0.1 fixed: an engine change retroactively changed the
violation count of domains in another repository with zero action on their part. It is benign (no new
obligation, self-healing by one `gen-spec` run, no consent required) — but it means the wave's implicit
"the consumers are green again" is true only modulo a regeneration nobody ran, and it is *precisely*
the announcement `ENGINE-VERSION.md` was supposed to make and structurally cannot (§3.1).

### 3.9 [INFO] `infrastructureOrIgnoredReason` is an unanchored substring search

`internal/invariants/model_complete_symmetric.go:96-123` searches the whole joined doc comment for the
literal `INFRASTRUCTURE:` / `IGNORED:` anywhere, including mid-sentence (explicitly by design). A doc
comment that merely *discusses* the convention — e.g. "unlike INFRASTRUCTURE: markers, this method …" —
silently exempts the symbol. Low probability, zero diagnostic when it happens. The decision to use prose
markers rather than directive lines is itself well-reasoned (`(*ast.CommentGroup).Text()` strips
directive-shaped lines); the lack of any anchoring is the loose end.

---

## 4. The wave's central law: is it being re-derived or reflexively applied?

I spot-checked three invocations against source, not against the CHANGELOG.

- **#396 (`check_public_surface_linked_or_marked`) — correctly applied, genuinely re-derived.**
  `model_complete_symmetric.go:23-47` does not merely cite the law; it derives the *specific* harm
  ("neither has ever cited an interface method or constructor — those categories did not exist in the
  scan before task #393 — nor added any infrastructure/ignored marker anywhere"). That is a real,
  domain-specific prediction, and it is correct: had this ridden `discipline:"full"`, `prat` and
  `gpsm-sm` would have gone from 1 violation to dozens. The trigger is genuinely independent
  (`:129-134`), not co-gated on discipline **[read]**.
- **#397 (`check_scenario_quality`) — correctly applied, and re-derived against a *harder* case.** This
  one is not a new duty in the clean sense; it is a tightening of an existing "has a scenario" bar. The
  `Why` confronts that head-on (`scenario_quality.go:384-391`): *"regardless of whether the change is
  framed as 'a new duty' or 'a tightened old one': the practical harm … is identical either way."* That
  is a real extension of the law to a case it did not literally cover, not a reflex.
- **#400 (`check_engine_docs_fingerprint_current`) — the law was invoked and correctly concluded that
  NO new trigger was needed.** `engine_version_current.go:140-152` registers it unconditionally, and the
  CHANGELOG argues explicitly why that is not a violation. I agree with the reasoning: the check is
  honest-no-op-when-absent, and the only way to reach a violation is *regenerate → change engine → don't
  regenerate*, so no domain can be surprised by an obligation it never enacted. **This is the strongest
  evidence against the reflexivity hypothesis** — the law was applied and produced a *negative* answer.

**But the counter-evidence is real, and it is one layer down.** The law itself is being re-derived; the
*mechanism* built around it is being copy-pasted. `claim_authority_ratchet.go`,
`public_surface_authority_ratchet.go` and `scenario_authority_ratchet.go` are near-verbatim clones of
`discipline_ratchet.go`, and the clone carried a factually false remediation sentence through three
generations without anyone re-reading `WriteLock` (§3.5) — and carried the latch/opt-in decoupling
through too (§3.4). The doc comments in this wave are extraordinarily long and self-referential
("mirroring X's own precedent, which mirrors Y's own precedent"), and that chain of precedent-citation
is exactly the shape in which a copied error propagates undetected.

**One more structural point.** `R-opt-in-trigger-owns-its-own-obligations` is registered as
`Enforcement: "ENFORCED"` with `EnforcedBy: ["check_claim_authority_ratchet"]`
(`internal/selfspec/requirements_authoredspec.go:297-314`). That check proves a strictly narrower
proposition: *one specific trigger cannot be silently un-set*. It says nothing about the law's actual
content — *a new obligation must not activate on an old trigger*. Nothing mechanically prevents the next
wave from writing `if g.Discipline == loader.DisciplineFull` at the top of a brand-new check and
reproducing #369 exactly. The wave's own central law is, in substance, PROSE wearing an ENFORCED label.
Given that `INHERENTLY_PROSE`-as-a-hole is a named finding of both prior reviews, this deserves to be
called out rather than left as an accounting detail.

---

## 5. Independent verification of the `life` acceptance evaluation

I checked the report's central claims rather than accepting them.

| claim | my finding |
|---|---|
| Criterion 3: `all-violations --domain domains/life` reports 0 | **CONFIRMED [verified]** — I ran it on current `master` from a clean build: `0 violations — graph clean`. |
| Criterion 3: the gate is genuinely active, not a silent no-op | **CONFIRMED, independently** — I did not reuse the report's own refutation. I copied the domain to a temp tree, removed the `INFRASTRUCTURE:` marker from `CalendarService.CheckAvailability`, and got exactly one violation from `check_public_surface_linked_or_marked` naming that symbol. I then restored it and separately blanked a scenario title, and `check_scenario_quality` fired on rule 1. **Both new gates in this wave are real.** |
| Criterion 4: `MODELS.md` renders `CalendarService` as `(interface, port)` with `InterfaceMethods` and `mockCalendarService` as `(struct, mock)` with `Methods` | **CONFIRMED [verified]** — read directly at `life/domains/life/docs/gen/MODELS.md:43-77`. The rendering is exactly as described, including per-method doc text and the `INFRASTRUCTURE:` reasons. |
| Criterion 2: exactly one EntityType, linked via `model_symbol` | **CONFIRMED** — one EntityType (`commitment`), `ENTITIES.md` present, `check_entity_type_model_symbol_resolves` active. |
| "real, meaningful domain logic, not a thin toy" | **MOSTLY CONFIRMED, with a caveat.** 759 lines across 12 files; `commitment.go` is 243 lines carrying a real hand-written state table (`lcState`/`lcTransition`/`commitmentLifecycle`) funnelled through one `transition()` choke point, 8 sentinel errors, and an `Accept` that genuinely composes two external checks (calendar port + resource constraint) before allowing the transition. Seven requirements, each with real `implemented_by` (up to 4 symbols) and one `verified_by`. **This is real domain logic, not a scaffold.** The caveat: it is 7 requirements. At that size the failure modes the prior reviews care most about — `INHERENTLY_PROSE` drift, scope-vs-normative Requirement conflation, `DRAFT` accumulation, cross-domain staleness — cannot manifest. The slice validates the *retrieval and model surface*; it does not and cannot validate *enforcement discipline at scale*. |
| Q9's gap (manifest opt-ins undocumented in the domain's own generated docs) | **CONFIRMED and I would rate it higher than the report does** — see §3.7. The report calls it "not a defect in this wave's actual mechanics"; I agree literally, but it is a defect in the wave's own thesis (§398/#399: the generated surface should be sufficient). |
| Efficiency numbers (median 1 tool call, ~15k tokens for all 15 questions) | **NOT INDEPENDENTLY REPRODUCIBLE** — the numbers are the evaluation subject's own self-reported counts, and the raw transcript is in `.scratch/` (uncommitted). The report is candid about this in its own "Honest limitations" section (one evaluator, one run, non-Claude model stack). I would treat the accuracy figure (14/15 against a pre-written key) as solid and the token/tool-call figures as indicative but self-reported. The *direction* is corroborated structurally: two shared reads (`MODELS.md` + `REQUIREMENTS.md`) really do contain the answers to most of the listed questions — I read both and can see that. |

**Overall on §5:** the acceptance report is honest and its verdict survives scrutiny. It is not a rubber
stamp — the Q9 partial is reported against its own interest, the limitations section is unusually frank,
and the two claims I could hardest-test (the gate firing, the 0-violation state) both reproduced exactly.
What it *cannot* do is what it says it cannot: one run, one evaluator, one narrow slice.

---

## 6. Recommendation: what the next wave should prioritize

Ranked by my own judgment, not by restating the plan's W4/future-work framing.

1. **Fix #400 or retire it (§3.1, §3.2).** Right now it is a mechanism that costs a per-run directory
   walk and a `docs/gen/` file for every domain and protects none of them. Two options, and the choice
   is the resolver's: (a) stamp a fingerprint the *binary* carries (embed it at build time so a
   consumer running a compiled `hotam` still gets a real value), which is the only way to reach a
   cross-repo consumer at all; or (b) delete it and stop paying for a no-op. Whichever way: fix the
   package set (add `internal/gate`, add `cmd/hotam`'s gen-spec surface, exclude `_test.go`), and stop
   `cleanupStaleGenFiles` from deleting a stamp it cannot rewrite.
2. **Close the latch/opt-in decoupling for all four triggers (§3.4).** An opt-in that only ratchets on
   an unrelated later event is not a one-way door. Either write the lock on `gen-spec` too, or make the
   ratchet checks read the *manifest history* rather than a pin that may never have been taken. As it
   stands, `life` — the wave's own proof-of-concept — can silently un-opt from all three new triggers.
3. **Make the wave's central law mechanically real, or downgrade its label (§4).** Add a check that
   fails when a newly-registered invariant's `Check` body reads an existing trigger field
   (`g.Discipline`, `g.ClaimAuthorityScenario`, …) without a correspondingly new manifest field — or
   accept it as PROSE and stop marking `R-opt-in-trigger-owns-its-own-obligations` as ENFORCED on the
   strength of a ratchet that proves something else. Given that this law is the load-bearing beam of
   the entire wave, leaving it PROSE-in-fact is the largest single credibility gap remaining.
4. **Project the manifest opt-ins into every domain's own generated surface (§3.7).** One generated
   section — "this domain's declared obligations, what each gates, and that each is one-way" — in
   `AGENT-CONTEXT.md` or a new `OBLIGATIONS.md`. Cheap, and it directly serves the wave's own thesis.
   Fold in a `hotam status` warning for `claim_authority:"scenario"` without `discipline:"full"` (§3.6).
5. **Finish W1.5's own sentence: reject empty step descriptions (§3.3).** One line. The steps are what
   become SPEC.md's normative body; an empty `Then("")` currently produces an empty normative sentence
   under a green gate.
6. **Then, and only then, the deep one: split `Requirement`'s three jobs (plan §1.5).** Both prior
   reviews and the plan agree this is the deepest debt, and I agree it should not be attempted before
   1–5 are done. But note that `life` is now the right instrument to try it on: a 7-requirement domain
   where a scope-element requirement (`R-FR-27`-shaped) can be introduced deliberately and its
   behavior under the gates observed, without risking `prat`/`gpsm-sm`. Do the experiment on `life`
   first, not on the self-domain.
7. **Deferred, but name it honestly:** `INHERENTLY_PROSE` remains an unaudited escape hatch (36 uses on
   `hotam-spec-self`). This wave did not touch it, correctly, but every wave that adds a gate while
   leaving that hatch open widens the gap between the framework's claimed and actual floor.

---

### Two things I want on record as *good*, because a review that only lists defects is misleading

- The **W4 289→43 correction** is the single most trustworthy signal in this whole wave. A team that
  measures a number it previously asserted, finds it wrong by 7×, and publishes the correction in its
  own public contract document is a team whose other measurements are worth believing. Very little of
  what I checked in this wave turned out to be overstated in kind.
- The **honest-no-op discipline is genuinely held**. Every new check I read bails at the top on its own
  trigger and does no work — no scan, no `go test`, no filesystem walk — before that trigger fires. I
  went looking for a check that computes first and gates later, and did not find one. That property is
  what made it safe to add six invariants in one wave without touching two production domains, and it
  is the reason the W0.1 class of regression did not recur.
