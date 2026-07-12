package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		printUsage(os.Stderr)
		os.Exit(2)
	}
	cmd := os.Args[1]
	args := reorderFlagsFirst(os.Args[2:])
	var err error
	switch cmd {
	case "gen-spec":
		err = cmdGenSpec(args)
	case "what-now":
		err = cmdWhatNow(args)
	case "apply-proposal":
		err = cmdApplyProposal(args)
	case "gate":
		err = cmdGate(args)
	case "all-violations":
		err = cmdAllViolations(args)
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
  gen-spec [--domain <path>]
        Generate all docs/gen/*.md + graph.json for a domain graph.
  what-now [--domain <path>] [--limit N]
        Print top-N diagnosed signals (default 20).
  apply-proposal <proposal.json> --domain <path> --today YYYY-MM-DD
        Apply a proposal to a domain graph.
  gate <target-anchor> [--domain <path>]
        Select Tier-1 tests for a target node.
  all-violations [--domain <path>]
        Print all invariant violations; exit 1 if any.

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
