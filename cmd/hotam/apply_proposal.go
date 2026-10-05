package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/proposal"
)

// cmdApplyProposal implements `hotam apply-proposal`: apply a single proposal
// (or a directory of them via --batch) to graph.json WITHOUT regenerating docs.
// It is the lower-level counterpart to `hotam land`, which wraps the same apply
// in a snapshot/gen-spec/all-violations pipeline.
//
// A matching unresolved Conflict carrier is the only gate blocker. The
// single-file path checks it before mutation and permits a recorded human
// decision; batch mode uses the same candidate-ID/member-ID check atomically,
// but has no per-item override. Lexical markers, shared subjects, source links,
// and relations remain advisory.
func cmdApplyProposal(args []string) error {
	fs := newFlagSet("apply-proposal")
	domain := fs.String("domain", "", "domain directory containing graph.json (default: active-domain chain — HOTAM_DOMAIN env, then .hotam-spec-project marker, then "+defaultDomainRel+")")
	today := fs.String("today", "", "date in YYYY-MM-DD format (required)")
	batchDir := fs.String("batch", "", "apply every *.json proposal file in <dir> atomically in filename order (alternative to a single positional proposal file)")
	ackConflict := fs.String("ack-conflict", "", "cite an unresolved Conflict whose members include the candidate requirement and a blocked member — overrides that formal blocker")
	decisionRef := fs.String("decision-ref", "", "record a human decision reference to override a matching unresolved Conflict carrier; persisted in requirement History")
	fs.Parse(args)

	if *today == "" {
		return fmt.Errorf("--today is required (YYYY-MM-DD)")
	}

	domainDir, err := resolveDomain(*domain)
	if err != nil {
		return err
	}

	if *batchDir != "" {
		proposals, err := loadBatchDir(*batchDir)
		if err != nil {
			return err
		}
		// Advisory confront-at-gate (warn-only, never blocks): run AFTER every
		// proposal is parsed but BEFORE ApplyBatch mutates the graph, so all N
		// candidates are checked against the SAME starting snapshot.
		if err := confrontBatchSummary(domainDir, proposals, false); err != nil {
			return err
		}
		gp := graphPathForDomain(domainDir)
		if err := proposal.ApplyBatch(gp, *today, proposals, batchConflictChecker, batchProvenanceChecker(domainDir)); err != nil {
			return err
		}
		fmt.Printf("applied batch of %d proposals to %s\n", len(proposals), relPathForDisplay(gp))
		fmt.Println("docs not regenerated; run hotam gen-spec or use hotam land")
		return nil
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: hotam apply-proposal <proposal.json> --domain <path> --today YYYY-MM-DD [--batch <dir>] [--ack-conflict <C-id>] [--decision-ref <text>]")
	}
	proposalFile := fs.Arg(0)
	// Parse the proposal ONCE so the advisory confront-at-gate (warn-only,
	// never blocks — R-ai-presents-not-decides) runs BEFORE the apply outcome
	// is printed, then the same parsed value drives the apply. A duplicate /
	// re-litigation warning is therefore visible even when the apply then
	// succeeds (it always proceeds regardless).
	p, err := parseProposalFile(proposalFile)
	if err != nil {
		return err
	}
	if err := confrontBeforeApply(domainDir, p, false); err != nil {
		return err
	}
	// Formal-conflict gate: refuse when an unresolved Conflict carrier names
	// this candidate requirement and another member, unless a matching carrier
	// citation or recorded decision overrides it. Lexical findings are advisory.
	ackOpts := landAckOptions{AckConflict: *ackConflict, DecisionRef: *decisionRef}
	hadConflict, err := semanticConflictGate(domainDir, p, ackOpts)
	if err != nil {
		return err
	}
	// Provenance gate (opt-in, task #158): same placement/rationale as
	// landProposalValue's — refuses BEFORE applyProposalValue mutates the
	// graph, no-op unless this domain's manifest.json sets
	// require_provenance: true.
	if err := provenanceGate(domainDir, p); err != nil {
		return err
	}
	gp, err := applyProposalValue(p, domainDir, *today)
	if err != nil {
		return err
	}
	// Persist the human-decision audit trail (--ack-conflict / --decision-ref)
	// AFTER apply wrote the node. apply-proposal does not regen docs, but
	// appendAckHistory writes directly to graph.json (no regen dependency), so
	// it is safe to call here. Guarded on hadConflict AND ackOpts.hasAck(): ack
	// flags without a real conflict must NOT write a false audit entry. A
	// failure here is a bare error return — apply-proposal has no rollback
	// machinery, consistent with how it handles every other error.
	if hadConflict && ackOpts.hasAck() {
		if err := appendAckHistory(gp, p, *today, ackOpts); err != nil {
			return err
		}
	}
	fmt.Printf("applied %s %s to %s\n", p.Kind(), p.TargetAnchor(), relPathForDisplay(gp))
	fmt.Println("docs not regenerated; run hotam gen-spec or use hotam land")
	return nil
}

// parseProposalFile reads a proposal JSON file from disk and strictly decodes
// it into a Proposed* value via the same parseProposal path. Shared by
// landProposalFile and by the command-level branches that parse the proposal
// ONCE for an advisory confront check before applying (so the file is read a
// single time, not twice).
func parseProposalFile(path string) (proposal.Proposal, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read proposal %s: %w", path, err)
	}
	return parseProposal(data)
}

// applyProposalValue applies an already-parsed proposal to domainDir's
// graph.json (internal/proposal.Apply re-verifies invariants against the
// mutated graph BEFORE writing — see internal/proposal/apply.go — so a
// successful return means the graph on disk is structurally valid). It returns
// the graph path that was written. This is the file-I/O-free apply core,
// shared by landProposalValue (land pipeline) and the command-level branches
// that parse once + run a confront check + apply.
func applyProposalValue(p proposal.Proposal, domainDir, today string) (string, error) {
	gp := graphPathForDomain(domainDir)
	if err := proposal.Apply(gp, today, p); err != nil {
		return "", err
	}
	return gp, nil
}

// loadBatchDir reads every *.json file in dir, sorted by filename, and
// parses each as a standalone proposal via the same strict parseProposal
// path used for single-proposal mode. All files are parsed BEFORE any graph
// I/O happens, so a structurally invalid JSON file fails the batch before
// the graph is even loaded — leaving disk untouched. The caller (ApplyBatch)
// then applies the parsed proposals atomically to one in-memory graph.
// Filename sort gives the resolver explicit control over application order
// (proposal 2 may reference a node proposal 1 just created): name files
// 01-*.json, 02-*.json, … to make the sequence self-documenting.
func loadBatchDir(dir string) ([]proposal.Proposal, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read batch dir %s: %w", dir, err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	if len(names) == 0 {
		return nil, fmt.Errorf("batch dir %s contains no *.json proposal files", dir)
	}
	proposals := make([]proposal.Proposal, 0, len(names))
	for _, name := range names {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read proposal %s: %w", path, err)
		}
		p, err := parseProposal(data)
		if err != nil {
			return nil, fmt.Errorf("proposal %s: %w", name, err)
		}
		proposals = append(proposals, p)
	}
	return proposals, nil
}

func parseProposal(data []byte) (proposal.Proposal, error) {
	var probe struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("parse proposal kind: %w", err)
	}
	switch probe.Kind {
	case proposal.KindRequirement:
		var p proposal.ProposedRequirement
		return p, unmarshalProposal(data, &p)
	case proposal.KindConflictTransition:
		var p proposal.ProposedConflictTransition
		return p, unmarshalProposal(data, &p)
	case proposal.KindRejection:
		var p proposal.ProposedRejection
		return p, unmarshalProposal(data, &p)
	case proposal.KindConflict:
		var p proposal.ProposedConflict
		return p, unmarshalProposal(data, &p)
	case proposal.KindOperatorBudget:
		var p proposal.ProposedOperatorBudget
		return p, unmarshalProposal(data, &p)
	case proposal.KindAxis:
		var p proposal.ProposedAxis
		return p, unmarshalProposal(data, &p)
	case proposal.KindStakeholder:
		var p proposal.ProposedStakeholder
		return p, unmarshalProposal(data, &p)
	case proposal.KindAssumption:
		var p proposal.ProposedAssumption
		return p, unmarshalProposal(data, &p)
	case proposal.KindAssumptionTransition:
		var p proposal.ProposedAssumptionTransition
		return p, unmarshalProposal(data, &p)
	case proposal.KindConflictMemberUpdate:
		var p proposal.ProposedConflictMemberUpdate
		return p, unmarshalProposal(data, &p)
	case proposal.KindEntityType:
		var p proposal.ProposedEntityType
		return p, unmarshalProposal(data, &p)
	case proposal.KindReviewMark:
		var p proposal.ProposedReviewMark
		return p, unmarshalProposal(data, &p)
	case proposal.KindProcess:
		var p proposal.ProposedProcess
		return p, unmarshalProposal(data, &p)
	case proposal.KindGateSignoffBatch:
		var p proposal.ProposedGateSignoffBatch
		return p, unmarshalProposal(data, &p)
	case proposal.KindAssumptionRewrite:
		var p proposal.ProposedAssumptionRewrite
		return p, unmarshalProposal(data, &p)
	case proposal.KindGoal:
		var p proposal.ProposedGoal
		return p, unmarshalProposal(data, &p)
	case proposal.KindEntityInstance:
		var p proposal.ProposedEntityInstance
		return p, unmarshalProposal(data, &p)
	default:
		return nil, fmt.Errorf("unknown proposal kind %q (expected one of: %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)",
			probe.Kind,
			proposal.KindRequirement, proposal.KindConflictTransition, proposal.KindRejection,
			proposal.KindConflict, proposal.KindOperatorBudget, proposal.KindAxis,
			proposal.KindStakeholder, proposal.KindAssumption, proposal.KindAssumptionTransition,
			proposal.KindConflictMemberUpdate, proposal.KindEntityType, proposal.KindReviewMark,
			proposal.KindProcess, proposal.KindGateSignoffBatch, proposal.KindAssumptionRewrite,
			proposal.KindGoal, proposal.KindEntityInstance)
	}
}

// unmarshalProposal decodes a proposal JSON object into target with a strict
// decoder: any field the target struct does not declare a json tag for is a
// hard error (DisallowUnknownFields), so a stale camelCase key or a typo'd
// snake_case key fails loudly instead of silently leaving the Go field at
// its zero value. The top-level "kind" selector field is legal on every
// proposal but is not part of any Proposed* struct, so it is stripped
// before the strict decode runs.
func unmarshalProposal(data []byte, target any) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return fmt.Errorf("parse proposal fields: %w", err)
	}
	delete(fields, "kind")

	stripped, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("parse proposal fields: %w", err)
	}

	dec := json.NewDecoder(bytes.NewReader(stripped))
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return fmt.Errorf("parse proposal fields: %w", err)
	}
	return nil
}
