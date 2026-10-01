# Quickstart for consumers

This is the path for a team that wants to **use** Hotam-Spec to hold a
shared, disciplined understanding of its own system — requirements, owners,
open tensions — via the `hotam` Go CLI, in your own repo, not by cloning the
framework repo and working inside it. If you *are* working inside a clone of
the HotamSpec repo itself (self-hosting mode against `domains/hotam-spec-self`),
see the [root README](../README.md) instead.

Everything below is CLI-only. No AI agent is required to follow this guide.

## 1. Build the CLI

There is no published package or `go install` target yet (tracked as
ongoing work) — build the binary from a clone of this repo:

```bash
git clone <this-repo-url> HotamSpec
cd HotamSpec
go build -o bin/hotam ./cmd/hotam
```

or run it without a persistent binary:

```bash
go run ./cmd/hotam <command> [flags] [args]
```

The rest of this guide assumes `hotam` on your `PATH` (or substitute
`go run ./cmd/hotam` — with a working directory inside this repo — for every
`hotam ...` call below).

## 2. Set up your project

`hotam init-project <dir>` is the recommended, one-command onboarding path:
it works from ANY working directory, against ANY target path, and does not
require your project to live inside this repository. In one call it
scaffolds a base domain (default name `main`) with a genuinely EMPTY graph
(0 nodes -- `all-violations`-clean by construction, task #364), defaults to
`--discipline full` (BORN FULLY OBLIGATED: it scaffolds `spec/go.mod` + the
vendored `hotamspec` scenario recorder, so every SETTLED requirement you add
later must carry a scenario-narrated `verified_by` test), writes the
project-root marker (`.hotam-spec-project`, recording `main` as the active
domain), and renders the root crystal (`CLAUDE.md`/`AGENTS.md`/`GEMINI.md`)
plus every `docs/gen/*` view — a fully working project the instant the
command returns:

```bash
mkdir -p my-project
cd my-project
hotam init-project .
```

This creates `domains/main/graph.json` (a genuinely EMPTY graph -- 0 nodes;
task #364 retired the earlier auto-seeded Stakeholder `owner` + Requirement
`R-domain-exists`), `domains/main/manifest.json` (`discipline: "full"`,
defaulted to the consumer gen-spec profile), `domains/main/spec/` (a Go
module + the vendored `hotamspec` recorder, ready for scenario-narrated
tests), `domains/main/docs/gen/`, and the root crystal. Inspect what it
made you:

```bash
hotam all-violations --domain domains/main   # 0 violations — graph clean
hotam what-now --domain domains/main         # the framework's live status tool
```

Because the marker already records `main` as the active domain, every
command below could in principle be run without `--domain domains/main` —
but this guide keeps `--domain` explicit on every example anyway, since
that is the most literal, unambiguous, copy-pasteable form for a first read.

The rest of this guide uses `domains/main` — the domain `init-project`
scaffolded above — throughout. If you gave `--domain <name>` a different
name at this step, or added a SECOND domain later, substitute that domain's
path everywhere `domains/main` appears below.

### Alternative: a bare domain without the full project scaffold

If you want a base domain somewhere other than `init-project`'s default
layout, or want to add a SECOND domain to a project that already has one,
use `hotam init` directly instead — it scaffolds only the domain (no
project marker, no root crystal):

```bash
hotam init domains/my-second-shop --name my-second-shop
```

This creates `domains/my-second-shop/graph.json` with a genuinely EMPTY
graph (0 nodes -- `hotam init` itself writes this empty graph under the
hood, task #364; an empty graph passes every invariant by construction),
`domains/my-second-shop/manifest.json` (`{"self_hosting": false,
"gen_profile": "consumer"}` -- NOTE: unlike `init-project`, a bare `hotam
init` does NOT set `discipline: "full"` or scaffold `spec/`), an empty
`domains/my-second-shop/docs/gen/` directory ready for `hotam gen-spec`, and
a `domains/my-second-shop/README.md` pointing back at the next commands to
run.

If you'd rather hand-write the graph.json literally (the exact shape `hotam
init` itself writes under the hood), it is just:

```bash
cat > domains/my-second-shop/graph.json <<'EOF'
{
  "axes": [],
  "stakeholders": [],
  "requirements": [],
  "conflicts": [],
  "assumptions": [],
  "entity_types": []
}
EOF
```

(The exact top-level key set is whatever `internal/loader.LoadGraph` decodes,
currently `axes` / `stakeholders` / `assumptions` / `requirements` /
`conflicts` / `operators` / `processes` / `goals` / `entity_types` /
`entities` — all optional and default to empty when omitted. If this list
drifts, cross-check `internal/ontology/graph.go`'s `Graph` struct tags, or
copy the shape of this repo's own `domains/hotam-spec-self/graph.json`.)

## 3. Check your pulse

```bash
hotam what-now --domain domains/main
```

This is the framework's live status tool: it reads your graph (currently
just `init-project`'s (or `hotam init`'s) seed stakeholder + seed
requirement, or empty if you hand-wrote a bare graph.json instead) and
tells you the next correct action.
Run it again after every change — it is how you navigate the methodology
without getting lost. `--limit N` caps how many signals it prints (default
20).

## 4. Create your first Stakeholder, Requirement, and Conflict

The graph is **never hand-edited** past the `hotam init-project` (or `hotam
init` / bare `graph.json`) bootstrap above. Every change goes through
`hotam apply-proposal`, which reads a small JSON file, applies it to
`domains/main/graph.json`, and fails closed (writes nothing) if the
change would introduce a new invariant violation. `hotam land` does the
same apply step and then also regenerates `docs/gen/` and re-verifies in one
call — prefer it over standalone `apply-proposal` unless you specifically
want to batch several proposals before regenerating docs once at the end.

```bash
# a) at least TWO stakeholders — a conflict's resolver may not own either side.
echo '{"kind":"Stakeholder","id":"alice","name":"Alice","domain":"product"}'    > sh1.json
echo '{"kind":"Stakeholder","id":"bob","name":"Bob","domain":"engineering"}'    > sh2.json
echo '{"kind":"Stakeholder","id":"carol","name":"Carol","domain":"governance"}' > sh3.json
hotam apply-proposal sh1.json --domain domains/main --today 2026-07-12
hotam apply-proposal sh2.json --domain domains/main --today 2026-07-12
hotam apply-proposal sh3.json --domain domains/main --today 2026-07-12   # a NEUTRAL resolver for the conflict below

# b) an axis — the shared dimension your first tension lives on.
echo '{"kind":"Axis","slug":"speed-vs-rigor","description":"ship fast vs verify thoroughly"}' > ax1.json
hotam apply-proposal ax1.json --domain domains/main --today 2026-07-12

# c) two requirements that will turn out to contradict each other.
# hotam land applies AND regenerates docs/gen AND re-verifies in one call.
echo '{"kind":"Requirement","id":"R-ship-fast","claim":"Ship within one week.","owner":"alice","status":"SETTLED","enforcement":"PROSE"}'          > r1.json
echo '{"kind":"Requirement","id":"R-verify-all","claim":"Verify every change before release.","owner":"bob","status":"SETTLED","enforcement":"PROSE"}' > r2.json
hotam land r1.json --domain domains/main --today 2026-07-12
hotam land r2.json --domain domains/main --today 2026-07-12

# d) your first Conflict — the tension between them, held by the neutral party.
echo '{"kind":"Conflict","axis":"speed-vs-rigor","context":"first release cadence","members":["R-ship-fast","R-verify-all"],"resolver":"carol"}' > c1.json
hotam apply-proposal c1.json --domain domains/main --today 2026-07-12

# e) re-check your pulse — the new conflict now awaits carol's ACKNOWLEDGE.
hotam what-now --domain domains/main
```

For the full set of proposal shapes (Requirement, Conflict, ConflictTransition,
Rejection, Assumption, AssumptionTransition, Stakeholder, Axis, EntityType,
OperatorBudget, ConflictMemberUpdate) with required/optional fields and one
worked example each, see [PROPOSAL-REFERENCE.md](PROPOSAL-REFERENCE.md).

## 5. Regenerate readable docs from the graph

The graph in `graph.json` is the source of truth, but it is not meant to be
read directly. `hotam land` (used for r1.json/r2.json above) already
regenerates docs/gen as part of its pipeline; `apply-proposal` alone (used
for the stakeholders/axis/conflict above) does not. Regenerate the Markdown
views (`REQUIREMENTS.md`, `TENSIONS.md`, `CONSTITUTION.md`, ...) under
`domains/main/docs/gen/` standalone at any time with:

```bash
hotam gen-spec --domain domains/main
```

## 6. Verify the graph stays structurally sound

```bash
hotam all-violations --domain domains/main
```

Prints every invariant violation and exits 1 if any exist; exits 0 with
`0 violations — graph clean` otherwise. `hotam apply-proposal` already
refuses to write a change that would introduce a NEW violation, so this is
mainly useful as a standalone health check (e.g. in CI) or after a batch of
several proposals.

## 7. Selecting a targeted test subset for a change

```bash
hotam gate <target-anchor> --domain domains/main
```

Given an anchor id (a Requirement/Conflict/Assumption id), prints a
best-effort Tier-1 subset of tests/checks relevant to that node — useful
once your own domain has code-level enforcers wired to `enforced_by`.

## Everyday commands

Once your domain has requirements, `hotam req` gives you fast, graph-backed
access without grepping generated docs:

```bash
hotam req list --domain domains/main --status SETTLED     # compact table: id / status / enforcement / owner
hotam req show R-ship-fast --domain domains/main           # full node details (add --json for machine output)
hotam req search "verify" --domain domains/main            # case-insensitive search across id / claim / why
hotam req context R-ship-fast --domain domains/main --json # agent-ready context package (owner + assumptions + conflicts)
hotam req related R-ship-fast --domain domains/main        # neighbor id + relation-kind list for any anchor
```

`hotam req list` also accepts `--owner` and `--enforcement` filters. There is
no `patch`/write subcommand under `req` today — every write still goes
through `hotam apply-proposal` (see above); `req` is read-only.

`--domain` resolution (see the root [README](../README.md)'s `--domain`
section for the full precedence chain): an explicit `--domain <path>`
always wins; otherwise a `HOTAM_DOMAIN` env var or the project-root marker's
active-domain preference (set automatically by `init-project`, or via
`hotam use <name>`) is used; only with none of these set does `--domain`
fall back to `domains/hotam-spec-self` (this repo's own default — not
relevant to an external project). A project set up via `init-project` above
therefore needs `--domain` only as an OVERRIDE, not on every call — this
guide keeps it explicit on every example anyway, as the most literal,
unambiguous, copy-pasteable form for a first read.

## Code-authority: requirements as Go code (optional)

The steps above author requirements as JSON proposals (`hotam apply-proposal`/
`land`) — `graph.json` is the source of truth. A consumer domain may instead
declare `"requirements_authority": "code"` in its `manifest.json` and keep its
requirements as Go literals in its own `spec/requirements.go`, with the graph
as a derived projection (task #365–#367). The flow:

```bash
hotam vendor-ontology --domain domains/main        # vendor the Requirement+Registry mirror into spec/hotamontology/
hotam scaffold-registrydump --domain domains/main  # write spec/registrydump/main.go (prints the registry as JSON)
# ...author requirements as []ontology.Requirement in domains/main/spec/requirements.go...
hotam sync-domain --domain domains/main            # dry-run: preview Go->graph diff; --confirm-hash <hex> to write
```

Under `discipline: "full"` in the manifest, the Claim of any requirement with
`verified_by` entries can be left empty in `spec/requirements.go` — it is
derived automatically from the verified_by test's recorded scenario title
(and a hand-written Claim for such a requirement is ignored, with a NOTE
printed by `sync-domain`).

Requirement owners can be declared in code too: add `var Stakeholders =
hotamontology.New[hotamontology.Stakeholder]()` (fields `ID`, `Name`, `Domain`)
to the `spec/` package, re-run `hotam vendor-ontology` (adds
`spec/hotamontology/stakeholder.go`) and `hotam scaffold-registrydump`, which
then emits a `{"requirements":[...],"stakeholders":[...]}` envelope instead of a
bare array (a domain without `Stakeholders` keeps the old scaffold, and
`sync-domain` reads both). `sync-domain` only ever ADDS stakeholders missing from
`graph.json` (listed in the dry-run, covered by the diff-hash, landed before the
requirements that reference them); existing graph stakeholders are never
rewritten or removed.

Declaring `requirements_authority: "code"` then locks `apply-proposal`/`land`
out of hand-authoring Requirement/Rejection proposals for that domain — the
Go registry is the only authority, mirroring how the framework's own
`hotam-spec-self` domain works (`internal/selfspec/requirements_*.go`, mirrored
via `hotam sync-self`). See the root [README](../README.md) for the full
`vendor-ontology`/`scaffold-registrydump`/`sync-domain` command reference, and
`docs/AUTHORED-SPEC-CONTRACT.md` §11 for the contract framing.

## What's next

- [PROPOSAL-REFERENCE.md](PROPOSAL-REFERENCE.md) — the full JSON reference for every proposal kind.
- The root [README.md](../README.md) — build instructions, the full command
  list, and repository layout.
