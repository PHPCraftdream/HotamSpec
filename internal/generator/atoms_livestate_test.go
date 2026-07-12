package generator

import (
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/PHPCraftdream/HotamSpecGo/internal/ontology"
)

func TestBuildAtomsOperator_ByteIdenticalToPython(t *testing.T) {
	g := loadDomainGraph(t)
	got := BuildAtomsOperator(g)
	want, err := os.ReadFile("testdata/atoms-operator.md")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "atoms-operator.md", got, string(want))
}

func TestBuildAtomsSubstrate_ByteIdenticalToPython(t *testing.T) {
	g := loadDomainGraph(t)
	got := BuildAtomsSubstrate(g)
	want, err := os.ReadFile("testdata/atoms-substrate.md")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "atoms-substrate.md", got, string(want))
}

func TestBuildAtomsDiscipline_ByteIdenticalToPython(t *testing.T) {
	g := loadDomainGraph(t)
	got := BuildAtomsDiscipline(g)
	want, err := os.ReadFile("testdata/atoms-discipline.md")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "atoms-discipline.md", got, string(want))
}

func TestBuildAtomsCheck_ByteIdenticalToPython(t *testing.T) {
	g := loadDomainGraph(t)
	got := BuildAtomsCheck(g)
	want, err := os.ReadFile("testdata/atoms-check.md")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "atoms-check.md", got, string(want))
}

func extractLiveStateBlock(text string) string {
	beginSentinel := "<!-- LIVE-STATE:BEGIN -->\n"
	endSentinel := "\n<!-- LIVE-STATE:END -->"
	bi := strings.Index(text, beginSentinel)
	if bi < 0 {
		return ""
	}
	start := bi + len(beginSentinel)
	ei := strings.Index(text[start:], endSentinel)
	if ei < 0 {
		return ""
	}
	return text[start : start+ei]
}

func TestBuildLiveState_ByteIdenticalToPython(t *testing.T) {
	g := loadDomainGraph(t)

	claudeMDPath := `D:\ai_dev\prat\HotamSpec\CLAUDE.md`
	claudeMDBytes, err := os.ReadFile(claudeMDPath)
	if err != nil {
		claudeMDBytes, err = os.ReadFile("testdata/CLAUDE-LIVE-STATE.md")
		if err != nil {
			t.Skipf("CLAUDE.md not found at %s and no testdata fallback: %v", claudeMDPath, err)
		}
	}

	claudeMDText := string(claudeMDBytes)
	charCount := utf8.RuneCountInString(claudeMDText)

	want := extractLiveStateBlock(claudeMDText)
	if want == "" {
		wantBytes, ferr := os.ReadFile("testdata/live-state.md")
		if ferr != nil {
			t.Fatalf("could not extract LIVE-STATE block and no testdata/live-state.md fallback")
		}
		want = string(wantBytes)
		charCount = 27646
	}

	got := BuildLiveState(g, charCount)
	diffReport(t, "LIVE-STATE", got, want)
}

func TestByteIdentical_AtomsLiveState_AgainstOriginalHotamSpecPath(t *testing.T) {
	g := loadDomainGraph(t)

	for _, c := range []struct {
		name string
		path string
		fn   func(*ontology.Graph) string
	}{
		{"atoms/operator.md", `D:\ai_dev\prat\HotamSpec\docs\methodology\atoms\operator.md`, BuildAtomsOperator},
		{"atoms/substrate.md", `D:\ai_dev\prat\HotamSpec\docs\methodology\atoms\substrate.md`, BuildAtomsSubstrate},
		{"atoms/discipline.md", `D:\ai_dev\prat\HotamSpec\docs\methodology\atoms\discipline.md`, BuildAtomsDiscipline},
		{"atoms/check.md", `D:\ai_dev\prat\HotamSpec\docs\methodology\atoms\check.md`, BuildAtomsCheck},
	} {
		want, err := os.ReadFile(c.path)
		if err != nil {
			t.Logf("skip %s: %v", c.name, err)
			continue
		}
		got := c.fn(g)
		diffReport(t, c.name+" (original path)", got, string(want))
	}

	claudeMDPath := `D:\ai_dev\prat\HotamSpec\CLAUDE.md`
	claudeMDBytes, err := os.ReadFile(claudeMDPath)
	if err != nil {
		t.Logf("skip LIVE-STATE (original path): %v", err)
		return
	}
	claudeMDText := string(claudeMDBytes)
	charCount := utf8.RuneCountInString(claudeMDText)
	want := extractLiveStateBlock(claudeMDText)
	if want == "" {
		t.Logf("skip LIVE-STATE (original path): sentinels not found")
		return
	}
	got := BuildLiveState(g, charCount)
	diffReport(t, "LIVE-STATE (original path)", got, want)
}
