# Quickstart for consumers

This is the path for a team that wants to **use** Hotam-Spec to hold a shared, disciplined understanding of its own system — requirements, owners, open tensions — via the `hotam` Go CLI, in your own repo. If you *are* self-hosting inside a clone of the HotamSpec repo (`domains/hotam-spec-self`), see the [root README](../README.md) instead. Everything below is CLI-only; no AI agent is required.

## Part 1 — the short path

A complete, verified run from zero to a live requirement whose generated prose is derived from executed code. Every command and output below is real.

### 1. Build the CLI

There is no published package or `go install` target yet — build from a clone of this repo:

```bash
git clone <this-repo-url> HotamSpec
cd HotamSpec
go build -o bin/hotam ./cmd/hotam
```

or run it without a persistent binary (`go run ./cmd/hotam <command> ...`). The rest of this guide assumes `hotam` on your `PATH`.

### 2. Scaffold the project

```bash
hotam init-project my-project && cd my-project
```

This scaffolds a base domain `main` under `domains/main`: a manifest `{"profile": "atoms"}` (expanding at load to `discipline: "full"`, `requirements_authority: "code"`, `self_executing_atoms: true` and the consumer gen-spec profile), an EMPTY 0-node `graph.json`, a `spec/` Go module (vendored `hotamspec` recorder, `hotamontology` mirror, `registrydump` bridge, empty `requirements.go`), `docs/gen/SPEC.md`, and the root crystal `CLAUDE.md`/`AGENTS.md`/`GEMINI.md`. The command ends with `initialized project at my-project (base domain "main" under domains\main)` plus printed next steps.

Two things the first `sync-domain` needs are seeded for you: `spec/stakeholders.go` declares a seed requirement owner (default id `owner`; customize with `hotam init-project my-project --owner alice`) and the manifest carries `"atom_defaults"` naming that owner — and because `stakeholders.go` exists at scaffold time, the `registrydump` bridge already prints the `{"requirements":[...],"stakeholders":[...]}` envelope `sync-domain` reads. No manual owner-registration step is required.

### 3. Write the first atom

Two files — methods in `spec/model/`, the test in a `_test.go` file (go test only runs `_test.go`):

**`spec/model/human.go`**

```go
package model

type BirthYear int

type Human struct {
	Born BirthYear
}

func Init() Human { return Human{Born: 1987} }

// Birth year
func (h Human) BirthYear() BirthYear { return h.Born }
```

**`spec/model/human_test.go`**

```go
package model

import (
	"testing"

	"main-spec/hotamspec"
)

func TestBirthYear(t *testing.T) {
	hotamspec.Fact(t, Init().BirthYear, BirthYear(1987))
}
```

### 4. (already done by init-project)

There is no owner-registration step anymore: `init-project` already wrote
`spec/stakeholders.go` (a seed stakeholder registered under the default id
`owner`), added `"atom_defaults"` to the manifest, and scaffolded the
`registrydump` bridge in envelope mode (`{"requirements":[...],"stakeholders":[...]}`) —
so the first `sync-domain` passes `check_no_dangling_requirement_owner` with zero
manual follow-up.

To use a different owner, pass it at init time (`hotam init-project my-project --owner alice`) — or, for an existing project, edit the registration in `spec/stakeholders.go` and the `atom_defaults.owner` field in `domains/main/manifest.json` to the same id.

### 5. Test and project onto the graph (dry-run by default)

```
$ cd domains/main/spec && go test ./...
ok  	main-spec/model
$ hotam sync-domain --domain domains/main
hotam sync-domain — DRY RUN (default mode; pass --confirm-hash <hex> to write)

[ADDED stakeholder] owner
    name: owner
    domain: main
[ADDED] R-human-birth-year

gate preview (7-9 — what a real --confirm-hash run would enforce):
  [7 confront] clear — no unresolved formal conflict carriers
  [8 pre/post-violations] clear — no new invariant violations
  [9 append-only] clear

diff-hash: 975fc7f03ce20c8772bd69ca9af85d69ebe1efc6a5034a44db24bbe69da5dd09
```

### 6. Land it and read the generated text

```bash
hotam sync-domain --domain domains/main --today 2026-10-05 --confirm-hash <hex>
```

```
synced 0 changed, 1 added requirement(s), 1 stakeholder(s) added into domains\main\graph.json
regenerated 6 doc(s)
sync-domain landed: graph synced, docs regenerated, 0 violations
```

`sync-domain` regenerates the docs; `hotam gen-spec --domain domains/main --claude-md CLAUDE.md --spec` re-renders them on demand (`--spec` requires a real passing test run). The claim is **derived, never hand-written**: the root `CLAUDE.md` now contains

```
- R-human-birth-year — Birth year — 1987. [E] ← TestBirthYear
```

and `docs/gen/spec/model.md` shows `**Claim:** Birth year — 1987.` with the link to `spec/model/human_test.go:TestBirthYear`.

### 7. Change the value and watch the text follow

Set `Born: 1990` in `Init()` and `BirthYear(1990)` in the test, re-run `go test ./...`, then dry-run + confirm-hash again (the same two commands; a new diff-hash is printed). `CLAUDE.md` now reads:

```
- R-human-birth-year — Birth year — 1990. [E] ← TestBirthYear
```

You never edited generated prose — the doc-phrase method plus the executed value ARE the requirement. Standalone health check: `hotam all-violations --domain domains/main` → `0 violations — graph clean`.

## Beyond the happy path

### What `init-project` made you

The graph starts genuinely empty (0 nodes), so `all-violations` is clean by
construction and `hotam what-now --domain domains/main` is the live status
tool that tells you the next correct action — run it after every change.

`--domain` resolution (full precedence chain in the root
[README](../README.md)'s `--domain` section): an explicit `--domain <path>`
always wins; otherwise a `HOTAM_DOMAIN` env var or the project-root marker's
active-domain preference (set automatically by `init-project`, or via
`hotam use <name>`); only with none of these set does it fall back to
`domains/hotam-spec-self`. A project set up via `init-project` therefore
needs `--domain` only as an OVERRIDE — this guide keeps it explicit anyway,
as the most copy-pasteable form.

To add a SECOND domain, or a base domain outside the default layout, use a
bare `hotam init domains/my-second-shop --name my-second-shop` instead — it
scaffolds only the domain (no project marker, no root crystal, and unlike
`init-project` no `spec/` scaffolding). Passing `--discipline ""` to
`init-project` gives a domain without `spec/` whose requirements go through
the JSON-proposal path (`hotam apply-proposal` / `hotam land`) — see
[PROPOSAL-REFERENCE.md](PROPOSAL-REFERENCE.md) for every proposal shape with
required/optional fields and worked examples. The graph is never hand-edited
past the bootstrap: every write goes through proposals or `sync-domain`,
which fail closed on new invariant violations.

### Everyday commands

Once your domain has requirements, `hotam req` gives you fast, graph-backed
access without grepping generated docs:

```bash
hotam req list --domain domains/main --status SETTLED     # compact table: id / status / enforcement / owner
hotam req show R-human-birth-year --domain domains/main           # full node details (add --json for machine output)
hotam req search "birth" --domain domains/main            # case-insensitive search across id / claim / why
hotam req context R-human-birth-year --domain domains/main --json # agent-ready context package (owner + assumptions + conflicts)
hotam req related R-human-birth-year --domain domains/main        # neighbor id + relation-kind list for any anchor
```

`hotam req list` also accepts `--owner` and `--enforcement` filters. There is
no `patch`/write subcommand under `req` today — every write still goes
through `sync-domain` or proposals; `req` is read-only.

### Bool relations (Holds)

For a bool relation use `hotamspec.Holds(t, predicate, evidence...)`; each
evidence argument is a result of `Fact`. `hotamspec.Expect(false)` checks a
false relation without wrapping or renaming its method, and false bool
methods carry the negation as a `// not: <phrase>` doc line. See the
[contract §8](AUTHORED-SPEC-CONTRACT.md#8-сценарный-слой-hotamspec--генерируемый-specmd--гейты-обязательности-w1w2).

### Multi-step scenarios

`hotamspec.NewScenario` gives multi-step narratives with
`.Given` / `.When` / `.Then` / `.Value`; the recorded steps become the body
of the generated `SPEC.md`. See the
[contract §8](AUTHORED-SPEC-CONTRACT.md#8-сценарный-слой-hotamspec--генерируемый-specmd--гейты-обязательности-w1w2).

### Multilingual atoms

The default-language path above stays unchanged. Supported service locales
are `en`, `ru` and `zh`; a single explicit locale uses a one-code `languages`
list and still accepts plain method comments. For a multilingual domain,
merge into the manifest:

```json
{"languages": ["en", "ru"], "default_language": "en"}
```

and write one translation block per declared language beside the method —
one marker syntax, no aliases:

```go
// >>>>> lang=en
// Birth year is at least one.
//
// >>>>> lang=ru
// Год рождения не меньше одного.
func (h Human) HasValidBirthYear() bool { return h.Born >= 1 }
```

The same atom ID, method, graph and test executions supply every locale
view. Unknown, duplicate, empty or missing blocks are errors, not a reason
to guess a language. Changing `default_language` changes the primary
claim/view, not atom identity or verdict. See the
[contract §13](AUTHORED-SPEC-CONTRACT.md#13-atomic-multilingual-and-conformance-specifications).

### Rule/case atoms (WithCase)

`WithCase` turns an existing `Fact` or `Holds` call into rule mode: the
method doc text is the stable norm, and each case supplies an independent,
independently authored expected value plus its own recorded observation
(`hotamspec.WithCase(hotamspec.CaseContext{ID: ..., Operation: ..., Target: ..., Producer: ...})`,
optionally `WithInput` for authored input data). `CaseContext.ID` is the
stable case identity; case inputs never create new requirements, and several
cases can exercise one stable rule atom. Manifest note: `conformance:
{"rule_cases": true}` is required ONLY for `WithCase` semantics — it is NOT
needed for method discovery or plain fact recording. See the
[contract §13](AUTHORED-SPEC-CONTRACT.md#132-rule-atoms-cases-and-execution)
for the full example and case-descriptor details.

### Evidence and findings

```bash
hotam evidence --domain domains/main --json --write
hotam findings list --domain domains/main --json
hotam findings show <F-id> --domain domains/main --json
hotam findings review <F-id> --domain domains/main \
  --kind needs_review --status open \
  --rationale "Explain the classification" --decision-ref "Recorded decision"
```

`evidence` saves positive and negative observations under `docs/gen/` even
when a test fails, then returns nonzero — this is how you inspect a failure
without publishing a false passing SPEC: `gen-spec --spec` and graph
mutation keep their strict passing-proof gates. `Fact` automatically records
actual and expected; use `WithInput`/`WithContext` for explicit context.
Declare `specification_sources` in the manifest to pin exact source bytes
(id, path, version, SHA-256) with typed `SourceLinks` anchors; coverage
qualifications (discrepancies, unreachable branches, unverified obligations)
always require rationale and source links and never erase an observed
failure. Finding review notes live separately from current observations and
are not a test-verdict cache. See the
[contract §12](AUTHORED-SPEC-CONTRACT.md#12-наблюдения-находки-и-источники).

### After an engine upgrade: `hotam upgrade`

When you pull a newer engine, a domain that vendors engine-generated Go files
will start reporting staleness violations
(`check_recorder_current`, `check_ontology_vendor_current`, stale
`spec/registrydump/main.go`, stale generated docs/`CLAUDE.md`). One command
refreshes all of them:

```bash
hotam upgrade --domain domains/main
```

It re-vendors the recorder and the ontology mirror, refreshes the
engine-generated registrydump, regenerates `docs/gen/*` and the crystal, then
prints the final `all-violations` result. Steps your domain doesn't use are
skipped (not errors), and a hand-modified `spec/registrydump/main.go` (no
engine banner) is reported and left untouched. Pass `--today YYYY-MM-DD` to
pin the date embedded in the regenerated docs.

### Requirements as code reference

The `"profile": "atoms"` manifest field is the named form of the
self-executing-atoms configuration; the scattered-flag equivalent is
`{"discipline": "full", "requirements_authority": "code",
"self_executing_atoms": true, "gen_profile": "consumer"}` (an explicitly set
flag always wins over the profile's default; an unknown profile name is a
load error). Under `requirements_authority: "code"` the Go registry is the
only requirement authority — `apply-proposal`/`land` cannot hand-author
Requirement proposals for the domain — and `requirements.go` holds only
REJECTED entries and explicit overrides. IDs default to
`R-<receiver-kebab>-<method-kebab>`; when renaming a method, explicitly
reject the old ID and give the successor a `replaces` relation (the graph
remains append-only). Tests under `spec/model/` are discovered recursively,
requirements follow source order, and recording uses native
`go test -json` caching. See the
[contract §11](AUTHORED-SPEC-CONTRACT.md#11-code-authority-требования-как-go-код-sync-domain)
and the root [README](../README.md)'s `sync-domain` entry for the full
`vendor-ontology` / `scaffold-registrydump` / `sync-domain` command
reference.

### What's next

- [PROPOSAL-REFERENCE.md](PROPOSAL-REFERENCE.md) — the full JSON reference for every proposal kind.
- The root [README.md](../README.md) — build instructions, the full command
  list, and repository layout.
