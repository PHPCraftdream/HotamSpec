package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/PHPCraftdream/HotamSpec/internal/diagnose"
)

func cmdWhatNow(args []string) error {
	fs := newFlagSet("what-now")
	domain := fs.String("domain", "", "domain directory (default: "+defaultDomainRel+")")
	limit := fs.Int("limit", 20, "maximum number of signals to print")
	todayFlag := fs.String("today", "", "date in YYYY-MM-DD format (default: system date)")
	asJSON := fs.Bool("json", false, "emit machine-readable JSON")
	fs.Parse(args)

	domainDir, err := resolveDomain(*domain)
	if err != nil {
		return err
	}
	today := *todayFlag
	if today == "" {
		today = time.Now().Format("2006-01-02")
	}
	g, err := loadDomainGraph(domainDir)
	if err != nil {
		return err
	}
	signals := diagnose.DiagnoseSignals(g, today)
	if *asJSON {
		return printJSON(limitSignals(signals, *limit))
	}
	fmt.Println(formatSignals(signals, *limit))
	return nil
}

func whatNow(domainDir string, limit int, today string) (string, error) {
	g, err := loadDomainGraph(domainDir)
	if err != nil {
		return "", err
	}
	return formatSignals(diagnose.DiagnoseSignals(g, today), limit), nil
}

// limitSignals applies the same truncation logic as formatSignals but returns
// the raw signal slice for JSON marshaling. It ensures a non-nil slice so
// `--json` always emits `[]` on a clean graph instead of `null`.
func limitSignals(signals []diagnose.Signal, limit int) []diagnose.Signal {
	if limit < 0 {
		limit = len(signals)
	}
	end := limit
	if end > len(signals) {
		end = len(signals)
	}
	out := make([]diagnose.Signal, end)
	copy(out, signals[:end])
	return out
}

// maxCollapsedIDs is the number of distinct target ids shown verbatim in a
// collapsed what-now line before the remainder are summarised as "(and N more)".
const maxCollapsedIDs = 8

func formatSignals(signals []diagnose.Signal, limit int) string {
	if len(signals) == 0 {
		return "none — graph clean"
	}
	if limit < 0 {
		limit = len(signals)
	}
	end := limit
	if end > len(signals) {
		end = len(signals)
	}
	limited := signals[:end]

	groups := groupSignals(limited)
	out := make([]string, 0, len(groups))
	for _, grp := range groups {
		if isCollapsible(grp) {
			out = append(out, formatCollapsedGroup(grp))
		} else {
			for _, s := range grp.signals {
				out = append(out, formatSingleSignal(s))
			}
		}
	}
	return joinLines(out)
}

func formatSingleSignal(s diagnose.Signal) string {
	return fmt.Sprintf("[P%d] %s on %s — %s", s.Priority, priorityLabel(s.Priority), s.Target, s.Message)
}

// formatCollapsedGroup renders a group of same-(check, priority) signals
// affecting distinct nodes as ONE line:
//
//	[P7] ADVISORY ×16: <representative message> — ids: R-a, R-b, ... (and N more)
//
// The representative message is the first member's message: within a single
// (check, priority) group every message is the same template (it only differs
// by the embedded target id), so the first one explains the whole class while
// the id list gives the full scope.
func formatCollapsedGroup(grp signalGroup) string {
	first := grp.signals[0]
	return fmt.Sprintf("[P%d] %s ×%d: %s — ids: %s",
		first.Priority, priorityLabel(first.Priority), len(grp.signals),
		first.Message, collapsedIDList(grp.signals),
	)
}

func collapsedIDList(signals []diagnose.Signal) string {
	names := make([]string, 0, len(signals))
	for _, s := range signals {
		names = append(names, s.Target)
	}
	if len(names) <= maxCollapsedIDs {
		return strings.Join(names, ", ")
	}
	shown := names[:maxCollapsedIDs]
	return fmt.Sprintf("%s (and %d more)", strings.Join(shown, ", "), len(names)-maxCollapsedIDs)
}

type groupKey struct {
	check    string
	priority int
}

type signalGroup struct {
	key     groupKey
	signals []diagnose.Signal
}

// groupSignals collects signals into (check, priority) groups in first-appearance
// order. Because DiagnoseSignals returns a slice sorted by priority (then target,
// then message), first-appearance order keeps priority bands contiguous and
// ascending, so the rendered output stays priority-sorted.
func groupSignals(signals []diagnose.Signal) []signalGroup {
	var groups []signalGroup
	index := map[groupKey]int{}
	for _, s := range signals {
		k := groupKey{check: s.Check, priority: s.Priority}
		if idx, ok := index[k]; ok {
			groups[idx].signals = append(groups[idx].signals, s)
		} else {
			index[k] = len(groups)
			groups = append(groups, signalGroup{key: k, signals: []diagnose.Signal{s}})
		}
	}
	return groups
}

// isCollapsible reports whether a group should be rendered as one collapsed
// line. It requires at least two signals AND all targets pairwise distinct.
// The distinct-target guard prevents collapsing signals that describe different
// aspects of the SAME node (e.g. the held-variant producer emits one signal per
// variant, all targeting the same conflict id) — those keep their per-line
// detail. The collapse is meant for "same issue, different nodes" only.
func isCollapsible(grp signalGroup) bool {
	if len(grp.signals) < 2 {
		return false
	}
	seen := map[string]struct{}{}
	for _, s := range grp.signals {
		if _, ok := seen[s.Target]; ok {
			return false
		}
		seen[s.Target] = struct{}{}
	}
	return true
}

func priorityLabel(p int) string {
	labels := map[int]string{
		0: "REFLECTION",
		1: "STRUCTURE",
		2: "DRIFT_FALLOUT",
		3: "CONFLICT_STALLED",
		4: "OPEN_ITEM",
		5: "LATENT_CONNECTOR",
		6: "PENDING_PROPOSAL",
		7: "ADVISORY",
	}
	if l, ok := labels[p]; ok {
		return l
	}
	return "UNKNOWN"
}

func joinLines(lines []string) string {
	return strings.Join(lines, "\n")
}
