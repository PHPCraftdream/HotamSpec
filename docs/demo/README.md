# Demo docs — generate, don't read a stale snapshot

This directory used to hold a checked-in snapshot of generated Markdown
(`REQUIREMENTS.md`, `TENSIONS.md`, `CONSTITUTION.md`, ...) from the
Python-era prototype. That snapshot went stale the moment the graph or the
generator changed, so it was removed rather than kept as a silently-drifting
example.

To see a live demo, generate it yourself against a real domain graph:

```bash
hotam gen-spec --domain <path-to-a-domain>
```

for example, against this repo's own self-hosted domain:

```bash
hotam gen-spec --domain domains/hotam-spec-self
```

which writes the current, byte-accurate `REQUIREMENTS.md` / `TENSIONS.md` /
`OPEN.md` / `UNENFORCED.md` / `GLOSSARY.md` / `HISTORY.md` / `CONSTITUTION.md`
/ `FRAMEWORK-INVARIANTS.md` / `REPO-MAP.md` (plus `DECISIONS.md` /
`ENTITIES.md` when the domain has content for them) under
`domains/<name>/docs/gen/`.

See [QUICKSTART-CONSUMER.md](../QUICKSTART-CONSUMER.md) for the full
walkthrough of building the CLI and bootstrapping your own domain from
scratch.
