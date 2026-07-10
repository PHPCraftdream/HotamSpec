# Quickstart for consumers

This is the path for a team that wants to **use** Hotam-Spec to hold a shared,
disciplined understanding of its own system — requirements, owners, open
tensions — as a `pip`/`git` dependency, in your own repo, not by cloning the
framework repo and working inside `spec/`. If you *are* working inside a clone
of the Hotam-Spec repo itself (self-hosting mode), see the
[root README](../README.md) instead.

Everything below is CLI-only. No AI agent is required to follow this guide —
see the note at the end about what changes if you add one.

## 1. Install

In your own project (a separate git repo, NOT this one):

```bash
# from PyPI (once published) or...
pip install hotam-spec

# ...for local development against a cloned copy of this repo:
pip install -e /path/to/HotamSpec/spec
```

This installs the `hotam-spec` package and 26 `hotam-*` console scripts
(`hotam-create-domain`, `hotam-what-now`, `hotam-apply-proposal`, ...). Run
`pip show -f hotam-spec` or check `spec/pyproject.toml`'s `[project.scripts]`
table for the full list.

## 2. Scaffold your domain

Run this **inside your own project directory** (not inside HotamSpec):

```bash
mkdir my-project && cd my-project   # or cd into your existing project instead
touch .hotam-spec-project   # marks this directory as your project root -- see
                            # "how project root is found" below. `git init`
                            # does NOT count as a marker on its own.

hotam-create-domain my-shop \
    --description "My shop's contradictory requirements" \
    --goals "hold the first tension;decide honestly" \
    --director-purpose "steward my-shop" \
    --activate
```

This creates `domains/my-shop/` (manifest, empty graph, director agent
scaffold, generated-docs placeholder) right there in `my-project/`, pins it as
the active domain (`domains/.active-domain`), and writes a `CLAUDE.md` at your
project root — that file is only relevant if you later add an AI operator
(see the note at the end).

**Already have a project?** Skip the `mkdir` — just `cd` into its root and run
`touch .hotam-spec-project` there before `hotam-create-domain`. The marker
file (or, once you've run `hotam-create-domain`, the `domains/` folder itself)
is all the tooling needs to find your project root; nothing about it requires
a fresh directory.

## 3. Check your pulse

```bash
hotam-what-now
```

This is the framework's live status tool: it reads your (currently empty)
graph and tells you the next correct action. Run it again after every change —
it is how you navigate the methodology without getting lost.

## 4. Create your first Stakeholder, Requirement, and Conflict

The graph is **never hand-edited**. Every change goes through
`hotam-apply-proposal`, which reads a small JSON file, writes the change into
`domains/my-shop/graph.py`, regenerates docs, and runs a verification gate.

```bash
# a) at least TWO stakeholders — a conflict's steward may not own either side.
echo '{"kind":"Stakeholder","id":"alice","name":"Alice","domain":"product"}'    > sh1.json
echo '{"kind":"Stakeholder","id":"bob","name":"Bob","domain":"engineering"}'    > sh2.json
echo '{"kind":"Stakeholder","id":"carol","name":"Carol","domain":"governance"}' > sh3.json
hotam-apply-proposal sh1.json
hotam-apply-proposal sh2.json
hotam-apply-proposal sh3.json   # a NEUTRAL steward for the conflict below

# b) an axis — the shared dimension your first tension lives on.
hotam-create-axis speed-vs-rigor --description "ship fast vs verify thoroughly"

# c) two requirements that will turn out to contradict each other.
echo '{"kind":"Requirement","id":"R-ship-fast","claim":"Ship within one week.","owner":"alice","status":"SETTLED","enforcement":"PROSE"}'          > r1.json
echo '{"kind":"Requirement","id":"R-verify-all","claim":"Verify every change before release.","owner":"bob","status":"SETTLED","enforcement":"PROSE"}' > r2.json
hotam-apply-proposal r1.json
hotam-apply-proposal r2.json

# d) your first Conflict — the tension between them, held by the neutral party.
echo '{"kind":"Conflict","axis":"speed-vs-rigor","context":"first release cadence","members":["R-ship-fast","R-verify-all"],"steward":"carol"}' > c1.json
hotam-apply-proposal c1.json

# e) re-check your pulse — the new conflict now awaits carol's ACKNOWLEDGE.
hotam-what-now
```

For the full set of proposal shapes (Requirement, Conflict, ConflictTransition,
Rejection, Assumption, AssumptionTransition, Stakeholder, Axis, EntityType,
OperatorBudget, ConflictMemberUpdate) with required/optional fields and one
worked example each, see [PROPOSAL-REFERENCE.md](PROPOSAL-REFERENCE.md).

## How project root is found (R1-R6)

Every `hotam-*` command needs to know **where your data lives** — i.e. where
`domains/` (and `tickets/`, `delegations/`, `CLAUDE.md`, ...) should be
created or read from. It resolves this automatically, in order, and stops at
the first thing that matches:

1. **Explicit env var** — if `HOTAM_SPEC_PROJECT_ROOT` is set to a directory
   that exists, that directory is your project root. The most direct override.
2. **Domains-root env var** — if `HOTAM_SPEC_DOMAINS_ROOT` is set, its
   *parent* directory is your project root (use this if you keep `domains/`
   somewhere unusual).
3. **Markers in the current directory, searched upward (up to 5 levels)** —
   this is the common case and needs **no configuration**. Starting at your
   current working directory and walking up through parent directories (at
   most 5 levels up), the tool looks for two tiers of marker:
   * **Reliable (any ONE is enough)** — a `domains/` folder, a `delegations/`
     folder, or a `pyproject.toml` that contains a `[tool.hotam-spec]` table.
     These are specific to a Hotam-Spec project, so one alone is trusted.
   * **Secondary (need 2 or more together)** — a `CLAUDE.md` file, a
     `.claude/` folder, or a `tickets/` folder. Any ONE of these alone is too
     generic (lots of unrelated Claude-Code repos have a `CLAUDE.md`; many
     projects have an unrelated `tickets/` folder), so a lone secondary
     marker does **not** match — it takes two or more of them together
     before the directory counts as your project root.

   The first directory (bottom-up) that satisfies either tier wins. In a deep
   monorepo, a project root more than 5 levels above your working directory
   will not be found this way — use the explicit `HOTAM_SPEC_PROJECT_ROOT`
   env var (R1) instead; it has no depth limit.
4. **A `.hotam-spec-project` marker file** — an empty file you can drop at
   your intended project root if none of the above markers apply yet (e.g.
   before you've run `hotam-create-domain` for the first time). Searched
   upward the same way (up to 5 levels).
5. **`pyproject.toml` config** — a `[tool.hotam-spec]` table with a
   `project_root = "..."` key (a path relative to that `pyproject.toml`),
   for projects that want to pin the root explicitly in version control.
   Searched upward the same way (up to 5 levels) to find the `pyproject.toml`
   itself.
6. **Self-hosting fallback** — only relevant if you are working *inside a
   clone of the Hotam-Spec repo itself*; consumers installing via `pip`
   will never hit this.

**In practice:** run `hotam-create-domain` once inside your project directory
(step 2 above) — it creates `domains/`, which becomes a permanent marker (rule
3). After that, every `hotam-*` command run from anywhere inside your project
tree finds the same root automatically. If a command ever resolves the wrong
root (or none at all), it raises a diagnostic listing exactly which of R1-R6
it checked and what it found — nothing is guessed silently.

## Everyday commands

Once your domain has requirements, `hotam-req` gives you fast, graph-backed
access without grepping generated docs:

```bash
hotam-req list --status SETTLED          # compact table: id / status / enforcement / owner
hotam-req show R-ship-fast               # full node details (add --json for machine output)
hotam-req search "verify"                # case-insensitive search across id / claim / why
hotam-req context R-ship-fast --json     # agent-ready context package (owner + assumptions + conflicts)
hotam-req patch R-ship-fast --set enforcement=STRUCTURAL --dry-run   # preview a field change
```

`patch` is pure UX sugar: it reads the current requirement, applies your
`--set` overrides, and routes the result through `hotam-apply-proposal` --
no new write path, no hand-edit.

## What's next

- [PROPOSAL-REFERENCE.md](PROPOSAL-REFERENCE.md) — the full JSON reference for every proposal kind.
- The root [README.md](../README.md) — concepts (Requirement/Conflict/Axis/...),
  and the AI-operator-only extras (hooks, sensorium, operator crystal) if you
  choose to add an AI agent as an operator later. None of that is required to
  use Hotam-Spec as a plain CLI discipline for a human team.
