package main

import (
	"fmt"
	"strings"

	"github.com/PHPCraftdream/HotamSpecGo/internal/diagnose"
)

func cmdWhatNow(args []string) error {
	fs := newFlagSet("what-now")
	domain := fs.String("domain", "", "domain directory (default: "+defaultDomainRel+")")
	limit := fs.Int("limit", 20, "maximum number of signals to print")
	fs.Parse(args)

	domainDir, err := resolveDomain(*domain)
	if err != nil {
		return err
	}
	out, err := whatNow(domainDir, *limit)
	if err != nil {
		return err
	}
	fmt.Println(out)
	return nil
}

func whatNow(domainDir string, limit int) (string, error) {
	g, err := loadDomainGraph(domainDir)
	if err != nil {
		return "", err
	}
	return formatSignals(diagnose.DiagnoseSignals(g), limit), nil
}

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
	out := make([]string, 0, end)
	for _, s := range signals[:end] {
		out = append(out, fmt.Sprintf("[P%d] %s on %s — %s", s.Priority, priorityLabel(s.Priority), s.Target, s.Message))
	}
	return joinLines(out)
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
