package docbundle

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Layout is the single naming contract for human-readable generated documents.
// Languages is empty for the legacy, implicit one-language layout.
type Layout struct {
	Languages       []string
	DefaultLanguage string
}

// NewLayout validates path-safety properties needed by output naming;
// semantic/catalog validation belongs to the manifest/schema layer.
func NewLayout(languages []string, defaultLanguage string) (Layout, error) {
	layout := Layout{Languages: append([]string(nil), languages...), DefaultLanguage: defaultLanguage}
	seen := make(map[string]bool, len(languages))
	hasDefault := false
	for _, language := range languages {
		if !safeLanguageSegment(language) {
			return Layout{}, fmt.Errorf("language %q is not safe for generated output paths", language)
		}
		folded := strings.ToLower(language)
		if seen[folded] {
			return Layout{}, fmt.Errorf("duplicate language %q", language)
		}
		seen[folded] = true
		if language == defaultLanguage {
			hasDefault = true
		}
	}
	if len(languages) > 1 && !hasDefault {
		return Layout{}, fmt.Errorf("default language %q is not in the declared language set", defaultLanguage)
	}
	if len(languages) == 1 && defaultLanguage != "" && defaultLanguage != languages[0] {
		return Layout{}, fmt.Errorf("default language %q differs from the only declared language %q", defaultLanguage, languages[0])
	}
	return layout, nil
}

func (l Layout) Multilingual() bool { return len(l.Languages) > 1 }

// LanguagesForViews is the ordered view set. Legacy graphs are represented by
// one empty language, preserving their unsuffixed paths and authored text.
func (l Layout) LanguagesForViews() []string {
	if len(l.Languages) == 0 {
		return []string{""}
	}
	return append([]string(nil), l.Languages...)
}

// DocumentPath applies the shared suffix rule to a domain document path such
// as docs/gen/REQUIREMENTS.md. Single-language output preserves its old path.
func (l Layout) DocumentPath(basePath, language string) (string, error) {
	if !safeRelativePath(basePath) || !strings.HasSuffix(basePath, ".md") {
		return "", fmt.Errorf("invalid generated document path %q", basePath)
	}
	if !l.hasLanguage(language) {
		return "", fmt.Errorf("language %q is not declared in the output layout", language)
	}
	if !l.Multilingual() {
		return basePath, nil
	}
	return strings.TrimSuffix(basePath, ".md") + "." + language + ".md", nil
}

// SpecIndexPath returns the one-language legacy name or a language-specific
// multilingual index name.
func (l Layout) SpecIndexPath(language string) (string, error) {
	return l.DocumentPath("docs/gen/SPEC.md", language)
}

// SpecShardPath maps a source package shard to its language-local output path.
// packagePath is a relative .md path and may contain nested package segments.
func (l Layout) SpecShardPath(language, packagePath string) (string, error) {
	if !safeRelativePath(packagePath) || !strings.HasSuffix(packagePath, ".md") {
		return "", fmt.Errorf("invalid SPEC package shard path %q", packagePath)
	}
	if !l.hasLanguage(language) {
		return "", fmt.Errorf("language %q is not declared in the output layout", language)
	}
	if l.Multilingual() {
		return "docs/gen/spec/" + language + "/" + packagePath, nil
	}
	return "docs/gen/spec/" + packagePath, nil
}

// IsSpecPath reports whether a candidate under genDir is a SPEC index or
// shard for this layout. It lets a non---spec run preserve only the currently
// declared SPEC bundle while cleanup still removes outputs for removed locales.
func (l Layout) IsSpecPath(genDir, candidate string) bool {
	relative, err := filepath.Rel(genDir, candidate)
	if err != nil {
		return false
	}
	relative = filepath.ToSlash(relative)
	if !safeRelativePath(relative) || !strings.HasSuffix(relative, ".md") {
		return false
	}
	if relative == "SPEC.md" && !l.Multilingual() {
		return true
	}
	if stem, language, ok := localizedStem(relative); ok && stem == "SPEC.md" && l.Multilingual() && l.hasLanguage(language) {
		return true
	}
	if strings.HasPrefix(relative, "spec/") {
		shard := strings.TrimPrefix(relative, "spec/")
		if !l.Multilingual() {
			return safeRelativePath(shard) && strings.HasSuffix(shard, ".md")
		}
		separator := strings.IndexByte(shard, '/')
		return separator > 0 && l.hasLanguage(shard[:separator])
	}
	return false
}

// IsSpecOutputPath recognizes every supported single/multilingual SPEC index
// and shard name relative to docs/gen. It identifies path candidates only;
// deleting or overwriting an existing shard also requires its generated banner.
func IsSpecOutputPath(relative string) bool {
	if !safeRelativePath(relative) || !strings.HasSuffix(relative, ".md") {
		return false
	}
	if relative == "SPEC.md" {
		return true
	}
	if stem, locale, ok := localizedStem(relative); ok && stem == "SPEC.md" && knownLocale(locale) {
		return true
	}
	return strings.HasPrefix(relative, "spec/")
}

// CrystalPath names the generated boot crystal for a language. The default
// locale owns the conventional CLAUDE.md; other locales get CLAUDE.<lang>.md.
func (l Layout) CrystalPath(language string) (string, error) {
	if !l.hasLanguage(language) {
		return "", fmt.Errorf("language %q is not declared in the output layout", language)
	}
	if !l.Multilingual() || language == l.DefaultLanguage {
		return "CLAUDE.md", nil
	}
	return "CLAUDE." + language + ".md", nil
}

// OwnedDomainPath reports whether a domain-relative path is in the closed set
// of generated docs this engine owns. It is narrower than a recursive glob.
func OwnedDomainPath(relative string) bool {
	relative = filepath.ToSlash(filepath.Clean(filepath.FromSlash(relative)))
	if !strings.HasPrefix(relative, "docs/gen/") {
		return false
	}
	sub := strings.TrimPrefix(relative, "docs/gen/")
	if strings.HasPrefix(sub, "spec/") {
		return strings.HasSuffix(sub, ".md") && safeRelativePath(sub)
	}
	for _, dir := range []string{"thinking", "tools"} {
		prefix := dir + "/"
		if strings.HasPrefix(sub, prefix) {
			rest := strings.TrimPrefix(sub, prefix)
			return strings.HasSuffix(rest, ".md") && !strings.Contains(rest, "/") && safeRelativePath(rest)
		}
	}
	if strings.Contains(sub, "/") {
		return false
	}
	if _, ok := domainMachineFiles[sub]; ok {
		return true
	}
	stem, locale, ok := localizedStem(sub)
	if ok && knownLocale(locale) {
		_, found := domainDocumentStems[stem]
		return found
	}
	_, ok = domainDocumentStems[sub]
	return ok
}

// OwnedProjectPath reports paths in the shared framework output inventory.
func OwnedProjectPath(relative string) bool {
	relative = filepath.ToSlash(filepath.Clean(filepath.FromSlash(relative)))
	if relative == "framework/GLOSSARY.md" {
		return true
	}
	local := strings.TrimPrefix(relative, "framework/")
	if stem, locale, ok := localizedStem(local); ok && stem == "GLOSSARY.md" && knownLocale(locale) {
		return true
	}
	local = strings.TrimPrefix(relative, "framework/tools/")
	return strings.HasPrefix(relative, "framework/tools/") && strings.HasSuffix(local, ".md") && safeRelativePath(local) && !strings.Contains(local, "/")
}

// OwnedCrystalPath recognizes only non-default localized crystal names. The
// conventional root crystal aliases have independent durable-note handling.
func OwnedCrystalPath(relative string) bool {
	if strings.Contains(filepath.ToSlash(relative), "/") {
		return false
	}
	stem, locale, ok := localizedStem(filepath.Base(filepath.Clean(filepath.FromSlash(relative))))
	return ok && stem == "CLAUDE.md" && knownLocale(locale)
}

// LocalizedOutputLanguage extracts a supported locale suffix from a generated
// Markdown basename. The path still needs to pass an ownership predicate and
// its contents a generated-banner check before deletion or overwrite.
func LocalizedOutputLanguage(relative string) (string, bool) {
	_, language, ok := localizedStem(filepath.Base(filepath.Clean(filepath.FromSlash(relative))))
	return language, ok && knownLocale(language)
}

// DomainCandidates enumerates only known generator-owned files under docs/gen,
// including localized names and every package shard.
func DomainCandidates(genDir string) ([]string, error) {
	var candidates []string
	for name := range domainMachineFiles {
		candidates = append(candidates, filepath.Join(genDir, name))
	}

	for stem := range domainDocumentStems {
		candidates = append(candidates, filepath.Join(genDir, stem))
		base := strings.TrimSuffix(stem, ".md")
		for _, locale := range knownLocales {
			candidates = append(candidates, filepath.Join(genDir, base+"."+locale+".md"))
		}
	}
	for _, sub := range []string{"thinking", "tools", "spec"} {
		root := filepath.Join(genDir, sub)
		if sub == "spec" {
			err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if !entry.IsDir() && strings.EqualFold(filepath.Ext(path), ".md") {
					candidates = append(candidates, path)
				}
				return nil
			})
			if err != nil && !os.IsNotExist(err) {
				return nil, fmt.Errorf("walk generated SPEC shards: %w", err)
			}
			continue
		}
		matches, err := filepath.Glob(filepath.Join(root, "*.md"))
		if err != nil {
			return nil, fmt.Errorf("glob generated %s documents: %w", sub, err)
		}
		candidates = append(candidates, matches...)
	}
	return uniqueSorted(candidates), nil
}

// UnknownLocaleSuffixedMarkdown reports a safe but unsupported locale-looking
// suffix, such as `thinking/topic.es.md`. Such names are outside the current
// generated inventory and must not be removed merely because they live in a
// generator-owned subdirectory.
func UnknownLocaleSuffixedMarkdown(relative string) bool {
	base := filepath.Base(filepath.Clean(filepath.FromSlash(relative)))
	_, locale, ok := localizedStem(base)
	return ok && safeLanguageSegment(locale) && !knownLocale(locale)
}

// ProjectCandidates enumerates the closed project-shared framework inventory.
func ProjectCandidates(frameworkDir string) ([]string, error) {
	candidates := []string{filepath.Join(frameworkDir, "GLOSSARY.md")}
	for _, locale := range knownLocales {
		candidates = append(candidates, filepath.Join(frameworkDir, "GLOSSARY."+locale+".md"))
	}
	matches, err := filepath.Glob(filepath.Join(frameworkDir, "tools", "*.md"))
	if err != nil {
		return nil, fmt.Errorf("glob generated framework tools: %w", err)
	}
	candidates = append(candidates, matches...)
	return uniqueSorted(candidates), nil
}

// CrystalCandidates enumerates the fixed supported set of localized boot
// crystal paths that a locale transition may retire.
func CrystalCandidates(directory string) []string {
	candidates := make([]string, 0, len(knownLocales))
	for _, language := range knownLocales {
		candidates = append(candidates, filepath.Join(directory, "CLAUDE."+language+".md"))
	}
	return candidates
}

// ReportCandidates enumerates the evidence/finding Markdown projections and
// their one shared raw JSON store, which are written by hotam evidence rather
// than gen-spec.
func ReportCandidates(genDir string) []string {
	candidates := []string{filepath.Join(genDir, "evidence.json")}
	for _, stem := range []string{"EVIDENCE", "FINDINGS"} {
		candidates = append(candidates, filepath.Join(genDir, stem+".md"))
		for _, language := range knownLocales {
			candidates = append(candidates, filepath.Join(genDir, stem+"."+language+".md"))
		}
	}
	return uniqueSorted(candidates)
}

// ResolveOutputPath confines domain-relative docs/gen keys and project-relative
// framework keys to the renderer's closed generated inventory.
func ResolveOutputPath(repoRoot, domainDir, relative string) (string, error) {
	if !safeRelativePath(relative) {
		return "", fmt.Errorf("path is empty, unsafe, or not slash-normalized")
	}
	native := filepath.FromSlash(relative)
	if filepath.IsAbs(native) || filepath.VolumeName(native) != "" {
		return "", fmt.Errorf("path must be repository-relative")
	}
	root := repoRoot
	switch {
	case OwnedDomainPath(relative) && !reservedSeparateOutput(relative):
		root = domainDir
	case OwnedProjectPath(relative):
	default:
		return "", fmt.Errorf("path is outside this renderer's generated document inventory")
	}
	path := filepath.Clean(filepath.Join(root, native))
	repoRelative, err := filepath.Rel(repoRoot, path)
	if err != nil {
		return "", err
	}
	if repoRelative == ".." || strings.HasPrefix(repoRelative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes the repository root")
	}
	return path, nil
}

func reservedSeparateOutput(domainPath string) bool {
	if domainPath == "docs/gen/graph.json" || domainPath == "docs/gen/evidence.json" || strings.HasPrefix(domainPath, "docs/gen/spec/") {
		return true
	}
	if !strings.HasPrefix(domainPath, "docs/gen/") {
		return false
	}
	name := strings.TrimPrefix(domainPath, "docs/gen/")
	if strings.Contains(name, "/") {
		return false
	}
	isReserved := func(stem string) bool {
		return stem == "SPEC.md" || stem == "EVIDENCE.md" || stem == "FINDINGS.md"
	}
	if isReserved(name) {
		return true
	}
	stem, locale, ok := localizedStem(name)
	return ok && knownLocale(locale) && isReserved(stem)
}

// StalePaths selects only obsolete paths from a previously classified owned
// inventory. Exempt paths preserve expensive projections deliberately skipped
// by a run (for example SPEC on a non---spec generation).
func StalePaths(candidates, current, exempt []string) []string {
	currentSet, exemptSet := pathSet(current), pathSet(exempt)
	var stale []string
	for _, candidate := range candidates {
		clean := filepath.Clean(candidate)
		if !currentSet[clean] && !exemptSet[clean] {
			stale = append(stale, clean)
		}
	}
	return uniqueSorted(stale)
}

func pathSet(paths []string) map[string]bool {
	set := make(map[string]bool, len(paths))
	for _, path := range paths {
		set[filepath.Clean(path)] = true
	}
	return set
}

func (l Layout) hasLanguage(language string) bool {
	if len(l.Languages) == 0 {
		return language == ""
	}
	for _, declared := range l.Languages {
		if language == declared {
			return true
		}
	}
	return false
}

func localizedStem(name string) (stem, locale string, ok bool) {
	if !strings.HasSuffix(name, ".md") {
		return "", "", false
	}
	withoutExt := strings.TrimSuffix(name, ".md")
	i := strings.LastIndexByte(withoutExt, '.')
	if i <= 0 || i == len(withoutExt)-1 {
		return "", "", false
	}
	return withoutExt[:i] + ".md", withoutExt[i+1:], true
}

func knownLocale(language string) bool {
	for _, locale := range knownLocales {
		if locale == language {
			return true
		}
	}
	return false
}

func safeLanguageSegment(language string) bool {
	if language == "" || language == "." || language == ".." || strings.Contains(language, "..") {
		return false
	}
	for _, r := range language {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			continue
		}
		return false
	}
	return true
}

func safeRelativePath(relative string) bool {
	if relative == "" || strings.ContainsAny(relative, "\\:") || strings.HasPrefix(relative, "/") {
		return false
	}
	for _, segment := range strings.Split(relative, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

func uniqueSorted(items []string) []string {
	sort.Strings(items)
	out := items[:0]
	for _, item := range items {
		if len(out) == 0 || out[len(out)-1] != item {
			out = append(out, item)
		}
	}
	return out
}

var knownLocales = []string{"en", "ru", "zh"}

var domainMachineFiles = map[string]struct{}{"evidence.json": {}, "graph.json": {}}

var domainDocumentStems = map[string]struct{}{
	"AGENT-CONTEXT.md": {}, "CONSTITUTION.md": {}, "COVERAGE.md": {}, "DECISIONS.md": {},
	"ENTITIES.md": {}, "ENGINE-VERSION.md": {}, "EVIDENCE.md": {}, "FINDINGS.md": {},
	"FRAMEWORK-INVARIANTS.md": {}, "GLOSSARY.md": {}, "HISTORY.md": {}, "MODELS.md": {},
	"OPEN.md": {}, "PIPELINE.md": {}, "REPO-MAP.md": {}, "REQUIREMENTS.md": {}, "SPEC.md": {},
	"TENSIONS.md": {}, "TRACEABILITY.md": {}, "UNENFORCED.md": {}, "atoms-check.md": {},
	"atoms-discipline.md": {}, "atoms-operator.md": {}, "atoms-substrate.md": {}, "live-state.md": {},
}
