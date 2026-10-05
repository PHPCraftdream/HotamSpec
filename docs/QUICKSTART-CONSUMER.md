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
later must carry a scenario-narrated `verified_by` test) with requirements in
code (`requirements_authority: "code"`: it also vendors the ontology mirror
and the registrydump bridge and writes an empty `spec/requirements.go`),
writes the project-root marker (`.hotam-spec-project`, recording `main` as the
active domain), and renders the lightweight root crystal
(`CLAUDE.md`/`AGENTS.md`/`GEMINI.md`, a few KB: requirements inline, status,
how to change, rules) plus the few `docs/gen/*` views that carry data — a
fully working project the instant the command returns:

```bash
mkdir -p my-project
cd my-project
hotam init-project .
```

This creates `domains/main/graph.json` (a genuinely EMPTY graph -- 0 nodes;
task #364 retired the earlier auto-seeded Stakeholder `owner` + Requirement
`R-domain-exists`), `domains/main/manifest.json` (`"profile": "atoms"` — the
named profile that expands, at load, to `discipline: "full"`,
`requirements_authority: "code"`, `self_executing_atoms: true` and the
consumer gen-spec profile), `domains/main/spec/` (a Go module + the vendored
`hotamspec` recorder, the `hotamontology` mirror, `registrydump`, and an
empty `requirements.go`), `domains/main/docs/gen/` (`SPEC.md`), and the root
crystal. Pass `--discipline ""` instead to get a domain without `spec/`
scaffolding whose requirements go through the JSON proposals of section 4.
Inspect what it made you:

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

> A default `hotam init-project` domain keeps its requirements in code: write
> them in `spec/requirements.go` and project them with `hotam sync-domain` (see
> "Requirements as code" below); `hotam land` refuses Requirement proposals
> there. The Requirement steps (c, d) below apply to a domain created with
> `hotam init-project --discipline ""` or a bare `hotam init`. Stakeholders,
> axes, conflicts and assumptions always use the JSON path.

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

The authoring source depends on `requirements_authority`: graph-authority
domains use proposals, while code-authority domains keep Requirements and
Rejections in `spec/requirements.go` and project them to the graph through
`sync-domain` (see Requirements as code below). In a proposal-authority domain,
`hotam land` applies a proposal and regenerates docs; `apply-proposal` alone
does not. Run `hotam gen-spec` to regenerate the current views under
`domains/main/docs/gen/` at any time:

```bash
hotam gen-spec --domain domains/main
```

Because this example has no `languages` declaration, the consumer profile
writes the existing one-language names: `SPEC.md` (with `--spec`),
`REQUIREMENTS.md` (not for code-authority + `discipline: "full"` domains,
whose text lives in `spec/requirements.go` and `SPEC.md`), and
`TENSIONS.md`/`PIPELINE.md`/`HISTORY.md`/`OPEN.md`/`UNENFORCED.md` only when
the domain has data for them. A language-configured domain uses the same
document kinds in language-suffixed views; SPEC indexes link to that locale's
shards. The engine's own self-documentation (`CONSTITUTION.md`,
`TRACEABILITY.md`, `COVERAGE.md`, `REPO-MAP.md`, `AGENT-CONTEXT.md`,
`framework/tools/`, `framework/GLOSSARY.md`, ...) is not written in consumer
profile; `hotam gen-spec --domain domains/main --profile full` renders it
one-shot, and the next consumer run removes it again.

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

## Requirements as code (the `init-project` default)

The JSON steps above author requirements as proposals (`hotam apply-proposal`/
`land`) — `graph.json` is the source of truth. A default `init-project` domain
instead declares `"requirements_authority": "code"` in its `manifest.json` and
keeps its requirements as Go literals in its own `spec/requirements.go`, with
the graph as a derived projection (task #365–#367). `init-project` already
vendored the ontology mirror, scaffolded `registrydump` and wrote an empty
registry. The flow:

```bash
# ...write the scenario test and register the requirement in domains/main/spec/requirements.go...
hotam sync-domain --domain domains/main            # dry-run: preview Go->graph diff + its hash
# show the diff to the owner; after approval:
hotam sync-domain --domain domains/main --today 2026-07-12 --confirm-hash <hex>
hotam all-violations --domain domains/main         # must print 0
```

For a domain created by bare `hotam init` or `--discipline ""`, opt in manually
first:

```bash
hotam vendor-ontology --domain domains/main        # vendor the Requirement+Registry mirror into spec/hotamontology/
hotam scaffold-registrydump --domain domains/main  # write spec/registrydump/main.go (prints the registry as JSON)
# ...then add "requirements_authority": "code" to manifest.json and author spec/requirements.go
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

## Self-executing atoms (per-domain opt-in)

The recommended way to turn atoms on for a consumer domain is the named
profile in its `manifest.json` — one field instead of several scattered
flags:

```json
{
  "profile": "atoms"
}
```

At load this expands to `discipline: "full"`,
`requirements_authority: "code"`, `self_executing_atoms: true` and the
consumer gen-spec profile. An explicitly set flag always wins over the
profile's default, and an unknown profile name is a load error. The
per-flag equivalent — for a domain that cannot use the profile — is the
scattered form:

```json
{
  "discipline": "full",
  "requirements_authority": "code",
  "self_executing_atoms": true,
  "gen_profile": "consumer",
  "atom_defaults": {
    "owner": "alice",
    "status": "SETTLED",
    "why": "Confirmed by the owner.",
    "created_at": "2026-10-01",
    "settled_at": "2026-10-01"
  }
}
```

(An existing domain that already carries the scattered flags keeps working
unchanged; the `profile` field is optional everywhere.)

Use this path for the legacy, one-language value-fact contract: the method's
doc phrase combines with its real executed value. It requires no language
configuration or rule/case metadata. The separate guide below covers
multilingual phrase blocks and explicitly declared rule/case atoms.

The owner must exist in the graph or the domain's Stakeholders registry.
Vendor the current recorder with `hotam vendor-recorder --domain domains/main`.
Write a method and one test; for this plain one-language fact, the first doc line is the phrase:

```go
// birth year
func (h Human) BirthYear() BirthYear { return h.born }

func TestBirthYear(t *testing.T) {
    hotamspec.Fact(t, Init().BirthYear, BirthYear(1987))
}
```

The resulting claim is **Birth year — 1987.** Change `Init` and the test's
expected value to change it; do not edit generated prose. A value method
contains exactly one return expression. Closures and free functions are
rejected because they do not identify a model method.

Use `hotamspec.Holds(t, predicate, evidence...)` for a bool relation;
each evidence argument is a result of `Fact`. `hotamspec.Expect(false)`
checks a false relation without wrapping or renaming its method.
`NewScenario` remains available for multi-step scenarios.

Run `sync-domain` first in dry-run mode, then confirm its returned hash
using the command above. `requirements.go` now holds only REJECTED entries
and explicit overrides, not a second fact roster. IDs default to
`R-<receiver-kebab>-<method-kebab>`; an override identifies its method through
`ImplementedBy`. When renaming a method, explicitly reject the old ID and
give the successor a `replaces` relation; the graph remains append-only.

Tests under `spec/model/` are discovered recursively. Requirements follow
source order, without hand-authored `DeclOrder`. `docs/gen/SPEC.md` indexes
`docs/gen/spec/<pkg>.md`; oversized consumer crystals show package links
and counters. Recording uses native `go test -json` caching with replayable
stdout artifacts, not a separate disk verdict cache.


## Multilingual and rule-case atoms

The default-language, one-language path above remains unchanged. Supported
service locales are `en`, `ru` and `zh`; one explicit locale uses a one-code
`languages` list and still accepts plain method comments. For a multilingual
domain using automatic method discovery, merge the following relevant fields
into the existing manifest:

```json
{
  "languages": ["en", "ru"],
  "default_language": "en",
  "requirements_authority": "code",
  "self_executing_atoms": true,
  "conformance": {
    "rule_cases": true
  }
}
```

Write the short translation pair beside the same method. Each declared
language appears exactly once; keep prose outside the blocks out of the
comment. There is one marker syntax, with no aliases:

```go
// >>>>> lang=en
// Birth year is at least one.
//
// >>>>> lang=ru
// Год рождения не меньше одного.
func (h Human) HasValidBirthYear() bool {
	return h.born >= 1
}
```

The same atom ID, method, graph and test executions supply every locale view.
One-language mode accepts an ordinary unmarked phrase; in multilingual mode
each declared language needs its own non-empty block. Unknown, duplicate,
empty or missing blocks are errors, not a reason to guess a language or fall
back to another translation. Changing `default_language` changes the primary
claim/view, not atom identity or verdict.

For automatic `sync-domain` method discovery of `Fact`/`Holds` atoms, the
manifest must declare `requirements_authority: "code"` and
`self_executing_atoms: true` — both supplied by the `"profile": "atoms"`
manifest field (see above). `conformance.rule_cases` is required ONLY for
`WithCase` semantics; it is NOT needed for method discovery or plain fact
recording. `requirements_authority` selects the Go-to-graph
source; `self_executing_atoms` enables atom discovery; `rule_cases` permits
explicit rule/case semantics. `self_executing_atoms` alone preserves legacy
`Fact`/`Holds` behavior and does not permit rule cases; `rule_cases` alone does
not scan model methods. Explicit graph-authority case descriptors can be
written through Requirement proposals; their `Test` links are case/report
references, not `verified_by` proof or a method-discovery trigger.

`WithCase` turns the existing `Fact` or `Holds` call into rule mode: the method doc text
is the stable norm; each case supplies an independent expected value and
records its actual observation separately. The bound method remains the
requirement subject. The case ID is stable and required; the test/subtest,
input, and expected value come from the real recorder invocation:

```go
type Parser struct{ input []byte }

// >>>>> lang=en
// The parser accepts a non-empty input not starting with '!'.
//
// >>>>> lang=ru
// Парсер принимает непустой ввод, не начинающийся с «!».
func (p Parser) Accepts() bool { return len(p.input) != 0 && p.input[0] != '!' }

func TestParserCases(t *testing.T) {
	cases := []struct {
		id    string
		input []byte
		want  bool
	}{
		{id: "valid-input", input: []byte("valid"), want: true},
		{id: "invalid-input", input: []byte("!"), want: false},
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			p := Parser{input: tc.input}
			hotamspec.Fact(t, p.Accepts, tc.want,
				hotamspec.WithInput(hotamspec.Bytes(tc.input)),
				hotamspec.WithCase(hotamspec.CaseContext{
					ID: tc.id, Operation: "parse", Target: "parser",
					Producer: "in-package-parser",
				}),
			)
		})
	}
}
```

Use an independently authored `want`; never derive it from the same actual
result or call the system under test again to make an oracle. `CaseContext.ID`
is the stable case identity. The real test/subtest name comes from execution;
`WithInput` and `want` supply authored input/oracle data. Discovery may project
these values into a new `CaseDefinition` (`Input` and `Expected`) while
keeping the actual result only in observation data. An explicit
registry/proposal case is an override: recorded declarations must agree or
`sync-domain` refuses the mismatch instead of overwriting the descriptor.
Do not maintain a second registry copy for ordinary test-table cases.

For a shared multi-property case, optional `CaseContext.Expected` carries a
`hotamspec.TypedValue` oracle at the artifact root as `case_expected`, never
inside `case.expected`. If absent, the root value comes from `Fact`'s explicit
`want` or `Holds`' `Expect`; each observation's `RawExpected` remains specific
to its own comparison.

Declare `Operation`, `Producer`, `Target`, and `Profile` where applicable;
they are provenance/selection metadata, not guessed from a result.
`Fixtures`, `Conditions`, `Sides`, and `Selection` carry only declared
references or evidence the producer actually observed. A failed comparison
fails the test while passing siblings remain observable.

Case inputs do not create new requirements. Several cases can exercise one
stable rule atom, and one fixture or execution can support several atoms
without duplicating a real operation. The case/corpus report describes its
declared evidence only; passing fixtures do not establish semantic
completeness or universal conformance.

Language views use one shared graph, evidence/review store and execution
snapshot. Multilingual output uses `SPEC.<lang>.md` and
`spec/<lang>/<pkg>.md`; other localized projections use the language suffix
where emitted. The default-language `CLAUDE.md` remains the standard boot
entry point, with additional `CLAUDE.<lang>.md` views where generated. See
[AUTHORED-SPEC-CONTRACT.md §13](AUTHORED-SPEC-CONTRACT.md#13-atomic-multilingual-and-conformance-specifications)
for source clauses, typed bytes/errors, profiles, precedence, composition and
the structural audit boundary.

## Inspect failing evidence without publishing a false passing spec

```bash
hotam evidence --domain domains/main --json --write
hotam findings list --domain domains/main --json
hotam findings show <F-id> --domain domains/main --json
hotam findings review <F-id> --domain domains/main \
  --kind needs_review --status open \
  --rationale "Explain the classification" --decision-ref "Recorded decision"
```

`evidence` saves positive and negative observations under `docs/gen/` even
when a test fails, then returns nonzero. A passing test remains visible when
its package has a failing sibling. Atom tests are observed before the first
successful sync too; report-only findings do not create graph requirements.
`gen-spec --spec` and graph mutation keep their strict passing-proof gates.

`Fact` automatically records actual and expected. Use `WithInput` and
`WithContext` for explicit input/version/profile context. A method can return
`hotamspec.Observed(value, hotamspec.Observe(name, input, actual, expected))`
to expose detailed comparisons inside a summary check; failed inner
comparisons fail the real test even when its summary equals `want`.

Declare `specification_sources` in the domain manifest: each source has
`id`, `path`, `version`, and the SHA-256 of its exact bytes. Add typed
`SourceLinks` (`source_id`, `anchor`) to the registry exception for its
method. Anchors resolve to headings or `Lx-Ly`/`Lx`; relative paths resolve
from the consumer domain, not an arbitrary current working directory.
Regenerate the vendored ontology before using these new fields.

Coverage distinguishes execution-verified evidence, discrepancies, unsupported
recommendations, profile-qualified unreachable branches, and unverified
obligations. Author qualifications require rationale and source links;
unreachable also requires a profile. They never erase an observed failure.
These links/statuses are structural evidence, not automatic semantic proof.

Finding review notes live separately from current observations and are not a
test-verdict cache. Classify `specification_issue`, `model_issue`,
`implementation_issue`, or `needs_review` deliberately; the tool does not
assign blame. Lexical `confront` suspicions likewise do not establish a
contradiction or block a write without an explicit unresolved Conflict carrier.

## What's next

- [PROPOSAL-REFERENCE.md](PROPOSAL-REFERENCE.md) — the full JSON reference for every proposal kind.
- The root [README.md](../README.md) — build instructions, the full command
  list, and repository layout.
