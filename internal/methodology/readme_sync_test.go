package methodology

import (
	"os"
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
