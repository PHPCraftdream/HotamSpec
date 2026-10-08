package gate

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// Coverage instrumentation is the union of the authored implementation targets
// applicable to each recorded test package, from the same fresh execution.
func snapshotCoverageFiles(g *ontology.Graph) map[string][]string {
	byPackage := make(map[string]map[string]bool)
	for _, requirement := range g.Requirements {
		if requirement.Status == ontology.StatusREJECTED {
			continue
		}
		for _, reference := range specTestReferences(requirement) {
			file, _, ok := ParseFileColonSymbol(strings.TrimSpace(reference))
			if !ok {
				continue
			}
			dir := filepath.ToSlash(filepath.Dir(filepath.FromSlash(file)))
			if byPackage[dir] == nil {
				byPackage[dir] = make(map[string]bool)
			}
			for _, implemented := range requirement.ImplementedBy {
				file, _, ok := ParseFileColonSymbol(strings.TrimSpace(implemented))
				if ok {
					byPackage[dir][file] = true
				}
			}
		}
	}
	files := make(map[string][]string, len(byPackage))
	for dir, targets := range byPackage {
		for file := range targets {
			files[dir] = append(files[dir], file)
		}
		sort.Strings(files[dir])
	}
	return files
}

// ForTest projects an individual terminal test/subtest verdict. Package-level
// failure must not erase passing siblings or invent a result for an absent test.
func (recorded RecordingResult) ForTest(test string) TestRunResult {
	result := recorded.TestRunResult
	if result.Err != nil || result.CompileFailed || result.Skipped {
		return result
	}
	result.Passed = false
	executed := false
	passed := true
	for _, verdict := range recorded.TestVerdicts {
		if verdict.Test != test && !strings.HasPrefix(verdict.Test, test+"/") {
			continue
		}
		executed = true
		if verdict.Verdict != "pass" {
			passed = false
		}
	}
	if !executed {
		result.Err = fmt.Errorf("test %q has no terminal verdict in the executed package", test)
		return result
	}
	result.Passed = passed
	return result
}
