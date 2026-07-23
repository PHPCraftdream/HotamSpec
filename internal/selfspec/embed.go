package selfspec

import (
	"embed"
	"strings"
)

// SourceFiles embeds a build-time-frozen copy of this package's own Go
// source files — every requirements_*.go (the 27 thematic registration
// files, task #345/RAC-A) plus selfspec.go (the Requirements registry
// declaration itself). It exists for a future stale-binary guard (task
// #349/RAC-B2 and beyond): comparing these embedded bytes against the same
// files read fresh off disk at runtime is how a caller detects "this binary
// was compiled from an older copy of the registry than what's on disk right
// now" — a real hazard for a long-running process or a stale build
// artifact, since Requirements itself (the in-memory registry) is frozen at
// compile time and cannot self-report staleness any other way.
//
// Deliberately excludes this package's _test.go files and any non-Go file:
// go:embed's glob (requirements_*.go, selfspec.go) matches only those two
// exact patterns, mirroring internal/recorder/canon's identical
// single-purpose embed of its own package (see that package's embed.go).
//
//go:embed requirements_*.go selfspec.go
var SourceFiles embed.FS

// SourceFileFor looks up which requirements_<topic>.go file registers id,
// by scanning SourceFiles for a file whose content contains the exact
// registration literal `MustRegister("<id>"` (matching real registration
// code, e.g. `Requirements.MustRegister("R-operator-acting-facet", ...)`).
// Returns the matching file's base name (e.g. "requirements_operator.go")
// and true, or "" and false if no embedded file contains that literal — the
// legitimate case for a brand-new ID that SyncGraph has just created in the
// graph (task #348/RAC-B1's CREATE path) but that has not yet been placed
// into any thematic requirements_<topic>.go file by a human/codegen pass.
func SourceFileFor(id string) (string, bool) {
	needle := `MustRegister("` + id + `"`
	entries, err := SourceFiles.ReadDir(".")
	if err != nil {
		return "", false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "requirements_") && name != "selfspec.go" {
			continue
		}
		data, err := SourceFiles.ReadFile(name)
		if err != nil {
			continue
		}
		if strings.Contains(string(data), needle) {
			return name, true
		}
	}
	return "", false
}
