// engine_version_current.go holds check_engine_docs_fingerprint_current
// (task #400, W2.3): the mechanical staleness gate for a domain's committed
// docs/gen/ENGINE-VERSION.md. This file is the read-side complement to
// generator.BuildEngineVersionMD (the write side): once a domain HAS a
// committed ENGINE-VERSION.md carrying a stamped engine identity fingerprint,
// it must match the CURRENT engine's own fingerprint — a mismatch means the
// engine that produced this domain's generated docs is not the engine running
// right now, and the docs may be stale (regenerate via `hotam gen-spec`).
//
// HONEST NO-OP when the file is absent: mirrors check_spec_md_current's own
// "a domain that has never run gen-spec since this feature landed has no
// ENGINE-VERSION.md to compare, so it contributes zero violations" pattern.
// The check also degrades to an honest no-op when the current engine's own
// fingerprint cannot be computed (gate.EngineDocsFingerprint returns a non-nil
// error — e.g. the three watched engine packages are not present under the
// resolved module root, as happens for a consumer repo whose engine is a
// compiled binary without source): without a current fingerprint there is
// nothing to compare against, so the check stays silent rather than firing a
// spurious violation.
//
// COMPARISON METHOD — content-hash fingerprint compare, NOT a full re-render:
// unlike check_spec_md_current (which must actually re-run `go test` per
// verified_by entry to prove SPEC.md is current), this check's stamped value
// is a pure function of the engine's source tree, so recomputing it is a
// cheap directory walk + sha256, not an expensive test execution.
package invariants

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// engineVersionMDRelPath is where genSpec writes ENGINE-VERSION.md, relative
// to a domain's own directory — mirrors the generator package's own
// engineVersionMDRelPath constant (internal/generator/engine_version.go).
// Duplicated here (rather than imported) because internal/invariants MUST NOT
// import internal/generator (a mechanically-enforced import cycle — see
// claude_md_current.go's own package doc comment for the full rationale).
const engineVersionMDRelPath = "docs/gen/ENGINE-VERSION.md"

// engineFingerprintLabel is the marker line prefix the check searches for to
// extract the stamped fingerprint — mirrors the generator's own constant.
const engineFingerprintLabel = "**Engine content fingerprint:**"

// engineFingerprintHexRe matches the backtick-wrapped hex value following the
// label line.
var engineFingerprintHexRe = regexp.MustCompile(regexp.QuoteMeta(engineFingerprintLabel) + `\s*\x60([0-9a-f]+)\x60`)

// resolveModuleRoot finds the Go module root (the directory containing
// go.mod) from a domain directory path, mirroring cmd/hotam's own
// repoRootForDomain logic: if the domain sits under a domains/ directory, the
// module root is two levels above it; otherwise, walk up looking for go.mod.
func resolveModuleRoot(domainDir string) string {
	if filepath.Base(filepath.Dir(domainDir)) == "domains" {
		return filepath.Dir(filepath.Dir(domainDir))
	}
	dir := domainDir
	for i := 0; i < 20; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return domainDir
}

// checkEngineDocsFingerprintCurrent is the mechanical freshness gate for
// docs/gen/ENGINE-VERSION.md. See the file-level doc comment above for the
// full rationale, honest-no-op conditions, and comparison method.
func checkEngineDocsFingerprintCurrent(g *ontology.Graph) []Violation {
	if g.DomainDir == "" {
		return nil
	}
	versionPath := filepath.Join(g.DomainDir, filepath.FromSlash(engineVersionMDRelPath))
	committed, err := os.ReadFile(versionPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return []Violation{{
			Check:   "check_engine_docs_fingerprint_current",
			ID:      g.DomainDir,
			Message: fmt.Sprintf("could not read %s: %v", versionPath, err),
		}}
	}

	match := engineFingerprintHexRe.FindStringSubmatch(string(committed))
	if match == nil {
		return []Violation{{
			Check: "check_engine_docs_fingerprint_current",
			ID:    g.DomainDir,
			Message: fmt.Sprintf(
				"%s exists but does not carry a parseable engine fingerprint (%q marker line with a backtick-wrapped hex value) -- "+
					"it may have been hand-edited; re-run `hotam gen-spec --domain %s` to regenerate it",
				versionPath, engineFingerprintLabel, g.DomainDir),
		}}
	}
	stamped := match[1]

	moduleRoot := resolveModuleRoot(g.DomainDir)
	current, err := gate.EngineDocsFingerprint(moduleRoot)
	if err != nil {
		// Honest no-op: the current engine's fingerprint cannot be computed
		// (e.g. the three watched engine packages are not present under the
		// resolved module root — a consumer repo whose engine is a compiled
		// binary). Without a current fingerprint there is nothing to compare
		// against.
		return nil
	}
	shortCurrent := current
	const shortLen = 16
	if len(shortCurrent) > shortLen {
		shortCurrent = shortCurrent[:shortLen]
	}

	if stamped != shortCurrent {
		return []Violation{{
			Check: "check_engine_docs_fingerprint_current",
			ID:    g.DomainDir,
			Message: fmt.Sprintf(
				"%s was stamped by engine fingerprint %s, but the current engine's fingerprint is %s -- "+
					"the engine that produced this domain's generated docs is not the engine you have now; "+
					"run `hotam gen-spec --domain %s` to regenerate",
				versionPath, stamped, shortCurrent, g.DomainDir),
		}}
	}
	return nil
}

var _ = All.MustRegister("check_engine_docs_fingerprint_current", Invariant{
	Name:                     "check_engine_docs_fingerprint_current",
	ComparesOnDiskProjection: true,
	Canon:                    methodology.Domain,
	Claim: "a domain's committed docs/gen/ENGINE-VERSION.md, if present, carries a stamped engine content fingerprint that matches the " +
		"current engine's own fingerprint (gate.EngineDocsFingerprint, a deterministic sha256 content-hash over internal/generator, " +
		"internal/ontology, internal/loader); a domain with no ENGINE-VERSION.md yet is an honest no-op.",
	Rule: "IF a file exists at <domainDir>/docs/gen/ENGINE-VERSION.md, its stamped fingerprint (the hex value on the " +
		"\"**Engine content fingerprint:**\" line) MUST equal the first 16 hex characters of gate.EngineDocsFingerprint(moduleRoot) " +
		"computed right now, where moduleRoot is resolved from <domainDir> the same way genSpec resolves it. A domain with NO file " +
		"at that path is an honest NO-OP -- mirroring check_spec_md_current's own opt-in-when-absent shape. The check is also an " +
		"honest no-op when the current engine's fingerprint cannot be computed (the three watched packages are absent under the " +
		"resolved module root), so a consumer repo whose engine is a compiled binary never fires a spurious violation.",
	Why: "Consumer-generated documentation (in a separate repository from the engine) can silently lag behind engine evolution: " +
		"the engine changes in its own commits, the consumer's generated docs sit in a different repository, and no mechanism tells " +
		"the consumer's resolver 'the engine that produced these docs is not the engine you have now.' This exact class of problem " +
		"caused the W0.1 regression (task #369's engine upgrade retroactively broke domains in another repository with zero warning). " +
		"This check closes that gap: the engine identity fingerprint (a content-hash, deliberately NOT a raw git commit SHA which " +
		"would invalidate on every unrelated commit, and NOT a manually-bumped version number which defaults to 'dev' for local " +
		"builds) is stamped into every domain's generated ENGINE-VERSION.md by genSpec, and this check fires when the stamped " +
		"fingerprint disagrees with the current engine's own -- a deterministic, content-sensitive signal of engine/doc drift. " +
		"RELATION TO check_spec_md_current: both are domain-wide, filesystem-aware, honest-no-op-when-absent invariants " +
		"(methodology.Domain canon, ComparesOnDiskProjection) that read g.DomainDir directly; check_spec_md_current guards a " +
		"GRAPH-DERIVED projection (its content depends on the domain's own requirements/tests), while this check guards an " +
		"ENGINE-DERIVED projection (its content depends only on the engine's source packages, not the domain graph at all).",
	Check: checkEngineDocsFingerprintCurrent,
})
