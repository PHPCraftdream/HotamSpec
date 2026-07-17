package invariants

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	recordervendor "github.com/PHPCraftdream/HotamSpec/internal/recorder/vendor"
)

// vendoredRecorderRelPath is where `hotam vendor-recorder`
// (cmd/hotam/vendor_recorder.go) writes the vendored copy, relative to a
// domain's own directory -- kept as a named constant here (rather than
// re-deriving filepath.Join("spec", "hotamspec", "hotamspec.go") inline)
// so the write side and the check side visibly agree on one literal path.
const vendoredRecorderRelPath = "spec/hotamspec/hotamspec.go"

// checkRecorderCurrent is the mechanical actuality gate for the vendored
// hotamspec scenario-recorder (PLAN-scenario-generated-spec.md §2 D1, task
// W1.1): IF a domain has ever vendored the recorder (a file sits at
// spec/hotamspec/hotamspec.go), its content -- after stripping the
// do-not-edit banner `hotam vendor-recorder` always stamps on top -- MUST
// sha256-match the engine's OWN canonical source
// (internal/recorder/canon/hotamspec.go, reached here via
// recordervendor.BodyForHash(), which is itself a go:embed of
// that exact file -- see internal/recorder/canon/embed.go). A mismatch means
// one of two things happened, and this check cannot and does not try to
// distinguish which (both are equally a violation):
//
//  1. STALE: the domain vendored an OLDER canon and never re-ran
//     `hotam vendor-recorder` after the engine's own recorder API changed.
//  2. FORGED/HAND-EDITED: someone edited spec/hotamspec/hotamspec.go by hand
//     (directly contradicting the file's own banner, see
//     internal/generator/recorder_vendor.go's vendorBanner text) -- e.g. to
//     make Then() silently record PASS without ever calling t.Errorf, which
//     would let a scenario-narrated verified_by test claim proof it never
//     actually performed.
//
// This is an HONEST NO-OP for a domain that has never vendored the recorder
// at all (no file at spec/hotamspec/hotamspec.go): the scenario layer is
// opt-in per PLAN-scenario-generated-spec.md's own wave structure (W1.1 adds
// the mechanism; W2.1's later `discipline: full` gate is what will make
// scenario coverage MANDATORY for a domain that has flipped that switch) --
// a domain that has not yet adopted scenarios is not lying about anything by
// not having a vendored copy on disk.
//
// This is a FILESYSTEM check (reads g.DomainDir directly), the same
// architectural shape as checkGraphLockPinsGraphJSON
// (internal/invariants/all_violations.go) -- not the honest-no-op
// domain-structure checks nearby in this same file's neighbors
// (checkDomainManifestValid et al.), which are documented placeholders for a
// future filesystem-aware layer that does not exist yet. This check's own
// filesystem access is exactly the precedent checkGraphLockPinsGraphJSON
// already established: g.DomainDir is populated by loader.LoadGraph for
// every domain (self-hosting or consumer), so a real path is always
// available to check against.
func checkRecorderCurrent(g *ontology.Graph) []Violation {
	if g.DomainDir == "" {
		// No on-disk domain to check against (an in-memory fixture graph
		// built without ever going through loader.LoadGraph) -- honest
		// no-op, mirroring checkGraphLockPinsGraphJSON's identical guard.
		return nil
	}
	vendoredPath := filepath.Join(g.DomainDir, filepath.FromSlash(vendoredRecorderRelPath))
	data, err := os.ReadFile(vendoredPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Recorder never vendored into this domain -- opt-in, not a
			// violation (see doc comment above).
			return nil
		}
		return []Violation{{
			Check:   "check_recorder_current",
			ID:      g.DomainDir,
			Message: fmt.Sprintf("could not read vendored recorder at %s: %v", vendoredPath, err),
		}}
	}

	body, ok := recordervendor.StripBanner(string(data))
	if !ok {
		return []Violation{{
			Check: "check_recorder_current",
			ID:    g.DomainDir,
			Message: fmt.Sprintf(
				"%s does not start with the expected `hotam vendor-recorder` do-not-edit banner -- "+
					"it was hand-created or hand-edited rather than produced by `hotam vendor-recorder`; "+
					"re-run `hotam vendor-recorder --domain %s` to restore a genuine vendored copy",
				vendoredPath, g.DomainDir),
		}}
	}

	gotHash := sha256Hex(body)
	wantHash := sha256Hex(recordervendor.BodyForHash())
	if gotHash != wantHash {
		return []Violation{{
			Check: "check_recorder_current",
			ID:    g.DomainDir,
			Message: fmt.Sprintf(
				"%s (sha256 %s) does not match the engine's current canonical recorder "+
					"(internal/recorder/canon/hotamspec.go, sha256 %s) -- either the vendored copy is stale "+
					"(re-run `hotam vendor-recorder --domain %s` to pick up the current canon) or it was hand-edited "+
					"despite the do-not-edit banner (restore it via the same command)",
				vendoredPath, gotHash, wantHash, g.DomainDir),
		}}
	}
	return nil
}

// sha256Hex is a tiny local helper so the check's own body reads as "hash
// this, hash that, compare" without importing crypto/sha256's two-step
// New/Sum dance inline at each call site.
func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

var _ = All.MustRegister("check_recorder_current", Invariant{
	Name:  "check_recorder_current",
	Canon: methodology.Domain,
	Claim: "a domain's vendored spec/hotamspec/hotamspec.go, if present, is byte-identical (post-banner) to the engine's own canonical recorder.",
	Rule: "IF a file exists at <domainDir>/spec/hotamspec/hotamspec.go, it MUST start with the exact `hotam vendor-recorder` " +
		"do-not-edit banner (internal/recorder/vendor's Banner), and the body following that banner MUST sha256-match " +
		"internal/recorder/canon/hotamspec.go's own current content (recordervendor.BodyForHash(), itself a " +
		"go:embed of that exact file). A domain with NO file at that path is an honest no-op -- vendoring the recorder is " +
		"opt-in per the domain's own adoption of the scenario layer (PLAN-scenario-generated-spec.md), not mandatory for " +
		"every domain unconditionally.",
	Why: "the vendored recorder is Go code a consumer domain's spec/ module COMPILES AND RUNS as part of every verified_by " +
		"test that uses it -- unlike a generated Markdown doc (drift there is merely stale prose), a stale or hand-edited " +
		"vendored recorder can silently change what Then()/Given()/Value() actually DO (e.g. a hand-edit that makes Then() " +
		"stop calling t.Errorf on a false condition would let a scenario-narrated test report PASS without ever really " +
		"asserting anything -- exactly the vacuous-test hazard check_verified_by_test_has_teeth already polices for plain " +
		"tests, reopened through a side door this check closes). Modeled directly on checkGraphLockPinsGraphJSON " +
		"(all_violations.go): both are graph-generic invariants that read g.DomainDir's own filesystem state (not just the " +
		"in-memory graph), both are honest no-ops when the relevant on-disk artifact was never created, and both exist to " +
		"catch a real drift class no purely in-memory graph check could ever see.",
	Check: checkRecorderCurrent,
})
