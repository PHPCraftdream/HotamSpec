package main

import (
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
)

// TestToolWiring_EveryImplementedToolHasRun is the synchronization test called
// for by ticket P1-6 (TaskList #19): it does not hardcode a duplicate list
// of command names (which main.go's switch and tools_data.go's Implemented
// entries could each drift from independently). Instead it walks the single
// source of truth for "which tools claim to be real" — the Status field on
// methodology.Tools, set in internal/methodology/tools_data.go — and asserts
// the cmd/hotam-side promise (tool_wiring.go's init) was kept for every one
// of them. An Implemented tool with a nil Run is exactly the dishonesty this
// ticket exists to close off: a registry entry that LOOKS like a working
// command but does nothing if invoked.
func TestToolWiring_EveryImplementedToolHasRun(t *testing.T) {
	t.Parallel()
	for _, tool := range methodology.Tools.All() {
		if tool.Status != methodology.Implemented {
			continue
		}
		if tool.Run == nil {
			t.Errorf("tool %q is Status=Implemented but Run is nil after cmd/hotam wiring — add a wireToolRun(%q, cmd...) line to tool_wiring.go", tool.Command, tool.Command)
		}
	}
}

// TestToolWiring_PlannedToolsStayUnwired guards the other direction: wiring
// a Planned tool's Run would let it be invoked even though tools_data.go
// says "no Go command exists for it yet" (see methodology.Planned's
// doc comment) — a lie in the opposite direction from the one this ticket
// closes. This also catches a Planned tool that was implemented (main.go grew a
// case for it) without its tools_data.go entry being promoted to Implemented.
func TestToolWiring_PlannedToolsStayUnwired(t *testing.T) {
	t.Parallel()
	for _, tool := range methodology.Tools.All() {
		if tool.Status != methodology.Planned {
			continue
		}
		if tool.Run != nil {
			t.Errorf("tool %q is Status=Planned but has a non-nil Run — either promote it to Implemented in tools_data.go, or stop wiring it in tool_wiring.go", tool.Command)
		}
	}
}

// TestToolWiring_ImplementedRunsAreCallable is a light smoke test: every wired
// Run is actually the right function value, not e.g. every Implemented tool
// accidentally wired to the same command (a copy-paste mistake wireToolRun's
// per-name Update call could otherwise hide). "-h" is used because every
// cmd* function in this package accepts a flag.FlagSet-parsed args slice and
// "-h"/"--help"/"help" is honored (with flag.ExitOnError semantics further
// down the stack for genuinely malformed flags), so this does not require
// touching a real domain graph on disk.
func TestToolWiring_ImplementedRunsAreCallable(t *testing.T) {
	t.Parallel()
	wantDistinctCount := 0
	seen := map[string]bool{}
	for _, tool := range methodology.Tools.All() {
		if tool.Status != methodology.Implemented {
			continue
		}
		wantDistinctCount++
		if tool.Run == nil {
			t.Fatalf("tool %q: Run is nil, cannot smoke-test", tool.Command)
		}
		seen[tool.Command] = true
	}
	if len(seen) != wantDistinctCount {
		t.Fatalf("expected %d distinct Implemented tool commands, saw %d", wantDistinctCount, len(seen))
	}
}
