package generator

import (
	"sort"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// BuildRepoMap builds REPO-MAP.md (repository file index). domainName and
// genDocs identify which domain this doc describes and which docs/gen/ files
// were actually written for it in this run — scanning domains/<name>/*.json
// for "Domain content" and the actual generated files for "Generated docs"
// rather than hardcoding either section (R-doc-names-reader's sibling bug:
// REPO-MAP.md must name the real domain, not always hotam-spec-self).
//
// frameworkDocs lists the files written into the PROJECT-root framework/
// directory this run (task #357: GLOSSARY.md + the tools/*.md registry —
// byte-identical for every domain, promoted from the former per-domain
// framework/ to a single shared copy sibling to domains/). REPO-MAP.md renders
// them under a distinct "Framework reference" section so its listing stays
// honest about what is actually on disk (a framework/ dir that exists but is
// unmentioned would be a stale, self-contradictory map).
//
// decisionsWritten/entitiesWritten additionally control the two conditional
// "_(not written: ...)_ " placeholder lines emitted when DECISIONS.md /
// ENTITIES.md were withheld because their source registry is empty.
// tensionsWritten/pipelineWritten/modelsWritten do the same for TENSIONS.md /
// PIPELINE.md / MODELS.md (task #361: a young domain with no conflicts/axes,
// no processes, or no authored spec/ model files gets an honest "not written"
// line instead of a pure-template file).
//
// consumer gates the Framework-body section (repoMapFrameworkBodyContent): it
// describes the FRAMEWORK's own internal/ Go package layout, which does not
// exist in an external consumer's project, so the entire section is omitted
// under consumer. The Tools section (renderRepoMapToolsSection — registry-
// derived CLI commands, no internal/ paths) stays in both profiles. Full
// profile renders byte-identical to before.
// RepoMapMDHasContent reports whether REPO-MAP.md carries real domain
// content — i.e. whether the graph is non-empty (task #364: withheld
// entirely from the docs/gen/ write set when the domain has zero axes/
// stakeholders/requirements/conflicts/assumptions/operators/processes/goals/
// entity_types/entities, mirroring the conditional-write pattern
// DECISIONS.md/ENTITIES.md/TENSIONS.md/PIPELINE.md/MODELS.md already use).
// This is self-consistent: when the graph is empty, every OTHER docs/gen/
// file this predicate's siblings gate is also withheld, so REPO-MAP.md would
// otherwise be the lone file listing an otherwise-empty directory — dropping
// it too means a genuinely empty domain now produces ZERO files under
// docs/gen/, not one calm-but-pointless index of nothing.
func RepoMapMDHasContent(g *ontology.Graph) bool {
	return !g.IsEmpty()
}

func BuildRepoMap(g *ontology.Graph, domainName string, genDocs []GenDocEntry, frameworkDocs []GenDocEntry, decisionsWritten, entitiesWritten, tensionsWritten, pipelineWritten, modelsWritten bool, consumer bool) string {
	lines := []string{localizedBanner(g), ReaderHeaderLine("REPO_MAP", g), ""}
	lines = append(lines, serviceText(g, "# REPO-MAP.md — Repository file index (Hotam-Spec)"))
	lines = append(lines, "")
	if !consumer {
		lines = append(lines, repoMapFrameworkBodyContentForLanguage(g.RenderLanguage))
		lines = append(lines, "")
	}
	lines = append(lines, renderRepoMapToolsSectionLocalized(g.RenderLanguage))
	lines = append(lines, "")

	lines = append(lines, serviceText(g, "**Domain content** (`domains/%s/`)", domainName))
	lines = append(lines, "")
	lines = append(lines, "- `domains/"+domainName+"/graph.json` — "+domainGraphPyRoleLocalized(g.RenderLanguage, domainName))
	lines = append(lines, "- `domains/"+domainName+"/manifest.json` — "+serviceText(g, "manifest of domain '%s'.", domainName))
	lines = append(lines, "")

	lines = append(lines, serviceText(g, "**Generated docs** (`domains/%s/docs/gen/`)", domainName))
	lines = append(lines, "")
	sortedDocs := make([]GenDocEntry, len(genDocs))
	copy(sortedDocs, genDocs)
	sort.Slice(sortedDocs, func(i, j int) bool { return sortedDocs[i].Filename < sortedDocs[j].Filename })
	for _, d := range sortedDocs {
		lines = append(lines, "- `domains/"+domainName+"/docs/gen/"+d.Filename+"` — "+mdTitle(d.Content))
	}
	if !decisionsWritten {
		lines = append(lines, serviceText(g, "- `domains/%s/docs/gen/DECISIONS.md` — _(not written: M-registry empty)_", domainName))
	}
	if !entitiesWritten {
		lines = append(lines, serviceText(g, "- `domains/%s/docs/gen/ENTITIES.md` — _(not written: no entity_types declared)_", domainName))
	}
	if !tensionsWritten {
		lines = append(lines, serviceText(g, "- `domains/%s/docs/gen/TENSIONS.md` — _(not written: no conflict or axis nodes)_", domainName))
	}
	if !pipelineWritten {
		lines = append(lines, serviceText(g, "- `domains/%s/docs/gen/PIPELINE.md` — _(not written: no process nodes)_", domainName))
	}
	if !modelsWritten {
		lines = append(lines, serviceText(g, "- `domains/%s/docs/gen/MODELS.md` — _(not written: no authored spec/ model files)_", domainName))
	}

	// Framework reference section (task #357): project-shared self-
	// documentation (GLOSSARY.md + the tools/*.md registry) lives at the
	// PROJECT-root framework/ directory — a single copy sibling to domains/,
	// byte-identical regardless of business content. Listed here (sorted, like
	// the Generated docs section above) with repo-root-relative paths so
	// REPO-MAP.md stays honest about every generated directory that exists on
	// disk. tools/*.md subdirectory contents are deliberately NOT enumerated
	// (mirroring docs/gen/'s own treatment of thinking/); the entry below is
	// the one top-level file (GLOSSARY.md) plus a pointer to the tools/ subdir.
	if len(frameworkDocs) > 0 {
		lines = append(lines, "")
		lines = append(lines, serviceText(g, "**Framework reference (project-shared)** (`framework/`)"))
		lines = append(lines, "")
		sortedFw := make([]GenDocEntry, len(frameworkDocs))
		copy(sortedFw, frameworkDocs)
		sort.Slice(sortedFw, func(i, j int) bool { return sortedFw[i].Filename < sortedFw[j].Filename })
		for _, d := range sortedFw {
			lines = append(lines, "- `framework/"+d.Filename+"` — "+mdTitle(d.Content))
		}
	}

	lines = append(lines, "")
	return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
}
