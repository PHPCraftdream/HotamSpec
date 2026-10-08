package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/PHPCraftdream/HotamSpec/internal/diagnose"
	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/invariants"
	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/registry"
	"github.com/PHPCraftdream/HotamSpec/internal/selfspec"
)

// cmdSyncDomain implements `hotam sync-domain --domain <path>` (task #366,
// RAC2 Phase B: "Go-code-only authority" for a CONSUMER domain's own
// Requirements, generalizing `hotam sync-self`/cmd/hotam/sync_self.go from
// the engine's self-hosting registry to any domain that has adopted the
// spec/requirements.go + spec/hotamontology Go-authoring path (R-domain-
// founded-in-wave-order step 6's "middle path" in the root CLAUDE.md).
//
// sync-domain's shape is deliberately modeled 1-in-1 on sync-self: same
// dry-run-by-default / --confirm-hash handshake, same gate order (7 confront
// -> 8 pre/post-violation-diff -> 9 append-only), same transactional
// snapshot/rollback machinery (land.go's snapshotGraphFiles/rollbackLand).
// It differs in exactly ONE structural way sync-self does not need: instead
// of reading a package-global Go registry the ENGINE binary was compiled
// with (selfspec.Requirements), sync-domain must reach ACROSS a Go module
// boundary into the domain's own separate spec/ module (per NEW-2-bis, this
// engine never wires a cross-module `replace` to bridge that gap directly).
// It does this the same way internal/gate/test_exec.go already bridges the
// identical module-boundary problem to execute a domain's verified_by
// tests: subprocess-exec `go run ./registrydump` INSIDE the domain's own
// spec/ module (see runRegistryDump below), capture its stdout as JSON,
// unmarshal into []ontology.Requirement, and build an in-process
// *registry.Registry[ontology.Requirement] from that -- which is then
// handed to the SAME (now-parameterized, see internal/selfspec/merge.go
// merge.go+sync.go) selfspec.MergeIntoGraph/SyncGraph sync-self itself
// calls, so the two commands' core sync mechanics can never drift apart.
//
// Deliberately NOT ported from sync-self: the stale-binary guard
// (selfspec.SourceFiles/SourceFileFor, gate 1 of sync-self). That guard
// exists ONLY because go:embed freezes internal/selfspec's own Go source
// into the compiled `hotam` binary at BUILD time, so a stale prebuilt binary
// can silently run against an outdated in-memory registry -- see
// selfspec/embed.go's own doc comment. sync-domain has no such freeze point:
// every invocation re-executes `go run ./registrydump` fresh (a real,
// uncached subprocess build+run, exactly like `go run ./cmd/hotam sync-self`
// itself is always fresh), so the entire "stale embedded binary" failure
// class this guard exists to catch is structurally impossible for this
// command -- there is nothing frozen at hotam's own build time for it to
// compare against.
//
// Gate order before any write (mirroring sync-self's own, minus the
// self-hosting-only stale-binary gate 1): registrydump(0) -> confront(7) ->
// pre/post-violations(8) -> append-only(9).
func cmdSyncDomain(args []string) error {
	fs := newFlagSet("sync-domain")
	domain := fs.String("domain", "", "consumer domain directory (default: "+defaultDomainRel+")")
	today := fs.String("today", "", "date in YYYY-MM-DD format (required in --confirm-hash mode)")
	confirmHash := fs.String("confirm-hash", "", "the diff-hash printed by a prior --dry-run; supplying it switches sync-domain from dry-run into the real, disk-writing mode")
	reason := fs.String("reason", "", "free-text reason recorded in the graph.lock note alongside the sync summary (optional)")
	ackConflict := fs.String("ack-conflict", "", "cite an existing Conflict node (C-...) whose members cover a confront-gate hit — overrides the confront-gate refusal")
	decisionRef := fs.String("decision-ref", "", "free-text reference to where a human decision was recorded — overrides the confront-gate refusal and is persisted as a HistoryEntry on each affected requirement")
	asJSON := fs.Bool("json", false, "emit machine-readable JSON")
	if err := parseCommandFlags(fs, args); err != nil {
		return err
	}

	domainDir, err := resolveDomain(*domain)
	if err != nil {
		return err
	}

	// --- Gate 0: registry dump. Everything downstream needs the domain's
	// current Go-authored Requirement set, so a subprocess failure here is
	// surfaced as an immediate, specific error -- never as a silently empty
	// registry that would make SyncGraph look like "nothing to sync" (see
	// runRegistryDump's own doc comment for the module-boundary mechanics).
	reg, dumpedStakeholders, err := domainDumpFromSubprocess(domainDir)
	if err != nil {
		return fmt.Errorf("sync-domain: %w", err)
	}

	gp := graphPathForDomain(domainDir)
	loadGraph := loader.LoadGraph
	manifest, manifestErr := loader.LoadManifest(filepath.Join(domainDir, "manifest.json"))
	if manifestErr == nil && (manifest.SelfHosting || manifest.RequirementsAuthority == loader.RequirementsAuthorityCode) {
		loadGraph = loader.LoadGraphForCodeProjection
	}

	syncToday := *today
	if syncToday == "" {
		syncToday = time.Now().Format("2006-01-02")
	}
	before, err := loadGraph(gp)
	if err != nil {
		return fmt.Errorf("sync-domain: load pre-sync graph: %w", err)
	}
	after, err := loadGraph(gp)
	if err != nil {
		return fmt.Errorf("sync-domain: load working graph: %w", err)
	}
	if before.SelfExecutingAtoms {
		reg, err = selfspec.DiscoverAtoms(gate.SpecRootForGraph(before), before.DomainDir, reg)
		if err != nil {
			return fmt.Errorf("sync-domain: discover atoms: %w", err)
		}
		for _, previous := range before.Requirements {
			if previous.Status == "REJECTED" {
				continue
			}
			if _, exists := reg.Get(previous.ID); !exists {
				return fmt.Errorf("sync-domain: requirement %s no longer has an executed atom; declare explicit REJECTED/replaces metadata for method removal or rename", previous.ID)
			}
		}
	}

	// --- Claim derivation (task #369, RAC3-A): for a discipline:"full"
	// domain, every in-scope Requirement's Claim (RequirementInClaimDerivationScope
	// -- not INHERENTLY_PROSE, carries verified_by entries) is REPLACED, in
	// reg, with the concatenation of its verified_by test(s)' recorded
	// hotamspec scenario description(s), BEFORE SyncGraph ever sees the
	// registry -- so a derived Claim flows through StructuralFieldDiffs/
	// SyncGraph/gate 7 (confront) exactly like any other authored structural
	// field, with zero special-casing downstream. A domain that has not
	// opted into discipline:"full" (every consumer domain in this wave)
	// sees this call do nothing at all (DeriveClaimsFromScenarios' own
	// top-of-function honest no-op) -- sync-domain's cost/behavior for such
	// a domain is completely unchanged by this task.
	//
	// Because the derivation overwrites reg IN PLACE, the registry's own
	// hand-authored Claim is snapshotted first: for every requirement whose
	// authored Claim is non-empty and differs from what was derived, an
	// output-only NOTE is emitted (dry-run AND confirm mode) telling the
	// author their text was ignored -- the note never feeds SyncGraph, the
	// gates, or the diff-hash (task: output-only author guidance).
	registryClaims := map[string]string{}
	for _, r := range reg.All() {
		registryClaims[r.ID] = r.Claim
	}
	specRoot := gate.SpecRootForGraph(before)
	derivedIDs := selfspec.DeriveClaimsFromScenarios(reg, specRoot, before.SelfHosting, before.Discipline)
	var claimNotes []string
	if before.Discipline == loader.DisciplineFull {
		sort.Strings(derivedIDs)
		for _, id := range derivedIDs {
			if registryClaims[id] == "" {
				continue
			}
			claimNotes = append(claimNotes, fmt.Sprintf("NOTE %s: registry Claim ignored — under discipline:\"full\" the claim is derived from the scenario title; leave Claim empty in spec/requirements.go", id))
		}
	}

	// Stakeholders first, so requirement owners added below resolve (gate 8).
	addedStk := appendDomainStakeholders(after, dumpedStakeholders)
	report, err := selfspec.SyncGraph(after, reg, syncToday)
	if err != nil {
		return fmt.Errorf("sync-domain: SyncGraph: %w", err)
	}
	if err := loader.ValidateGraph(after); err != nil {
		return fmt.Errorf("sync-domain: validate projected graph: %w", err)
	}

	diffHash, err := computeDomainSyncDiffHash(gp, report, addedStk)
	if err != nil {
		return fmt.Errorf("sync-domain: compute diff hash: %w", err)
	}

	ackOpts := landAckOptions{AckConflict: *ackConflict, DecisionRef: *decisionRef}
	gateReport, gateErr := runDomainSyncGates(domainDir, reg, before, after, report, ackOpts)

	out := syncOut(*asJSON)

	if *confirmHash == "" {
		renderDomainSyncDryRun(out, report, addedStk, diffHash, gateReport, gateErr, claimNotes)
		if *asJSON {
			return printJSON(newSyncDomainResult(false, report, diffHash, gateReport, gateErr, nil, claimNotes).withStakeholders(addedStk))
		}
		return nil
	}

	if *today == "" {
		return fmt.Errorf("sync-domain: --today is required with --confirm-hash (YYYY-MM-DD)")
	}
	if len(report.Entries) == 0 && len(addedStk) == 0 {
		return fmt.Errorf("sync-domain: nothing to sync — the domain's spec/requirements.go registry already matches %s", relPathForDisplay(gp))
	}
	if *confirmHash != diffHash {
		return fmt.Errorf(
			"sync-domain: diff changed since PRESENT; re-run --dry-run — supplied --confirm-hash %s does not match the freshly recomputed diff-hash %s (graph.json on disk changed, or the registry changed, since the dry-run that produced the supplied hash)",
			*confirmHash, diffHash)
	}
	if gateErr != nil {
		return fmt.Errorf("sync-domain: refusing to write: %w", gateErr)
	}

	return runSyncDomainWrite(domainDir, gp, before, after, report, addedStk, diffHash, syncToday, *reason, ackOpts, out, *asJSON, claimNotes)
}

// --- Gate 0: registry dump (the structural difference from sync-self) ---

// domainRegistrySubprocessTimeout bounds how long `go run ./registrydump` is
// given to compile and execute inside the domain's own spec/ module — a real
// (uncached, per NEW-2-bis's module-boundary constraint) `go run` compile,
// so this is deliberately generous, mirroring internal/gate's own
// defaultTestExecTimeout sizing rationale (test_exec.go) for the identical
// "loaded host, real go compile" concern.
const domainRegistrySubprocessTimeout = 180 * time.Second

// domainRegistryFromSubprocess is gate 0: it locates <domainDir>/spec/
// (the domain's own separate Go module carrying registrydump/main.go,
// scaffolded by `hotam scaffold-registrydump`), runs `go run ./registrydump`
// inside it, decodes its stdout as a JSON []ontology.Requirement array, and
// builds a fresh in-process *registry.Registry[ontology.Requirement] from
// it — the exact registry shape internal/selfspec.MergeIntoGraph/SyncGraph
// already accept as of their task #366 parameterization.
//
// Every failure mode (missing spec/ dir, missing registrydump, a subprocess
// that exits non-zero, or output that is not valid JSON) is surfaced as a
// SPECIFIC, actionable error naming what went wrong and where — never
// silently swallowed into an empty registry, which would make SyncGraph
// report "0 entries to sync" indistinguishable from "the registry genuinely
// already matches the graph" (the task brief's explicit anti-goal).
func domainRegistryFromSubprocess(domainDir string) (*registry.Registry[ontology.Requirement], error) {
	reg, _, err := domainDumpFromSubprocess(domainDir)
	return reg, err
}

// domainDumpFromSubprocess is domainRegistryFromSubprocess plus the
// Stakeholders registry. registrydump stdout is either the legacy bare
// Requirement array (a domain without Stakeholders, or an old scaffold) or
// the envelope {"requirements":[...],"stakeholders":[...]} (see
// registrydumpEnvelopeTemplate); the first non-space byte tells them apart.
func domainDumpFromSubprocess(domainDir string) (*registry.Registry[ontology.Requirement], []ontology.Stakeholder, error) {
	specDir := filepath.Join(domainDir, "spec")
	if _, err := os.Stat(specDir); err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("registry dump: %s does not exist — this domain has not adopted the spec/requirements.go Go-authoring path yet (see R-domain-founded-in-wave-order step 6)", specDir)
		}
		return nil, nil, fmt.Errorf("registry dump: stat %s: %w", specDir, err)
	}
	registrydumpDir := filepath.Join(specDir, "registrydump")
	if _, err := os.Stat(registrydumpDir); err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("registry dump: %s does not exist — run `hotam scaffold-registrydump --domain %s` first", registrydumpDir, domainDir)
		}
		return nil, nil, fmt.Errorf("registry dump: stat %s: %w", registrydumpDir, err)
	}

	stdout, err := runRegistryDump(specDir)
	if err != nil {
		return nil, nil, err
	}

	var entries []ontology.Requirement
	var stakeholders []ontology.Stakeholder
	if trimmed := bytes.TrimSpace(stdout); len(trimmed) > 0 && trimmed[0] == '{' {
		var env struct {
			Requirements []ontology.Requirement `json:"requirements"`
			Stakeholders []ontology.Stakeholder `json:"stakeholders"`
		}
		if err := json.Unmarshal(trimmed, &env); err != nil {
			return nil, nil, fmt.Errorf("registry dump: %s produced output that is not a valid JSON {requirements, stakeholders} envelope: %w\noutput was:\n%s", registrydumpDir, err, boundedForError(stdout))
		}
		entries, stakeholders = env.Requirements, env.Stakeholders
	} else if err := json.Unmarshal(stdout, &entries); err != nil {
		return nil, nil, fmt.Errorf("registry dump: %s produced output that is not a valid JSON []Requirement array: %w\noutput was:\n%s", registrydumpDir, err, boundedForError(stdout))
	}
	seenStk := map[string]bool{}
	for _, s := range stakeholders {
		if s.ID == "" {
			return nil, nil, fmt.Errorf("registry dump: %s produced a stakeholder with an empty ID — refusing to sync it", registrydumpDir)
		}
		if seenStk[s.ID] {
			return nil, nil, fmt.Errorf("registry dump: %s produced a duplicate stakeholder ID %q — refusing to sync it", registrydumpDir, s.ID)
		}
		seenStk[s.ID] = true
	}

	reg := registry.New[ontology.Requirement]()
	for _, e := range entries {
		if e.ID == "" {
			return nil, nil, fmt.Errorf("registry dump: %s produced an entry with an empty ID — refusing to build a registry from it", registrydumpDir)
		}
		if _, dup := reg.Get(e.ID); dup {
			// registry.Registry.MustRegister PANICS on a duplicate name (see
			// internal/registry/registry.go) -- that is the right contract for
			// a package registering its OWN literals at init() time (a
			// programmer error caught immediately at startup), but this
			// registry is built from EXTERNAL, subprocess-produced JSON at
			// runtime, so a malformed dump must surface as an ordinary error
			// here, never as an unrecovered panic that crashes `hotam`
			// mid-command. A real registrydump built from a real
			// hotamontology.Registry can never itself emit a duplicate (that
			// registry's own MustRegister already panics inside the
			// subprocess at registration time) -- this guards against a
			// hand-broken or future non-Registry-backed registrydump program.
			return nil, nil, fmt.Errorf("registry dump: %s produced a duplicate requirement ID %q — refusing to build a registry from it", registrydumpDir, e.ID)
		}
		reg.MustRegister(e.ID, e)
	}
	return reg, stakeholders, nil
}

// appendDomainStakeholders appends to g every dumped stakeholder whose ID is
// absent from g.Stakeholders and returns them as added. Append-only: an
// existing graph stakeholder is never rewritten or removed, even if the code
// declaration differs.
func appendDomainStakeholders(g *ontology.Graph, dumped []ontology.Stakeholder) []ontology.Stakeholder {
	have := map[string]bool{}
	next := 0
	for _, s := range g.Stakeholders {
		have[s.ID] = true
		if s.DeclOrder >= next {
			next = s.DeclOrder + 1
		}
	}
	var added []ontology.Stakeholder
	for _, s := range dumped {
		if have[s.ID] {
			continue
		}
		s.DeclOrder = next
		next++
		g.Stakeholders = append(g.Stakeholders, s)
		added = append(added, s)
	}
	return added
}

// runRegistryDump spawns `go run ./registrydump` with cmd.Dir=specDir — the
// module-boundary bridge (the execution session's runner applies the
// identical "subprocess-exec inside the OTHER module's own directory"
// principle to execute a domain's verified_by tests; this is the same
// mechanism applied to a one-shot dump program instead of a test binary).
// Returns the subprocess's raw stdout on success (exit 0); any non-zero
// exit, timeout, or failure to even start `go` is a specific, named error
// carrying stderr/stdout so a broken registrydump program (a compile error
// in the domain's own spec/requirements.go, for instance) is immediately
// diagnosable rather than surfacing as an opaque "sync-domain failed".
func runRegistryDump(specDir string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), domainRegistrySubprocessTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "run", "./registrydump")
	cmd.Dir = specDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("registry dump: `go run ./registrydump` in %s timed out after %s", specDir, domainRegistrySubprocessTimeout)
	}
	if err != nil {
		return nil, fmt.Errorf("registry dump: `go run ./registrydump` in %s failed: %w\nstderr:\n%s\nstdout:\n%s",
			specDir, err, boundedForError(stderr.Bytes()), boundedForError(stdout.Bytes()))
	}
	return stdout.Bytes(), nil
}

const maxErrorOutputBytes = 4000

// boundedForError trims subprocess output embedded in an error message to a
// bounded size, mirroring internal/gate/test_exec.go's boundOutput (same
// rationale: an error string must never balloon to an unbounded blob, and
// the trailing lines are usually the most informative — a compiler error or
// a panic's final frame lives at the end).
func boundedForError(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) <= maxErrorOutputBytes {
		return s
	}
	return "...(truncated)...\n" + s[len(s)-maxErrorOutputBytes:]
}

// --- diff-hash (byte-identical shape to sync-self's computeSyncDiffHash) ---

// computeDomainSyncDiffHash is sync-domain's own "hash of the diff": sha256
// over the canonical JSON of {base_graph_sha256, diff}, the SAME shape and
// algorithm as sync-self's computeSyncDiffHash (cmd/hotam/sync_self.go) —
// duplicated here rather than shared because the two commands' diff-hash
// payloads are independent artifacts of independent commands, and sharing a
// helper across them would couple two otherwise-unrelated CLI surfaces for
// no behavioral benefit.
func computeDomainSyncDiffHash(graphPath string, report *selfspec.SyncReport, addedStk []ontology.Stakeholder) (string, error) {
	baseHash, err := sha256HexFile(graphPath)
	if err != nil {
		return "", fmt.Errorf("hash base graph %s: %w", graphPath, err)
	}

	entries := append([]selfspec.SyncReportEntry(nil), report.Entries...)
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })

	payload := struct {
		BaseGraphSHA256 string              `json:"base_graph_sha256"`
		Diff            selfspec.SyncReport `json:"diff"`
		// omitempty: a requirements-only sync hashes byte-identically to before.
		AddedStakeholders []ontology.Stakeholder `json:"added_stakeholders,omitempty"`
	}{
		BaseGraphSHA256:   baseHash,
		Diff:              selfspec.SyncReport{Entries: entries},
		AddedStakeholders: addedStk,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal diff-hash payload: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// --- gates 7-9 (byte-identical LOGIC to sync-self's runSyncGates, just
// parameterized over reg instead of reading selfspec.Requirements directly)
// ---

// domainSyncGateReport mirrors syncGateReport (sync_self.go) exactly — kept
// as sync-domain's own type (rather than reusing syncGateReport) so the two
// commands' JSON envelopes stay independently versionable, the same
// reasoning computeDomainSyncDiffHash's doc comment gives for not sharing
// that helper either.
type domainSyncGateReport struct {
	ConfrontBlockers  []confrontBlockerDigest `json:"confront_blockers,omitempty"`
	ConfrontAcked     bool                    `json:"confront_acked"`
	NewViolations     []invariants.Violation  `json:"new_violations,omitempty"`
	AppendOnlyChecked bool                    `json:"append_only_checked"`
}

// runDomainSyncGates runs gates 7-9 against the (before, after) graph pair,
// the same three gates and the same order as sync-self's runSyncGates
// (cmd/hotam/sync_self.go) — see that function's doc comment for the full
// per-gate rationale, unchanged here. The only difference is gate 7's claim
// lookup for an ADDED entry: sync-self's claimForConfront reads the
// package-global selfspec.Requirements directly; domainClaimForConfront
// (below) reads the reg parameter this function receives instead, so the
// exact same gate semantics apply to whichever registry this call was built
// from (subprocess-dumped, for sync-domain).
func runDomainSyncGates(domainDir string, reg *registry.Registry[ontology.Requirement], before, after *ontology.Graph, report *selfspec.SyncReport, ackOpts landAckOptions) (*domainSyncGateReport, error) {
	gr := &domainSyncGateReport{}

	// Gate 7: confront.
	var blockers []confrontBlockerDigest
	requirements := syncConfrontRequirements(after, report)
	for _, entry := range report.Entries {
		claim, ok := domainClaimForConfront(reg, entry)
		if !ok {
			continue
		}
		result := diagnose.ConfrontRequirement(after, syncConfrontCandidate(requirements, entry.ID, claim))
		for _, h := range result.FormalConflicts {
			if !diagnose.IsBlockingHit(h) {
				continue
			}
			if h.ID == entry.ID {
				continue
			}
			blockers = append(blockers, confrontBlockerDigest{
				EntryID:        entry.ID,
				HitID:          h.ID,
				HitClaim:       h.Claim,
				Classification: h.Classification,
				Confidence:     h.Confidence,
				Reasons:        h.Reasons,
				ConflictIDs:    h.ConflictIDs,
			})
		}
	}
	gr.ConfrontBlockers = blockers

	if len(blockers) > 0 {
		if err := validateAckConflict(domainDir, ackOpts); err != nil {
			return gr, err
		}
		if err := validateSyncConflictCoverage(ackOpts, blockers); err != nil {
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
		return gr, fmt.Errorf("sync-domain: SyncGraph would introduce %d new invariant violation(s):\n%s",
			len(newViolations), formatSyncViolations(newViolations))
	}

	// Gate 9: append-only (last resort — only reached once 7/8 are clean).
	gr.AppendOnlyChecked = true
	if err := selfspec.VerifyAppendOnly(before, after); err != nil {
		return gr, fmt.Errorf("sync-domain: append-only guard: %w", err)
	}

	return gr, nil
}

// domainClaimForConfront mirrors claimForConfront (sync_self.go) exactly,
// parameterized over reg instead of reading the package-global
// selfspec.Requirements — see that function's doc comment for the full
// ADDED-vs-CHANGED trigger-condition rationale, unchanged here.
func domainClaimForConfront(reg *registry.Registry[ontology.Requirement], entry selfspec.SyncReportEntry) (string, bool) {
	switch entry.Kind {
	case selfspec.SyncKindAdded:
		r, ok := reg.Get(entry.ID)
		if !ok {
			return "", false
		}
		return r.Claim, true
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

// --- confirm-mode write (reusing land.go's transactional machinery) ---

// runSyncDomainWrite is the confirm-mode write path, reached only after
// every gate has already passed — the domain-generalized counterpart of
// sync-self's runSyncSelfWrite (cmd/hotam/sync_self.go), reusing the exact
// same land.go transactional snapshot/rollback machinery
// (snapshotGraphFiles/rollbackLand) and the same SPEC.md-specific
// snapshot/restore pair (snapshotSpecMD/rollbackSyncSelf) that function
// already established — see runSyncSelfWrite's own doc comment for the full
// rationale on why includeSpec=true and why SPEC.md needs its own
// snapshot/restore alongside rollbackLand's graph-only restore. That
// rationale is domain-agnostic (it follows from genSpec/check_spec_md_current's
// own contracts, not from anything self-hosting-specific), so this function
// reuses rollbackSyncSelf directly rather than duplicating it under a new
// name.
func runSyncDomainWrite(domainDir, gp string, before, after *ontology.Graph, report *selfspec.SyncReport, addedStk []ontology.Stakeholder, diffHash, today, reason string, ackOpts landAckOptions, out *os.File, asJSON bool, claimNotes []string) error {
	claudeMDPath := resolveClaudeMDPath(domainDir, "")
	snapshot, err := snapshotGraphFiles(domainDir)
	if err != nil {
		return fmt.Errorf("sync-domain: pre-write snapshot failed, nothing synced: %w", err)
	}
	specSnapshot, specPresent, err := snapshotSpecMD(domainDir)
	if err != nil {
		return fmt.Errorf("sync-domain: pre-write SPEC.md snapshot failed, nothing synced: %w", err)
	}

	added, changed := countSyncKinds(report)
	shortHash := diffHash
	if len(shortHash) > 12 {
		shortHash = shortHash[:12]
	}
	note := fmt.Sprintf("sync-domain: %d changed, %d added; diff-hash %s", changed, added, shortHash)
	if len(addedStk) > 0 {
		note = fmt.Sprintf("sync-domain: %d changed, %d added, %d stakeholder(s) added; diff-hash %s", changed, added, len(addedStk), shortHash)
	}
	if reason != "" {
		note += "; reason: " + reason
	}
	if err := loader.WriteGraph(gp, after); err != nil {
		return fmt.Errorf("sync-domain: write graph: %w", err)
	}
	if err := loader.WriteLock(gp, note); err != nil {
		rerr := rollbackLand(domainDir, snapshot, claudeMDPath, today)
		return rolledBackError("write lock failed", err, rerr)
	}
	for _, n := range claimNotes {
		fmt.Fprintln(out, n)
	}
	fmt.Fprintf(out, "synced %d changed, %d added requirement(s), %d stakeholder(s) added into %s\n", changed, added, len(addedStk), relPathForDisplay(gp))

	if ackOpts.hasAck() {
		if err := appendDomainSyncAckHistory(gp, report, today, ackOpts); err != nil {
			rerr := rollbackLand(domainDir, snapshot, claudeMDPath, today)
			return rolledBackError("ack history append failed", err, rerr)
		}
	}

	written, _, err := genSpec(domainDir, claudeMDPath, today, "", true)
	if err != nil {
		rerr := rollbackSyncSelf(domainDir, snapshot, specSnapshot, specPresent, claudeMDPath, today)
		return rolledBackError("doc regeneration failed", err, rerr)
	}
	fmt.Fprintf(out, "regenerated %d doc(s)\n", len(written))

	violations, err := allViolationsAsOf(domainDir, today)
	if err != nil {
		rerr := rollbackSyncSelf(domainDir, snapshot, specSnapshot, specPresent, claudeMDPath, today)
		return rolledBackError("violation check failed to run", err, rerr)
	}
	if len(violations) > 0 {
		for _, v := range violations {
			fmt.Fprintf(out, "[%s] %s: %s\n", v.Check, v.ID, v.Message)
		}
		cause := fmt.Errorf("%d invariant violation(s) found after gen-spec (sync already validated the graph before writing it — this signals drift introduced by gen-spec or a concurrent change, not the sync itself)", len(violations))
		rerr := rollbackSyncSelf(domainDir, snapshot, specSnapshot, specPresent, claudeMDPath, today)
		return rolledBackError("graph invalid after gen-spec", cause, rerr)
	}

	fmt.Fprintln(out, "sync-domain landed: graph synced, docs regenerated, 0 violations")
	if asJSON {
		return printJSON(newSyncDomainResult(true, report, diffHash, nil, nil, violations, claimNotes).withStakeholders(addedStk))
	}
	return nil
}

// appendDomainSyncAckHistory mirrors appendSyncAckHistory (sync_self.go)
// exactly, parameterized over report only (it never needs the registry
// itself — the ack summary text is registry-independent).
func appendDomainSyncAckHistory(graphPath string, report *selfspec.SyncReport, today string, ackOpts landAckOptions) error {
	if len(report.Entries) == 0 {
		return nil
	}
	g, err := loader.LoadGraph(graphPath)
	if err != nil {
		return fmt.Errorf("load graph for sync ack history: %w", err)
	}
	flagged := syncConfrontFlaggedEntries(g, report)
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

// --- dry-run render (byte-identical SHAPE to sync-self's renderSyncDryRun)
// ---

func renderDomainSyncDryRun(out *os.File, report *selfspec.SyncReport, addedStk []ontology.Stakeholder, diffHash string, gateReport *domainSyncGateReport, gateErr error, claimNotes []string) {
	fmt.Fprintln(out, "hotam sync-domain — DRY RUN (default mode; pass --confirm-hash <hex> to write)")
	fmt.Fprintln(out)
	if len(report.Entries) == 0 && len(addedStk) == 0 {
		fmt.Fprintln(out, "no differences: the domain's spec/requirements.go registry already matches the on-disk graph")
	}
	for _, s := range addedStk {
		fmt.Fprintf(out, "[ADDED stakeholder] %s\n", s.ID)
		fmt.Fprintf(out, "    name: %s\n    domain: %s\n", abbrevSyncValue(s.Name), abbrevSyncValue(s.Domain))
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
		fmt.Fprintln(out, "  [7 confront] clear — no unresolved formal conflict carriers")
	} else {
		for _, h := range gateReport.ConfrontBlockers {
			fmt.Fprintf(out, "  [7 confront] BLOCKED: %s vs %s: conflicts: [%s]; reasons: [%s]\n",
				h.EntryID, h.HitID, strings.Join(h.ConflictIDs, ", "), strings.Join(h.Reasons, "; "))
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
	for _, n := range claimNotes {
		fmt.Fprintln(out, n)
	}
	fmt.Fprintln(out)
	fmt.Fprintf(out, "diff-hash: %s\n", diffHash)
}

// syncDomainResult is the --json envelope for `hotam sync-domain`, mirroring
// syncSelfResult's shape.
type syncDomainResult struct {
	Landed     bool                       `json:"landed"`
	DiffHash   string                     `json:"diff_hash"`
	Added      int                        `json:"added"`
	Changed    int                        `json:"changed"`
	Entries    []selfspec.SyncReportEntry `json:"entries"`
	GateReport *domainSyncGateReport      `json:"gate_report,omitempty"`
	GateError  string                     `json:"gate_error,omitempty"`
	Violations []invariants.Violation     `json:"violations,omitempty"`
	Notes      []string                   `json:"notes,omitempty"`

	AddedStakeholders []ontology.Stakeholder `json:"added_stakeholders,omitempty"`
}

func (r *syncDomainResult) withStakeholders(added []ontology.Stakeholder) *syncDomainResult {
	r.AddedStakeholders = added
	return r
}

func newSyncDomainResult(landed bool, report *selfspec.SyncReport, diffHash string, gateReport *domainSyncGateReport, gateErr error, violations []invariants.Violation, notes []string) *syncDomainResult {
	added, changed := countSyncKinds(report)
	res := &syncDomainResult{
		Landed:     landed,
		DiffHash:   diffHash,
		Added:      added,
		Changed:    changed,
		Entries:    report.Entries,
		GateReport: gateReport,
		Violations: violations,
		Notes:      notes,
	}
	if gateErr != nil {
		res.GateError = gateErr.Error()
	}
	return res
}
