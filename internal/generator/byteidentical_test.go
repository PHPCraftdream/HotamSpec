package generator

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpecGo/internal/loader"
	"github.com/PHPCraftdream/HotamSpecGo/internal/ontology"
)

const domainGraphPath = "../../domains/hotam-spec-self/graph.json"

func loadDomainGraph(t *testing.T) *ontology.Graph {
	t.Helper()
	g, err := loader.LoadGraph(domainGraphPath)
	if err != nil {
		t.Fatalf("LoadGraph(%s): %v", domainGraphPath, err)
	}
	return g
}

func diffReport(t *testing.T, name, got, want string) {
	t.Helper()
	if got == want {
		return
	}
	gotLines := strings.Split(got, "\n")
	wantLines := strings.Split(want, "\n")
	max := len(gotLines)
	if len(wantLines) < max {
		max = len(wantLines)
	}
	first := max
	for i := 0; i < max; i++ {
		if gotLines[i] != wantLines[i] {
			first = i
			break
		}
	}
	start := first - 3
	if start < 0 {
		start = 0
	}
	end := first + 5
	var b strings.Builder
	b.WriteString("\n=== byte-identity FAILED for " + name + " ===\n")
	b.WriteString("got bytes=" + strconv.Itoa(len(got)) + " want bytes=" + strconv.Itoa(len(want)) + "\n")
	b.WriteString("first differing line index: " + strconv.Itoa(first) + "\n")
	for i := start; i < end; i++ {
		gotLine := ""
		if i < len(gotLines) {
			gotLine = gotLines[i]
		}
		marker := "  "
		wantLine := ""
		if i < len(wantLines) {
			wantLine = wantLines[i]
		}
		if i >= len(wantLines) {
			marker = "G>"
		} else if i >= len(gotLines) {
			marker = "W<"
		} else if gotLine != wantLine {
			marker = "* "
		}
		b.WriteString(marker + " line[" + strconv.Itoa(i) + "]\n    got:  " + truncate(gotLine, 160) + "\n    want: " + truncate(wantLine, 160) + "\n")
	}
	t.Error(b.String())
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func TestBuildRequirements_ByteIdenticalToPython(t *testing.T) {
	g := loadDomainGraph(t)
	got := BuildRequirements(g)
	want, err := os.ReadFile("testdata/REQUIREMENTS.md")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "REQUIREMENTS.md", got, string(want))
}

func TestBuildTensions_ByteIdenticalToPython(t *testing.T) {
	g := loadDomainGraph(t)
	got := BuildTensions(g)
	want, err := os.ReadFile("testdata/TENSIONS.md")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "TENSIONS.md", got, string(want))
}

func TestByteIdentical_AgainstOriginalHotamSpecPath(t *testing.T) {
	for _, c := range []struct {
		name string
		path string
		fn   func(*ontology.Graph) string
	}{
		{"REQUIREMENTS.md", `D:\ai_dev\prat\HotamSpec\domains\hotam-spec-self\docs\gen\REQUIREMENTS.md`, BuildRequirements},
		{"TENSIONS.md", `D:\ai_dev\prat\HotamSpec\domains\hotam-spec-self\docs\gen\TENSIONS.md`, BuildTensions},
	} {
		want, err := os.ReadFile(c.path)
		if err != nil {
			t.Logf("skip %s: %v", c.name, err)
			continue
		}
		g := loadDomainGraph(t)
		got := c.fn(g)
		diffReport(t, c.name+" (original path)", got, string(want))
	}
}
