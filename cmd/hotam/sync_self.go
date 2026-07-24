package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/diagnose"
	"github.com/PHPCraftdream/HotamSpec/internal/invariants"
	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/selfspec"
)

// cmdSyncSelf implements `hotam sync-self`: the RAC-B2 (task #349) CLI that
// finally exercises internal/selfspec.SyncGraph (RAC-B1, task #348) as a real
// authority-flip command — mirroring internal/selfspec.Requirements' current
// state ONTO domains/hotam-spec-self/graph.json, the engine's own
// self-hosting domain.
//
// Unlike `hotam land`, which applies a resolver-authored ProposedRequirement
// JSON, sync-self's "proposal" is implicit: the Go registry itself (edited by
// hand across requirements_<topic>.go files) IS the change, and this command
// mechanically projects it onto the graph exactly the way `hotam apply-
// proposal`/`hotam land` project a JSON proposal. It is scoped EXCLUSIVELY to
// the self-hosting domain (see the --domain validation below) — this is not a
// general "sync a Go registry onto any graph" tool.
//
// Two modes:
//
//   - dry-run (DEFAULT, no --confirm-hash): computes the SyncReport a real
//     run would produce, renders it human-readably, runs every gate (7-9)
//     as PREVIEW diagnostics, prints "diff-hash: <hex>", and writes NOTHING.
//   - confirm (--confirm-hash <hex>): recomputes the SyncReport + hash from
//     the CURRENT on-disk state (which may have moved since the dry-run),
//     refuses on any mismatch, then runs the full gate sequence for real and
//     — only if every gate passes — writes graph.json + graph.lock, re-runs
//     gen-spec, and re-verifies with all-violations. Any failure AFTER the
//     write triggers a rollback to the pre-write snapshot (the exact same
//     transactional machinery `hotam land` uses — see land.go's
//     snapshotGraphFiles/rollbackLand).
//
// Gate order before any write (task brief's point 10, strictly enforced):
// stale-binary(1) -> confront(7) -> pre/post-violations(8) -> append-only(9).
func cmdSyncSelf(args []string) error {
	fs := newFlagSet("sync-self")
	domain := fs.String("domain", "", "self-hosting domain directory (default: "+defaultDomainRel+"); MUST be a domain with self_hosting=true in manifest.json")
	today := fs.String("today", "", "date in YYYY-MM-DD format (required in --confirm-hash mode)")
	confirmHash := fs.String("confirm-hash", "", "the diff-hash printed by a prior --dry-run; supplying it switches sync-self from dry-run into the real, disk-writing mode")
	reason := fs.String("reason", "", "free-text reason recorded in the graph.lock note alongside the sync summary (optional)")
	ackConflict := fs.String("ack-conflict", "", "cite an existing Conflict node (C-...) whose members cover a confront-gate hit — overrides the confront-gate refusal")
	decisionRef := fs.String("decision-ref", "", "free-text reference to where a human decision was recorded — overrides the confront-gate refusal and is persisted as a HistoryEntry on each affected requirement")
	asJSON := fs.Bool("json", false, "emit machine-readable JSON")
	fs.Parse(args)

	domainDir, err := resolveDomain(*domain)
	if err != nil {
		return err
	}

	// --- Gate 0: this domain must genuinely be the self-hosting one. ---
	// internal/selfspec.Requirements mirrors ONE graph by construction (see
	// that package's own doc comment) — running sync against any other
	// domain would silently CREATE hundreds of unrelated nodes (SyncGraph's
	// ADDED path has no "wrong domain" detection of its own).
	g, err := loadDomainGraph(domainDir)
	if err != nil {
		return fmt.Errorf("sync-self: load domain graph: %w", err)
	}
	if !g.SelfHosting {
		return fmt.Errorf("sync-self: %s is not the self-hosting domain (manifest.json self_hosting != true) — this command only syncs internal/selfspec.Requirements onto the engine's own domains/hotam-spec-self", domainDir)
	}

	// --- Gate 1: stale-binary guard. ---
	// Compares the build-time-embedded copy of internal/selfspec's own
	// source files (selfspec.SourceFiles) against the SAME files read fresh
	// off disk in the engine repo right now. A mismatch means this binary
	// was compiled from an older (or newer, or just different) copy of the
	// registry than what's on disk — `go run ./cmd/hotam sync-self` always
	// compiles fresh so it trivially passes; a stale prebuilt hotam(.exe)
	// does not.
	if err := checkSelfspecBinaryFresh(domainDir); err != nil {
		return err
	}

	gp := graphPathForDomain(domainDir)

	// --- Compute the SyncReport + diff-hash from the CURRENT on-disk state. ---
	// "before" is a fresh independent load (never mutated) so the append-
	// only guard (gate 9) and the pre-mutation violation baseline (gate 8)
	// both compare against a genuinely untouched snapshot; "after" is a
	// SEPARATE fresh load that SyncGraph mutates in place — reloading twice
	// is the simple, unambiguous stand-in for a deep-clone helper (see this
	// function's own doc comment / the task brief's point 3).
	syncToday := *today
	if syncToday == "" {
		syncToday = "1970-01-01" // dry-run never persists a History entry timestamp for real; a real run requires --today (checked below).
	}
	before, err := loader.LoadGraph(gp)
	if err != nil {
		return fmt.Errorf("sync-self: load pre-sync graph: %w", err)
	}
	after, err := loader.LoadGraph(gp)
	if err != nil {
		return fmt.Errorf("sync-self: load working graph: %w", err)
	}
	report, err := selfspec.SyncGraph(after, syncToday)
	if err != nil {
		return fmt.Errorf("sync-self: SyncGraph: %w", err)
	}

	diffHash, err := computeSyncDiffHash(gp, report)
	if err != nil {
		return fmt.Errorf("sync-self: compute diff hash: %w", err)
	}

	// --- Gates 7-9, run as PREVIEW diagnostics in dry-run, for real otherwise. ---
	ackOpts := landAckOptions{AckConflict: *ackConflict, DecisionRef: *decisionRef}
	gateReport, gateErr := runSyncGates(domainDir, before, after, report, ackOpts)

	out := syncOut(*asJSON)

	if *confirmHash == "" {
		// DRY-RUN (default mode): render the report + gate preview + hash,
		// write nothing, regardless of any other flag.
		renderSyncDryRun(out, report, diffHash, gateReport, gateErr)
		if *asJSON {
			return printJSON(newSyncSelfResult(false, report, diffHash, gateReport, gateErr, nil))
		}
		return nil
	}

	// --- CONFIRM mode: everything below can write to disk. ---
	if *today == "" {
		return fmt.Errorf("sync-self: --today is required with --confirm-hash (YYYY-MM-DD)")
	}
	if len(report.Entries) == 0 {
		return fmt.Errorf("sync-self: nothing to sync — internal/selfspec.Requirements already matches %s", relPathForDisplay(gp))
	}
	if *confirmHash != diffHash {
		return fmt.Errorf(
			"sync-self: diff changed since PRESENT; re-run --dry-run — supplied --confirm-hash %s does not match the freshly recomputed diff-hash %s (graph.json on disk changed, or the registry changed, since the dry-run that produced the supplied hash)",
			*confirmHash, diffHash)
	}
	if gateErr != nil {
		return fmt.Errorf("sync-self: refusing to write: %w", gateErr)
	}

	return runSyncSelfWrite(domainDir, gp, before, after, report, diffHash, syncToday, *reason, ackOpts, out, *asJSON)
}

// syncOut mirrors landOut: operational sync-self messages go to stderr when
// --json is set (so stdout carries exactly one JSON document), stdout
// otherwise.
func syncOut(asJSON bool) *os.File {
	if asJSON {
		return os.Stderr
	}
	return os.Stdout
}

// checkSelfspecBinaryFresh is gate 1 (the stale-binary guard): it compares
// every file embedded in selfspec.SourceFiles against the SAME file read
// fresh off disk from the ENGINE repository's own internal/selfspec/
// directory, byte for byte. domainDir is expected to be the self-hosting
// domain (<repoRoot>/domains/hotam-spec-self by construction — see
// repoRootForDomain's tier-1 rule), so repoRootForDomain(domainDir) resolves
// the engine repo root regardless of whether this process's CWD happens to be
// the repo root (it is not, in general — a `hotam` binary can be invoked from
// anywhere).
func checkSelfspecBinaryFresh(domainDir string) error {
	repoRoot := repoRootForDomain(domainDir)
	selfspecDir := filepath.Join(repoRoot, "internal", "selfspec")

	entries, err := selfspec.SourceFiles.ReadDir(".")
	if err != nil {
		return fmt.Errorf("sync-self: read embedded selfspec source list: %w", err)
	}
	var stale []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		embedded, err := selfspec.SourceFiles.ReadFile(name)
		if err != nil {
			return fmt.Errorf("sync-self: read embedded %s: %w", name, err)
		}
		diskPath := filepath.Join(selfspecDir, name)
		disk, err := os.ReadFile(diskPath)
		if err != nil {
			// A file the binary was built with that is no longer on disk (or
			// unreadable) is itself staleness — surfaced the same way as a
			// content mismatch, not swallowed.
			stale = append(stale, name+" (disk read failed: "+err.Error()+")")
			continue
		}
		if string(embedded) != string(disk) {
			stale = append(stale, name)
		}
	}
	if len(stale) > 0 {
		sort.Strings(stale)
		return fmt.Errorf(
			"sync-self: stale binary; run via `go run ./cmd/hotam sync-self` or rebuild — this binary's embedded internal/selfspec source differs from %s for: %s",
			selfspecDir, strings.Join(stale, ", "))
	}
	return nil
}

// computeSyncDiffHash is gate 5 (the "hash of the diff"): sha256 over the
// canonical JSON of {base_graph_sha256, diff}, where base_graph_sha256 is the
// sha256 of the CURRENT on-disk graph.json (the same algorithm
// internal/loader/lock.go's sha256File uses) and diff is the *SyncReport
// as-is — full, un-truncated FieldDiff.Old/New values, entries sorted by ID
// for determinism (SyncGraph already iterates registeredIDsSorted(), so
// report.Entries is already ID-sorted on entry; this function sorts a COPY
// defensively so the hash's own contract does not silently depend on that
// upstream ordering never changing).
func computeSyncDiffHash(graphPath string, report *selfspec.SyncReport) (string, error) {
	baseHash, err := sha256HexFile(graphPath)
	if err != nil {
		return "", fmt.Errorf("hash base graph %s: %w", graphPath, err)
	}

	entries := append([]selfspec.SyncReportEntry(nil), report.Entries...)
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })

	payload := struct {
		BaseGraphSHA256 string              `json:"base_graph_sha256"`
		Diff            selfspec.SyncReport `json:"diff"`
	}{
		BaseGraphSHA256: baseHash,
		Diff:            selfspec.SyncReport{Entries: entries},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal diff-hash payload: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// sha256HexFile hashes the bytes at path with sha256, hex-encoded — the same
// algorithm internal/loader/lock.go's unexported sha256File uses, duplicated
// here (that helper is unexported and loader has no reason to expose it) so
// computeSyncDiffHash's base_graph_sha256 is computed identically to what a
// future graph.lock re-verification would compute.
func sha256HexFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// syncGateReport carries the outcome of gates 7 (confront), 8 (pre/post
// violation diff), and 9 (append-only), so both the dry-run preview render
// and the confirm-mode real gate check share one code path
// (runSyncGates) and therefore can never disagree about what a real run
// would do.
type syncGateReport struct {
	ConfrontBlockers  []confrontBlockerDigest `json:"confront_blockers,omitempty"`
	ConfrontAcked     bool                    `json:"confront_acked"`
	NewViolations     []invariants.Violation  `json:"new_violations,omitempty"`
	AppendOnlyChecked bool                    `json:"append_only_checked"`
}

// confrontBlockerDigest is the JSON/render-friendly shape of one blocking
// diagnose.ConfrontHit found for one SyncReportEntry.
type confrontBlockerDigest struct {
	EntryID        string   `json:"entry_id"`
	HitID          string   `json:"hit_id"`
	HitClaim       string   `json:"hit_claim"`
	OppositeMarker string   `json:"opposite_marker"`
	Shared         []string `json:"shared"`
}

// runSyncGates runs gates 7-9 against the (before, after) graph pair and
// returns a syncGateReport describing exactly what was found, plus a non-nil
// error IFF a real run would refuse at this point (an un-acked confront
// blocker, a new violation, or an append-only breach). It is called from
// BOTH the dry-run path (as a preview — its error is shown but never fatal to
// the dry-run itself) and the confirm path (where a non-nil error DOES abort
// before any write).
//
// GATE 7 — confront: for every ADDED entry, and every CHANGED entry whose
// FieldDiffs include Field=="Claim", diagnose.Confront(after, claim) is run
// and blocking hits (diagnose.IsBlockingHit) are collected — EXCLUDING, for a
// CHANGED entry, any hit whose ID equals the entry's own ID (a requirement's
// own claim changing must never self-block against its own prior claim).
// Blockers require an ack (--ack-conflict / --decision-ref, landAckOptions)
// to proceed, mirroring semanticConflictGate's override contract.
//
// GATE 8 — pre/post violation diff: invariants.AllViolationsForProposalGate
// is computed on `before` (untouched) and on `after` (post-SyncGraph-mutation)
// and any violation present in the latter but not the former blocks.
//
// GATE 9 — append-only: selfspec.VerifyAppendOnly(before, after) — the very
// last check, so it only needs to run once gates 7/8 already found nothing
// blocking (an expensive-to-explain append-only violation is a LAST resort
// diagnostic, not the first thing an operator should see).
func runSyncGates(domainDir string, before, after *ontology.Graph, report *selfspec.SyncReport, ackOpts landAckOptions) (*syncGateReport, error) {
	gr := &syncGateReport{}

	// Gate 7: confront.
	var blockers []confrontBlockerDigest
	for _, entry := range report.Entries {
		claim, ok := claimForConfront(entry)
		if !ok {
			continue
		}
		result := diagnose.Confront(after, claim)
		for _, h := range result.Settled {
			if !diagnose.IsBlockingHit(h) {
				continue
			}
			if entry.Kind == selfspec.SyncKindChanged && h.ID == entry.ID {
				// A requirement's own claim change must never self-block
				// against its own (now-stale) prior claim.
				continue
			}
			blockers = append(blockers, confrontBlockerDigest{
				EntryID:        entry.ID,
				HitID:          h.ID,
				HitClaim:       h.Claim,
				OppositeMarker: h.OppositeMarker,
				Shared:         h.Shared,
			})
		}
	}
	gr.ConfrontBlockers = blockers

	if len(blockers) > 0 {
		if err := validateAckConflict(domainDir, ackOpts); err != nil {
			return gr, err
		}
		if !ackOpts.hasAck() {
			return gr, formatConfrontBlockersError(blockers)
		}
		gr.ConfrontAcked = true
	}

	// Gate 8: pre/post violation diff.
	beforeViolations := indexSyncViolations(invariants.AllViolationsForProposalGate(before))
	afterViolations := invariants.AllViolationsForProposalGate(after)
	newViolations := newSyncViolationsSince(beforeViolations, afterViolations)
	gr.NewViolations = newViolations
	if len(newViolations) > 0 {
		return gr, fmt.Errorf("sync-self: SyncGraph would introduce %d new invariant violation(s):\n%s",
			len(newViolations), formatSyncViolations(newViolations))
	}

	// Gate 9: append-only (last resort — only reached once 7/8 are clean).
	gr.AppendOnlyChecked = true
	if err := selfspec.VerifyAppendOnly(before, after); err != nil {
		return gr, fmt.Errorf("sync-self: append-only guard: %w", err)
	}

	return gr, nil
}

// claimForConfront returns the claim text gate 7 should confront for entry,
// and whether entry is in scope for the confront gate at all: an ADDED entry
// always is (its Claim lives on the freshly created node, not in FieldDiffs —
// SyncGraph's own doc comment: FieldDiffs is always empty for ADDED), and a
// CHANGED entry is in scope only when one of its FieldDiffs names Field ==
// "Claim" (the task brief's exact trigger condition).
func claimForConfront(entry selfspec.SyncReportEntry) (string, bool) {
	switch entry.Kind {
	case selfspec.SyncKindAdded:
		reg, ok := selfspec.Requirements.Get(entry.ID)
		if !ok {
			return "", false
		}
		return reg.Claim, true
	case selfspec.SyncKindChanged:
		for _, d := range entry.FieldDiffs {
			if d.Field == "Claim" {
				if s, ok := d.New.(string); ok {
					return s, true
				}
			}
		}
		return "", false
	default:
		return "", false
	}
}

// validateAckConflict mirrors semanticConflictGate's own --ack-conflict
// existence check: a supplied --ack-conflict must name a real Conflict node
// in the domain graph, checked BEFORE it is trusted to override the gate.
func validateAckConflict(domainDir string, ackOpts landAckOptions) error {
	if ackOpts.AckConflict == "" {
		return nil
	}
	g, err := loadDomainGraph(domainDir)
	if err != nil {
		return fmt.Errorf("sync-self: validate --ack-conflict: %w", err)
	}
	for _, c := range g.Conflicts {
		if c.ID == ackOpts.AckConflict {
			return nil
		}
	}
	return fmt.Errorf("sync-self: --ack-conflict %q does not match any Conflict node in the graph", ackOpts.AckConflict)
}

// formatConfrontBlockersError renders the same "name the specific conflicting
// anchors + suggest remediation" shape semanticConflictGate's own refusal
// uses, adapted for potentially MULTIPLE SyncReportEntry sources instead of
// one ProposedRequirement.
func formatConfrontBlockersError(blockers []confrontBlockerDigest) error {
	var b strings.Builder
	fmt.Fprintf(&b, "refusing to sync: %d requirement(s) semantically contradict SETTLED requirement(s) (opposite-marker signal):\n", len(blockers))
	for _, h := range blockers {
		fmt.Fprintf(&b, "  - %s vs %s: %q\n     opposite markers: %s; shared tokens: [%s]\n",
			h.EntryID, h.HitID, h.HitClaim, h.OppositeMarker, strings.Join(h.Shared, ", "))
	}
	b.WriteString("a human decision must be recorded before this can sync. Use one of:\n")
	b.WriteString("  --ack-conflict <C-id>       cite an existing Conflict node whose members cover this tension\n")
	b.WriteString("  --decision-ref <text>       record a free-text reference to where the decision was made\n")
	b.WriteString("\nThis gate does not decide correctness — it requires that a decision be RECORDED first ")
	b.WriteString("(R-ai-presents-not-decides, R-decided-needs-human-signoff).")
	return fmt.Errorf("%s", b.String())
}

func indexSyncViolations(vs []invariants.Violation) map[string]struct{} {
	out := make(map[string]struct{}, len(vs))
	for _, v := range vs {
		out[v.Check+"\x00"+v.ID] = struct{}{}
	}
	return out
}

func newSyncViolationsSince(before map[string]struct{}, after []invariants.Violation) []invariants.Violation {
	var fresh []invariants.Violation
	for _, v := range after {
		if _, ok := before[v.Check+"\x00"+v.ID]; !ok {
			fresh = append(fresh, v)
		}
	}
	return fresh
}

func formatSyncViolations(vs []invariants.Violation) string {
	var b strings.Builder
	for _, v := range vs {
		fmt.Fprintf(&b, "  - %s %s: %s\n", v.Check, v.ID, v.Message)
	}
	return b.String()
}

// runSyncSelfWrite is the confirm-mode write path (task brief's point 11),
// reached only after every gate has already passed. It reuses land.go's
// exact transactional snapshot/rollback machinery (snapshotGraphFiles /
// rollbackLand) so a failure at any step after the write restores the domain
// to its pre-sync state, rather than inventing a parallel transaction
// mechanism.
func runSyncSelfWrite(domainDir, gp string, before, after *ontology.Graph, report *selfspec.SyncReport, diffHash, today, reason string, ackOpts landAckOptions, out *os.File, asJSON bool) error {
	claudeMDPath := resolveClaudeMDPath(domainDir, "")
	snapshot, err := snapshotGraphFiles(domainDir)
	if err != nil {
		return fmt.Errorf("sync-self: pre-write snapshot failed, nothing synced: %w", err)
	}

	added, changed := countSyncKinds(report)
	shortHash := diffHash
	if len(shortHash) > 12 {
		shortHash = shortHash[:12]
	}
	note := fmt.Sprintf("sync-self: %d changed, %d added; diff-hash %s", changed, added, shortHash)
	if reason != "" {
		note += "; reason: " + reason
	}
	if err := loader.WriteGraph(gp, after); err != nil {
		return fmt.Errorf("sync-self: write graph: %w", err)
	}
	if err := loader.WriteLock(gp, note); err != nil {
		rerr := rollbackLand(domainDir, snapshot, claudeMDPath, today)
		return rolledBackError("write lock failed", err, rerr)
	}
	fmt.Fprintf(out, "synced %d changed, %d added requirement(s) into %s\n", changed, added, relPathForDisplay(gp))

	// Persist the confront-ack audit trail (--ack-conflict / --decision-ref)
	// on every entry whose confront gate actually fired, mirroring
	// appendAckHistory's placement: AFTER the write, BEFORE regen, so the
	// History entry appears in the freshly rendered docs.
	if ackOpts.hasAck() {
		if err := appendSyncAckHistory(gp, report, today, ackOpts); err != nil {
			rerr := rollbackLand(domainDir, snapshot, claudeMDPath, today)
			return rolledBackError("ack history append failed", err, rerr)
		}
	}

	written, _, err := genSpec(domainDir, claudeMDPath, today, "", false)
	if err != nil {
		rerr := rollbackLand(domainDir, snapshot, claudeMDPath, today)
		return rolledBackError("doc regeneration failed", err, rerr)
	}
	fmt.Fprintf(out, "regenerated %d doc(s)\n", len(written))

	violations, err := allViolations(domainDir)
	if err != nil {
		rerr := rollbackLand(domainDir, snapshot, claudeMDPath, today)
		return rolledBackError("violation check failed to run", err, rerr)
	}
	if len(violations) > 0 {
		for _, v := range violations {
			fmt.Fprintf(out, "[%s] %s: %s\n", v.Check, v.ID, v.Message)
		}
		cause := fmt.Errorf("%d invariant violation(s) found after gen-spec (sync already validated the graph before writing it — this signals drift introduced by gen-spec or a concurrent change, not the sync itself)", len(violations))
		rerr := rollbackLand(domainDir, snapshot, claudeMDPath, today)
		return rolledBackError("graph invalid after gen-spec", cause, rerr)
	}

	fmt.Fprintln(out, "sync-self landed: graph synced, docs regenerated, 0 violations")
	if asJSON {
		return printJSON(newSyncSelfResult(true, report, diffHash, nil, nil, violations))
	}
	return nil
}

// appendSyncAckHistory persists a --ack-conflict / --decision-ref audit
// HistoryEntry on every SyncReportEntry that gate 7 actually flagged as a
// confront blocker, mirroring appendAckHistory's shape/placement for `hotam
// land` but fanning out over potentially several affected requirements
// instead of exactly one ProposedRequirement.
func appendSyncAckHistory(graphPath string, report *selfspec.SyncReport, today string, ackOpts landAckOptions) error {
	flagged := map[string]bool{}
	for _, entry := range report.Entries {
		claim, ok := claimForConfront(entry)
		if !ok || claim == "" {
			continue
		}
		flagged[entry.ID] = true
	}
	if len(flagged) == 0 {
		return nil
	}

	var summary string
	switch {
	case ackOpts.AckConflict != "" && ackOpts.DecisionRef != "":
		summary = fmt.Sprintf("semantic conflict acknowledged via Conflict %s; decision ref: %s", ackOpts.AckConflict, ackOpts.DecisionRef)
	case ackOpts.AckConflict != "":
		summary = fmt.Sprintf("semantic conflict acknowledged via Conflict %s", ackOpts.AckConflict)
	default:
		summary = fmt.Sprintf("semantic conflict acknowledged — human decision recorded: %s", ackOpts.DecisionRef)
	}

	g, err := loader.LoadGraph(graphPath)
	if err != nil {
		return fmt.Errorf("load graph for sync ack history: %w", err)
	}
	touched := false
	for i, r := range g.Requirements {
		if !flagged[r.ID] {
			continue
		}
		g.Requirements[i].History = append(g.Requirements[i].History, ontology.HistoryEntry{
			At:      today,
			Summary: summary,
		})
		touched = true
	}
	if !touched {
		return nil
	}
	if err := loader.WriteGraph(graphPath, g); err != nil {
		return fmt.Errorf("write graph for sync ack history: %w", err)
	}
	return nil
}

func countSyncKinds(report *selfspec.SyncReport) (added, changed int) {
	for _, e := range report.Entries {
		switch e.Kind {
		case selfspec.SyncKindAdded:
			added++
		case selfspec.SyncKindChanged:
			changed++
		}
	}
	return added, changed
}

// renderSyncDryRun writes the human-readable PRESENT artifact (task brief's
// point 4): one block per SyncReportEntry (ID, Kind, and for CHANGED the
// FieldDiffs abbreviated to ~150 runes for readability — display-only
// truncation, never applied to the bytes computeSyncDiffHash hashes), a gate
// preview section (7-9), and the trailing "diff-hash: <hex>" line.
func renderSyncDryRun(out *os.File, report *selfspec.SyncReport, diffHash string, gateReport *syncGateReport, gateErr error) {
	fmt.Fprintln(out, "hotam sync-self — DRY RUN (default mode; pass --confirm-hash <hex> to write)")
	fmt.Fprintln(out)
	if len(report.Entries) == 0 {
		fmt.Fprintln(out, "no differences: internal/selfspec.Requirements already matches the on-disk graph")
	}
	for _, entry := range report.Entries {
		fmt.Fprintf(out, "[%s] %s\n", entry.Kind, entry.ID)
		for _, d := range entry.FieldDiffs {
			fmt.Fprintf(out, "    field %s: %s -> %s\n", d.Field, abbrevSyncValue(d.Old), abbrevSyncValue(d.New))
		}
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, "gate preview (7-9 — what a real --confirm-hash run would enforce):")
	if len(gateReport.ConfrontBlockers) == 0 {
		fmt.Fprintln(out, "  [7 confront] clear — no blocking semantic-conflict hits")
	} else {
		for _, h := range gateReport.ConfrontBlockers {
			fmt.Fprintf(out, "  [7 confront] BLOCKED: %s vs %s: opposite markers: %s; shared: [%s]\n",
				h.EntryID, h.HitID, h.OppositeMarker, strings.Join(h.Shared, ", "))
		}
	}
	if len(gateReport.NewViolations) == 0 {
		fmt.Fprintln(out, "  [8 pre/post-violations] clear — no new invariant violations")
	} else {
		for _, v := range gateReport.NewViolations {
			fmt.Fprintf(out, "  [8 pre/post-violations] BLOCKED: [%s] %s: %s\n", v.Check, v.ID, v.Message)
		}
	}
	if gateErr != nil && len(gateReport.ConfrontBlockers) == 0 && len(gateReport.NewViolations) == 0 {
		fmt.Fprintf(out, "  [9 append-only] BLOCKED: %v\n", gateErr)
	} else {
		fmt.Fprintln(out, "  [9 append-only] clear")
	}
	fmt.Fprintln(out)
	fmt.Fprintf(out, "diff-hash: %s\n", diffHash)
}

// abbrevSyncValue renders a FieldDiff.Old/New value (any) for the DISPLAY-ONLY
// dry-run render, truncated to ~150 runes with a trailing ellipsis — never
// used for computeSyncDiffHash, which always hashes the full untruncated
// SyncReport.
func abbrevSyncValue(v any) string {
	s := fmt.Sprintf("%v", v)
	fields := strings.Join(strings.Fields(s), " ")
	runes := []rune(fields)
	const limit = 150
	if len(runes) > limit {
		return string(runes[:limit-1]) + "…"
	}
	return fields
}

// syncSelfResult is the --json envelope for `hotam sync-self`, covering both
// the dry-run and confirm-mode outcomes.
type syncSelfResult struct {
	Landed     bool                       `json:"landed"`
	DiffHash   string                     `json:"diff_hash"`
	Added      int                        `json:"added"`
	Changed    int                        `json:"changed"`
	Entries    []selfspec.SyncReportEntry `json:"entries"`
	GateReport *syncGateReport            `json:"gate_report,omitempty"`
	GateError  string                     `json:"gate_error,omitempty"`
	Violations []invariants.Violation     `json:"violations,omitempty"`
}

func newSyncSelfResult(landed bool, report *selfspec.SyncReport, diffHash string, gateReport *syncGateReport, gateErr error, violations []invariants.Violation) *syncSelfResult {
	added, changed := countSyncKinds(report)
	res := &syncSelfResult{
		Landed:     landed,
		DiffHash:   diffHash,
		Added:      added,
		Changed:    changed,
		Entries:    report.Entries,
		GateReport: gateReport,
		Violations: violations,
	}
	if gateErr != nil {
		res.GateError = gateErr.Error()
	}
	return res
}
