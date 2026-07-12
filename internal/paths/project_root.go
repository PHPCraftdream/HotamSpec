package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	EnvProjectRoot   = "HOTAM_SPEC_PROJECT_ROOT"
	EnvDomainsRoot   = "HOTAM_SPEC_DOMAINS_ROOT"
	MarkerFilename   = ".hotam-spec-project"
	PyprojectTable   = "hotam-spec"
	PyprojectRootKey = "project_root"
	MaxMarkerSearchDepth = 5
	SecondaryMarkerMinCount = 2
)

var ReliableMarkerPaths = []string{"domains", "delegations"}

var SecondaryMarkerPaths = []string{"CLAUDE.md", ".claude", "tickets"}

var MarkerPaths = append(append([]string{}, ReliableMarkerPaths...), SecondaryMarkerPaths...)

type ProjectRootUnresolved struct {
	Diagnostic string
}

func (e *ProjectRootUnresolved) Error() string {
	return e.Diagnostic
}

func envDir(name string) (string, bool) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return "", false
	}
	abs, err := filepath.Abs(raw)
	if err != nil {
		return "", false
	}
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return "", false
	}
	return abs, true
}

func stripTomlComment(line string) string {
	inQuote := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		if c == '"' {
			inQuote = !inQuote
		} else if c == '#' && !inQuote {
			return line[:i]
		}
	}
	return line
}

func parseTomlValue(raw string) string {
	v := strings.TrimSpace(raw)
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		inner := v[1 : len(v)-1]
		inner = strings.ReplaceAll(inner, `\"`, `"`)
		inner = strings.ReplaceAll(inner, `\\`, `\`)
		return inner
	}
	return v
}

type pyprojectInfo struct {
	hasHotamTable bool
	projectRoot   string
}

func readPyproject(path string) (pyprojectInfo, bool) {
	info := pyprojectInfo{}
	data, err := os.ReadFile(path)
	if err != nil {
		return info, false
	}
	currentTable := ""
	for _, line := range strings.Split(string(data), "\n") {
		line = stripTomlComment(line)
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			currentTable = strings.TrimSpace(trimmed[1 : len(trimmed)-1])
			if currentTable == "tool."+PyprojectTable {
				info.hasHotamTable = true
			}
			continue
		}
		if currentTable == "tool."+PyprojectTable {
			eq := strings.IndexByte(trimmed, '=')
			if eq < 0 {
				continue
			}
			key := strings.TrimSpace(trimmed[:eq])
			val := parseTomlValue(trimmed[eq+1:])
			if key == PyprojectRootKey && val != "" {
				info.projectRoot = val
			}
		}
	}
	return info, true
}

func hasMarker(candidate string) bool {
	for _, rel := range ReliableMarkerPaths {
		if _, err := os.Stat(filepath.Join(candidate, rel)); err == nil {
			return true
		}
	}
	pyproject := filepath.Join(candidate, "pyproject.toml")
	if info, err := os.Stat(pyproject); err == nil && !info.IsDir() {
		if pp, ok := readPyproject(pyproject); ok && pp.hasHotamTable {
			return true
		}
	}
	matched := 0
	for _, rel := range SecondaryMarkerPaths {
		if _, err := os.Stat(filepath.Join(candidate, rel)); err == nil {
			matched++
		}
	}
	return matched >= SecondaryMarkerMinCount
}

func searchMarkersUpward(start string, maxDepth int) (string, bool) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", false
	}
	current := abs
	for i := 0; i <= maxDepth; i++ {
		if hasMarker(current) {
			return current, true
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return "", false
}

func searchMarkerFileUpward(start string, maxDepth int) (string, bool) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", false
	}
	current := abs
	for i := 0; i <= maxDepth; i++ {
		if _, err := os.Stat(filepath.Join(current, MarkerFilename)); err == nil {
			return current, true
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return "", false
}

func resolvePyproject(start string, maxDepth int) (string, bool) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", false
	}
	current := abs
	for i := 0; i <= maxDepth; i++ {
		pyproject := filepath.Join(current, "pyproject.toml")
		if info, err := os.Stat(pyproject); err == nil && !info.IsDir() {
			if pp, ok := readPyproject(pyproject); ok && pp.projectRoot != "" {
				resolved := filepath.Join(current, pp.projectRoot)
				absResolved, err := filepath.Abs(resolved)
				if err == nil {
					if st, err := os.Stat(absResolved); err == nil && st.IsDir() {
						return absResolved, true
					}
				}
			}
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return "", false
}

func repoRoot() (string, bool) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", false
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func isInside(child, parent string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".."
}

func ProjectRoot() (string, bool) {
	if r1, ok := envDir(EnvProjectRoot); ok {
		return r1, true
	}

	if domainsDir, ok := envDir(EnvDomainsRoot); ok {
		parent := filepath.Dir(domainsDir)
		if info, err := os.Stat(parent); err == nil && info.IsDir() {
			return parent, true
		}
	}

	cwd, err := os.Getwd()
	if err == nil {
		if r3, ok := searchMarkersUpward(cwd, MaxMarkerSearchDepth); ok {
			return r3, true
		}
		if r4, ok := searchMarkerFileUpward(cwd, MaxMarkerSearchDepth); ok {
			return r4, true
		}
		if r5, ok := resolvePyproject(cwd, MaxMarkerSearchDepth); ok {
			return r5, true
		}
	}

	repo, ok := repoRoot()
	if !ok {
		return "", false
	}
	if cwd != "" && isInside(cwd, repo) {
		return repo, true
	}
	return "", false
}

func buildDiagnostic() string {
	var lines []string
	lines = append(lines,
		"ProjectRoot() could not resolve a project root.",
		"The following sources were checked (R1–R6):")

	r1Raw := strings.TrimSpace(os.Getenv(EnvProjectRoot))
	if r1Raw != "" {
		st, err := os.Stat(r1Raw)
		state := "exists"
		if err != nil || !st.IsDir() {
			state = "NOT a directory/missing"
		}
		lines = append(lines, fmt.Sprintf("  R1 env %s='%s' — %s", EnvProjectRoot, r1Raw, state))
	} else {
		lines = append(lines, fmt.Sprintf("  R1 env %s — not set", EnvProjectRoot))
	}

	r2Raw := strings.TrimSpace(os.Getenv(EnvDomainsRoot))
	if r2Raw != "" {
		st, err := os.Stat(r2Raw)
		state := "exists"
		if err != nil || !st.IsDir() {
			state = "NOT a directory/missing"
		}
		lines = append(lines, fmt.Sprintf("  R2 env %s='%s' — %s", EnvDomainsRoot, r2Raw, state))
	} else {
		lines = append(lines, fmt.Sprintf("  R2 env %s — not set", EnvDomainsRoot))
	}

	cwd, _ := os.Getwd()
	lines = append(lines, fmt.Sprintf(
		"  R3 CWD=%s — RELIABLE markers (any one suffices): %s, pyproject.toml[tool.%s]; SECONDARY markers (need %d+ together): %s",
		cwd, strings.Join(ReliableMarkerPaths, ", "), PyprojectTable, SecondaryMarkerMinCount, strings.Join(SecondaryMarkerPaths, ", ")))
	lines = append(lines, fmt.Sprintf("  R4 marker file '%s' — searched %d levels up from CWD", MarkerFilename, MaxMarkerSearchDepth))
	lines = append(lines, fmt.Sprintf("  R5 pyproject.toml [tool.%s].%s — searched %d levels up from CWD", PyprojectTable, PyprojectRootKey, MaxMarkerSearchDepth))
	lines = append(lines, "  R6 self-hosting fallback (repo root) — returned None or not applicable")
	lines = append(lines,
		"Set one of: env HOTAM_SPEC_PROJECT_ROOT=<dir>, HOTAM_SPEC_DOMAINS_ROOT=<domains-dir>, create a RELIABLE marker (domains/, delegations/, or pyproject.toml[tool.hotam-spec]) in CWD, or a .hotam-spec-project file.")
	return strings.Join(lines, "\n")
}

func ProjectRootOrRaise() (string, error) {
	root, ok := ProjectRoot()
	if !ok {
		return "", &ProjectRootUnresolved{Diagnostic: buildDiagnostic()}
	}
	return root, nil
}
