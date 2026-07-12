package main

import (
	"fmt"
	"os"
	"strings"
)

// version is set at build time via:
//   go build -ldflags "-X main.version=v0.1.0" ...
// Left as "dev" for local/unreleased builds (go run, plain go build, go install
// without -ldflags).
var version = "dev"

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
		fmt.Println("hotam " + version)
		return
	case "init":
		err = cmdInit(args)
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
	case "due":
		err = cmdDue(args)
	case "inspect":
		err = cmdInspect(args)
	case "confront":
		err = cmdConfront(args)
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
  due [--domain <path>] [--today YYYY-MM-DD] [--json]
        Advisory report of OVERDUE and NEVER-REVIEWED SETTLED requirements.
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
  version, --version
        Print the hotam binary version.

  --domain defaults to domains/hotam-spec-self resolved via the project root.
`)
}

func reorderFlagsFirst(args []string) []string {
	var flags, positional []string
	i := 0
	for i < len(args) {
		a := args[i]
		if strings.HasPrefix(a, "-") && a != "-" {
			flags = append(flags, a)
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
