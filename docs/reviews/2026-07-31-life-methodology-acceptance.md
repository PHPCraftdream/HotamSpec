# Life domain methodology acceptance evaluation — 2026-07-31

Task #401/W3.1's real-world acceptance test of the whole W0-W2 engine wave (docs/PLAN-code-authority-completion.md),
executed as two sequential sub-tasks: #404 (build the vertical slice) and #405 (this report — the cold-start
evaluation). Domain: `D:\ai_dev\prat\life\domains\life`, a separate repository from the HotamSpec engine.

## Verdict: all 6 acceptance criteria PASS, several by a wide margin.

| # | Criterion | Result |
|---|---|---|
| 1 | No generated document is ever hand-edited | **PASS** — two consecutive `hotam gen-spec` runs on an unchanged tree produced byte-identical `docs/gen/` output (`diff -rq` exit 0), verified personally by the resolver, not just the delegate's claim. |
| 2 | No disconnected duplication of an object between graph and Go | **PASS** — exactly one EntityType (`commitment`), correctly linked via `model_symbol: spec/model/commitment.go:Commitment` (task #395's link, first live use anywhere this session); no other EntityType exists to duplicate anything. |
| 3 | Every public method linked to a requirement+scenario or explicitly excluded with a reason | **PASS, mechanically enforced** — `domains/life/manifest.json` opts into `public_surface_authority:"linked"` (task #396), the first real domain to do so; `hotam all-violations --domain domains/life` reports 0. Personally reproduced one of the delegate's own refutation tests: removed an `INFRASTRUCTURE:` marker from `CalendarService.CheckAvailability`, confirmed `check_public_surface_linked_or_marked` fired, reverted, reconfirmed 0 violations — the gate is genuinely active, not a silent no-op. |
| 4 | Mock visible in model docs together with the interface's methods and its scenarios | **PASS** — `docs/gen/MODELS.md` renders `CalendarService` as `(interface, port)` with its own `InterfaceMethods`, and `mockCalendarService` as `(struct, mock)` with its own `Methods` — both confirmed by direct read, not just the delegate's excerpt. |
| 5 | AI answers 15 prepared questions without reading the raw graph or the whole source tree | **PASS** — see full evaluation below; the eval subject was explicitly instructed not to read `graph.json` or `spec/` wholesale, and its own tool-call log shows it never did. |
| 6 | Baseline ≥90% accuracy, median ≤5 reads, ≤15k input tokens per cold question | **PASS, by a wide margin** — 14/15 (93%) fully correct, 1/15 an honest partial (see Q9 below); median 1 tool call per question; ~15,000 tokens read for **all 15 questions combined**, not per question (the PRAT baseline this task's own plan cites was ~54k tokens for a **single** question). |

## What was built (task #404)

7 objects (`Person`, `Commitment`, `Project`, `CalendarBlock`, `Decision`, `Resource`, `Constraint`) in
`domains/life/spec/model/`; one real Lifecycle (`Commitment`: `PROPOSED -> ACCEPTED -> ACTIVE ->
{FULFILLED, BROKEN, RENEGOTIATED}`, funneled through a single `transition()` choke point); one external
port/mock (`CalendarService` + `mockCalendarService`, mirroring `PRAT-hotam/domains/gpsm-sm`'s
`OcrRecognizer`/`mockOcrRecognizer` shape); 7 requirements with real `implemented_by`/`verified_by` and
`hotamspec`-recorded Given/When/Then scenarios narrating genuine life situations (calendar conflicts,
resource limits, broken commitments requiring rationale, competing commitments resolved by a recorded
Decision). `manifest.json` opts into all three of this wave's new gates: `discipline:"full"`,
`public_surface_authority:"linked"`, `scenario_authority:"quality"` — the first real domain to adopt any of
them. Committed in the `life` repository as `26de6e6`.

## The cold-start evaluation (task #405)

**Methodology.** 15 questions with a fixed answer key were written *before* the evaluation ran (this file's
own companion artifacts, `.scratch/life-eval-405-questions.md` / `-answer-key.md` in the HotamSpec repo, not
committed — working notes), so grading could not be post-hoc rationalized. A genuinely fresh evaluation
subject was launched (`crush run`, a brand-new session with zero prior context of this conversation or the
domain's design), pointed only at `D:\ai_dev\prat\life`, instructed to answer using only the generated
`docs/gen/*.md` surface and the `./hotam` CLI, explicitly forbidden from reading `graph.json` directly or
`spec/` wholesale, and asked to self-report its own tool-call count and token estimate per question.

**Results, graded by the resolver against the pre-written answer key:**

- Q1 (lifecycle states) — correct.
- Q2 (which object carries the lifecycle) — correct.
- Q3 (the port + its two methods) — correct.
- Q4 (the mock's name + file kind) — correct.
- Q5 (calendar-conflict requirement) — correct.
- Q6 (Break's empty-rationale error) — correct.
- Q7 (resource-limit requirement's carriers) — correct, all four methods+files named exactly.
- Q8 (EntityType + model_symbol) — correct.
- Q9 (the three manifest opt-ins) — **partial, and genuinely informative.** The agent correctly named all
  three flags and correctly explained `discipline:"full"`, but honestly reported that
  `public_surface_authority`/`scenario_authority` are **not documented anywhere in the domain's own
  `docs/gen/`** (they only appear in `manifest.json` itself) and that it could only infer their meaning from
  the flag names, not confirm it from generated documentation. This is not a wrong answer — it is an honest,
  correctly-reasoned "I cannot verify this from what's in front of me" — but it surfaces a real, actionable
  gap: a consumer domain's own generated docs do not currently explain *what* its manifest opt-ins actually
  gate (that knowledge lives only in the engine's own invariant registry). Worth a follow-up task; not a
  defect in this wave's actual mechanics (the gates themselves work, per criteria 3/4/6's own evidence).
- Q10 (conflicting commitments) — correct.
- Q11 (requirement count + status) — correct.
- Q12 (no-owner requirement + error) — correct.
- Q13 (the `transition` choke point) — correct, detailed, accurate.
- Q14 (a constructor marked IGNORED) — correct (`NewCalendarBlock`, matching the delegate's own object-model
  table from task #404).
- Q15 (hand-editing MODELS.md) — correct.

**Accuracy: 14/15 unambiguously correct (93%), 1/15 an honest, well-reasoned partial rather than a wrong
guess or hallucination — comfortably clears the ≥90% bar either way it's counted.**

**Efficiency, against the per-question `→ N tool calls` the agent reported for each answer:**
`1,1,1,1,1,2,1,1,5,1,3,2,1,1,2` → **median = 1 tool call per question** (bar: ≤5). Two large, shared reads
(`MODELS.md` + `REQUIREMENTS.md`) alone answered 8 of the 15 questions in one pass — a real demonstration
that the compact evidence-packet/generated-doc surface this wave built (tasks #393-#399) does its job.

**Tokens:** the agent's own final summary reports **~15,000 tokens read in total across all 15 questions**
(not per question) — `MODELS.md ≈3.5k, TRACEABILITY ≈1.8k, commitment.go ≈2.2k, COVERAGE ≈2.5k,
AGENT-CONTEXT ≈1.1k, ENTITIES ≈0.9k, hotam -h ≈0.7k, REQUIREMENTS ≈0.6k, greps/CLI/manifest/ls ≈1.7k`. The
task's own baseline citation (a saved PRAT evaluation run) was **~54k tokens for a single question**; this
run used roughly a quarter of that budget for the *entire 15-question set combined* — a dramatic improvement,
consistent with the whole wave's stated purpose (make proof/structure visible and cheap, tasks #398/#399).

## Honest limitations of this evaluation

- One evaluator, one run, no repeated trials — a single data point, not a statistically rigorous sample. The
  margin over the acceptance bar (93%+ accuracy vs. a 90% bar, 1 vs. ≤5 median reads, ~1000 vs. 15,000 tokens
  per question) is wide enough that ordinary run-to-run variance is very unlikely to flip the verdict, but
  this was not repeated to confirm that.
- The evaluation subject and the domain-building agent (task #404) both ran on `crush`'s own configured
  models, not on Claude/this session's own model family — this measures the methodology's documentation
  surface, not this specific LLM stack's own retrieval behavior; a different model family might read the same
  docs differently (better or worse).
- Q9's gap (manifest opt-ins undocumented in the domain's own generated surface) is real and worth a
  follow-up task, but is explicitly out of scope for W3.1 itself.

## Conclusion

The W0-W2 engine wave's actual real-world value is confirmed on a genuine, non-toy vertical slice: a fresh
agent, given only the generated documentation and the CLI this wave built, answered domain-specific questions
about lifecycle states, error handling, requirement-to-code traceability, port/mock contracts, and the
regeneration contract — accurately, cheaply, and without touching the raw graph or reading the source tree
wholesale. This is materially better than the PRAT baseline the plan itself cited as evidence the methodology
needed this wave's work in the first place.
