package generator

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/PHPCraftdream/HotamSpec/internal/localization"
	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
)

// internalPkgParenRE matches a parenthetical clause that points at one or more
// framework-internal Go package paths — e.g. " (internal/diagnose)" or
// " (internal/proposal + internal/generator + internal/invariants)" or
// " (internal/query.Brief)". These pointers are meaningful inside the
// framework's own source tree but are dead-end references for an external
// consumer who only has the installed `hotam` binary (no `internal/` tree
// exists in their project). The leading space is part of the match so the
// punctuation that followed the parenthetical — a colon, semicolon, comma, or
// sentence period — lands cleanly against the preceding word after removal:
// "proposals (internal/proposal):" -> "proposals:", "graph (internal/x);" ->
// "graph;". Applied ONLY at render time under the consumer profile; the
// registry's Purpose text (internal/methodology/tools_data.go) stays the single
// source of truth for both profiles.
var internalPkgParenRE = regexp.MustCompile(` \(internal/[^)]*\)`)

// stripInternalPkgRefs removes every "(internal/...)" parenthetical package
// pointer from a Purpose/Canon string, returning grammatically clean prose
// (no double spaces, no dangling punctuation). regexp.Regexp.ReplaceAllString
// is safe for concurrent use, so this helper may be called from parallel
// goroutines (BuildToolDocs renders tools concurrently).
func stripInternalPkgRefs(s string) string {
	return internalPkgParenRE.ReplaceAllString(s, "")
}

// BuildToolDocs renders one Markdown doc per tool. Each tool's content is a
// pure function of that tool's own fields (read only, no shared mutable
// state), so the renders run concurrently — same indexed-slice-then-merge
// shape as invariants.AllViolations — while the final map assembly stays
// single-threaded (concurrent map writes are not safe in Go even when keys
// are disjoint).
//
// consumer selects the output profile (loader.GenProfileConsumer when true).
// Under consumer, each Implemented tool's Purpose text has its
// "(internal/...)" package pointers stripped at render time (see
// stripInternalPkgRefs); Planned tools are never written under consumer (the
// toolIsImplemented filter in gen_spec.go skips them), so their content is
// irrelevant under that profile. Full-profile (consumer==false) output is
// byte-identical to before this parameter existed — the registry's source
// strings are rendered verbatim.
func BuildToolDocs(consumer bool) map[string]string {
	docs, _ := BuildToolDocsLocalized("", consumer)
	return docs
}

// BuildToolDocsLocalized renders each methodology tool with one explicit
// locale. It preflights all translations before starting renderer goroutines,
// so a missing key cannot escape as an untyped goroutine panic.
func BuildToolDocsLocalized(language string, consumer bool) (map[string]string, error) {
	tools := methodology.Tools.All()
	banner, err := localization.Lookup(language, Banner)
	if err != nil {
		return nil, err
	}
	statusHeading, err := localization.Lookup(language, "## Status")
	if err != nil {
		return nil, err
	}
	canonHeading, err := localization.Lookup(language, "## Canon")
	if err != nil {
		return nil, err
	}
	purposeHeading, err := localization.Lookup(language, "## Purpose")
	if err != nil {
		return nil, err
	}
	implementedBadge, err := localization.Lookup(language, "[IMPLEMENTED]")
	if err != nil {
		return nil, err
	}
	plannedBadge, err := localization.Lookup(language, "[PLANNED — not implemented]")
	if err != nil {
		return nil, err
	}
	implementedLine, err := localization.Lookup(language, "Implemented — this is a real `hotam` CLI subcommand; running it does something.")
	if err != nil {
		return nil, err
	}
	plannedLine, err := localization.Lookup(language, "Planned — methodology surface only; no Go command exists for it yet; invoking it as `hotam <name>` will fail with \"unknown command\".")
	if err != nil {
		return nil, err
	}

	purposes := make([]string, len(tools))
	for i, tool := range tools {
		purpose, err := localization.Lookup(language, tool.Purpose)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", tool.Command, err)
		}
		if consumer && tool.Status == methodology.Implemented {
			purpose = stripInternalPkgRefs(purpose)
		}
		purposes[i] = purpose
	}
	keys := make([]string, len(tools))
	contents := make([]string, len(tools))
	var wg sync.WaitGroup
	for i, tool := range tools {
		wg.Add(1)
		go func(index int, entry methodology.Tool, purpose string) {
			defer wg.Done()
			badge, status := "["+string(entry.Status)+"]", string(entry.Status)
			switch entry.Status {
			case methodology.Implemented:
				badge, status = implementedBadge, implementedLine
			case methodology.Planned:
				badge, status = plannedBadge, plannedLine
			}
			lines := []string{
				banner,
				"",
				"# " + entry.Command + " " + badge,
				"",
				statusHeading,
				"",
				status,
				"",
				canonHeading,
				"",
				entry.Canon,
				"",
				purposeHeading,
				"",
				purpose,
			}
			keys[index] = entry.Command
			contents[index] = strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
		}(i, tool, purposes[i])
	}
	wg.Wait()
	out := make(map[string]string, len(tools))
	for i, key := range keys {
		out[key] = contents[i]
	}
	return out, nil
}

func statusBadgeLocalized(language string, status methodology.Status) string {
	switch status {
	case methodology.Implemented:
		return localization.Text(language, "[IMPLEMENTED]")
	case methodology.Planned:
		return localization.Text(language, "[PLANNED — not implemented]")
	default:
		return "[" + string(status) + "]"
	}
}

func statusLineLocalized(language string, status methodology.Status) string {
	switch status {
	case methodology.Implemented:
		return localization.Text(language, "Implemented — this is a real `hotam` CLI subcommand; running it does something.")
	case methodology.Planned:
		return localization.Text(language, "Planned — methodology surface only; no Go command exists for it yet; invoking it as `hotam <name>` will fail with \"unknown command\".")
	default:
		return string(status)
	}
}

// statusBadge renders the short inline marker appended to a tool doc's H1,
// so a reader scanning docs/gen/tools/*.md (or the file listing) sees
// working-vs-aspirational at a glance without opening the file — the same
// distinction methodology.Status exists to make (see internal/methodology/
// tool.go's doc comment: "registry stores only rules and commands that
// actually work, not intentions").
func statusBadge(s methodology.Status) string {
	return statusBadgeLocalized("", s)
}

func statusLine(s methodology.Status) string {
	return statusLineLocalized("", s)
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
	doc, _ := BuildToolDocsIndexLocalized("", consumer)
	return doc
}

// BuildToolDocsIndexLocalized renders the registry navigation page for one
// explicit locale, returning a typed refusal if any fixed template is absent.
func BuildToolDocsIndexLocalized(language string, consumer bool) (doc string, err error) {
	err = localization.SafeRender(func() error {
		doc = buildToolDocsIndex(language, consumer)
		return nil
	})
	return doc, err
}

func buildToolDocsIndex(language string, consumer bool) string {
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
		localization.Text(language, Banner),
		"",
		localization.Text(language, "# Tool docs index"),
		"",
		localization.Text(language, "%d tools registered — **%d Implemented** (real `hotam` CLI subcommands) · **%d Planned** (methodology surface only; no Go command exists yet).", len(sorted), len(implemented), len(planned)),
		"",
		localization.Text(language, "This index splits the tool registry so a browser of `docs/gen/tools/` can tell at a glance which entries are real commands versus aspirational methodology surface. The root crystal's Tool reference block (`EMBEDDED-TOOLS`) collapses the Planned tools into a one-line summary; each per-tool `.md` file below carries full Status/Canon/Purpose detail."),
		"",
		localization.Text(language, "## Implemented (real commands)"),
		"",
		localization.Text(language, "These %d are real `hotam` CLI subcommands wired in `cmd/hotam/main.go` — running them does something.", len(implemented)),
		"",
	}
	for _, t := range implemented {
		displayName := strings.ReplaceAll(t.Command, "_", "-")
		desc := purposeExcerpt(localization.Text(language, t.Purpose))
		if consumer {
			desc = stripInternalPkgRefs(desc)
		}
		lines = append(lines, localization.Text(language, "- [`hotam %s`](%s.md) — %s", displayName, t.Command, desc))
	}

	lines = append(lines, "", localization.Text(language, "## Planned (methodology surface only — no command exists)"), "")
	if len(planned) == 0 {
		lines = append(lines, localization.Text(language, "_(none — all registered tools are implemented.)_"))
	} else {
		if consumer {
			lines = append(lines, localization.Text(language, "These %d are registered in the methodology registry as future-work surface (see `hotam -h` / `hotam status` for the real command set). Invoking any of them as `hotam <name>` fails with \"unknown command\".", len(planned)))
		} else {
			lines = append(lines, localization.Text(language, "These %d are registered in the methodology registry (`internal/methodology/tools_data.go`) as future-work surface. Invoking any of them as `hotam <name>` fails with \"unknown command\". Their per-tool `.md` files exist for design-continuity reference only.", len(planned)))
		}
		lines = append(lines, "")
		for _, t := range planned {
			displayName := strings.ReplaceAll(t.Command, "_", "-")
			desc := purposeExcerpt(localization.Text(language, t.Purpose))
			if consumer {
				lines = append(lines, localization.Text(language, "- `hotam %s` — %s", displayName, desc))
			} else {
				lines = append(lines, localization.Text(language, "- [`hotam %s`](%s.md) — %s", displayName, t.Command, desc))
			}
		}
	}
	return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
}
