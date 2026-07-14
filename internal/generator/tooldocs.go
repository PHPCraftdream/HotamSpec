package generator

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
)

// BuildToolDocs renders one Markdown doc per tool. Each tool's content is a
// pure function of that tool's own fields (read only, no shared mutable
// state), so the renders run concurrently — same indexed-slice-then-merge
// shape as invariants.AllViolations — while the final map assembly stays
// single-threaded (concurrent map writes are not safe in Go even when keys
// are disjoint).
func BuildToolDocs() map[string]string {
	tools := methodology.Tools.All()
	keys := make([]string, len(tools))
	contents := make([]string, len(tools))
	var wg sync.WaitGroup
	for i, t := range tools {
		wg.Add(1)
		go func(idx int, tool methodology.Tool) {
			defer wg.Done()
			lines := []string{
				Banner,
				"",
				"# " + tool.Command + " " + statusBadge(tool.Status),
				"",
				"## Status",
				"",
				statusLine(tool.Status),
				"",
				"## Canon",
				"",
				tool.Canon,
				"",
				"## Purpose",
				"",
				tool.Purpose,
			}
			keys[idx] = tool.Command
			contents[idx] = strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
		}(i, t)
	}
	wg.Wait()
	out := make(map[string]string, len(tools))
	for i, k := range keys {
		out[k] = contents[i]
	}
	return out
}

// statusBadge renders the short inline marker appended to a tool doc's H1,
// so a reader scanning docs/gen/tools/*.md (or the file listing) sees
// working-vs-aspirational at a glance without opening the file — the same
// distinction methodology.Status exists to make (see internal/methodology/
// tool.go's doc comment: "registry stores only rules and commands that
// actually work, not intentions").
func statusBadge(s methodology.Status) string {
	switch s {
	case methodology.Implemented:
		return "[IMPLEMENTED]"
	case methodology.Planned:
		return "[PLANNED — not implemented]"
	default:
		return "[" + string(s) + "]"
	}
}

// statusLine renders the one-line "## Status" section body: a longer,
// unambiguous prose form of statusBadge's short marker, for the reader who
// opens the doc after the badge caught their eye.
func statusLine(s methodology.Status) string {
	switch s {
	case methodology.Implemented:
		return "Implemented — this is a real `hotam` CLI subcommand; running it does something."
	case methodology.Planned:
		return "Planned — methodology surface only; no Go command exists for it yet; invoking it as `hotam <name>` will fail with \"unknown command\"."
	default:
		return string(s)
	}
}

// purposeExcerpt strips the "Usage: hotam <cmd> [flags]. " prefix from an
// Implemented tool's Purpose text, yielding just the descriptive sentence for
// the compact INDEX listing. Purpose fields that don't start with "Usage:"
// (Planned tools use "Not implemented. Historically: …") are returned
// unchanged — they're already short.
func purposeExcerpt(p string) string {
	const usagePrefix = "Usage:"
	if !strings.HasPrefix(p, usagePrefix) {
		return p
	}
	rest := p[len(usagePrefix):]
	if idx := strings.Index(rest, ". "); idx >= 0 {
		return strings.TrimSpace(rest[idx+2:])
	}
	return strings.TrimSpace(rest)
}

// BuildToolDocsIndex renders docs/gen/tools/INDEX.md: a single entry-point
// page that splits the tool registry into Implemented (real `hotam` CLI
// subcommands a consumer can run) and Planned (methodology surface only, no
// Go command exists), so a browser of docs/gen/tools/ is not misled by the
// raw file count (40 .md files, only 13 of which back runnable commands) into
// thinking every entry is a working command.
//
// It is purely additive: BuildToolDocs still emits one .md per tool unchanged.
// The index reuses the same methodology.Tools registry and the same
// statusBadge/statusLine vocabulary the per-tool docs already carry, so the
// distinction "implemented vs planned" is consistent everywhere it appears
// (per-tool badge → root-crystal EMBEDDED-TOOLS collapse → this index).
//
// consumer selects the output profile (loader.GenProfileConsumer when true,
// matching genSpec's own local): under the consumer profile genSpec writes
// per-tool `.md` pages ONLY for Implemented tools (the toolIsImplemented
// filter in gen_spec.go skips Planned tools entirely), so a markdown link to
// a Planned tool's `.md` would point at a file that was never written. Under
// consumer the Planned section therefore renders tool names as plain
// backtick code spans (no `[...](....md)` link wrapper) and drops the
// framework-internal source-file reference from its intro sentence. Under
// the full profile the Planned section renders byte-identical to before.
func BuildToolDocsIndex(consumer bool) string {
	tools := methodology.Tools.All()
	sorted := make([]methodology.Tool, len(tools))
	copy(sorted, tools)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Command < sorted[j].Command })

	var implemented, planned []methodology.Tool
	for _, t := range sorted {
		if t.Status == methodology.Implemented {
			implemented = append(implemented, t)
		} else {
			planned = append(planned, t)
		}
	}

	lines := []string{
		Banner,
		"",
		"# Tool docs index",
		"",
		fmt.Sprintf("%d tools registered — **%d Implemented** (real `hotam` CLI subcommands) · **%d Planned** (methodology surface only; no Go command exists yet).",
			len(sorted), len(implemented), len(planned)),
		"",
		"This index splits the tool registry so a browser of `docs/gen/tools/` can tell at a glance which entries are real commands versus aspirational methodology surface. The root crystal's Tool reference block (`EMBEDDED-TOOLS`) collapses the Planned tools into a one-line summary; each per-tool `.md` file below carries full Status/Canon/Purpose detail.",
		"",
		"## Implemented (real commands)",
		"",
		fmt.Sprintf("These %d are real `hotam` CLI subcommands wired in `cmd/hotam/main.go` — running them does something.", len(implemented)),
		"",
	}
	for _, t := range implemented {
		displayName := strings.ReplaceAll(t.Command, "_", "-")
		lines = append(lines, fmt.Sprintf("- [`hotam %s`](%s.md) — %s", displayName, t.Command, purposeExcerpt(t.Purpose)))
	}

	lines = append(lines, "", "## Planned (methodology surface only — no command exists)", "")
	if len(planned) == 0 {
		lines = append(lines, "_(none — all registered tools are implemented.)_")
	} else {
		if consumer {
			// Consumer: genSpec writes NO per-tool .md pages for Planned
			// tools (the toolIsImplemented filter skips them), so the
			// full-profile intro's reference to `internal/methodology/
			// tools_data.go` (a framework SOURCE FILE that does not exist in
			// an external consumer's project) and its claim that "Their
			// per-tool `.md` files exist" are both false under this profile.
			// Rephrase to point at the CLI's own discovery surface instead.
			lines = append(lines, fmt.Sprintf(
				"These %d are registered in the methodology registry as future-work surface (see `hotam -h` / `hotam status` for the real command set). Invoking any of them as `hotam <name>` fails with \"unknown command\".",
				len(planned),
			))
		} else {
			lines = append(lines, fmt.Sprintf(
				"These %d are registered in the methodology registry (`internal/methodology/tools_data.go`) as future-work surface. Invoking any of them as `hotam <name>` fails with \"unknown command\". Their per-tool `.md` files exist for design-continuity reference only.",
				len(planned),
			))
		}
		lines = append(lines, "")
		for _, t := range planned {
			displayName := strings.ReplaceAll(t.Command, "_", "-")
			if consumer {
				// No per-tool .md page was written for this Planned tool, so
				// a markdown link would be dead — render the command name as
				// a plain backtick code span (no link wrapper).
				lines = append(lines, fmt.Sprintf("- `hotam %s` — %s", displayName, purposeExcerpt(t.Purpose)))
			} else {
				lines = append(lines, fmt.Sprintf("- [`hotam %s`](%s.md) — %s", displayName, t.Command, purposeExcerpt(t.Purpose)))
			}
		}
	}

	return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
}
