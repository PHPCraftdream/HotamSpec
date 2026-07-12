package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sync"

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

// writeFilesParallel writes each (path, content) pair in paths[i]/contents[i]
// to disk concurrently — one goroutine per file, each touching only its own
// path, so there is no write collision to guard against. It mirrors
// invariants.AllViolations' indexed-slice-then-merge shape: results land in
// a pre-sized slot per index (never via append from a goroutine, which would
// race), so the caller can rebuild output in the original, deterministic
// order after wg.Wait() rather than in whatever order goroutines happen to
// finish.
//
// It returns the first error encountered, selected deterministically by
// index (lowest i wins) rather than by goroutine completion order, so a
// failure is reproducible across runs even though the writes themselves are
// concurrent.
func writeFilesParallel(paths []string, contents [][]byte) error {
	errs := make([]error, len(paths))
	var wg sync.WaitGroup
	for i := range paths {
		wg.Add(1)
		go func(idx int, p string, data []byte) {
			defer wg.Done()
			errs[idx] = writeFileMkdir(p, data)
		}(i, paths[i], contents[i])
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return err
		}
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
