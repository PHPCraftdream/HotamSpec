// Package source verifies structural links from graph requirements to pinned
// specification sources. It never infers semantic completeness from source text.
package source

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

const (
	StatusVerified = "verified"
	StatusMissing  = "missing"
	StatusDrift    = "drift"
	StatusInvalid  = "invalid"
)

// Check is one deterministic structural source or source-link check result.
type Check struct {
	SourceID       string `json:"source_id"`
	Anchor         string `json:"anchor,omitempty"`
	Path           string `json:"path"`
	Version        string `json:"version"`
	ExpectedSHA256 string `json:"expected_sha256"`
	ActualSHA256   string `json:"actual_sha256,omitempty"`
	Status         string `json:"status"`
	Message        string `json:"message"`
}

// Verify checks every declared source and each requirement or source-clause
// link. Entries and links are processed in stable order; verification is
// limited to source bytes, declared integrity pins, and structural anchors.
func Verify(g *ontology.Graph) []Check {
	if g == nil {
		return nil
	}
	checks := append(VerifySources(g), VerifyLinks(g)...)
	sortChecks(checks)
	return checks
}

// VerifySources checks only explicitly declared SpecificationSources. An empty
// source list is an honest no-op for older domains.
func VerifySources(g *ontology.Graph) []Check {
	if g == nil || len(g.SpecificationSources) == 0 {
		return nil
	}
	counts := make(map[string]int, len(g.SpecificationSources))
	for _, spec := range g.SpecificationSources {
		counts[spec.ID]++
	}
	out := make([]Check, 0, len(g.SpecificationSources))
	for _, spec := range g.SpecificationSources {
		check := Check{
			SourceID:       spec.ID,
			Path:           spec.Path,
			Version:        spec.Version,
			ExpectedSHA256: spec.SHA256,
			Status:         StatusVerified,
			Message:        "source bytes match the declared SHA-256",
		}
		invalid := []string{}
		trimmedID := strings.TrimSpace(spec.ID)
		if trimmedID == "" {
			invalid = append(invalid, "source ID is empty")
		} else {
			if trimmedID != spec.ID {
				invalid = append(invalid, "source ID has surrounding whitespace")
			}
			if counts[spec.ID] > 1 {
				invalid = append(invalid, "source ID is duplicated")
			}
		}
		if strings.TrimSpace(spec.Path) == "" {
			invalid = append(invalid, "source path is empty")
		}
		if strings.TrimSpace(spec.Version) == "" {
			invalid = append(invalid, "source version is empty")
		}
		hashValid := validSHA256(spec.SHA256)
		if !hashValid {
			invalid = append(invalid, "source sha256 must be exactly 64 hexadecimal characters")
		}

		var data []byte
		var readErr error
		var haveBytes bool
		if strings.TrimSpace(spec.Path) != "" {
			var path string
			path, readErr = resolvePath(g, spec.Path)
			if readErr == nil {
				data, readErr = os.ReadFile(path)
				haveBytes = readErr == nil
			}
		}
		if haveBytes {
			check.ActualSHA256 = digest(data)
		} else if readErr != nil {
			switch {
			case os.IsNotExist(readErr):
				if len(invalid) == 0 {
					check.Status = StatusMissing
					check.Message = "source file does not exist"
				} else {
					invalid = append(invalid, "source file does not exist")
				}
			default:
				invalid = append(invalid, readErr.Error())
			}
		}
		if len(invalid) != 0 {
			check.Status = StatusInvalid
			check.Message = strings.Join(invalid, "; ")
			out = append(out, check)
			continue
		}
		if check.Status == StatusMissing {
			out = append(out, check)
			continue
		}
		if !strings.EqualFold(check.ActualSHA256, spec.SHA256) {
			check.Status = StatusDrift
			check.Message = "source bytes differ from the declared SHA-256"
		}
		out = append(out, check)
	}
	sortChecks(out)
	return out
}

// VerifyLinks checks explicit Requirement.SourceLinks and inventory
// SourceClause.SourceLinks. Each link must name a declared source and resolve
// to a real line range or Markdown heading. Source drift is reported separately
// by VerifySources.
func VerifyLinks(g *ontology.Graph) []Check {
	if g == nil {
		return nil
	}
	type sourceBytes struct {
		spec ontology.SpecificationSource
		data []byte
		read bool
	}
	type ownedLink struct {
		owner string
		kind  string
		link  ontology.SourceLink
	}
	var links []ownedLink
	needed := map[string]bool{}
	for _, req := range g.Requirements {
		for _, link := range req.SourceLinks {
			needed[link.SourceID] = true
			links = append(links, ownedLink{owner: req.ID, kind: "requirement", link: link})
		}
	}
	if g.Conformance != nil {
		for _, clause := range g.Conformance.Clauses {
			for _, link := range clause.SourceLinks {
				needed[link.SourceID] = true
				links = append(links, ownedLink{owner: clause.ID, kind: "source clause", link: link})
			}
		}
	}
	sources := make(map[string]sourceBytes, len(g.SpecificationSources))
	for _, spec := range g.SpecificationSources {
		if !needed[spec.ID] {
			continue
		}
		entry, exists := sources[spec.ID]
		if exists && !specificationSourceLess(spec, entry.spec) {
			continue
		}
		sources[spec.ID] = sourceBytes{spec: spec}
	}
	for id, entry := range sources {
		if strings.TrimSpace(entry.spec.Path) != "" {
			if path, err := resolvePath(g, entry.spec.Path); err == nil {
				if data, err := os.ReadFile(path); err == nil {
					entry.data = data
					entry.read = true
				}
			}
		}
		sources[id] = entry
	}
	sort.Slice(links, func(i, j int) bool {
		if links[i].kind != links[j].kind {
			return links[i].kind < links[j].kind
		}
		if links[i].owner != links[j].owner {
			return links[i].owner < links[j].owner
		}
		if links[i].link.SourceID != links[j].link.SourceID {
			return links[i].link.SourceID < links[j].link.SourceID
		}
		return links[i].link.Anchor < links[j].link.Anchor
	})
	var out []Check
	for _, item := range links {
		entry, ok := sources[item.link.SourceID]
		check := Check{SourceID: item.link.SourceID, Anchor: item.link.Anchor, Status: StatusVerified}
		if strings.TrimSpace(item.link.SourceID) == "" || !ok {
			check.Status = StatusInvalid
			check.Message = fmt.Sprintf("%s %q links unknown specification source %q", item.kind, item.owner, item.link.SourceID)
			out = append(out, check)
			continue
		}
		check.Path = entry.spec.Path
		check.Version = entry.spec.Version
		check.ExpectedSHA256 = entry.spec.SHA256
		if !entry.read {
			// The source-level check reports missing/unreadable bytes. A link
			// cannot be resolved against bytes that are not available.
			continue
		}
		check.ActualSHA256 = digest(entry.data)
		if strings.TrimSpace(item.link.Anchor) == "" {
			check.Status = StatusInvalid
			check.Message = fmt.Sprintf("%s %q source link has no anchor", item.kind, item.owner)
			out = append(out, check)
			continue
		}
		if err := verifyAnchor(entry.spec.Path, entry.data, item.link.Anchor); err != nil {
			check.Status = StatusInvalid
			check.Message = fmt.Sprintf("%s %q: %v", item.kind, item.owner, err)
		} else {
			check.Message = fmt.Sprintf("%s %q source anchor resolves", item.kind, item.owner)
		}
		out = append(out, check)
	}
	sortChecks(out)
	return out
}

// ValidateCoverage validates only authored qualification statuses. Passing or
// failing outcome labels are derived by evidence collection, not authored.
func ValidateCoverage(r ontology.Requirement) error {
	if r.Coverage == nil {
		return nil
	}
	c := r.Coverage
	switch c.Status {
	case ontology.CoverageVerified, ontology.CoverageDiscrepancy:
		return fmt.Errorf("requirement %q coverage %q is a computed outcome and cannot be authored", r.ID, c.Status)
	case ontology.CoverageUnsupported, ontology.CoverageUnreachable, ontology.CoverageUnverified:
		// These are the only authored qualification states.
	default:
		return fmt.Errorf("requirement %q has unknown authored coverage status %q", r.ID, c.Status)
	}
	if strings.TrimSpace(c.Rationale) == "" {
		return fmt.Errorf("requirement %q coverage %q requires a non-empty rationale", r.ID, c.Status)
	}
	switch c.Status {
	case ontology.CoverageUnsupported:
		if len(r.SourceLinks) == 0 {
			return fmt.Errorf("requirement %q coverage %q requires at least one source_link", r.ID, c.Status)
		}
	case ontology.CoverageUnreachable:
		if strings.TrimSpace(c.Profile) == "" {
			return fmt.Errorf("requirement %q coverage %q requires a profile", r.ID, c.Status)
		}
		if len(r.SourceLinks) == 0 {
			return fmt.Errorf("requirement %q coverage %q requires at least one source_link", r.ID, c.Status)
		}
	case ontology.CoverageUnverified:
		// Rationale is the complete authored qualification for unverified.
	}
	return nil
}

func resolvePath(g *ontology.Graph, sourcePath string) (string, error) {
	if filepath.IsAbs(sourcePath) {
		return filepath.Clean(sourcePath), nil
	}
	if g == nil || strings.TrimSpace(g.DomainDir) == "" {
		return "", fmt.Errorf("relative source path %q has no domain source root", sourcePath)
	}
	root := g.DomainDir
	if g.SelfHosting {
		var err error
		root, err = selfHostingSourceRoot(g.DomainDir)
		if err != nil {
			return "", err
		}
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve source root %q: %w", root, err)
	}
	resolved := filepath.Clean(filepath.Join(rootAbs, filepath.Clean(sourcePath)))
	rel, err := filepath.Rel(rootAbs, resolved)
	if err != nil {
		return "", fmt.Errorf("resolve source path %q from %q: %w", sourcePath, root, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("relative source path %q escapes its source root", sourcePath)
	}
	return resolved, nil
}

func selfHostingSourceRoot(domainDir string) (string, error) {
	candidate, err := filepath.Abs(domainDir)
	if err != nil {
		return "", fmt.Errorf("resolve self-hosting domain path %q: %w", domainDir, err)
	}
	for {
		info, statErr := os.Stat(filepath.Join(candidate, "go.mod"))
		if statErr == nil {
			if !info.IsDir() {
				return candidate, nil
			}
			return "", fmt.Errorf("self-hosting source root marker %s is not a file", filepath.Join(candidate, "go.mod"))
		}
		if !os.IsNotExist(statErr) {
			return "", fmt.Errorf("inspect self-hosting source root %s: %w", candidate, statErr)
		}
		parent := filepath.Dir(candidate)
		if parent == candidate {
			break
		}
		candidate = parent
	}
	return "", fmt.Errorf("no go.mod source root found above self-hosting domain %s", domainDir)
}

func verifyAnchor(path string, data []byte, anchor string) error {
	if first, last, ok, malformed := lineRange(anchor); malformed {
		return fmt.Errorf("invalid line anchor %q (expected Lx or Lx-Ly)", anchor)
	} else if ok {
		lineCount := bytes.Count(data, []byte{'\n'})
		if len(data) > 0 && data[len(data)-1] != '\n' {
			lineCount++
		}
		if first < 1 || last < first || last > lineCount {
			return fmt.Errorf("line anchor %q is outside source range 1-L%d", anchor, lineCount)
		}
		return nil
	}
	if !isMarkdown(path) {
		return fmt.Errorf("heading anchor %q requires a Markdown source", anchor)
	}
	want := headingSlug(anchor)
	if want == "" {
		return fmt.Errorf("invalid empty heading anchor %q", anchor)
	}
	for _, slug := range markdownHeadingSlugs(string(data)) {
		if slug == want {
			return nil
		}
	}
	return fmt.Errorf("heading anchor %q does not resolve in %s", anchor, path)
}

func lineRange(anchor string) (first, last int, recognized, malformed bool) {
	if !strings.HasPrefix(anchor, "L") {
		return 0, 0, false, false
	}
	rest := strings.TrimPrefix(anchor, "L")
	parts := strings.Split(rest, "-L")
	if len(parts) > 2 {
		return 0, 0, false, true
	}
	if !isASCIIDigits(parts[0]) {
		// A heading such as "Lifecycle" is not a line anchor.
		if len(parts) == 1 && parts[0] != "" {
			return 0, 0, false, false
		}
		return 0, 0, false, true
	}
	first, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false, true
	}
	last = first
	if len(parts) == 2 {
		if !isASCIIDigits(parts[1]) {
			return 0, 0, false, true
		}
		last, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, false, true
		}
	}
	return first, last, true, false
}

func isASCIIDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isMarkdown(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".markdown":
		return true
	default:
		return false
	}
}

func markdownHeadingSlugs(text string) []string {
	lines := strings.Split(text, "\n")
	var out []string
	seen := map[string]int{}
	fenceChar := byte(0)
	fenceSize := 0
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSuffix(lines[i], "\r")
		indent := len(line) - len(strings.TrimLeft(line, " "))
		trimmed := line[indent:]
		if fenceChar != 0 {
			if indent <= 3 && closesFence(trimmed, fenceChar, fenceSize) {
				fenceChar = 0
				fenceSize = 0
			}
			continue
		}
		if indent > 3 {
			continue
		}
		if marker, count, ok := opensFence(trimmed); ok {
			fenceChar, fenceSize = marker, count
			continue
		}
		if title, ok := atxHeading(trimmed); ok {
			out = appendHeadingSlug(out, seen, title)
			continue
		}
		if i+1 < len(lines) {
			next := strings.TrimSuffix(lines[i+1], "\r")
			nextIndent := len(next) - len(strings.TrimLeft(next, " "))
			if nextIndent <= 3 && setextUnderline(strings.TrimSpace(next)) && strings.TrimSpace(line) != "" {
				out = appendHeadingSlug(out, seen, strings.TrimSpace(line))
				i++
			}
		}
	}
	return out
}

func opensFence(line string) (byte, int, bool) {
	if len(line) < 3 || (line[0] != '`' && line[0] != '~') {
		return 0, 0, false
	}
	marker := line[0]
	i := 0
	for i < len(line) && line[i] == marker {
		i++
	}
	return marker, i, i >= 3
}

func closesFence(line string, marker byte, minSize int) bool {
	i := 0
	for i < len(line) && line[i] == marker {
		i++
	}
	return i >= minSize && strings.TrimSpace(line[i:]) == ""
}

func atxHeading(line string) (string, bool) {
	if line == "" || line[0] != '#' {
		return "", false
	}
	i := 0
	for i < len(line) && line[i] == '#' {
		i++
	}
	if i > 6 || (i < len(line) && line[i] != ' ' && line[i] != '\t') {
		return "", false
	}
	title := strings.TrimSpace(line[i:])
	title = strings.TrimRight(title, "# \t")
	return title, true
}

func setextUnderline(line string) bool {
	if len(line) == 0 || (line[0] != '=' && line[0] != '-') {
		return false
	}
	for _, r := range line {
		if r != rune(line[0]) {
			return false
		}
	}
	return true
}

func appendHeadingSlug(out []string, seen map[string]int, title string) []string {
	base := headingSlug(title)
	if base == "" {
		return out
	}
	count := seen[base]
	seen[base] = count + 1
	if count > 0 {
		base += "-" + strconv.Itoa(count)
	}
	return append(out, base)
}

func headingSlug(value string) string {
	value = strings.TrimSpace(strings.TrimPrefix(value, "#"))
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(value) {
		switch {
		case unicode.IsLetter(r), unicode.IsNumber(r), r == '_':
			b.WriteRune(r)
			lastDash = false
		case unicode.IsSpace(r) || r == '-':
			if b.Len() > 0 && !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		default:
			// GitHub-style heading anchors omit punctuation and inline markup.
		}
	}
	return strings.Trim(b.String(), "-")
}

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func specificationSourceLess(a, b ontology.SpecificationSource) bool {
	if a.ID != b.ID {
		return a.ID < b.ID
	}
	if a.Path != b.Path {
		return a.Path < b.Path
	}
	if a.Version != b.Version {
		return a.Version < b.Version
	}
	return a.SHA256 < b.SHA256
}

func sortChecks(checks []Check) {
	sort.Slice(checks, func(i, j int) bool {
		if checks[i].SourceID != checks[j].SourceID {
			return checks[i].SourceID < checks[j].SourceID
		}
		if checks[i].Anchor != checks[j].Anchor {
			return checks[i].Anchor < checks[j].Anchor
		}
		if checks[i].Path != checks[j].Path {
			return checks[i].Path < checks[j].Path
		}
		if checks[i].Status != checks[j].Status {
			return checks[i].Status < checks[j].Status
		}
		if checks[i].Message != checks[j].Message {
			return checks[i].Message < checks[j].Message
		}
		if checks[i].ExpectedSHA256 != checks[j].ExpectedSHA256 {
			return checks[i].ExpectedSHA256 < checks[j].ExpectedSHA256
		}
		if checks[i].ActualSHA256 != checks[j].ActualSHA256 {
			return checks[i].ActualSHA256 < checks[j].ActualSHA256
		}
		return checks[i].Version < checks[j].Version
	})
}
