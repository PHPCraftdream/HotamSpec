// coverage.go parses Go's own text coverage-profile format (the
// "-coverprofile" output RunVerifiedByTestRecording already collects, see
// test_exec.go's RecordingResult.CoverProfile doc comment) and answers the
// one question PLAN-scenario-generated-spec.md §2 D3 / task W2.2's
// check_scenario_executes_impl needs: did a verified_by test's run actually
// EXECUTE (count > 0) at least one statement block whose source lines fall
// inside a given implemented_by symbol's own declaration range?
//
// Deliberately a HAND-ROLLED parser, not golang.org/x/tools/cover: this
// module's go.mod/go.sum (checked before writing this file) do not already
// depend on x/tools, and the coverprofile text format is small and stable
// enough (documented by `go help testflag` / `go tool cover`'s own source)
// that adding a new module dependency just to parse it is not worth the
// supply-chain surface for a five-field-per-line format. The format, one
// line per counted statement block, after a single leading "mode: <mode>"
// header line:
//
//	<file>:<startLine>.<startCol>,<endLine>.<endCol> <numStmt> <count>
//
// <file> is the block's Go IMPORT PATH joined with its base file name
// (e.g. "prat-spec/model/forecast.go"), NOT an OS filesystem path -- this is
// why ImportPathForFile exists: a caller holding an OS path (as every
// implemented_by entry resolves to, via ResolveSpecSymbolRange) must first
// convert it to the import-path form before it can be matched against a
// parsed coverage block's File field.
package gate

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// CoverageBlock is one parsed line of a Go text coverage profile: the
// import-path-qualified file a counted statement block belongs to, its
// [StartLine, EndLine] source range (1-indexed, inclusive, matching Go's own
// convention -- EndLine is the line the block's closing position falls on,
// which may itself contain covered code, so callers treating this as an
// inclusive range match cover's own semantics), and Count (the number of
// times `go test` executed that block -- zero means instrumented but never
// reached).
type CoverageBlock struct {
	File      string
	StartLine int
	EndLine   int
	Count     int
}

// ParseCoverProfile parses the raw bytes of a `go test -coverprofile` text
// file (RecordingResult.CoverProfile) into CoverageBlock values, skipping the
// leading "mode: <mode>" header line and any blank line. Malformed lines
// (unexpected shape -- should not occur for a profile `go test` itself wrote,
// but a caller must not panic on a hand-edited or truncated file) are
// silently skipped rather than erroring the whole parse: a partial profile is
// still useful for the coverage-proof question this package answers, and a
// hard parse failure would turn an unrelated formatting quirk into a false
// "impl not covered" violation rather than the honest "we could not read this
// line" it actually is.
func ParseCoverProfile(data []byte) []CoverageBlock {
	var blocks []CoverageBlock
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}
		block, ok := parseCoverProfileLine(line)
		if !ok {
			continue
		}
		blocks = append(blocks, block)
	}
	return blocks
}

// parseCoverProfileLine parses one non-header, non-blank line of the form
// "<file>:<startLine>.<startCol>,<endLine>.<endCol> <numStmt> <count>". The
// file portion can itself legitimately contain ":" (a Windows-style import
// path never does in practice, but this parser does not assume otherwise) --
// it splits on the LAST ":" before the position spec instead of the first,
// by first splitting off the two trailing whitespace-separated numeric
// fields (numStmt, count) and then locating the position spec
// "<startLine>.<startCol>,<endLine>.<endCol>" at the end of what remains via
// the last ":".
func parseCoverProfileLine(line string) (CoverageBlock, bool) {
	fields := strings.Fields(line)
	if len(fields) != 3 {
		return CoverageBlock{}, false
	}
	fileAndPos, numStmtStr, countStr := fields[0], fields[1], fields[2]
	_ = numStmtStr // numStmt itself is not needed for the coverage-proof question
	count, err := strconv.Atoi(countStr)
	if err != nil {
		return CoverageBlock{}, false
	}

	idx := strings.LastIndex(fileAndPos, ":")
	if idx < 0 {
		return CoverageBlock{}, false
	}
	file := fileAndPos[:idx]
	posSpec := fileAndPos[idx+1:]
	if file == "" || posSpec == "" {
		return CoverageBlock{}, false
	}

	commaIdx := strings.Index(posSpec, ",")
	if commaIdx < 0 {
		return CoverageBlock{}, false
	}
	startSpec := posSpec[:commaIdx]
	endSpec := posSpec[commaIdx+1:]

	startLine, ok := parseLineFromLineDotCol(startSpec)
	if !ok {
		return CoverageBlock{}, false
	}
	endLine, ok := parseLineFromLineDotCol(endSpec)
	if !ok {
		return CoverageBlock{}, false
	}

	return CoverageBlock{File: file, StartLine: startLine, EndLine: endLine, Count: count}, true
}

// parseLineFromLineDotCol extracts the line number from a "<line>.<col>"
// position spec (Go coverage profile's own sub-format for both the start and
// end positions of a block).
func parseLineFromLineDotCol(spec string) (int, bool) {
	dotIdx := strings.Index(spec, ".")
	if dotIdx < 0 {
		return 0, false
	}
	line, err := strconv.Atoi(spec[:dotIdx])
	if err != nil {
		return 0, false
	}
	return line, true
}

// SymbolRangeCoveredByProfile reports whether blocks (already parsed via
// ParseCoverProfile) contains at least one block whose File matches
// importPath (exact string match -- the import-path form ImportPathForFile
// computes, e.g. "prat-spec/model/forecast.go") AND whose [StartLine,EndLine]
// range OVERLAPS [symbolStartLine, symbolEndLine] AND whose Count is > 0 --
// the direct mechanical answer to "did THIS run actually execute at least one
// statement inside this symbol's own declaration range", which is the whole
// point of task W2.2's coverage-proof: a green verified_by test that never
// once touches the implemented_by symbol's lines produces zero overlapping
// blocks with Count>0 here, regardless of how the test's assertions read.
//
// Overlap (not containment) is the deliberate comparison: a single coverage
// block can legitimately span a range that starts before or ends after the
// symbol's own declaration lines (Go's cover instrumentation groups
// consecutive statements into one block, and a block boundary does not
// necessarily align exactly with a FuncDecl's own Pos/End) -- requiring the
// block to be fully CONTAINED within the symbol range would under-count
// (a real, in-symbol executed statement could sit inside a slightly wider
// block whose boundary crosses the symbol's own start/end) whereas overlap
// is the correct, conservative "some covered code touches this symbol" test.
func SymbolRangeCoveredByProfile(blocks []CoverageBlock, importPath string, symbolStartLine, symbolEndLine int) bool {
	for _, b := range blocks {
		if b.File != importPath {
			continue
		}
		if b.Count <= 0 {
			continue
		}
		if rangesOverlap(b.StartLine, b.EndLine, symbolStartLine, symbolEndLine) {
			return true
		}
	}
	return false
}

// rangesOverlap reports whether inclusive integer ranges [aStart,aEnd] and
// [bStart,bEnd] share at least one line.
func rangesOverlap(aStart, aEnd, bStart, bEnd int) bool {
	return aStart <= bEnd && bStart <= aEnd
}

// ImportPathForFile computes the Go import-path-qualified form of absFile
// (an absolute OS path to a .go file inside the module rooted at
// moduleRoot) -- the same "<module path>/<dir>/<base>.go" form Go's own
// coverage profile writer uses for a block's File field (see this file's
// package doc comment). Reads moduleRoot/go.mod's own "module " directive
// (readModulePath) rather than accepting the module path as a separate
// parameter, so a caller only ever has to know the filesystem location, the
// same convention gate.ModuleRoot/relativePackagePattern already follow.
func ImportPathForFile(moduleRoot, absFile string) (string, error) {
	modulePath, err := readModulePath(moduleRoot)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(moduleRoot, absFile)
	if err != nil {
		return "", err
	}
	relSlash := filepath.ToSlash(rel)
	if strings.HasPrefix(relSlash, "../") || relSlash == ".." {
		return "", &importPathEscapeError{moduleRoot: moduleRoot, absFile: absFile}
	}
	return modulePath + "/" + relSlash, nil
}

// importPathEscapeError reports that absFile does not live inside
// moduleRoot -- ImportPathForFile cannot compute an import path for a file
// outside the module whose go.mod it was asked to read.
type importPathEscapeError struct {
	moduleRoot string
	absFile    string
}

func (e *importPathEscapeError) Error() string {
	return "file " + e.absFile + " is not inside module root " + e.moduleRoot
}

// readModulePath reads moduleRoot/go.mod and extracts the module path from
// its "module <path>" directive (the first non-comment, non-blank line
// starting with "module " -- go.mod's own grammar guarantees this directive
// is always a single line, never a `require`-style block form). Deliberately
// a tiny hand-rolled reader (not golang.org/x/mod/modfile) for the identical
// reason ParseCoverProfile is hand-rolled: this module does not already
// depend on x/mod, and a single directive line is not worth a new dependency.
func readModulePath(moduleRoot string) (string, error) {
	data, err := os.ReadFile(filepath.Join(moduleRoot, "go.mod"))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			path := strings.TrimSpace(strings.TrimPrefix(line, "module "))
			// A module path can be quoted in go.mod (rare, but valid syntax);
			// strip matching double quotes if present.
			path = strings.Trim(path, "\"")
			return path, nil
		}
	}
	return "", &noModuleDirectiveError{moduleRoot: moduleRoot}
}

// noModuleDirectiveError reports that moduleRoot/go.mod has no "module "
// directive line -- a malformed or unexpected go.mod, since every valid
// go.mod must declare its module path.
type noModuleDirectiveError struct {
	moduleRoot string
}

func (e *noModuleDirectiveError) Error() string {
	return "no \"module \" directive found in " + filepath.Join(e.moduleRoot, "go.mod")
}
