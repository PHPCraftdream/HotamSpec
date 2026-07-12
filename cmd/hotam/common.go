package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/PHPCraftdream/HotamSpecGo/internal/loader"
	"github.com/PHPCraftdream/HotamSpecGo/internal/ontology"
	"github.com/PHPCraftdream/HotamSpecGo/internal/paths"
)

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	fs.SetOutput(os.Stderr)
	return fs
}

const defaultDomainRel = "domains/hotam-spec-self"

func resolveDomain(domainFlag string) (string, error) {
	if domainFlag != "" {
		abs, err := filepath.Abs(domainFlag)
		if err != nil {
			return "", fmt.Errorf("resolve --domain %q: %w", domainFlag, err)
		}
		return abs, nil
	}
	root, err := paths.ProjectRootOrRaise()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, defaultDomainRel), nil
}

func graphPathForDomain(domainDir string) string {
	return filepath.Join(domainDir, "graph.json")
}

func loadDomainGraph(domainDir string) (*ontology.Graph, error) {
	gp := graphPathForDomain(domainDir)
	g, err := loader.LoadGraph(gp)
	if err != nil {
		return nil, err
	}
	return g, nil
}

func writeFileMkdir(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir for %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func domainNameFromDir(domainDir string) string {
	return filepath.Base(domainDir)
}

func relPathForDisplay(path string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return path
	}
	rel, err := filepath.Rel(cwd, path)
	if err != nil {
		return path
	}
	return rel
}
