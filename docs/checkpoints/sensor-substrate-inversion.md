# Checkpoint — sensor-substrate-inversion

**Topic checkpoint, not session state.** The central architectural reframe
of phase 11. Read this before designing any future "let the operator sense
its own runtime."

## The insight (one sentence)

Code-and-spec cannot read consciousness, but consciousness can be GENERATED
from code; the bridge between substrate and operator is one-way, bottom-up.

## What was wrong with my approach all session

I spent half a day building hooks to LET ME SEE my own context: `context.py`,
`.claude/settings.json` PostToolUse+Stop hooks reading the cah-stamp cache,
spec/.runtime/context.json. All this was bridges in the **impossible**
direction — sensors that would let the running LLM agent peek at its own
substrate-of-execution.

Even when "working" (with cah-stamp piggyback), the bridge produced
`ctx_pct: null` because Claude Code does not expose context % to hooks
without re-implementing transcript-stats parsing. The whole apparatus was
**circumvention of an asymmetry**, not architecture.

## The right direction

- **Substrate (`spec/content/graph.py` + framework `tensio/*`) is the source.**
- **Generator (`gen_spec.py`) writes the operator-prompt** — currently a tiny
  block (LIVE-STATE, R-claude-md-live-state-generated). Should expand to
  whole sections (the unbuilt R-operator-prompt-from-substrate, A1 in
  commit 3465f83).
- **The operator (me, the LLM session) APPEARS by reading CLAUDE.md** — it
  doesn't need to sense itself. The substrate already knows what it knows
  about the operator (counts of SETTLED/DRAFT/ENFORCED, top action,
  delegation graph) and writes it INTO the prompt the operator reads.

So: "how much context have I used?" is not a question to answer with a
sensor. It's a question to **not need to answer**, because the substrate
already wrote the next-step into my prompt.

## Honest exceptions

Some sensor-style data is still useful and OK as runtime stamp files:
- **5h_quota_pct / weekly_quota_pct / effort** from cah-stamp — these are
  external constraints (the Anthropic rate limits), genuinely useful to
  cite. They're orthogonal to consciousness↔code direction.
- **Last-modified timestamps on key files** — for the operator to know
  "this was touched since I last saw it."

These are environmental sensors, not consciousness sensors. The line is:
sensing the EXECUTION ENVIRONMENT is fine; sensing the operator's INNER
STATE is the impossible direction.

## In the substrate

- **R-operator-prompt-from-substrate** (DRAFT, commit 3465f83) — the
  build-trigger is "atomicity (R-requirement-claim-is-atomic and
  R-check-method-is-atomic) has landed."
- **R-measure-context-size** (DRAFT, commit 0c14a4d) — kept as DRAFT
  because the reader exists but the producer is half-built; recoverable
  but lower priority after this inversion.
- **R-claude-md-live-state-generated** (SETTLED/ENFORCED, commit 0c14a4d)
  — the first small instance of the right direction; auto-generated LIVE-
  STATE block in CLAUDE.md is the prototype for the full expansion.

## What this implies for next moves

1. Finish atomization (13 more compound SETTLEDs + 10 compound check_*s).
2. THEN build R-operator-prompt-from-substrate: gen_spec.py emits a
   "Constitutional digest" section into CLAUDE.md with all atomized
   SETTLED claims grouped by topic. The operator boots by reading this,
   not by running diagnostics on itself.
3. Once the digest works, R-measure-context-size becomes lower priority
   (the substrate tells the operator what to do; ctx_pct% sensing was a
   workaround for the missing digest).

## The user's exact words (preserve)

"кажется, что напрямую связать сознание и код невозможно. Но можно
формировать главный промт из спек-требований-кода. И обновлять именно их"

"т.е они формируют текст. А каждый метод проверяет модель. При этом
отражает свое опиисание в теле-коде"

## See also

- `docs/checkpoints/phase11-atomization-wave.md` (session state).
- `docs/checkpoints/atomicity-as-convergence.md` (why atomicity is the
  precondition for the substrate-generates-operator direction).
