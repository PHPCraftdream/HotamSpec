package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	ontologyvendor "github.com/PHPCraftdream/HotamSpec/internal/ontology/vendor"
	recordervendor "github.com/PHPCraftdream/HotamSpec/internal/recorder/vendor"
)

// cmdUpgrade implements `hotam upgrade [--domain <path>] [--today YYYY-MM-DD]`:
// the one-shot post-engine-upgrade refresh for a consumer domain. After the
// engine's canon advances, check_recorder_current / check_ontology_vendor_current
// (stale vendored copies), a stale engine-generated spec/registrydump/main.go,
// and check_domain_claude_md_current / stale docs/gen/* all fire at once;
// today the operator had to remember to run vendor-recorder, vendor-ontology,
// scaffold-registrydump and gen-spec manually. This command chains exactly
// those steps, SKIPPING (never failing) the ones the domain does not use:
//   - vendoring is skipped entirely when spec/go.mod is absent (the domain's
//     spec/ tree is not its own Go module — vendorRecorder/vendorOntology
//     would refuse) or when the vendored copy itself never existed (upgrade
//     refreshes an existing vendor setup, it does not introduce one);
//   - registrydump is refreshed ONLY when it exists AND carries the
//     engine's do-not-edit banner (registrydumpBanner): a file without the
//     banner is treated as hand-modified and left byte-identical, reported
//     in the summary instead of overwritten;
//   - docs + the project crystal are always regenerated via the same genSpec
//     call `hotam gen-spec` makes, with includeSpec=specRenderNeeded(domainDir)
//     — the same gate `hotam land` uses: SPEC.md is re-rendered exactly when
//     check_spec_md_current can fire (a committed SPEC.md exists, or the
//     manifest declares discipline:"full"), keeping the render cheap for
//     domains where the check is an honest no-op. Then all invariant
//     violations are printed exactly like
//     `hotam all-violations` (informational here: the exit code is 0 unless
//     a real error occurs).
//
// Idempotent: every upgrade-owned write goes through writeFileIfChanged, so
// a second run on an already-current domain rewrites nothing.
func cmdUpgrade(args []string) error {
	fs := newFlagSet("upgrade")
	domain := fs.String("domain", "", "domain directory (default: "+defaultDomainRel+")")
	todayFlag := fs.String("today", "", "date in YYYY-MM-DD format (default: system date) — threaded into the regenerated docs and crystal, same as gen-spec --today")
	fs.Parse(args)

	domainDir, err := resolveDomain(*domain)
	if err != nil {
		return err
	}
	today := *todayFlag
	if today == "" {
		today = time.Now().Format("2006-01-02")
	}
	claudeMDPath := resolveClaudeMDPath(domainDir, "")

	lines, err := runUpgradeSteps(domainDir, claudeMDPath, today)
	if err != nil {
		return err
	}
	fmt.Printf("hotam upgrade — %s\n", relPathForDisplay(domainDir))
	for _, l := range lines {
		fmt.Println(l)
	}

	// Print the final all-violations result the SAME way cmdAllViolations
	// does — but violations are informational output here, never an exit
	// code: the operator asked for a refresh, and the violation list tells
	// them what (if anything) still needs a manual step (e.g. the
	// hand-modified registrydump case above).
	violations, err := allViolations(domainDir)
	if err != nil {
		return err
	}
	for _, v := range violations {
		fmt.Printf("[%s] %s: %s\n", v.Check, v.ID, v.Message)
	}
	if len(violations) > 0 {
		fmt.Fprintf(os.Stderr, "%d violation(s) found\n", len(violations))
		return printAdvisorySection(domainDir)
	}
	fmt.Println("0 violations — graph clean")
	return printAdvisorySection(domainDir)
}

// runUpgradeSteps performs the refresh and returns human-readable per-step
// summary lines ("refreshed: <path>", "skipped: <path> (<reason>)", ...) so
// tests can assert behavior without parsing stdout.
func runUpgradeSteps(domainDir, claudeMDPath, today string) ([]string, error) {
	specDir := filepath.Join(domainDir, "spec")
	_, specGoModErr := os.Stat(filepath.Join(specDir, "go.mod"))
	hasSpecModule := specGoModErr == nil

	var lines []string

	// Recorder re-vendor: only when the domain already vendors it.
	recorderTarget := filepath.Join(specDir, "hotamspec", "hotamspec.go")
	switch {
	case !hasSpecModule:
		lines = append(lines, "skipped: spec/hotamspec (no spec/go.mod — domain has no authored spec/ Go module)")
	case !fileExists(recorderTarget):
		lines = append(lines, "skipped: spec/hotamspec (never vendored — run `hotam vendor-recorder` to introduce it)")
	case fileEquals(recorderTarget, []byte(recordervendor.Source())):
		lines = append(lines, "already current: spec/hotamspec/hotamspec.go")
	default:
		if _, err := vendorRecorder(domainDir); err != nil {
			return lines, fmt.Errorf("upgrade: re-vendor recorder: %w", err)
		}
		lines = append(lines, "refreshed: spec/hotamspec/hotamspec.go")
	}

	// Ontology mirror re-vendor: only when the domain already vendors it.
	ontologyReq := filepath.Join(specDir, "hotamontology", "requirement.go")
	switch {
	case !hasSpecModule:
		lines = append(lines, "skipped: spec/hotamontology (no spec/go.mod — domain has no authored spec/ Go module)")
	case !fileExists(ontologyReq):
		lines = append(lines, "skipped: spec/hotamontology (never vendored — run `hotam vendor-ontology` to introduce it)")
	case ontologyCurrent(specDir):
		lines = append(lines, "already current: spec/hotamontology/")
	default:
		if _, err := vendorOntology(domainDir); err != nil {
			return lines, fmt.Errorf("upgrade: re-vendor ontology: %w", err)
		}
		lines = append(lines, "refreshed: spec/hotamontology/")
	}

	// Registrydump refresh: only when it exists AND is engine-generated
	// (carries the do-not-edit banner). A banner-less file is hand-modified —
	// never overwritten, reported so the operator knows it will drift from
	// the current template.
	rdTarget := filepath.Join(specDir, "registrydump", "main.go")
	switch {
	case !hasSpecModule || !fileExists(rdTarget):
		lines = append(lines, "skipped: spec/registrydump/main.go (not present — nothing generated to refresh)")
	default:
		rd, err := os.ReadFile(rdTarget)
		if err != nil {
			return lines, fmt.Errorf("upgrade: read %s: %w", rdTarget, err)
		}
		if !strings.HasPrefix(string(rd), registrydumpBanner) {
			lines = append(lines, "kept (hand-modified, no engine banner): spec/registrydump/main.go — NOT overwritten")
			break
		}
		modulePath, err := readGoModModulePath(filepath.Join(specDir, "go.mod"))
		if err != nil {
			return lines, fmt.Errorf("upgrade: registrydump refresh: %w", err)
		}
		expected := []byte(registrydumpSource(modulePath, declaresStakeholders(specDir)))
		changed, err := writeFileIfChanged(rdTarget, expected)
		if err != nil {
			return lines, fmt.Errorf("upgrade: registrydump refresh: %w", err)
		}
		if changed {
			lines = append(lines, "refreshed: spec/registrydump/main.go")
		} else {
			lines = append(lines, "already current: spec/registrydump/main.go")
		}
	}

	// Docs + crystal: the same genSpec call `hotam land` makes, with the same
	// defaults (profile resolved from the domain's manifest, crystal path via
	// resolveClaudeMDPath) and the same includeSpec gate — specRenderNeeded
	// re-renders SPEC.md exactly when check_spec_md_current can fire (a
	// committed SPEC.md exists or discipline is "full"); without it, one
	// upgrade pass leaves such a domain non-converged (stale SPEC.md, and a
	// crystal whose LIVE-STATE snapshot disagrees with the post-pass state).
	written, removed, err := genSpec(domainDir, claudeMDPath, today, "", specRenderNeeded(domainDir))
	if err != nil {
		return lines, fmt.Errorf("upgrade: regenerate docs: %w", err)
	}
	lines = append(lines, fmt.Sprintf("regenerated: docs + crystal (%d file(s) written, %d stale removed)", len(written), len(removed)))
	return lines, nil
}

// ontologyCurrent reports whether all three vendored ontology mirror files
// already byte-match the engine's canon — the idempotency check that lets a
// second upgrade run skip the vendorOntology write entirely.
func ontologyCurrent(specDir string) bool {
	pairs := [][2]string{
		{filepath.Join(specDir, "hotamontology", "requirement.go"), ontologyvendor.RequirementSource()},
		{filepath.Join(specDir, "hotamontology", "registry.go"), ontologyvendor.RegistrySource()},
		{filepath.Join(specDir, "hotamontology", "stakeholder.go"), ontologyvendor.StakeholderSource()},
	}
	for _, p := range pairs {
		if !fileEquals(p[0], []byte(p[1])) {
			return false
		}
	}
	return true
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func fileEquals(path string, want []byte) bool {
	got, err := os.ReadFile(path)
	return err == nil && bytes.Equal(got, want)
}

// writeFileIfChanged writes want to path only when the on-disk bytes differ,
// so an already-current domain's second upgrade run touches nothing (mtimes
// included). Returns whether a write happened.
func writeFileIfChanged(path string, want []byte) (bool, error) {
	if fileEquals(path, want) {
		return false, nil
	}
	return true, writeFileMkdir(path, want)
}
