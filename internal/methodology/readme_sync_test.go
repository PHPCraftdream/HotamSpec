package methodology

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// readmePath is the repo-root-relative path to README.md, resolved against
// this test file's own package directory (internal/methodology, two levels
// below repo root) the same way internal/generator/tool_reqs_test.go's
// mainGoPath constant ("../../cmd/hotam/main.go") resolves repo-relative
// sources from a similarly-nested package.
const readmePath = "../../README.md"

// TestREADMEDocumentsEveryImplementedCommand guards against the README ↔ CLI
// drift class fixed by review item R5-d: a real `hotam` CLI subcommand
// (registered in methodology.Tools with Status == Implemented) that README's
// command list omits. It reads README.md at test time and asserts every
// Implemented tool's hyphenated command appears as "hotam <command>" in
// README's text — matching the exact style every existing command entry uses
// in the fenced command list. It cannot detect a mis-named or mis-described
// command (that needs a Markdown parser), but it pins THIS specific class of
// omission so a regression that adds a new Implemented command without
// documenting it in README fails loudly. Same smoke-guard spirit as
// internal/generator's TestToolRegistry_ImplementedCommandsWiredInCLI.
func TestREADMEDocumentsEveryImplementedCommand(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("read %s: %v", readmePath, err)
	}
	readme := string(body)
	for _, tool := range Tools.All() {
		if tool.Status != Implemented {
			continue
		}
		hyphenated := strings.ReplaceAll(tool.Command, "_", "-")
		needle := "hotam " + hyphenated
		if !strings.Contains(readme, needle) {
			t.Errorf("Implemented tool %q: command %q not documented in %s",
				tool.Command, needle, readmePath)
		}
	}
}

// implementsCommandsCountRE matches README's "implements N commands" sentence
// (`## CLI commands` section intro), capturing the digit count. The sentence
// is written as a literal digit (e.g. "implements 17 commands") specifically
// so this test can parse it with a plain digit regex instead of an
// English-number-word lookup table — the simpler, least-brittle form.
var implementsCommandsCountRE = regexp.MustCompile(`implements (\d+) commands`)

// TestREADMECommandCountMatchesRegistry guards against the drift class that
// let README's "fifteen commands" prose go stale while the registry grew to
// 17 Implemented tools (review-6 R6-g): it parses the digit count out of
// README's "implements N commands" sentence and asserts it equals
// len(methodology.Tools.All()) filtered to Status == Implemented. Unlike
// TestREADMEDocumentsEveryImplementedCommand (which only checks that each
// Implemented command's NAME appears somewhere in README), this test pins
// the stated COUNT itself, so a future Implemented-tool addition that
// forgets to update the count sentence fails loudly here.
func TestREADMECommandCountMatchesRegistry(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("read %s: %v", readmePath, err)
	}
	readme := string(body)

	m := implementsCommandsCountRE.FindStringSubmatch(readme)
	if m == nil {
		t.Fatalf("%s: no \"implements N commands\" sentence found (expected digit form, e.g. \"implements 17 commands\")", readmePath)
	}
	statedCount, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatalf("%s: could not parse command count %q: %v", readmePath, m[1], err)
	}

	implementedCount := 0
	for _, tool := range Tools.All() {
		if tool.Status == Implemented {
			implementedCount++
		}
	}

	if statedCount != implementedCount {
		t.Errorf("%s states %d but methodology.Tools registry has %d Implemented tools — update the README sentence to match the registry",
			m[0], statedCount, implementedCount)
	}
}
