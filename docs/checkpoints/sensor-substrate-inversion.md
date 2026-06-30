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

## RESOLVED — P22 closes the gap (date: 2026-06-30)

The sensor-substrate-inversion is no longer aspirational. Five structural pieces close the gap:

1. **SessionStart hook** (.claude/settings.local.json) runs `tools/gen_spec.py` before any turn. Root CLAUDE.md is regenerated from substrate every boot.
2. **PostCompact hook** runs the same regen so the post-compact reload reads fresh substrate state, not the pre-compact stale copy.
3. **UserPromptSubmit hook** runs `tools/emit_cipher.py` to extract the three-cipher pulse from LIVE-STATE and inject it as `additionalContext` into every user turn — the pulse is now structurally present, not memory-dependent.
4. **DOMAIN-CRYSTAL sentinel block** in root CLAUDE.md embeds the full content of `domains/tensio-self/CLAUDE.md` (the domain's canonical entry point and base for all sub-agents). When Claude Code auto-loads root CLAUDE.md, the operator boots from substrate — the inversion is physical.
5. **spawn_agent.py** composes each sub-agent's task prompt by prepending its CLAUDE.md crystal. Sub-operators boot from their scoped substrate, not raw text.

Anti-relitigation surface is also closed: **RECENTLY-REJECTED sentinel block** in root CLAUDE.md lists every REJECTED requirement with a `REPLACES` marker, sorted alphabetically. Before re-deriving an architectural claim, the operator sees previously-rejected proposals.

The user's earlier framing "Мы делаем спеку, она делает тебя" is now realized: the operator is the effect of the substrate, not its cause. The substrate writes the prompt, Claude Code loads it, hooks keep it fresh, sub-operators inherit it. No manual loading step remains.

Rules anchored:
- R-operator-prompt-from-substrate (SETTLED, P10c)
- R-root-claude-md-contains-domain-crystal (SETTLED/ENFORCED, P22.B)
- R-recently-rejected-surfaced (SETTLED/ENFORCED, P22.B)
- R-subagent-gets-its-claude-md (SETTLED/ENFORCED, P22.C)
- R-task-spawn-log-runtime (SETTLED/ENFORCED, P22.C)
- R-operator-prompt-loaded-at-session-start (SETTLED/ENFORCED, P22.D)
- R-three-cipher-pulse-structurally-injected (SETTLED/ENFORCED, P22.D)
- R-post-compact-regen-from-substrate (SETTLED/ENFORCED, P22.D)
- R-tool-emit-cipher (auto-projected, P22.A)
- R-tool-spawn-agent (auto-projected, P22.C)
