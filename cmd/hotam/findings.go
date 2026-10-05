package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/evidence"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

const findingReviewFile = "docs/reviews/finding-reviews.json"
const findingReviewSchemaVersion = 1

type findingReview struct {
	FindingID         string `json:"finding_id"`
	Kind              string `json:"kind"`
	Status            string `json:"status"`
	Rationale         string `json:"rationale"`
	DecisionReference string `json:"decision_reference"`
}

type findingReviewFileData struct {
	SchemaVersion int             `json:"schema_version"`
	Reviews       []findingReview `json:"reviews"`
}

type findingView struct {
	Finding evidence.Finding `json:"finding"`
	Review  *findingReview   `json:"review,omitempty"`
}

func cmdFindings(args []string) error {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			printFindingsUsage()
			return nil
		}
	}
	if len(args) == 1 && args[0] == "help" {
		printFindingsUsage()
		return nil
	}
	sub, rest, ok := splitSubcommand(args)
	if !ok {
		return fmt.Errorf("usage: hotam findings <list|show|review> [args] [--domain <path>] [--json]")
	}
	switch sub {
	case "list":
		return cmdFindingsList(rest)
	case "show":
		return cmdFindingsShow(rest)
	case "review":
		return cmdFindingsReview(rest)
	case "help":
		printFindingsUsage()
		return nil
	default:
		return fmt.Errorf("hotam findings: unknown subcommand %q (want list|show|review)", sub)
	}
}

func printFindingsUsage() {
	fmt.Print(`hotam findings — inspect current observed findings and record human review notes

Usage:
  hotam findings list [--domain <path>] [--json]
  hotam findings show <finding-id> [--domain <path>] [--json]
  hotam findings review <finding-id> --kind <classification> --status <open|resolved> --rationale <text> --decision-ref <reference> [--domain <path>] [--json]

Classifications: specification_issue | model_issue | implementation_issue | needs_review
Reviews are human-authored notes in docs/reviews/finding-reviews.json; they do not change the generated evidence report or assign an unrecorded resolver decision.
`)
}

func cmdFindingsList(args []string) error {
	fs := newFlagSet("findings list")
	domain := fs.String("domain", "", "domain directory (default: "+defaultDomainRel+")")
	asJSON := fs.Bool("json", false, "emit machine-readable JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("usage: hotam findings list [--domain <path>] [--json]")
	}
	_, report, reviews, err := loadFindingState(*domain)
	if err != nil {
		return err
	}
	views := findingViews(report, reviews)
	if *asJSON {
		return writeJSON(os.Stdout, views)
	}
	if len(views) == 0 {
		fmt.Println("No findings in the current generated evidence report.")
		return nil
	}
	for _, view := range views {
		review := "human-review=none; raw-status=" + view.Finding.ReviewStatus
		if view.Review != nil {
			review = "human-review=" + view.Review.Kind + "/" + view.Review.Status
		}
		fmt.Printf("%s  raw-kind=%s  disposition=%s  case=%s  target=%s  producer=%s  %s  %s\n",
			view.Finding.ID, view.Finding.Kind, view.Finding.Disposition,
			valueOrNone(view.Finding.CaseID), valueOrNone(view.Finding.Target),
			valueOrNone(view.Finding.Producer), review, findingShortDescription(view.Finding))
	}
	return nil
}

func cmdFindingsShow(args []string) error {
	fs := newFlagSet("findings show")
	domain := fs.String("domain", "", "domain directory (default: "+defaultDomainRel+")")
	asJSON := fs.Bool("json", false, "emit machine-readable JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: hotam findings show <finding-id> [--domain <path>] [--json]")
	}
	_, report, reviews, err := loadFindingState(*domain)
	if err != nil {
		return err
	}
	finding, ok := findReportFinding(report, fs.Arg(0))
	if !ok {
		return staleFindingError(fs.Arg(0))
	}
	view := findingView{Finding: finding, Review: reviewFor(reviews, finding.ID)}
	if *asJSON {
		return writeJSON(os.Stdout, view)
	}
	printFindingDetail(view)
	return nil
}

func cmdFindingsReview(args []string) error {
	fs := newFlagSet("findings review")
	domain := fs.String("domain", "", "domain directory (default: "+defaultDomainRel+")")
	kind := fs.String("kind", "", "human classification: specification_issue|model_issue|implementation_issue|needs_review")
	status := fs.String("status", "", "human review status: open|resolved")
	rationale := fs.String("rationale", "", "human rationale for the classification")
	decisionRef := fs.String("decision-ref", "", "reference to the human decision record (not a verified signoff)")
	asJSON := fs.Bool("json", false, "emit the saved review note as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: hotam findings review <finding-id> --kind <classification> --status <open|resolved> --rationale <text> --decision-ref <reference> [--domain <path>] [--json]")
	}
	if !validFindingReviewKind(*kind) {
		return fmt.Errorf("--kind must be specification_issue, model_issue, implementation_issue, or needs_review; got %q", *kind)
	}
	if *status != "open" && *status != "resolved" {
		return fmt.Errorf("--status must be open or resolved; got %q", *status)
	}
	required := []struct {
		flagName string
		value    string
	}{
		{flagName: "--rationale", value: *rationale},
		{flagName: "--decision-ref", value: *decisionRef},
	}
	for _, field := range required {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("%s is required and must be non-empty", field.flagName)
		}
	}

	domainDir, report, reviews, err := loadFindingState(*domain)
	if err != nil {
		return err
	}
	findingID := fs.Arg(0)
	if _, ok := findReportFinding(report, findingID); !ok {
		return staleFindingError(findingID)
	}
	review := findingReview{
		FindingID:         findingID,
		Kind:              *kind,
		Status:            *status,
		Rationale:         strings.TrimSpace(*rationale),
		DecisionReference: strings.TrimSpace(*decisionRef),
	}
	reviews = upsertFindingReview(reviews, review)
	if err := writeFindingReviews(domainDir, reviews); err != nil {
		return fmt.Errorf("save finding review: %w", err)
	}
	if *asJSON {
		return writeJSON(os.Stdout, review)
	}
	fmt.Printf("recorded human review for %s: %s/%s (decision reference: %s); observed evidence was not changed\n", review.FindingID, review.Kind, review.Status, review.DecisionReference)
	return nil
}

func loadFindingState(domainFlag string) (string, evidence.Report, []findingReview, error) {
	domainDir, err := resolveDomain(domainFlag)
	if err != nil {
		return "", evidence.Report{}, nil, err
	}
	reportPath := filepath.Join(domainDir, "docs", "gen", "evidence.json")
	data, err := os.ReadFile(reportPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", evidence.Report{}, nil, fmt.Errorf("current evidence report not found at %s; run `hotam evidence --domain %s --write` first", reportPath, domainDir)
		}
		return "", evidence.Report{}, nil, fmt.Errorf("read current evidence report: %w", err)
	}
	var report evidence.Report
	if err := json.Unmarshal(data, &report); err != nil {
		return "", evidence.Report{}, nil, fmt.Errorf("decode current evidence report %s: %w", reportPath, err)
	}
	if report.SchemaVersion <= 0 {
		return "", evidence.Report{}, nil, fmt.Errorf("current evidence report %s has no valid schema version", reportPath)
	}
	reviews, err := readFindingReviews(domainDir)
	if err != nil {
		return "", evidence.Report{}, nil, err
	}
	return domainDir, report, reviews, nil
}

func readFindingReviews(domainDir string) ([]findingReview, error) {
	path := filepath.Join(domainDir, filepath.FromSlash(findingReviewFile))
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read human finding reviews: %w", err)
	}
	var stored findingReviewFileData
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, fmt.Errorf("decode human finding reviews %s: %w", path, err)
	}
	if stored.SchemaVersion != findingReviewSchemaVersion {
		return nil, fmt.Errorf("unsupported finding review schema version %d in %s", stored.SchemaVersion, path)
	}
	seen := make(map[string]struct{}, len(stored.Reviews))
	for _, review := range stored.Reviews {
		if strings.TrimSpace(review.FindingID) == "" || !validFindingReviewKind(review.Kind) || (review.Status != "open" && review.Status != "resolved") || strings.TrimSpace(review.Rationale) == "" || strings.TrimSpace(review.DecisionReference) == "" {
			return nil, fmt.Errorf("invalid human finding review for %q in %s", review.FindingID, path)
		}
		if _, ok := seen[review.FindingID]; ok {
			return nil, fmt.Errorf("duplicate human finding review for %q in %s", review.FindingID, path)
		}
		seen[review.FindingID] = struct{}{}
	}
	return stored.Reviews, nil
}

func writeFindingReviews(domainDir string, reviews []findingReview) error {
	sort.Slice(reviews, func(i, j int) bool { return reviews[i].FindingID < reviews[j].FindingID })
	stored := findingReviewFileData{SchemaVersion: findingReviewSchemaVersion, Reviews: reviews}
	data, err := marshalJSON(stored)
	if err != nil {
		return fmt.Errorf("encode review notes: %w", err)
	}
	return writeFileMkdir(filepath.Join(domainDir, filepath.FromSlash(findingReviewFile)), data)
}

func marshalJSON(value any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(value); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeJSON(w *os.File, value any) error {
	data, err := marshalJSON(value)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func findingViews(report evidence.Report, reviews []findingReview) []findingView {
	views := make([]findingView, 0, len(report.Findings))
	for _, finding := range report.Findings {
		views = append(views, findingView{Finding: finding, Review: reviewFor(reviews, finding.ID)})
	}
	return views
}

func findReportFinding(report evidence.Report, id string) (evidence.Finding, bool) {
	for _, finding := range report.Findings {
		if finding.ID == id {
			return finding, true
		}
	}
	return evidence.Finding{}, false
}

func reviewFor(reviews []findingReview, id string) *findingReview {
	for i := range reviews {
		if reviews[i].FindingID == id {
			review := reviews[i]
			return &review
		}
	}
	return nil
}

func upsertFindingReview(reviews []findingReview, review findingReview) []findingReview {
	for i := range reviews {
		if reviews[i].FindingID == review.FindingID {
			reviews[i] = review
			return reviews
		}
	}
	return append(reviews, review)
}

func validFindingReviewKind(kind string) bool {
	switch kind {
	case "specification_issue", "model_issue", "implementation_issue", "needs_review":
		return true
	default:
		return false
	}
}

func staleFindingError(id string) error {
	return fmt.Errorf("finding %q is not present in the current generated report (unknown or stale ID); regenerate with `hotam evidence --write` and use an ID from that report", id)
}

func findingShortDescription(f evidence.Finding) string {
	prefix := f.RequirementID
	if prefix == "" {
		prefix = f.Subject
	}
	if f.ClauseID != "" {
		if prefix != "" {
			prefix += " "
		}
		prefix += "clause=" + f.ClauseID
	}
	if f.CaseID != "" {
		if prefix != "" {
			prefix += " "
		}
		prefix += "case=" + f.CaseID
	}
	if prefix != "" && f.Claim != "" {
		return prefix + ": " + f.Claim
	}
	if prefix != "" {
		return prefix
	}
	return f.Claim
}

func printFindingDetail(view findingView) {
	f := view.Finding
	fmt.Printf("Finding: %s\nObserved kind: %s\nDisposition: %s\nRaw report review status: %s\nRequirement: %s\nClause: %s\nCase: %s\nTarget: %s\nOperation: %s\nProducer: %s\nClaim: %s\nTest: %s\nSubject: %s\n",
		f.ID, f.Kind, valueOrNone(f.Disposition), f.ReviewStatus,
		valueOrNone(f.RequirementID), valueOrNone(f.ClauseID), valueOrNone(f.CaseID),
		valueOrNone(f.Target), valueOrNone(f.Operation), valueOrNone(f.Producer), f.Claim,
		valueOrNone(f.Test), valueOrNone(f.Subject))
	if len(f.ClaimTexts) > 0 {
		fmt.Println("Localized authored claims (raw):")
		languages := make([]string, 0, len(f.ClaimTexts))
		for language := range f.ClaimTexts {
			languages = append(languages, language)
		}
		sort.Strings(languages)
		for _, language := range languages {
			fmt.Printf("  %s: %s\n", language, f.ClaimTexts[language])
		}
	}
	if view.Review == nil {
		fmt.Println("Human review: none")
	} else {
		fmt.Printf("Human review: %s/%s\nRationale: %s\nDecision reference: %s (reference only; not a verified signoff)\n", view.Review.Kind, view.Review.Status, view.Review.Rationale, view.Review.DecisionReference)
	}
	if len(f.SourceLinks) > 0 {
		fmt.Println("Source links:")
		for _, link := range f.SourceLinks {
			fmt.Printf("  %s#%s\n", link.SourceID, link.Anchor)
		}
	}
	if len(f.Observations) > 0 {
		fmt.Println("Observations:")
		for _, observation := range f.Observations {
			verdict := "pass"
			if !observation.Passed {
				verdict = "fail"
			}
			fmt.Printf("  %s | input: %s [raw %s] | actual: %s [raw %s] | expected: %s [raw %s] | verdict: %s\n",
				observation.Name, observation.Input, rawFindingValue(observation.RawInput),
				observation.Actual, rawFindingValue(observation.RawActual),
				observation.Expected, rawFindingValue(observation.RawExpected), verdict)
		}
	}
	fmt.Printf("Context: implementation=%s implementation_version=%s spec_version=%s profile=%s operation=%s target=%s producer=%s\n",
		valueOrNone(f.Context.Implementation), valueOrNone(f.Context.ImplementationVersion),
		valueOrNone(f.Context.SpecVersion), valueOrNone(f.Context.Profile),
		valueOrNone(f.Context.Operation), valueOrNone(f.Context.Target), valueOrNone(f.Context.Producer))
	fmt.Printf("Profile fingerprint=%s producer fingerprint=%s case fingerprint=%s\n",
		valueOrNone(f.Context.ProfileFingerprint), valueOrNone(f.Context.ProducerFingerprint),
		valueOrNone(f.Context.CaseFingerprint))
	fmt.Printf("Declared composition=%s\nMeasured components=%s\n",
		rawFindingJSON(f.Context.Composition), rawFindingJSON(f.Context.MeasuredComponents))
}

func rawFindingValue(value *ontology.ObservedValue) string {
	if value == nil {
		return "absent"
	}
	return rawFindingJSON(value)
}

func rawFindingJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("<unavailable: %v>", err)
	}
	return string(data)
}

func valueOrNone(value string) string {
	if value == "" {
		return "—"
	}
	return value
}
