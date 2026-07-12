package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/query"
)

// cmdReq is the compact agentic read interface over a domain graph:
// `hotam req show/list/search/context/related`. It exists so an agent
// never has to read the full graph.json (hundreds of KB) or a generated
// Markdown doc just to answer "what is R-x" or "what touches R-x".
//
// args has already been through main's reorderFlagsFirst, which hoists
// EVERY flag (across the whole command line) ahead of every positional —
// so the subcommand name ("show"/"list"/...) is not necessarily args[0],
// it is the first non-flag token. splitSubcommand below extracts it and
// hands back the remaining tokens in their original relative order so the
// per-subcommand flag.FlagSet can still parse them correctly.
func cmdReq(args []string) error {
	// -h/--help/help is checked directly against the raw args (before
	// splitSubcommand) because splitSubcommand treats -h/--help as a
	// boolean flag with no value — by design, so it never eats a real
	// subcommand token like "show" — which means it would never surface
	// as the detected "subcommand" itself. A plain "hotam req -h" or
	// "hotam req help" has nothing else to disambiguate, so it is safe
	// to match it here first.
	for _, a := range args {
		if a == "-h" || a == "--help" || a == "help" {
			printReqUsage()
			return nil
		}
	}
	sub, rest, ok := splitSubcommand(args)
	if !ok {
		return fmt.Errorf("usage: hotam req <show|list|search|context|related> [args] [--domain <path>] [--json]")
	}
	switch sub {
	case "show":
		return cmdReqShow(rest)
	case "list":
		return cmdReqList(rest)
	case "search":
		return cmdReqSearch(rest)
	case "context":
		return cmdReqContext(rest)
	case "related":
		return cmdReqRelated(rest)
	default:
		return fmt.Errorf("hotam req: unknown subcommand %q (want show|list|search|context|related)", sub)
	}
}

// reqBooleanFlags lists every `hotam req *` flag that takes no value
// (unlike reorderFlagsFirst, which cannot know this and greedily treats
// any following non-flag token as a value). splitSubcommand needs the
// real arity to avoid swallowing the subcommand name itself when it
// directly follows a boolean flag, e.g. "--json show R-x" after
// reordering must still resolve to subcommand "show", not "R-x".
var reqBooleanFlags = map[string]struct{}{
	"-json":  {},
	"--json": {},
	"-h":     {},
	"--help": {},
}

// splitSubcommand pulls the first non-flag token out of args (the
// req-level subcommand name) and returns it plus the remaining tokens in
// original order, so a value like "--domain X" is never mistaken for the
// subcommand and the subcommand itself is never mistaken for a flag's
// value. ok is false when no non-flag token exists.
func splitSubcommand(args []string) (sub string, rest []string, ok bool) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") && a != "-" {
			_, isBool := reqBooleanFlags[a]
			if !isBool && !strings.Contains(a, "=") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
			}
			continue
		}
		sub = a
		rest = append(rest, args[:i]...)
		rest = append(rest, args[i+1:]...)
		return sub, rest, true
	}
	return "", nil, false
}

func printReqUsage() {
	fmt.Print(`hotam req — compact agentic read interface over a domain graph

Usage:
  hotam req show <anchor-id> [--domain <path>] [--json]
        Full card for a Requirement/Conflict/Assumption anchor.
  hotam req list [--status S] [--owner O] [--enforcement E] [--domain <path>] [--json]
        Compact id+summary+status roster of Requirements.
  hotam req search "<text>" [--domain <path>] [--json]
        Case-insensitive search, ranked id > claim > why.
  hotam req context <requirement-id> [--domain <path>] [--json]
        Requirement + one-hop neighborhood: relations, assumptions, conflicts, shared-assumption peers.
  hotam req related <anchor-id> [--domain <path>] [--json]
        Just the neighbor id+relation-kind list for any anchor.
`)
}

func cmdReqShow(args []string) error {
	fs := newFlagSet("req show")
	domain := fs.String("domain", "", "domain directory (default: "+defaultDomainRel+")")
	asJSON := fs.Bool("json", false, "emit machine-readable JSON")
	fs.Parse(args)

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: hotam req show <anchor-id> [--domain <path>] [--json]")
	}
	id := fs.Arg(0)

	g, err := loadReqDomain(*domain)
	if err != nil {
		return err
	}
	card, err := query.Show(g, id)
	if err != nil {
		return err
	}
	if *asJSON {
		return printJSON(card)
	}
	text, err := query.FormatShow(card)
	if err != nil {
		return err
	}
	fmt.Println(text)
	return nil
}

func cmdReqList(args []string) error {
	fs := newFlagSet("req list")
	domain := fs.String("domain", "", "domain directory (default: "+defaultDomainRel+")")
	status := fs.String("status", "", "filter by exact Status")
	owner := fs.String("owner", "", "filter by exact Owner")
	enforcement := fs.String("enforcement", "", "filter by exact Enforcement level")
	asJSON := fs.Bool("json", false, "emit machine-readable JSON")
	fs.Parse(args)

	g, err := loadReqDomain(*domain)
	if err != nil {
		return err
	}
	items := query.List(g, query.ListFilter{
		Status:      *status,
		Owner:       *owner,
		Enforcement: *enforcement,
	})
	if *asJSON {
		return printJSON(items)
	}
	fmt.Println(query.FormatList(items))
	return nil
}

func cmdReqSearch(args []string) error {
	fs := newFlagSet("req search")
	domain := fs.String("domain", "", "domain directory (default: "+defaultDomainRel+")")
	asJSON := fs.Bool("json", false, "emit machine-readable JSON")
	fs.Parse(args)

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: hotam req search \"<text>\" [--domain <path>] [--json]")
	}
	text := fs.Arg(0)

	g, err := loadReqDomain(*domain)
	if err != nil {
		return err
	}
	results := query.Search(g, text)
	if *asJSON {
		return printJSON(results)
	}
	fmt.Println(query.FormatSearch(results))
	return nil
}

func cmdReqContext(args []string) error {
	fs := newFlagSet("req context")
	domain := fs.String("domain", "", "domain directory (default: "+defaultDomainRel+")")
	asJSON := fs.Bool("json", false, "emit machine-readable JSON")
	fs.Parse(args)

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: hotam req context <requirement-id> [--domain <path>] [--json]")
	}
	id := fs.Arg(0)

	g, err := loadReqDomain(*domain)
	if err != nil {
		return err
	}
	cc, err := query.Context(g, id)
	if err != nil {
		return err
	}
	if *asJSON {
		return printJSON(cc)
	}
	fmt.Println(query.FormatContext(cc))
	return nil
}

func cmdReqRelated(args []string) error {
	fs := newFlagSet("req related")
	domain := fs.String("domain", "", "domain directory (default: "+defaultDomainRel+")")
	asJSON := fs.Bool("json", false, "emit machine-readable JSON")
	fs.Parse(args)

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: hotam req related <anchor-id> [--domain <path>] [--json]")
	}
	id := fs.Arg(0)

	g, err := loadReqDomain(*domain)
	if err != nil {
		return err
	}
	refs, err := query.Related(g, id)
	if err != nil {
		return err
	}
	if *asJSON {
		return printJSON(refs)
	}
	fmt.Println(query.FormatRelated(refs))
	return nil
}

func loadReqDomain(domainFlag string) (*ontology.Graph, error) {
	domainDir, err := resolveDomain(domainFlag)
	if err != nil {
		return nil, err
	}
	return loadDomainGraph(domainDir)
}

func printJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	fmt.Println(string(data))
	return nil
}
