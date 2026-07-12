package main

import (
	"fmt"
	"os"
	"path/filepath"
	"unicode/utf8"

	"github.com/PHPCraftdream/HotamSpecGo/internal/generator"
)

func cmdGenSpec(args []string) error {
	fs := newFlagSet("gen-spec")
	domain := fs.String("domain", "", "domain directory (default: "+defaultDomainRel+")")
	claudeMD := fs.String("claude-md", "", "path to CLAUDE.md for rune count")
	fs.Parse(args)

	domainDir, err := resolveDomain(*domain)
	if err != nil {
		return err
	}
	written, err := genSpec(domainDir, *claudeMD)
	if err != nil {
		return err
	}
	for _, p := range written {
		fmt.Println(relPathForDisplay(p))
	}
	return nil
}

func genSpec(domainDir, claudeMDPath string) ([]string, error) {
	g, err := loadDomainGraph(domainDir)
	if err != nil {
		return nil, err
	}
	genDir := filepath.Join(domainDir, "docs", "gen")
	domainName := domainNameFromDir(domainDir)
	charCount := 0
	if claudeMDPath != "" {
		data, err := os.ReadFile(claudeMDPath)
		if err != nil {
			return nil, fmt.Errorf("read --claude-md %s: %w", claudeMDPath, err)
		}
		charCount = utf8.RuneCountInString(string(data))
	}

	var written []string

	type docEntry struct {
		filename string
		content  string
	}
	mdDocs := []docEntry{
		{"REQUIREMENTS.md", generator.BuildRequirements(g)},
		{"TENSIONS.md", generator.BuildTensions(g)},
		{"OPEN.md", generator.BuildOpen(g)},
		{"UNENFORCED.md", generator.BuildUnenforced(g)},
		{"GLOSSARY.md", generator.BuildGlossary(g)},
		{"HISTORY.md", generator.BuildHistory(g)},
		{"CONSTITUTION.md", generator.BuildConstitution(g)},
		{"FRAMEWORK-INVARIANTS.md", generator.BuildFrameworkInvariants(g, domainName)},
		{"REPO-MAP.md", generator.BuildRepoMap(g)},
		{"atoms-operator.md", generator.BuildAtomsOperator(g)},
		{"atoms-substrate.md", generator.BuildAtomsSubstrate(g)},
		{"atoms-discipline.md", generator.BuildAtomsDiscipline(g)},
		{"atoms-check.md", generator.BuildAtomsCheck(g)},
		{"live-state.md", generator.BuildLiveState(g, charCount)},
	}
	if generator.DecisionsMDHasContent(g) {
		mdDocs = append(mdDocs, docEntry{"DECISIONS.md", generator.BuildDecisions(g)})
	}
	if generator.EntitiesMDHasContent(g) {
		mdDocs = append(mdDocs, docEntry{"ENTITIES.md", generator.BuildEntities(g)})
	}

	for _, d := range mdDocs {
		p := filepath.Join(genDir, d.filename)
		if err := writeFileMkdir(p, []byte(d.content)); err != nil {
			return written, err
		}
		written = append(written, p)
	}

	graphJSON, err := generator.BuildGraphJSON(g)
	if err != nil {
		return written, fmt.Errorf("build graph.json: %w", err)
	}
	gp := filepath.Join(genDir, "graph.json")
	if err := writeFileMkdir(gp, []byte(graphJSON)); err != nil {
		return written, err
	}
	written = append(written, gp)

	for key, content := range generator.BuildThinkingDocs() {
		p := filepath.Join(genDir, "thinking", key+".md")
		if err := writeFileMkdir(p, []byte(content)); err != nil {
			return written, err
		}
		written = append(written, p)
	}
	for cmd, content := range generator.BuildToolDocs() {
		p := filepath.Join(genDir, "tools", cmd+".md")
		if err := writeFileMkdir(p, []byte(content)); err != nil {
			return written, err
		}
		written = append(written, p)
	}

	return written, nil
}
