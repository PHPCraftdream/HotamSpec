package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/diagnose"
	"github.com/PHPCraftdream/HotamSpec/internal/proposal"
)

// cmdPropose implements `hotam propose <kind> [flags]`: the single command that
// collapses the draft→confront→write (and optionally land) workflow into one
// invocation. Instead of an agent hand-authoring a proposal JSON from memory
// of the schema, the tool WRITES valid proposal JSON itself — the agent
// supplies content via flags, the tool owns the schema shape.
//
// Before writing, cmdPropose runs an AUTOMATIC confront check
// (diagnose.Confront — the same engine `hotam confront` uses) against the
// domain's current graph, surfacing duplicate/re-litigation risk immediately.
// The confront result is advisory: like `hotam confront` itself, it NEVER
// blocks the write (R-ai-presents-not-decides).
//
// proposal.Validate runs before the file is written: a validation failure is a
// normal command error (non-zero exit, clear message), no file written.
//
// The optional --land flag composes apply+regen+reverify in the same call
// (reusing landProposalFile, shared with `hotam land`'s single-file mode, so
// both paths are provably identical).
//
// Implemented kinds: requirement, rejection, stakeholder, axis, assumption,
// conflict. Other complex kinds (ConflictTransition, EntityType, etc.) keep
// the existing hand-authored-JSON path (`hotam apply-proposal <file.json>` /
// `hotam land <file.json>`).
func cmdPropose(args []string) error {
	kind, rest, err := extractProposeKind(args)
	if err != nil {
		return err
	}
	switch kind {
	case "requirement":
		return cmdProposeRequirement(rest)
	case "rejection":
		return cmdProposeRejection(rest)
	case "stakeholder":
		return cmdProposeStakeholder(rest)
	case "axis":
		return cmdProposeAxis(rest)
	case "assumption":
		return cmdProposeAssumption(rest)
	case "conflict":
		return cmdProposeConflict(rest)
	default:
		return fmt.Errorf("unknown propose kind %q — kinds: requirement, rejection, stakeholder, axis, assumption, conflict", kind)
	}
}

// extractProposeKind scans args (already passed through main.go's
// reorderFlagsFirst, which moves every flag+value pair before positionals,
// because Go's stdlib flag package stops parsing flags at the first
// non-flag token) for the first non-flag token — the proposal <kind> — and
// returns it along with the remaining args (kind removed, order otherwise
// preserved) for the chosen kind's own FlagSet to parse.
//
// It scans using the SAME isBoolFlag heuristic reorderFlagsFirst uses to
// skip flag-value pairs: a token starting with "-" that is a known bool
// flag (boolFlagNames, main.go) never consumes the next token; any other
// flag-shaped token consumes the following token as its value unless it
// already carries "=" or there is no following token. This must stay in
// sync with boolFlagNames — notably "-h"/"-help"/"--help" are listed there
// specifically so they are recognized as value-less here too, letting
// e.g. `hotam propose requirement -h` correctly find "requirement" as the
// kind and hand `["-h"]` to cmdProposeRequirement's own FlagSet.Parse,
// which is what prints the real per-flag help.
func extractProposeKind(args []string) (kind string, rest []string, err error) {
	kindIdx := -1
	i := 0
	for i < len(args) {
		a := args[i]
		if strings.HasPrefix(a, "-") && a != "-" {
			if isBoolFlag(a) {
				i++
				continue
			}
			if !strings.Contains(a, "=") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i += 2
				continue
			}
			i++
			continue
		}
		kindIdx = i
		break
	}
	if kindIdx < 0 {
		return "", nil, fmt.Errorf("usage: hotam propose <kind> [flags] — kinds: requirement, rejection, stakeholder, axis, assumption, conflict")
	}
	kind = args[kindIdx]
	rest = append([]string{}, args[:kindIdx]...)
	rest = append(rest, args[kindIdx+1:]...)
	return kind, rest, nil
}

// proposeResult is the --json envelope for a successful `hotam propose`
// invocation: the kind/anchor of the constructed proposal, the confront
// result (always present — confront always runs for a valid draft), the path
// the JSON was written to, and whether --land applied it.
type proposeResult struct {
	Kind            string                  `json:"kind"`
	Anchor          string                  `json:"anchor"`
	Confront        diagnose.ConfrontResult `json:"confront"`
	Written         string                  `json:"written"`
	Landed          bool                    `json:"landed"`
	RegeneratedDocs int                     `json:"regenerated_docs,omitempty"`
	Violations      []string                `json:"violations,omitempty"`
}

// ---- requirement ----

func cmdProposeRequirement(args []string) error {
	fs := newFlagSet("propose requirement")
	id := fs.String("id", "", "requirement id, e.g. R-example-thing (required)")
	claim := fs.String("claim", "", "requirement claim text (required)")
	owner := fs.String("owner", "", "owner stakeholder id (required)")
	status := fs.String("status", "", "lifecycle status: DRAFT, SETTLED, etc. (required)")
	why := fs.String("why", "", "rationale / why this requirement exists")
	enforcement := fs.String("enforcement", "", "ENFORCED | STRUCTURAL | PROSE")
	enforcedBy := fs.String("enforced-by", "", "comma-separated enforcer ids (use \"<clear>\" to empty)")
	enforceability := fs.String("enforceability", "", "ENFORCEABLE | UNENFORCEABLE")
	mTag := fs.String("m-tag", "", "milestone tag, e.g. M20")
	assumptions := fs.String("assumptions", "", "comma-separated assumption ids (A-...)")
	evidence := fs.String("evidence", "", "comma-separated evidence references")
	sourceRefs := fs.String("source-refs", "", "comma-separated source references")
	blockedOn := fs.String("blocked-on", "", "free-text label describing what blocks closure (e.g. feature:missing-cli)")
	domain := fs.String("domain", "", "domain directory (default: "+defaultDomainRel+")")
	today := fs.String("today", "", "date in YYYY-MM-DD (required when --land is set)")
	out := fs.String("out", "", "output path for the proposal JSON (default: proposals/draft-<id>.json relative to cwd)")
	land := fs.Bool("land", false, "after writing, immediately apply+regen+reverify (same pipeline as hotam land)")
	ackConflict := fs.String("ack-conflict", "", "cite an existing Conflict node (C-...) whose members cover a semantic conflict the gate detected — overrides the semantic-conflict refusal (only meaningful with --land)")
	decisionRef := fs.String("decision-ref", "", "free-text reference to where a human decision was recorded (ticket, meeting, steward+date) — overrides the semantic-conflict refusal and is persisted in the requirement's History (only meaningful with --land)")
	claudeMD := fs.String("claude-md", "", "path to CLAUDE.md for rune count (only meaningful with --land, passed through to gen-spec)")
	asJSON := fs.Bool("json", false, "emit machine-readable JSON instead of the human-readable report")
	fs.Parse(args)

	for _, c := range []struct{ flag, label string }{
		{*id, "id"}, {*claim, "claim"}, {*owner, "owner"}, {*status, "status"},
	} {
		if strings.TrimSpace(c.flag) == "" {
			return fmt.Errorf("--%s is required for propose requirement", c.label)
		}
	}

	pr := proposal.ProposedRequirement{
		ID:             *id,
		Claim:          *claim,
		Owner:          *owner,
		Status:         *status,
		Why:            *why,
		Assumptions:    splitCSV(*assumptions),
		Enforcement:    *enforcement,
		EnforcedBy:     splitCSV(*enforcedBy),
		MTag:           *mTag,
		Enforceability: *enforceability,
		Evidence:       splitCSV(*evidence),
		SourceRefs:     splitCSV(*sourceRefs),
		BlockedOn:      *blockedOn,
	}

	ackOpts := landAckOptions{AckConflict: *ackConflict, DecisionRef: *decisionRef}
	return runPropose(pr, *domain, *today, *out, *land, ackOpts, *claudeMD, *asJSON)
}

// ---- rejection ----

func cmdProposeRejection(args []string) error {
	fs := newFlagSet("propose rejection")
	reqID := fs.String("requirement-id", "", "id of the requirement to reject (required)")
	reason := fs.String("reason", "", "why this requirement is rejected (required)")
	replacedBy := fs.String("replaced-by", "", "comma-separated successor requirement ids that replace the rejected one")
	domain := fs.String("domain", "", "domain directory (default: "+defaultDomainRel+")")
	today := fs.String("today", "", "date in YYYY-MM-DD (required when --land is set)")
	out := fs.String("out", "", "output path for the proposal JSON (default: proposals/draft-reject-<requirement-id>.json relative to cwd)")
	land := fs.Bool("land", false, "after writing, immediately apply+regen+reverify (same pipeline as hotam land)")
	claudeMD := fs.String("claude-md", "", "path to CLAUDE.md for rune count (only meaningful with --land, passed through to gen-spec)")
	asJSON := fs.Bool("json", false, "emit machine-readable JSON instead of the human-readable report")
	fs.Parse(args)

	for _, c := range []struct{ flag, label string }{
		{*reqID, "requirement-id"}, {*reason, "reason"},
	} {
		if strings.TrimSpace(c.flag) == "" {
			return fmt.Errorf("--%s is required for propose rejection", c.label)
		}
	}

	p := proposal.ProposedRejection{
		RequirementID: *reqID,
		Reason:        *reason,
		ReplacedBy:    splitCSV(*replacedBy),
	}

	return runPropose(p, *domain, *today, *out, *land, landAckOptions{}, *claudeMD, *asJSON)
}

// ---- stakeholder ----

func cmdProposeStakeholder(args []string) error {
	fs := newFlagSet("propose stakeholder")
	id := fs.String("id", "", "stakeholder id, e.g. S-team-a (required)")
	name := fs.String("name", "", "human-readable name (required)")
	domn := fs.String("stakeholder-domain", "", "the stakeholder's own business domain field (required)")
	why := fs.String("why", "", "rationale for this stakeholder's existence")
	domainFlag := fs.String("domain", "", "target graph directory (default: "+defaultDomainRel+") — same meaning as --domain on every other propose subcommand; NOT the stakeholder's own business domain, use --stakeholder-domain for that")
	today := fs.String("today", "", "date in YYYY-MM-DD (required when --land is set)")
	out := fs.String("out", "", "output path for the proposal JSON (default: proposals/draft-<id>.json relative to cwd)")
	land := fs.Bool("land", false, "after writing, immediately apply+regen+reverify (same pipeline as hotam land)")
	claudeMD := fs.String("claude-md", "", "path to CLAUDE.md for rune count (only meaningful with --land, passed through to gen-spec)")
	asJSON := fs.Bool("json", false, "emit machine-readable JSON instead of the human-readable report")
	fs.Parse(args)

	for _, c := range []struct{ flag, label string }{
		{*id, "id"}, {*name, "name"}, {*domn, "stakeholder-domain"},
	} {
		if strings.TrimSpace(c.flag) == "" {
			return fmt.Errorf("--%s is required for propose stakeholder", c.label)
		}
	}

	p := proposal.ProposedStakeholder{
		ID:     *id,
		Name:   *name,
		Domain: *domn,
		Why:    *why,
	}

	return runPropose(p, *domainFlag, *today, *out, *land, landAckOptions{}, *claudeMD, *asJSON)
}

// ---- axis ----

func cmdProposeAxis(args []string) error {
	fs := newFlagSet("propose axis")
	slug := fs.String("slug", "", "kebab-case axis slug, e.g. latency-vs-cost (required)")
	description := fs.String("description", "", "axis description text (required)")
	why := fs.String("why", "", "rationale / why this axis exists")
	domain := fs.String("domain", "", "domain directory (default: "+defaultDomainRel+")")
	today := fs.String("today", "", "date in YYYY-MM-DD (required when --land is set)")
	out := fs.String("out", "", "output path for the proposal JSON (default: proposals/draft-<slug>.json relative to cwd)")
	land := fs.Bool("land", false, "after writing, immediately apply+regen+reverify (same pipeline as hotam land)")
	claudeMD := fs.String("claude-md", "", "path to CLAUDE.md for rune count (only meaningful with --land, passed through to gen-spec)")
	asJSON := fs.Bool("json", false, "emit machine-readable JSON instead of the human-readable report")
	fs.Parse(args)

	for _, c := range []struct{ flag, label string }{
		{*slug, "slug"}, {*description, "description"},
	} {
		if strings.TrimSpace(c.flag) == "" {
			return fmt.Errorf("--%s is required for propose axis", c.label)
		}
	}

	p := proposal.ProposedAxis{
		Slug:        *slug,
		Description: *description,
		Why:         *why,
	}

	return runPropose(p, *domain, *today, *out, *land, landAckOptions{}, *claudeMD, *asJSON)
}

// ---- assumption ----

func cmdProposeAssumption(args []string) error {
	fs := newFlagSet("propose assumption")
	id := fs.String("id", "", "assumption id, e.g. A-something (required)")
	statement := fs.String("statement", "", "assumption statement text (required)")
	status := fs.String("status", "", "assumption status: HOLDS, UNCERTAIN, DEAD, IMPLEMENTS (required)")
	owner := fs.String("owner", "", "owner stakeholder id (required)")
	why := fs.String("why", "", "rationale / why this assumption exists")
	createdAt := fs.String("created-at", "", "creation date YYYY-MM-DD (defaults to --today)")
	domain := fs.String("domain", "", "domain directory (default: "+defaultDomainRel+")")
	today := fs.String("today", "", "date in YYYY-MM-DD (required when --land is set)")
	out := fs.String("out", "", "output path for the proposal JSON (default: proposals/draft-<id>.json relative to cwd)")
	land := fs.Bool("land", false, "after writing, immediately apply+regen+reverify (same pipeline as hotam land)")
	claudeMD := fs.String("claude-md", "", "path to CLAUDE.md for rune count (only meaningful with --land, passed through to gen-spec)")
	asJSON := fs.Bool("json", false, "emit machine-readable JSON instead of the human-readable report")
	fs.Parse(args)

	for _, c := range []struct{ flag, label string }{
		{*id, "id"}, {*statement, "statement"}, {*status, "status"}, {*owner, "owner"},
	} {
		if strings.TrimSpace(c.flag) == "" {
			return fmt.Errorf("--%s is required for propose assumption", c.label)
		}
	}

	p := proposal.ProposedAssumption{
		ID:        *id,
		Statement: *statement,
		Status:    *status,
		Owner:     *owner,
		Why:       *why,
		CreatedAt: *createdAt,
	}

	return runPropose(p, *domain, *today, *out, *land, landAckOptions{}, *claudeMD, *asJSON)
}

// ---- conflict ----

func cmdProposeConflict(args []string) error {
	fs := newFlagSet("propose conflict")
	axis := fs.String("axis", "", "existing axis slug, e.g. latency-vs-cost (required)")
	context := fs.String("context", "", "free-text context describing the tension (required)")
	members := fs.String("members", "", "comma-separated requirement ids (R-...) in tension, at least two distinct (required)")
	steward := fs.String("steward", "", "stakeholder id who stewards this conflict — must NOT own any member (required)")
	sharedAssumption := fs.String("shared-assumption", "", "assumption id (A-...) shared by the conflict members")
	note := fs.String("note", "", "free-text note")
	initialLifecycle := fs.String("initial-lifecycle", "", "initial lifecycle: DETECTED (default) or DECIDED(...)")
	decidedBy := fs.String("decided-by", "", "human decider id (required when --initial-lifecycle starts with DECIDED)")
	domain := fs.String("domain", "", "domain directory (default: "+defaultDomainRel+")")
	today := fs.String("today", "", "date in YYYY-MM-DD (required when --land is set)")
	out := fs.String("out", "", "output path for the proposal JSON (default: proposals/draft-<conflict-id>.json relative to cwd)")
	land := fs.Bool("land", false, "after writing, immediately apply+regen+reverify (same pipeline as hotam land)")
	claudeMD := fs.String("claude-md", "", "path to CLAUDE.md for rune count (only meaningful with --land, passed through to gen-spec)")
	asJSON := fs.Bool("json", false, "emit machine-readable JSON instead of the human-readable report")
	fs.Parse(args)

	for _, c := range []struct{ flag, label string }{
		{*axis, "axis"}, {*context, "context"}, {*members, "members"}, {*steward, "steward"},
	} {
		if strings.TrimSpace(c.flag) == "" {
			return fmt.Errorf("--%s is required for propose conflict", c.label)
		}
	}

	p := proposal.ProposedConflict{
		Axis:             *axis,
		Context:          *context,
		Members:          splitCSV(*members),
		Steward:          *steward,
		SharedAssumption: *sharedAssumption,
		Note:             *note,
		InitialLifecycle: *initialLifecycle,
		DecidedBy:        *decidedBy,
	}

	return runPropose(p, *domain, *today, *out, *land, landAckOptions{}, *claudeMD, *asJSON)
}

// ---- shared pipeline ----

// runPropose is the shared draft→confront→write→(land) pipeline used by every
// propose subcommand. It validates the constructed proposal (gating the write),
// runs the automatic confront check against the domain graph, writes valid
// proposal JSON to --out, and optionally lands it. p must be a concrete
// Proposed* value (already populated from flags by the caller).
func runPropose(
	p proposal.Proposal,
	domainFlag, today, outPath string,
	land bool, ackOpts landAckOptions, claudeMDPath string, asJSON bool,
) error {
	// Validate the constructed proposal before any graph I/O: a validation
	// failure is a normal command error (non-zero exit), no file written, no
	// confront wasted.
	if err := proposal.Validate(p); err != nil {
		return err
	}

	// If --land is requested, --today is required (landProposalFile needs it).
	// Check BEFORE any I/O so a missing --today doesn't leave a written draft
	// on disk from a command that ultimately failed.
	if land && today == "" {
		return fmt.Errorf("--today is required when --land is set")
	}

	domainDir, err := resolveDomain(domainFlag)
	if err != nil {
		return err
	}

	// Confront: load the graph and run the SAME advisory check `hotam
	// confront` runs. This is advisory only — a high-overlap hit prints a
	// warning but never blocks the write.
	g, err := loadDomainGraph(domainDir)
	if err != nil {
		return err
	}
	confrontResult := diagnose.Confront(g, proposeConfrontText(p))

	// Resolve the output path.
	if outPath == "" {
		outPath = defaultProposeOutPath(p)
	}

	// Marshal and write.
	data, err := marshalProposalFile(p)
	if err != nil {
		return err
	}
	if err := writeFileMkdir(outPath, data); err != nil {
		return err
	}

	if asJSON {
		// JSON path: run the land step (if any) FIRST, in stderr-routed mode,
		// then emit exactly ONE JSON document to stdout with the land outcome
		// populated. This is the fix for the review-8 R8-c bug where the early
		// printJSON left a JSON document on stdout BEFORE landProposalFile
		// printed its own prose to the same stream.
		result := proposeResult{
			Kind:     p.Kind(),
			Anchor:   p.TargetAnchor(),
			Confront: confrontResult,
			Written:  outPath,
			Landed:   land,
		}
		if land {
			lr, err := landProposalFile(outPath, domainDir, claudeMDPath, today, ackOpts, true)
			if err != nil {
				return err
			}
			result.RegeneratedDocs = lr.RegeneratedDocs
			result.Violations = lr.Violations
		}
		return printJSON(result)
	}

	// Non-JSON path: byte-identical to pre-fix behavior — print the confront
	// report and write confirmation immediately, THEN run the land step (if
	// any) with its own prose on stdout.
	fmt.Print(formatConfrontReport(confrontResult))
	fmt.Printf("wrote %s %s proposal to %s\n", p.Kind(), p.TargetAnchor(), relPathForDisplay(outPath))

	if land {
		if _, err := landProposalFile(outPath, domainDir, claudeMDPath, today, ackOpts, false); err != nil {
			return err
		}
	}

	return nil
}

// proposeConfrontText extracts the candidate text for the automatic confront
// check from a proposal. Requirement uses its Claim; Rejection uses its Reason
// (the claim being rejected is already in the graph — the confront guard here
// is against duplicating the REJECTION rationale, and more importantly against
// re-deriving an already-rejected idea); Stakeholder uses its Name+Why
// (stakeholder proposals are low-text, so we combine the two).
func proposeConfrontText(p proposal.Proposal) string {
	switch v := p.(type) {
	case proposal.ProposedRequirement:
		return v.Claim
	case proposal.ProposedRejection:
		return v.Reason
	case proposal.ProposedStakeholder:
		return v.Name + " " + v.Why
	case proposal.ProposedAxis:
		return v.Slug + " " + v.Description
	case proposal.ProposedAssumption:
		return v.Statement
	case proposal.ProposedConflict:
		return v.Context
	default:
		return p.TargetAnchor()
	}
}

// defaultProposeOutPath picks a predictable default for --out when the caller
// omits it. Requirement: proposals/draft-<id>.json. Rejection:
// proposals/draft-reject-<requirement-id>.json. Stakeholder:
// proposals/draft-<id>.json. The path is relative to the current working
// directory (standard CLI convention).
func defaultProposeOutPath(p proposal.Proposal) string {
	switch v := p.(type) {
	case proposal.ProposedRejection:
		return filepath.Join("proposals", "draft-reject-"+v.RequirementID+".json")
	case proposal.ProposedAxis:
		// ProposedAxis.TargetAnchor() returns "Axis:" + slug (note the COLON),
		// which is illegal in a Windows filename. Use the bare slug instead,
		// mirroring how ProposedRejection gets its own explicit case.
		return filepath.Join("proposals", "draft-"+v.Slug+".json")
	default:
		return filepath.Join("proposals", "draft-"+p.TargetAnchor()+".json")
	}
}

// marshalProposalFile serializes a Proposed* value into the same JSON shape
// parseProposal / unmarshalProposal consume: a top-level "kind" selector field
// plus the struct's own json-tagged fields.
func marshalProposalFile(p proposal.Proposal) ([]byte, error) {
	envelope := struct {
		Kind string `json:"kind"`
	}{Kind: p.Kind()}

	raw, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshal proposal: %w", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, fmt.Errorf("marshal proposal: %w", err)
	}
	kindBytes, err := json.Marshal(envelope.Kind)
	if err != nil {
		return nil, fmt.Errorf("marshal proposal: %w", err)
	}
	fields["kind"] = kindBytes
	return json.MarshalIndent(fields, "", "  ")
}

// splitCSV parses a comma-separated string into a trimmed, non-empty slice.
// An empty (or all-whitespace) input returns nil — important for the patch /
// coalesce semantics of ProposedRequirement.mutate, where nil means "leave the
// old value alone".
func splitCSV(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
