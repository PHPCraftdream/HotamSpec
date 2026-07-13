package main

import (
	"fmt"
	"os"
	"strings"
)

// Build-time metadata, injected via -ldflags:
//
//	go build -ldflags \
//	  "-X main.version=v0.1.0 -X main.commit=abc1234 -X main.buildDate=2026-07-12" ...
//
// Left as defaults for local/unreleased builds (go run, plain go build,
// go install without -ldflags).
var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

// versionString renders the single line printed by `hotam version` /
// `hotam --version`.
func versionString() string {
	return fmt.Sprintf("hotam %s (commit: %s, built: %s)", version, commit, buildDate)
}

// cmdVersion is the Run-shaped (func(args []string) error) wrapper around
// versionString, so `version` can be registered as an Implemented tool in
// internal/methodology/tools_data.go and wired via tool_wiring.go like every
// other real subcommand (TestToolWiring_EveryImplementedToolHasRun). main's
// switch below calls it directly for both the "version" and "--version"
// spellings instead of duplicating the Println.
func cmdVersion(args []string) error {
	fmt.Println(versionString())
	return nil
}

func main() {
	if len(os.Args) < 2 {
		printUsage(os.Stderr)
		os.Exit(2)
	}
	cmd := os.Args[1]
	args := reorderFlagsFirst(os.Args[2:])
	var err error
	switch cmd {
	case "version", "--version":
		err = cmdVersion(args)
	case "init":
		err = cmdInit(args)
	case "init-project":
		err = cmdInitProject(args)
	case "use":
		err = cmdUse(args)
	case "gen-spec":
		err = cmdGenSpec(args)
	case "what-now":
		err = cmdWhatNow(args)
	case "apply-proposal":
		err = cmdApplyProposal(args)
	case "land":
		err = cmdLand(args)
	case "gate":
		err = cmdGate(args)
	case "all-violations":
		err = cmdAllViolations(args)
	case "req":
		err = cmdReq(args)
	case "brief":
		err = cmdBrief(args)
	case "due":
		err = cmdDue(args)
	case "status":
		err = cmdStatus(args)
	case "inspect":
		err = cmdInspect(args)
	case "confront":
		err = cmdConfront(args)
	case "propose":
		err = cmdPropose(args)
	case "-h", "--help", "help":
		printUsage(os.Stdout)
		return
	default:
		fmt.Fprintf(os.Stderr, "hotam: unknown command %q\n", cmd)
		printUsage(os.Stderr)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "hotam %s: %v\n", cmd, err)
		os.Exit(1)
	}
}

func printUsage(w *os.File) {
	fmt.Fprint(w, `hotam — Hotam-Spec CLI

Usage:
  hotam <command> [flags] [args]

Commands:
  init <dir> [--name <domain-name>]
        Scaffold a new domain: minimal graph.json (seed Stakeholder + seed
        SETTLED Requirement, all-violations=0 immediately), manifest.json,
        docs/gen/, and a README.md pointing at the next commands to run.
        <dir> may be anywhere on disk — it does not need to live under this
        repository or contain a domains/ ancestor.
  init-project <dir> [--domain <name>] [--today YYYY-MM-DD]
        Bootstrap an external business project's full Hotam-Spec layout in one
        call: scaffold a base domain under <dir>/domains/<name> (default
        <name>=main), write the project-root marker (.hotam-spec-project), and
        render the root crystal (CLAUDE.md/AGENTS.md/GEMINI.md) + all docs/gen/*
        via gen-spec. Refuses to overwrite an existing project marker or
        CLAUDE.md. <dir> may be anywhere on disk.
  use <domain-name>
        Set the active-domain preference for the current project: records
        {"active_domain": "<name>"} in the project-root marker so a bare
        hotam <command> (no --domain) targets the chosen domain. Refuses if
        <root>/domains/<name>/graph.json does not exist.
  gen-spec [--domain <path>]
        Generate all docs/gen/*.md + graph.json for a domain graph.
  what-now [--domain <path>] [--limit N]
        Print top-N diagnosed signals (default 20).
  apply-proposal <proposal.json> --domain <path> --today YYYY-MM-DD [--batch <dir>]
        Apply a proposal to a domain graph. Low-level: does not regenerate
        docs — run gen-spec after, or use land instead. With --batch <dir>,
        every *.json in <dir> is applied atomically in filename order
        (all-or-nothing): if any proposal fails the graph is untouched.
  land <proposal.json> --domain <path> --today YYYY-MM-DD [--claude-md <path>] [--batch <dir>]
        Apply a proposal, then regenerate docs/gen for the domain, then
        re-check all invariants. Steps 1-2 are the primary pipeline; step 3
        (all-violations) is a safety-net re-verification, not the main gate
        — apply-proposal already refuses to write a graph that would
        introduce new invariant violations (internal/proposal/apply.go), so
        a non-zero exit here signals drift from gen-spec itself or a
        concurrent change, not a bad proposal. With --batch <dir>, every
        *.json in <dir> is applied atomically in filename order and docs are
        regenerated exactly once (not once per proposal).
  gate <target-anchor> [--domain <path>]
        Select Tier-1 tests for a target node.
  all-violations [--domain <path>]
        Print all invariant violations; exit 1 if any.
  req <show|list|search|context|related> [args] [--domain <path>] [--json]
        Compact agentic read interface over the domain graph (hotam req -h for details).
  brief <anchor-id> [--domain <path>] [--today YYYY-MM-DD] [--json]
        Single-call orientation brief for any anchor (Requirement, Conflict,
        or Assumption): full card + one-hop neighborhood + freshness (for
        Requirements), replacing req show + req context + req related + due.
  due [--domain <path>] [--today YYYY-MM-DD] [--json]
        Advisory report of OVERDUE and NEVER-REVIEWED SETTLED requirements.
  status [--domain <path>] [--today YYYY-MM-DD] [--json]
        Single-shot compact summary combining what-now's top action + debt,
        due's freshness counts, and all-violations' violation count, so an
        agent doesn't need to run all three separately. Never gates; exit
        code always 0.
  inspect [--domain <path>] [--json] [--limit N] [--min-score N]
        Advisory listing of semantic conflict candidates with evidence
        (shared-assumption clusters, entity-state suspects, lexical claim
        overlap, axis co-reference). --min-score (default 5) suppresses
        low-signal candidates; 0 shows all. Never gates; exit code always 0.
  confront <text> [--domain <path>] [--file <path>] [--json]
        CONFRONT step of the mediation loop: checks a candidate claim for
        lexical overlap with SETTLED requirements (duplicate guard) and
        REJECTED history (anti-relitigation) before anything is written.
        <text> is a quoted positional; --file <path> reads a long draft.
        Reuses the inspect overlap engine. Never gates; exit code always 0.
  propose <requirement|rejection|stakeholder> [flags]
        Draft a proposal JSON from flags (schema knowledge lives in the tool,
        not the agent's memory), run an automatic CONFRONT check before
        writing, and optionally --land (apply+regen+reverify) in the same call.
        Kinds: requirement (--id, --claim, --owner, --status, …), rejection
        (--requirement-id, --reason, --replaced-by), stakeholder (--id,
        --name, --domain, --why). Complex kinds (Conflict, EntityType, …)
        keep the hand-authored-JSON path (hotam land <file.json>).
  version, --version
        Print the hotam binary version.

  --domain resolution: an explicit --domain <path> always wins; otherwise
  HOTAM_DOMAIN env names a domain by name; otherwise the active_domain
  recorded in .hotam-spec-project (set via "hotam use <name>", or by
  init-project at scaffold time) is used; only then does it fall back to
  domains/hotam-spec-self (this repository's own default).
`)
}

// boolFlagNames is the flat, package-wide set of flag names that take NO value
// (registered via fs.Bool somewhere in cmd/hotam), so reorderFlagsFirst never
// swallows the following token as a flag value. Go's stdlib flag package cannot
// be consulted here: reorderFlagsFirst is a pre-processing step over raw
// os.Args that runs BEFORE subcommand dispatch, with no per-subcommand FlagSet
// in scope yet. "--version" is deliberately excluded: main()'s switch handles
// it at os.Args[1] (the command slot) before reorderFlagsFirst even runs on
// os.Args[2:], so it can never appear here as a flag-with-a-value.
//
// KEEP THIS LIST IN SYNC with every fs.Bool(...) call in cmd/hotam/*.go: adding
// a new boolean flag to a subcommand's FlagSet without adding its name here
// reintroduces the exact bug this map fixes (the new bool flag would eat the
// next positional token as its "value"). Today the only boolean flag is --json.
var boolFlagNames = map[string]bool{
	"json": true,
	"land": true,
}

// reorderFlagsFirst moves every token starting with "-" (and its value, if it
// takes one) ahead of the positional args, because Go's stdlib flag package
// stops parsing flags at the first non-flag token. A flag consumes the
// following token as its value only when it is NOT a known boolean
// (boolFlagNames) and is written bare (no "="); boolean flags never consume
// the next token, so `--json <positional>` keeps the positional in place.
func reorderFlagsFirst(args []string) []string {
	var flags, positional []string
	i := 0
	for i < len(args) {
		a := args[i]
		if strings.HasPrefix(a, "-") && a != "-" {
			flags = append(flags, a)
			if isBoolFlag(a) {
				i++
				continue
			}
			if !strings.Contains(a, "=") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				flags = append(flags, args[i+1])
				i += 2
				continue
			}
			i++
		} else {
			positional = append(positional, a)
			i++
		}
	}
	return append(flags, positional...)
}

// isBoolFlag reports whether a raw argv token (e.g. "--json", "-json", or
// "--json=true") names a known value-less boolean flag. It strips the leading
// dashes and any "=value" suffix before consulting boolFlagNames.
func isBoolFlag(token string) bool {
	name := strings.TrimLeft(token, "-")
	if eq := strings.IndexByte(name, '='); eq >= 0 {
		name = name[:eq]
	}
	return boolFlagNames[name]
}
