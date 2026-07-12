# Quickstart for consumers

This is the path for a team that wants to **use** Hotam-Spec to hold a
shared, disciplined understanding of its own system — requirements, owners,
open tensions — via the `hotam` Go CLI, in your own repo, not by cloning the
framework repo and working inside it. If you *are* working inside a clone of
the HotamSpecGo repo itself (self-hosting mode against `domains/hotam-spec-self`),
see the [root README](../README.md) instead.

Everything below is CLI-only. No AI agent is required to follow this guide.

## 1. Build the CLI

There is no published package or `go install` target yet (tracked as
ongoing work) — build the binary from a clone of this repo:

```bash
git clone <this-repo-url> HotamSpecGo
cd HotamSpecGo
go build -o bin/hotam ./cmd/hotam
```

or run it without a persistent binary:

```bash
go run ./cmd/hotam <command> [flags] [args]
```

The rest of this guide assumes `hotam` on your `PATH` (or substitute
`go run ./cmd/hotam` — with a working directory inside this repo — for every
`hotam ...` call below).

## 2. Set up your domain directory

A domain is a directory holding one `graph.json` file at its root — the
entire graph as data, no source-code scaffold to generate. `hotam gen-spec`
later derives readable Markdown views (and its OWN copy of the graph, at
`docs/gen/graph.json`) FROM this file — that generated copy is a build
artifact, never the one you hand-edit or bootstrap.

`hotam init` scaffolds this for you — it works from ANY working directory,
against ANY target path, and does not require your project to live inside
this repository or contain a `domains/` ancestor at all:

```bash
mkdir -p my-project
cd my-project
hotam init domains/my-shop --name my-shop
```

This creates `domains/my-shop/graph.json` with one seed Stakeholder
(`owner`) and one seed SETTLED Requirement (`R-domain-exists`) — already
`all-violations`-clean, so there is no "empty invalid graph" state to dig
out of — plus `domains/my-shop/manifest.json` (`{"self_hosting": false}`),
an empty `domains/my-shop/docs/gen/` directory ready for `hotam gen-spec`,
and a `domains/my-shop/README.md` pointing back at the next commands to
run. Inspect what it made you:

```bash
hotam all-violations --domain domains/my-shop   # 0 violations — graph clean
hotam what-now --domain domains/my-shop         # the framework's live status tool
hotam req show R-domain-exists --domain domains/my-shop
```

If you'd rather start from a completely empty graph (no seed nodes at all —
an empty graph passes every invariant too) you can still hand-write one, as
`hotam init` itself does under the hood:

```bash
cat > domains/my-shop/graph.json <<'EOF'
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
hotam what-now --domain domains/my-shop
```

This is the framework's live status tool: it reads your graph (currently
just `hotam init`'s seed stakeholder + seed requirement, or empty if you
hand-wrote a bare graph.json instead) and tells you the next correct action.
Run it again after every change — it is how you navigate the methodology
without getting lost. `--limit N` caps how many signals it prints (default
20).

## 4. Create your first Stakeholder, Requirement, and Conflict

The graph is **never hand-edited** past the `hotam init` (or bare
`graph.json`) bootstrap above. Every change goes through
`hotam apply-proposal`, which reads a small JSON file, applies it to
`domains/my-shop/graph.json`, and fails closed (writes nothing) if the
change would introduce a new invariant violation. `hotam land` does the
same apply step and then also regenerates `docs/gen/` and re-verifies in one
call — prefer it over standalone `apply-proposal` unless you specifically
want to batch several proposals before regenerating docs once at the end.

```bash
# a) at least TWO stakeholders — a conflict's steward may not own either side.
echo '{"kind":"Stakeholder","id":"alice","name":"Alice","domain":"product"}'    > sh1.json
echo '{"kind":"Stakeholder","id":"bob","name":"Bob","domain":"engineering"}'    > sh2.json
echo '{"kind":"Stakeholder","id":"carol","name":"Carol","domain":"governance"}' > sh3.json
hotam apply-proposal sh1.json --domain domains/my-shop --today 2026-07-12
hotam apply-proposal sh2.json --domain domains/my-shop --today 2026-07-12
hotam apply-proposal sh3.json --domain domains/my-shop --today 2026-07-12   # a NEUTRAL steward for the conflict below

# b) an axis — the shared dimension your first tension lives on.
echo '{"kind":"Axis","slug":"speed-vs-rigor","description":"ship fast vs verify thoroughly"}' > ax1.json
hotam apply-proposal ax1.json --domain domains/my-shop --today 2026-07-12

# c) two requirements that will turn out to contradict each other.
# hotam land applies AND regenerates docs/gen AND re-verifies in one call.
echo '{"kind":"Requirement","id":"R-ship-fast","claim":"Ship within one week.","owner":"alice","status":"SETTLED","enforcement":"PROSE"}'          > r1.json
echo '{"kind":"Requirement","id":"R-verify-all","claim":"Verify every change before release.","owner":"bob","status":"SETTLED","enforcement":"PROSE"}' > r2.json
hotam land r1.json --domain domains/my-shop --today 2026-07-12
hotam land r2.json --domain domains/my-shop --today 2026-07-12

# d) your first Conflict — the tension between them, held by the neutral party.
echo '{"kind":"Conflict","axis":"speed-vs-rigor","context":"first release cadence","members":["R-ship-fast","R-verify-all"],"steward":"carol"}' > c1.json
hotam apply-proposal c1.json --domain domains/my-shop --today 2026-07-12

# e) re-check your pulse — the new conflict now awaits carol's ACKNOWLEDGE.
hotam what-now --domain domains/my-shop
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
`domains/my-shop/docs/gen/` standalone at any time with:

```bash
hotam gen-spec --domain domains/my-shop
```

(Known issue at the time of writing: `gen-spec` can fail while writing its
per-tool doc pages if a command's usage string contains characters that are
illegal in a filename on your OS — e.g. Windows rejects `[`/`]`/`<`/`>` in
path segments. `all-violations`/`apply-proposal`/`req`/`gate` above are
unaffected; only the `docs/gen/tools/*.md` step of `gen-spec` can trip this.)

## 6. Verify the graph stays structurally sound

```bash
hotam all-violations --domain domains/my-shop
```

Prints every invariant violation and exits 1 if any exist; exits 0 with
`0 violations — graph clean` otherwise. `hotam apply-proposal` already
refuses to write a change that would introduce a NEW violation, so this is
mainly useful as a standalone health check (e.g. in CI) or after a batch of
several proposals.

## 7. Selecting a targeted test subset for a change

```bash
hotam gate <target-anchor> --domain domains/my-shop
```

Given an anchor id (a Requirement/Conflict/Assumption id), prints a
best-effort Tier-1 subset of tests/checks relevant to that node — useful
once your own domain has code-level enforcers wired to `enforced_by`.

## Everyday commands

Once your domain has requirements, `hotam req` gives you fast, graph-backed
access without grepping generated docs:

```bash
hotam req list --domain domains/my-shop --status SETTLED     # compact table: id / status / enforcement / owner
hotam req show R-ship-fast --domain domains/my-shop           # full node details (add --json for machine output)
hotam req search "verify" --domain domains/my-shop            # case-insensitive search across id / claim / why
hotam req context R-ship-fast --domain domains/my-shop --json # agent-ready context package (owner + assumptions + conflicts)
hotam req related R-ship-fast --domain domains/my-shop        # neighbor id + relation-kind list for any anchor
```

`hotam req list` also accepts `--owner` and `--enforcement` filters. There is
no `patch`/write subcommand under `req` today — every write still goes
through `hotam apply-proposal` (see above); `req` is read-only.

`--domain` defaults to `domains/hotam-spec-self` (this repo's own
self-hosted domain) if omitted — always pass `--domain <path>` explicitly
when working against your own project, as in every example above.

## What's next

- [PROPOSAL-REFERENCE.md](PROPOSAL-REFERENCE.md) — the full JSON reference for every proposal kind.
- The root [README.md](../README.md) — build instructions, the full command
  list, and repository layout.
