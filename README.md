# HotamSpec

HotamSpec is a framework for writing requirements as executable code.

A requirement is not prose that code later tries to match. It becomes an
atomic object with a method that does exactly what the requirement says. The
test that exercises the method is also the generator of the requirement's
text: one run both proves the behavior and emits the sentence describing it.

The generated text is deliberately minimal — just enough to be mirrored back
into the same code: from the sentence you can find or rewrite the method; from
the method you can reconstruct the sentence.

```go
s := hotamspec.NewScenario(t, "R-signoff-zero-blockers", "Sign-off requires zero blockers")
s.Given("a package with one outstanding blocker")
err := p.SignOff()
s.When("SignOff is called")
s.Then("sign-off is rejected", errors.Is(err, ErrHasBlockers))
```

`hotam gen-spec --spec` runs the test and renders, under
`R-signoff-zero-blockers` in `SPEC.md`:

```markdown
**Sign-off requires zero blockers**

- Given a package with one outstanding blocker
- When SignOff is called
- Then sign-off is rejected — **held**
```

The text appears only for a passing run. A plain `go test` stays pure asserts.

The `hotam` CLI keeps the bookkeeping around this core: the requirement
registry (`graph.json`), links from each requirement to the code that
implements it and the test that proves it, structural checks, and generated
docs. Conflicting requirements are kept open as explicit `Conflict` nodes,
never silently dropped.

> Background: an earlier Python prototype of this methodology lives in the git
> history (it was superseded by this Go implementation, which is now the
> canonical one). It is referenced only for historical context, not as a
> supported artifact.

## Install

```bash
go install github.com/PHPCraftdream/HotamSpec/cmd/hotam@latest
```

This puts the `hotam` binary in `$GOBIN` (or `$(go env GOPATH)/bin`). It requires
Go 1.25+ (see `go.mod`). The module path in `go.mod` matches the git remote;
until a first release tag is published, `@latest` will not resolve — use
`go install <path>@<specific-tag-or-commit>` or build from source (see
[Build](#build) below).

### Version

```bash
hotam version
# or
hotam --version
```

Prints `hotam dev (commit: unknown, built: unknown)` for a plain local build.
Release builds inject the version (and commit/build date) via `-ldflags`:

```bash
go build -ldflags "-X main.version=v0.1.0 -X main.commit=abc1234 -X main.buildDate=2026-07-12" \
  -o bin/hotam ./cmd/hotam
```

### Tagging a release (process, not done in this wave)

When the resolver decides to publish a version: confirm the module path ↔ remote
match (see `go.mod`) → run `go test ./...` on a clean tree → place a git tag of
the form `v0.x.y` on the commit → (optionally) build release binaries with
`-ldflags "-X main.version=v0.x.y ..."` for the target platforms. Tagging is a
manual resolver step, not something this document performs.

## Build

```bash
go build -o bin/hotam ./cmd/hotam
```

or, without producing a binary:

```bash
go run ./cmd/hotam <command> [flags] [args]
```

## CLI commands

The `hotam` binary (see `cmd/hotam/main.go`) implements 21 commands:

```
hotam init <dir> [--name <domain-name>] [--profile consumer|full]
        Scaffold a new domain: a genuinely EMPTY graph.json (0 nodes;
        all-violations=0 by construction -- task #364 retired the earlier
        auto-seeded Stakeholder + SETTLED Requirement), manifest.json
        (defaults to the consumer gen-spec profile, matching init-project),
        docs/gen/, and a README.md pointing at the next commands to run.
        --profile full overrides to the heavier framework-self-hosting doc
        set. <dir> may be anywhere on disk — it does not need to live under
        this repository or contain a domains/ ancestor.

hotam init-project <dir> [--domain <name>] [--today YYYY-MM-DD]
        Bootstrap an external business project's full Hotam-Spec layout in
        one call: scaffold a base domain under <dir>/domains/<name> (default
        <name>=main, empty graph -- 0 nodes, task #364), write the project-root
        marker (.hotam-spec-project), and render the root crystal
        (CLAUDE.md/AGENTS.md/GEMINI.md) + all docs/gen/* via gen-spec. Defaults
        to --discipline full (BORN FULLY OBLIGATED: scaffolds spec/go.mod + the
        vendored hotamspec recorder, so every SETTLED requirement carries a
        scenario-narrated verified_by test from the start; pass --discipline ""
        for a bare soft-discipline domain matching `hotam init`). Refuses to
        overwrite an existing project marker or CLAUDE.md. <dir> may be anywhere
        on disk.

hotam use <domain-name>
        Set the active-domain preference for the current project: records
        {"active_domain": "<name>"} in the project-root marker so a bare
        `hotam <command>` (no --domain) targets the chosen domain. Refuses
        if <root>/domains/<name>/graph.json does not exist.

hotam gen-spec [--domain <path>] [--today YYYY-MM-DD] [--claude-md <path>]
                [--profile consumer|full]
        Generate all docs/gen/*.md + graph.json for a domain graph.
        --profile overrides the domain's manifest.json gen_profile for this
        run (without rewriting it): consumer skips thinking/*.md, Planned
        tool pages, and empty atoms docs for a leaner consumer-facing
        output; full is the default, backward-compatible output. Omitted
        (empty) resolves from the domain's manifest, defaulting to full.

hotam what-now [--domain <path>] [--limit N] [--today YYYY-MM-DD]
        Print the top-N diagnosed signals (default 20).

hotam apply-proposal <proposal.json> --domain <path> --today YYYY-MM-DD [--batch <dir>]
        Apply a proposal to a domain graph. Low-level: does not regenerate
        docs — run gen-spec after, or use land instead. With --batch <dir>,
        every *.json in <dir> is applied atomically in filename order
        (all-or-nothing): if any proposal fails the graph is untouched.

hotam land <proposal.json> --domain <path> --today YYYY-MM-DD [--claude-md <path>] [--batch <dir>]
        Apply a proposal, regenerate docs/gen for the domain, then re-check
        all invariants in one step. With --batch <dir>, every *.json in <dir>
        is applied atomically in filename order and docs are regenerated
        exactly once (not once per proposal).

hotam gate <target-anchor> [--domain <path>]
        Select a Tier-1 test subset for a target node.

hotam all-violations [--domain <path>]
        Print all invariant violations; exit 1 if any are found.

hotam req <show|list|search|context|related> [args] [--domain <path>] [--json]
        Compact agentic read interface over the domain graph
        (hotam req -h for details).

hotam brief <anchor-id> [--domain <path>] [--today YYYY-MM-DD] [--json]
        Single-call orientation brief for any anchor (Requirement, Conflict,
        or Assumption): full card + one-hop neighborhood + freshness (for
        Requirements), replacing req show + req context + req related + due.

hotam due [--domain <path>] [--today YYYY-MM-DD] [--json]
        Advisory report of OVERDUE and NEVER-REVIEWED SETTLED requirements.
        Never gates; exit code always 0.

hotam status [--domain <path>] [--today YYYY-MM-DD] [--json]
        Single-shot compact summary combining what-now's top action + debt,
        due's freshness counts, and all-violations' violation count, so an
        agent doesn't need to run all three separately. Never gates; exit
        code always 0.

hotam inspect [--domain <path>] [--json] [--limit N] [--min-score N]
        Advisory listing of semantic conflict candidates with evidence
        (shared-assumption clusters, entity-state suspects, lexical claim
        overlap, axis co-reference). --min-score (default 5) suppresses
        low-signal candidates; 0 shows all. Never gates; exit code always 0.

hotam confront <text> [--domain <path>] [--file <path>] [--proposal <path>] [--json]
        CONFRONT step of the mediation loop: checks a candidate claim for
        lexical overlap with SETTLED requirements (duplicate guard) and
        REJECTED history (anti-relitigation) before anything is written.
        <text> is a quoted positional; --file <path> reads a long draft.
        --proposal <path> confronts a full draft proposal JSON file instead:
        runs both the lexical check AND a structural check (shared-assumption
        / axis overlap) against the proposal's kind; mutually exclusive with
        <text>/--file. Reuses the inspect overlap engine. Never gates; exit
        code always 0.

hotam propose <requirement|rejection|stakeholder|axis|assumption|conflict> [flags]
        Draft a proposal JSON from flags (schema knowledge lives in the
        tool, not agent memory), run an automatic CONFRONT check before
        writing, and optionally --land (apply+regen+reverify) in the same
        call. Kinds: requirement (--id, --claim, --owner, --status, …),
        rejection (--requirement-id, --reason, --replaced-by), stakeholder
        (--id, --name, --domain, --why), axis (--slug, --description),
        assumption (--id, --statement, --status, --owner), conflict (--axis,
        --context, --members, --resolver). Other complex kinds
        (ConflictTransition, EntityType, …) keep the hand-authored-JSON path
        (hotam land <file.json>).

hotam sync-self [--domain <path>] [--today YYYY-MM-DD] [--confirm-hash <hex>]
                [--reason "..."] [--ack-conflict <C-id>] [--decision-ref "..."] [--json]
        Mirror internal/selfspec.Requirements (the Go registry) onto
        domains/hotam-spec-self/graph.json — the engine's own self-hosting
        domain, and only that domain. Default mode is dry-run: computes the
        pending ADDED/CHANGED SyncReport, previews the confront/pre-post-
        violation/append-only gates, and prints a diff-hash; writes nothing.
        Pass the printed hash back via --confirm-hash (with --today) to write
        for real, after re-verifying the hash against the current on-disk
        state and a stale-binary check (run via `go run ./cmd/hotam sync-self`
        to always be fresh).

hotam vendor-ontology --domain <path>
        Copy the engine's canonical minimal Requirement + Registry[T] mirror
        (internal/ontology/canon) into <domain>/spec/hotamontology/
        {requirement,registry}.go, banner-stamped do-not-edit. Requires
        <domain>/spec/go.mod to already exist (the domain's own Go module
        for its authored spec/ tree); idempotent, always overwrites with the
        current canon. Infrastructure prerequisite for a consumer domain to
        author its own Go requirements registry (see the "middle path" in
        CLAUDE.md's founding-canvas step 6).

hotam scaffold-registrydump --domain <path>
        Write <domain>/spec/registrydump/main.go, a minimal Go program that
        imports the domain's own spec/ module root package's Requirements
        registry plus the vendored spec/hotamontology package, and prints
        json.Marshal(Requirements.All()) to stdout. Requires spec/go.mod and
        an already-vendored spec/hotamontology (run vendor-ontology first);
        idempotent. Bridges the module boundary hotam sync-domain needs to
        read a consumer domain's own Go requirements registry.

hotam sync-domain --domain <path> [--today YYYY-MM-DD] [--confirm-hash <hex>]
                   [--reason "..."] [--ack-conflict <C-id>] [--decision-ref "..."] [--json]
        Mirror a CONSUMER domain's own Go requirements registry
        (<domain>/spec/requirements.go, read via a `go run ./registrydump`
        subprocess inside the domain's own spec/ module) onto that domain's
        graph.json — the domain-scoped generalization of sync-self. Same
        dry-run-by-default / --confirm-hash handshake and confront/
        pre-post-violation/append-only gate order as sync-self. Declaring
        `"requirements_authority": "code"` in the domain's manifest.json
        then locks apply-proposal/land out of hand-authoring
        Requirement/Rejection proposals for that domain (the consumer-domain
        analogue of sync-self's own self-hosting lock).

hotam version | hotam --version
        Print the hotam binary version (see Version above).
```

`--domain` resolution (see `cmd/hotam/common.go`'s `resolveDomain`): an
explicit `--domain <path>` always wins; otherwise a `HOTAM_DOMAIN` env var
names a domain by name; otherwise a per-project preference recorded in
`.hotam-spec-project` (set via `hotam use <name>`, or set automatically by
`init-project` at scaffold time) is used; only when none of these is set
does `--domain` fall back to `domains/hotam-spec-self` (this repository's
own default). Tiers 2 and 3 emit one stderr notice so a bare command never
silently targets an unexpected domain.

## Tests

```bash
go test ./...
go vet ./...
go test -race ./...
```

## Repository structure

```
cmd/hotam/            CLI entry point and subcommands
internal/
  ontology/           graph node types (Requirement, Conflict, Assumption, ...)
  loader/             graph.json reader
  proposal/           validation and application of Proposed* structures
  invariants/         check_* graph invariants
  diagnose/           signal computation (what-now, inspect, confront)
  gate/               Tier-1 test selection by anchor
  generator/          docs/gen/*.md generation from the graph
  freshness/          OVERDUE / NEVER-REVIEWED reporting (due)
  query/              compact read interface (req)
  methodology/        methodology reference data
  registry/           tool registry
  paths/              project-root resolution
domains/
  hotam-spec-self/    the methodology modeling itself
  hotam-dev/          development of this very repository
docs/                 reference docs (incl. PROPOSAL-REFERENCE.md, QUICKSTART-CONSUMER.md)
```

## Agent workflow loop

1. `hotam what-now` — learn the prioritized next action.
2. Draft a JSON proposal — format in `docs/PROPOSAL-REFERENCE.md`.
3. `hotam apply-proposal <file.json> --domain <path> --today YYYY-MM-DD` — apply
   (or use `hotam land` to also regenerate docs and re-check invariants).
4. `hotam gen-spec --domain <path>` — regenerate documentation from the graph
   (already done by `land`; run separately if you used `apply-proposal`).
5. `hotam all-violations --domain <path>` — confirm the graph stays structurally
   sound.

Hand-editing `graph.json` is not allowed — every change goes through
`apply-proposal` / `land` (see `CONTRIBUTING.md`).

## License

Dual-licensed under MIT OR Apache-2.0 — see `LICENSE-MIT` and `LICENSE-APACHE`.
