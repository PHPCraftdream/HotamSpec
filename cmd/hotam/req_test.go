package main

import (
	"testing"
)

func TestSplitSubcommand_PlainShow(t *testing.T) {
	t.Parallel()
	sub, rest, ok := splitSubcommand([]string{"show", "R-x"})
	if !ok || sub != "show" || len(rest) != 1 || rest[0] != "R-x" {
		t.Fatalf("got sub=%q rest=%v ok=%v", sub, rest, ok)
	}
}

func TestSplitSubcommand_BooleanFlagBeforeSubcommand(t *testing.T) {
	t.Parallel()
	// Mirrors what reorderFlagsFirst produces for "req show R-x --json":
	// the trailing boolean --json is hoisted in front of "show R-x".
	sub, rest, ok := splitSubcommand([]string{"--json", "show", "R-x"})
	if !ok || sub != "show" {
		t.Fatalf("got sub=%q rest=%v ok=%v, want sub=show", sub, rest, ok)
	}
	want := map[string]bool{"--json": false, "R-x": false}
	for _, r := range rest {
		if _, known := want[r]; known {
			want[r] = true
		}
	}
	for k, seen := range want {
		if !seen {
			t.Errorf("expected %q preserved in rest=%v", k, rest)
		}
	}
}

func TestSplitSubcommand_ValuedFlagBeforeSubcommand(t *testing.T) {
	t.Parallel()
	// Mirrors "req context R-x --json --domain /tmp/d" after reordering.
	sub, rest, ok := splitSubcommand([]string{"--json", "--domain", "/tmp/d", "context", "R-x"})
	if !ok || sub != "context" {
		t.Fatalf("got sub=%q rest=%v ok=%v, want sub=context", sub, rest, ok)
	}
	if len(rest) != 4 {
		t.Fatalf("expected 4 remaining tokens, got %v", rest)
	}
}

func TestSplitSubcommand_NoSubcommand(t *testing.T) {
	t.Parallel()
	_, _, ok := splitSubcommand([]string{"--json", "--domain", "/tmp/d"})
	if ok {
		t.Fatal("expected ok=false when no non-flag token is present")
	}
}

func TestCmdReq_UnknownSubcommand(t *testing.T) {
	t.Parallel()
	err := cmdReq([]string{"bogus"})
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
}

func TestCmdReq_NoArgs(t *testing.T) {
	t.Parallel()
	err := cmdReq(nil)
	if err == nil {
		t.Fatal("expected error for empty args")
	}
}

func TestCmdReq_Help(t *testing.T) {
	t.Parallel()
	if err := cmdReq([]string{"-h"}); err != nil {
		t.Fatalf("cmdReq -h: %v", err)
	}
	if err := cmdReq([]string{"help"}); err != nil {
		t.Fatalf("cmdReq help: %v", err)
	}
}

func TestCmdReqShow_SmokeOnSelfDomain(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	g, err := loadDomainGraph(domainDir)
	if err != nil {
		t.Fatalf("loadDomainGraph: %v", err)
	}
	if len(g.Requirements) == 0 {
		t.Fatal("expected non-empty requirements in self domain fixture")
	}
	// Exercise the same path cmdReqShow uses end-to-end via the package
	// API, on a real SETTLED anchor known to exist in hotam-spec-self.
	err = cmdReqShow([]string{"--domain", domainDir, "R-anchor-everything", "--json"})
	if err != nil {
		t.Fatalf("cmdReqShow: %v", err)
	}
}

func TestCmdReqContext_SmokeShowsConflict(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	// R-ai-presents-not-decides is a member of conflict C-186c4347 in the
	// real hotam-spec-self domain fixture — context on it must surface
	// that conflict.
	err := cmdReqContext([]string{"--domain", domainDir, "R-ai-presents-not-decides", "--json"})
	if err != nil {
		t.Fatalf("cmdReqContext: %v", err)
	}
}

func TestCmdReqList_SmokeNoPanic(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	if err := cmdReqList([]string{"--domain", domainDir, "--status", "SETTLED"}); err != nil {
		t.Fatalf("cmdReqList: %v", err)
	}
}

func TestCmdReqSearch_SmokeNoPanic(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	if err := cmdReqSearch([]string{"--domain", domainDir, "anchor"}); err != nil {
		t.Fatalf("cmdReqSearch: %v", err)
	}
}

func TestCmdReqRelated_SmokeNoPanic(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	if err := cmdReqRelated([]string{"--domain", domainDir, "R-anchor-everything"}); err != nil {
		t.Fatalf("cmdReqRelated: %v", err)
	}
}
