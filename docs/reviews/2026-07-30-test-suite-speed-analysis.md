# Test-suite speed analysis — 2026-07-30

Read-only investigation. No source file was modified. Everything below is either a
measurement taken today, a measurement read out of a prior session's artifacts under
`.scratch/`, or an explicitly-labelled inference.

Artifacts produced by this investigation (both under `.scratch/`, untracked):

- `.scratch/cpu-395-allviol.prof` (в репозитории HotamSpec) — fresh CPU profile, today,
  post-`0933ddb`, post-#393/#394.
- `.scratch/hotam-395.test.exe` (в репозитории HotamSpec) — the matching test binary
  (needed to symbolize the profile above).

Prior-wave commits read in full before starting, so nothing here re-proposes them:
`fbb1b34` (build-output exclusion from `hashPackageInputs`), `fdb8bfe` (in-process
`perFileHashCache` + `testFileParseCache`), `0933ddb` (process-lifetime
`coverageRunCache`), plus their two predecessors `1fc9440` (`-short` gating of RAC3
subprocess tests, `-vet=off` in the compile cache) and `9cda1f2`
(`EnsureContentAddressedFixture`).

---

## 1. Measured facts

### 1.1 Compiling the repo's own test binaries is ~1% of the suite. The build cache is fine.

Measured today, warm cache, `репозиторий HotamSpec`:

```
go build ./...                                 3.6 s
go test ./... -run '^ZZZ_NoSuchTest$' -count=1 5.6 s   (20 test binaries built, linked,
                                                        started, zero tests executed)
```

`GOCACHE=внешний каталог сборки Go`, `GOFLAGS=-mod=mod`, `GOMAXPROCS` unset,
`nproc = 16`. No `go clean` is invoked anywhere in `Makefile` or
`.github/workflows/`.

**Every second of the ~5-10 minute suite is test *execution*, not compilation of this
repo.** Question 4 of the brief ("is the build cache being invalidated more than
necessary?") is answered: no, and it would not matter if it were — the entire cold-ish
build path is 5.6 s.

### 1.2 `-count=1` is free for every package that matters.

Across 60+ full-suite logs recorded under `.scratch/*.log` this session and the
previous ones, the packages that ever print `(cached)` are:

| package | times seen `(cached)` |
|---|---|
| `internal/registry`, `internal/recorder/vendor`, `internal/recorder/canon`, `internal/query`, `internal/methodology`, `internal/fsio`, `internal/freshness` | 14 each |
| `internal/paths`, `internal/graphfacts` | 13 each |
| `internal/selfspec` | 10 |
| `internal/proposal`, `internal/ontology` | 4 each |
| `internal/selfcheck`, `internal/loader` | 3 each |
| `internal/gate` | 2 |
| **`cmd/hotam`, `internal/generator`, `internal/invariants`, `internal/diagnose`** | **0 — never** |

The four packages that dominate wall time are structurally uncacheable by Go's test
cache, and `internal/gate` (the fifth) cached twice in 60+ runs. Dropping `-count=1`
would therefore convert at most the cheap/medium tail into cache hits — roughly 50-60 s
of *package* time — and would save close to **zero wall time**, because that tail
already runs concurrently with `cmd/hotam` (see 1.4) and finishes long before it.

**Recommendation preview: keep `-count=1` unconditionally. The zero-trust discipline
costs nothing here.** This is the opposite of what the brief hypothesised, and it is
measured, not assumed.

### 1.3 `-short` saves 6% in `cmd/hotam`, not the "quick signal" the Makefile promises.

From the prior wave's own artifacts, both clean runs on the same tree:

```
.scratch/cmdhotam-full-clean-round2.stdout   ok  cmd/hotam  211.793s
.scratch/cmdhotam-short-round2.stdout        ok  cmd/hotam  198.938s   (-6.1%)
```

`cmd/hotam` carries 50 `testing.Short()` gates — they gate exactly the
subprocess-spawning e2e/killswitch tests, which are *not* on the critical path. The
expensive work (`gen-spec`, `status`, `sync-self`, `all-violations` driven in-process
against a copy of the real self-hosting domain) is deliberately not gated, and is what
sets the wall floor. `make test-fast` is a much weaker signal than its comment claims.

### 1.4 Cross-package parallelism is already good; within-package parallelism is already high.

`.scratch/full-suite-w0.5-392.log`: per-package times sum to **1,969.8 s** against a
wall of roughly 570-600 s → ~3.3x overlap. `go test` defaults to `-p GOMAXPROCS` = 16
here, and nothing in `Makefile` or `.github/workflows/ci.yml` passes `-p` or
`-parallel`.

Inside `cmd/hotam`: 261 `t.Parallel()` calls across 283 `func Test*` declarations in 42
`_test.go` files. `.scratch/slowest-tests.txt` (captured from a 766 s package run) lists
per-test elapsed times summing to roughly **6,000 s** — ~8x overlap inside one binary.

**Neither `-p` tuning nor broader `t.Parallel()` adoption is the lever.** The package is
already saturating; the problem is total work, not scheduling.

### 1.5 The 572 s `cmd/hotam` figure in the brief is a *contended* measurement.

Two full-suite runs on the same day, ~2 h apart:

| package | `task389-full-suite.log` (10:27) | `full-suite-w0.5-392.log` (12:47) | inflation |
|---|---|---|---|
| `internal/fsio` | 1.97 s | 11.28 s | **5.7x** |
| `internal/selfspec` | 43.98 s | 205.08 s | **4.7x** |
| `internal/diagnose` | 58.33 s | 200.54 s | 3.4x |
| `internal/invariants` | 82.00 s | 257.13 s | 3.1x |
| `internal/generator` | 104.02 s | 277.50 s | 2.7x |
| `internal/gate` | 114.99 s | 293.00 s | 2.5x |
| `cmd/hotam` | 323.50 s | 572.85 s | 1.8x |
| **sum of all packages** | **762.9 s** | **1,969.8 s** | 2.6x |

Packages that gained *no* new tests between those two runs inflated **more** than
`cmd/hotam` did. This is machine contention, not test growth. The honest uncontended
`cmd/hotam` baseline for 2026-07-30 is **~323 s**, and the honest full-suite baseline is
**~5-6 min**, not 8-10.

### 1.6 The in-process caches from `fdb8bfe`/`0933ddb` are working — very well.

Measured today, same tree, back to back:

```
go test ./cmd/hotam -count=1 -run '^TestAllViolations_SmokeNoPanic$'          66.06 s
go test ./cmd/hotam -count=1 -run '^(TestAllViolations_SmokeNoPanic|
    TestWhatNow_SmokeNoPanic|TestCmdStatus_SmokeNoPanicOnRealDomain|
    TestCmdStatus_JSONFlagNoPanic)$'                                          86.99 s
```

Three *additional* full engine passes over the real self-hosting domain cost **20.9 s
combined (~7 s each)** on top of the first pass's 66 s — a ~90% marginal saving. So
`cmd/hotam`'s cost is **not** "N tests x one full engine pass". It is
**one expensive cold warm-up per process**, plus everything that cannot share it.

### 1.7 Fresh profile: `go test -c` is now the single dominant cost.

`.scratch/cpu-395-allviol.prof`, captured today from `TestAllViolations_SmokeNoPanic`
alone (Duration 118.44 s, Total samples 297.54 s = 251% — i.e. ~2.5 threads busy, and
98.3% of all samples sit in `runtime.cgocall`, which on Windows is the syscall entry
point; **read this as a wall-time/blocked-thread profile, not a CPU profile**):

| symbol | cum | cum% |
|---|---|---|
| `internal/gate.doCompileTestBinary` (all of it `os/exec.(*Cmd).Run`) | 125.33 s | **42.12%** |
| `internal/invariants.checkScenarioExecutesImpl.func1` | 86.26 s | 29.0% |
| ↳ `internal/gate.RunVerifiedByTestRecording` | 75.45 s | 25.4% |
| `internal/invariants.verifiedByTestPassesViolation` → `gate.RunVerifiedByTest` | 73.56 s | 24.7% |
| ↳ `internal/gate.runGoTest` | 59.90 s | 20.1% |
| `internal/gate.hashPackageInputs` | 24.42 s | 8.2% |
| `path/filepath.walkDir` (flat-heaviest Go frame) | 24.87 s | 8.4% |

For comparison, `.scratch/pprof-top-cum.txt` (2026-07-28, pre-`0933ddb`) had
`scenarioExecutesImplViolation` at 8.26% of a whole-package run and `hashPackageInputs`
nowhere near the top after `fdb8bfe`. **The prior wave's fixes held; the bottleneck
moved.** `hashPackageInputs` is down from the 20.6-28.8% that motivated `fbb1b34` to
8.2%.

### 1.8 Why `go test -c` is called so often, and what one call costs.

`domains/hotam-spec-self/graph.json` carries **18 `verified_by` entries across 5
distinct packages**: `internal/invariants` ×8, `internal/generator` ×4,
`internal/proposal` ×3, `internal/ontology` ×2, `internal/loader` ×1.

The compile-cache key is `(moduleRoot, pkgPattern, coverPkgPattern)`
(`internal/gate/compile_cache.go`, `type compileCacheKey`), and
`checkScenarioExecutesImpl` supplies a **per-entry** `coverPkgFile`. So one cold engine
pass needs ≤5 compiles for the plain-verdict path plus up to 18 more for the
coverage-recording path — **up to ~23 `go test -c` invocations per process.**

Direct measurement today (very noisy — see §4; best-of-3 shown, worst observed in
parentheses):

```
go test -c -vet=off -o <tmp> ./internal/invariants     4.2 s   (13.4 s)
go test -c -vet=off -o <tmp> ./internal/ontology      13.9 s  (123.5 s)
go list ./internal/ontology                            1.35 s   -- bare `go` tool startup
```

Output sizes: 5.2 MB (`ontology`), 8.8 MB (`invariants`), 9.3 MB (`generator`),
9.6 MB (`invariants` with `-coverpkg`).

Cross-check against §1.1: `go test ./... -run '^ZZZ$'` links **and starts** all 20 test
binaries in 5.6 s total. So the 4-14 s per `go test -c` is **not compilation** — Go's
build cache already has the compiled and linked artifacts. It is `go` command startup
(~1.4 s floor) plus writing a 5-9 MB `.exe` to disk, which on this Windows host means an
antivirus scan. `cmd/hotam/testbinary_test.go:19-24` already documents exactly this
("a single `go build` of this module is dominated by antivirus scanning of the freshly
written exe").

---

## 2. Root causes

### RC1 — The compiled-test-binary cache is process-scoped, so every process recompiles everything.

`internal/gate/compile_cache.go:508-513` — `ensureCompileTmpDir` does
`os.MkdirTemp("", "hotam-compile-")` **once per process**; `CleanupCompileCache`
(`internal/gate/compile_cache.go:217`) deletes that directory at process exit; the
`compileCache` map itself is an in-memory `sync.Map`. The file's own doc comment
(`internal/gate/compile_cache.go:79-88`) states the design intent plainly: "the cache is
PROCESS-LIFETIME".

Consequence: **zero cross-process reuse.** Each of the ~23 `go test -c` calls in §1.8 is
paid again, in full, by:

- `cmd/hotam`'s test binary,
- `internal/gate`'s test binary,
- `internal/generator`'s test binary,
- `internal/invariants`'s test binary,
- `internal/diagnose`'s test binary,
- `internal/selfspec`'s test binary,
- **and every single `hotam.exe` child process a `cmd/hotam` test spawns.**

This is why the five heavy packages have such similar shapes (115 s / 104 s / 82 s /
58 s / 44 s uncontended): each is largely the same cold engine warm-up, repeated in its
own OS process.

### RC2 — Process fan-out inside `cmd/hotam`: each child is a fresh cold engine.

`cmd/hotam` builds the CLI binary exactly once (`cmd/hotam/testbinary_test.go:26-52`,
`buildSharedHotamBinary` + `sync.Once`) — that part is already optimal and should not be
re-proposed. But that binary is then **invoked as a subprocess 48 times** across the
package (`grep -c 'exec.Command(binPath'`), and every invocation that targets a
*self-hosting* domain fixture re-pays RC1's full cold cost inside the child.

The distinction that matters:

- `copySelfDomain` (`cmd/hotam/main_test.go:32`) copies the real 1.09 MB `graph.json`
  with `self_hosting: true` and its 18 `verified_by` entries intact. Because the domain
  is self-hosting, `gate.SpecRoot` resolves those entries against **this repo's own
  module**, so a child process running against such a fixture executes the full engine
  cold. Cost: ~30-66 s per child.
- `copyNonSelfHostingDomain` (`cmd/hotam/main_test.go:112`) strips every
  `implemented_by`/`verified_by` (`makeNonSelfHosting`, `cmd/hotam/main_test.go:146-181`).
  Children against that fixture do no gate work at all. Cost: seconds.

`.scratch/slowest-tests.txt` shows the price directly:
`TestCmdAllViolations_JSONExitCode` 372.83 s, `TestCmdWhatNow_JSON` 273.87 s,
`TestCmdAllViolations_JSON` 273.57 s, `TestAllJSONCommands_SingleDocument/{status,
all-violations,what-now}` 110.65 / 110.41 / 107.18 s.
`TestAllJSONCommands_SingleDocument` (`cmd/hotam/json_contract_test.go:232-238`) uses
`copySelfDomain` while asserting only that stdout is a single well-formed JSON document.

### RC3 — `-coverpkg` multiplies compiles by a factor of ~4.

Because `coverPkgPattern` is part of `compileCacheKey` (correctly — Go bakes coverage
instrumentation in at compile time, and the file's doc comment at
`internal/gate/compile_cache.go:89-100` explains why conflating them would be wrong),
18 recording calls over 5 packages produce up to 18 distinct compiles instead of 5.

### RC4 — Five separate test binaries each pay their own cold warm-up. Structural.

`go test` compiles one binary per package. `internal/gate`, `internal/generator`,
`internal/invariants`, `internal/diagnose` and `internal/selfspec` all exercise the same
engine against the same real domain, in five separate OS processes, with five
independent sets of process-lifetime caches. Nothing short of RC1's fix helps here.

### RC5 — Oversubscription and inter-agent contention.

`-p 16` (packages) × `-parallel 16` (tests within a package) × each test spawning `go`
subprocesses that themselves fan out. Layered on top of that, this session runs several
agents against the same working tree. §1.5 quantifies the result: a 2.6x inflation of
total package time, with the *cheapest* packages inflating the *most* (5.7x for
`internal/fsio`) — the classic signature of scheduler thrash rather than real work.

---

## 3. Recommendations, prioritized

### (a) Adoptable this session, no code change, no risk

**A1 — Stop paying two full-suite runs per task. Highest immediate value.**
The brief states the suite is run "at least once, often twice (once mid-verification,
once by the resolver independently)" per task, ×8+ tasks. At an honest uncontended
~5-6 min each, the second run is ~45 min of session wall time. Worse, when two agents
run it *concurrently*, §1.5 shows both runs inflate 1.8-5.7x — so a concurrent duplicate
costs far more than 2x. Concretely: designate one full-suite run per commit boundary,
record the tree hash (`git rev-parse HEAD` + `git status --porcelain` digest) alongside
the log in `.scratch/`, and let the resolver *read* that log rather than re-run when the
tree is byte-identical. If an independent re-run is genuinely required by the zero-trust
posture, serialize it (never concurrent with another agent's run).
**Impact: ~40-50% of this session's test wall time. Risk: none (a convention, not code).**

**A2 — Keep `-count=1` exactly as-is; do not introduce a "middle ground".**
§1.2 measures that the five dominant packages never hit Go's test cache at all, so
`-count=1` costs ≈0 wall time. Weakening the discipline to save it would trade a real
correctness guarantee for a measured non-saving. **Impact: 0 s saved by changing it —
which is the point. Risk of the change: real. Recommendation: reject.**

**A3 — Do not use `-short` / `make test-fast` as the iteration loop for `cmd/hotam`.**
It saves 6% (§1.3). The fast loop for `cmd/hotam` work is a targeted `-run` regex:
one real-domain test is 66 s versus ~323 s for the package.
**Impact: ~4x faster inner loop on `cmd/hotam` work. Risk: none.**

**A4 — Delete the ~50 MB of stray build outputs at the module root.**
`dump-enforced.exe`, `gate.test.exe`, `generator.test.exe`, `hotam.exe`,
`hotam.test.exe`, `hotam-review.exe`, `invariants.test.exe`, `seed-due.exe`,
`task376-demo.exe`, `wlock_tmp.exe` — all gitignored, all untracked. `fbb1b34` stopped
*hashing* them, but `filepath.WalkDir` still *visits* them and their directory entries;
`path/filepath.walkDir` is still the heaviest pure-Go frame in today's profile at 8.4%.
Not done by this task (read-only mandate) — it is a one-line `rm` for whoever owns the
tree. **Impact: a fraction of 8%, call it 1-3%. Risk: none, but confirm nothing in
flight is using `hotam.exe`.**

**A5 — Do not tune `-p` / `-parallel`.** §1.4 measures 3.3x cross-package and ~8x
within-package overlap already. **Impact: ~0. Skip.**

### (b) Real code changes, each needing its own zero-trust-verified task

Sized against this session's own commits (`fdb8bfe`: 5 files, +1211/-27, 11 new tests;
`0933ddb`: 4 files, +447/-52, 3 new tests) — that is the calibration for "one task".

**B1 — Cross-process, content-addressed compiled-test-binary cache. Largest available
win. Needs an explicit resolver decision before anyone starts.**
Mirror the already-accepted pattern in `internal/gate/fixture_cache.go`
(`EnsureContentAddressedFixture`, commit `9cda1f2`): publish each compiled `.test.exe`
under a stable path keyed by the **existing** `gate.HashPackageInputs` module hash plus
`pkgPattern` plus `coverPkgPattern`, instead of the per-process
`os.MkdirTemp("", "hotam-compile-")` at `internal/gate/compile_cache.go:510`. A second
process with an unchanged module converges on the same path and skips `go test -c`
entirely.
*Expected impact:* removes RC1 for every process after the first. `doCompileTestBinary`
is 42% of a cold pass (§1.7), and that cost is paid by six test binaries plus ~15-20
`hotam.exe` children per suite run. Rough estimate: **25-40% off the whole suite** —
larger than `fdb8bfe`'s already-dramatic win, and it is the only recommendation here
that also helps `internal/gate`/`generator`/`invariants`/`diagnose`/`selfspec` (RC4).
*Risk: HIGH, and specifically a security-design question, not merely an engineering one.*
This repo deliberately refuses an on-disk verdict store, and proves it with
`TestKillswitch_CleanRun_NeverCreatesOnDiskVerdictStore`,
`TestKillswitch_ForgedVerdictCacheFile_DoesNotSilenceRedDomain` and
`TestKillswitch_ForgedMarkerFile_DoesNotRestoreKillSwitch`
(`cmd/hotam/killswitch_e2e_test.go`). A cached compiled binary is a *different* artifact
from a cached verdict — it still has to execute and produce a real PASS — but a binary at
a *predictable* path is a strictly better forgery target than a verdict file: a planted
binary that prints `PASS` forges the verdict *and* the execution in one move. The
per-process random tmp dir currently makes that attack impossible. Any design must put
that on the table explicitly (candidate mitigations: hash-verify the cached binary's own
bytes immediately before `exec`; refuse the cache if the directory is not owner-only;
or gate the whole thing behind an env opt-in that CI and the pre-commit gate never set —
which would still capture the entire local-dev win). **Do not land this without a
resolver decision recorded as a Conflict or a signed-off Requirement.**
*Size: larger than `fdb8bfe`. Budget a full task plus a design conversation.*

**B2 — Collapse per-entry `-coverpkg` compiles into one union-instrumented binary per
test package.**
Addresses RC3. `go test -c` accepts a comma-separated `-coverpkg` list, so one binary
instrumented over the union of all `coverPkgFile` packages for a given test package can
serve every recording call for that package. Callers already filter the resulting
profile by symbol range (`gate.SymbolRangeCoveredByProfile`), so extra packages in the
profile should be inert.
*Expected impact:* ~23 `go test -c` calls per cold pass drop to ~5-6. On today's profile
that is roughly **55-80 s of the 125 s compile cost per cold pass**, i.e. ~15-25% of a
cold engine pass — and unlike B1 it needs no new on-disk artifact and no security
conversation.
*Risk: medium.* Changes what the coverage profile contains; `check_scenario_executes_impl`
and its tests in `internal/invariants/scenario_coverage_test.go` must be re-proven, and a
union `-coverpkg` makes each binary larger (more instrumentation), partly offsetting the
win. Verify before/after, do not assume.
*Size: comparable to `0933ddb`.* **This is the recommendation I would start with** —
it is the best impact-per-unit-risk on the list.

**B3 — Audit the `copySelfDomain` + `exec.Command(binPath, ...)` tests and downgrade the
ones that do not need a self-hosting fixture.**
Addresses RC2. For each such test, ask: does the assertion actually depend on gate
output, or is it CLI plumbing (stdout shape, exit code, flag parsing, stderr routing)?
`copyNonSelfHostingDomain` (`cmd/hotam/main_test.go:112`) already exists and is already
used by 17 tests in `propose_test.go` alone. First candidate:
`TestAllJSONCommands_SingleDocument` (`cmd/hotam/json_contract_test.go:232`), 110 s per
subcommand for a single-JSON-document shape assertion. Also review
`json_flags_test.go` (4 self-domain fixtures, 4 spawns), `confront_test.go` (5/5),
`confront_gate_test.go` (6/6), `confront_proposal_test.go` (4/4), `inspect_test.go` (2/2).
*Expected impact:* each converted test drops from ~60-110 s to ~2-5 s of its own
subprocess time. With ~8-12 convertible tests and ~8x parallel overlap, expect
**10-20% off `cmd/hotam`'s wall**, not the naive sum.
*Risk: low-to-medium per test* — the whole risk is "does this assertion secretly depend
on the gate having run?", answerable by reading each test. *Size: one careful audit task,
mechanical diff, one test at a time.*

**B4 — Share the killswitch fixture domain. LOW priority despite appearances.**
`killswitchFixtureDomain` (`cmd/hotam/killswitch_e2e_test.go:41`) is called by 6 tests
(lines 188, 243, 305, 438, 524, 567), each rebuilding the identical fixture via ~6
`hotam.exe` invocations, and `.scratch/slowest-tests.txt` shows those 6 tests at
292-362 s each — the most eye-catching numbers in the file. **Do not be fooled by them.**
Those tests are `-short`-gated, they run in parallel, and §1.3 measures the entire
`-short` delta for the package at 6.1% / 12.9 s. Their elapsed times are mostly *waiting*,
not the critical path. If it is ever done, the shape is a published base fixture built
once and copied per test (each test mutates its own copy — that is the point of the
test), for which `gate.EnsureContentAddressedFixture` is the existing precedent.
*Expected impact: ≤6% of `cmd/hotam`, probably ~2-3%. Risk: medium (these are integrity
tests; a shared fixture must not weaken what they prove). Size: one task. Priority: last.*

**B5 — Splitting `cmd/hotam`'s test package. NOT recommended.**
Go compiles one test binary per package, so splitting `cmd/hotam` into an in-process half
and an `e2e` half would let `go test ./...` schedule them as two concurrent packages.
But §1.4 measures ~8x overlap *already* inside the single binary; the wall floor is set by
total work and by the longest single chain, not by lack of package-level concurrency.
`TestMain` (`cmd/hotam/testbinary_test.go:56`) and the `buildSharedHotamBinary`
`sync.Once` would have to be duplicated into the new package, and the two halves would
then build **two** CLI binaries instead of one — plausibly a net loss.
*Expected impact: small, possibly negative. Recommendation: reject unless B1/B2/B3 are
exhausted.*

### Summary table

| # | Change | Impact | Risk | Size |
|---|---|---|---|---|
| A1 | One full-suite run per commit boundary; serialize across agents | **~40-50% of session test wall** | none | convention |
| A2 | Keep `-count=1` (reject the "middle ground") | 0 s (measured non-saving) | n/a | none |
| A3 | Targeted `-run` instead of `-short` for `cmd/hotam` iteration | ~4x inner loop | none | habit |
| A4 | Delete stray root `*.exe` build outputs | ~1-3% | none | one `rm` |
| B2 | Union `-coverpkg` per test package | ~15-25% of a cold engine pass | medium | ≈ `0933ddb` |
| B3 | Downgrade self-hosting fixtures in CLI-plumbing tests | ~10-20% of `cmd/hotam` | low-med | audit task |
| B1 | Cross-process content-addressed compile cache | **~25-40% of the whole suite** | **high (security)** | > `fdb8bfe` + design call |
| B4 | Share killswitch fixture | ~2-3% | medium | one task |
| B5 | Split `cmd/hotam` test package | ~0 or negative | med | reject |

Suggested order: **A1 and A3 today (free), then B2, then B3, then a resolver
conversation about B1.**

---

## 4. What I could not verify — the honest boundary

**Everything I measured today is contaminated by a concurrently-running agent.** I
observed identical `go test -c ./internal/ontology` invocations take 13.9 s and 123.5 s
within the same five-minute window. Treat every absolute number in §1.6-1.8 as an
**upper bound** and the **ratios** as the signal. The one measurement I trust
unconditionally is §1.1 (5.6 s to build 20 test binaries), because it is far too small
to be explained by anything but the true cost.

**I did not run a full `go test ./...`.** It would have taken 5-10 min of saturated CPU
and would have collided head-on with the other agent's own work — precisely the failure
mode A1 exists to prevent. All full-suite numbers above are read out of `.scratch/`
logs written by earlier runs, and I have labelled each with its source file and
timestamp.

**I did not test a cold `GOCACHE`.** That needs `go clean -cache`, which would have
destroyed the other agent's build cache. So §1.1's "the build cache is fine" is verified
for the **warm** case only. Inference (not verified): a cold cache would add a one-time
cost to the first run after a `go clean` and nothing thereafter — and since nothing in
`Makefile` or `.github/workflows/` ever calls `go clean`, this is not a recurring cost.

**I verified *that* `cmd/hotam` never hits Go's test cache (0/60+ runs), not *why*.**
My inference — untested — is `os.Chdir` via `chdirAndRestore`
(`cmd/hotam/common_test.go:26`, and `t.Chdir`-style patterns elsewhere) plus heavy
`t.TempDir()` file access. `GODEBUG=gocachetest=1` would confirm it in one run. I did not
do this because the conclusion (A2: keep `-count=1`) is the same either way.

**The 42% `doCompileTestBinary` figure comes from a profile of one test**
(`TestAllViolations_SmokeNoPanic`), not of the whole `cmd/hotam` package, and it was
captured under load (the profiled run took 118 s versus 66 s unprofiled for the same
test — profiling overhead and contention are both in there). I chose the single-test
profile deliberately: a whole-package profile would have taken 5-10 min of saturated CPU.
The direction is corroborated independently by §1.8's direct `go test -c` timings and by
reading `compile_cache.go`'s own scope, so I am confident in the *finding*; I am less
confident in the *exact* 42%.

**I did not measure B1's, B2's or B3's actual win** — each would require implementing it.
All impact estimates in §3(b) are arithmetic on the measured profile shares, not
observations. In particular, B2's estimate assumes union `-coverpkg` instrumentation does
not materially inflate link time or binary size; that assumption is untested and could
halve the win.

**I did not read all 42 `cmd/hotam` test files.** I read `common_test.go`,
`testbinary_test.go`, `main_test.go` (fixture helpers), `json_contract_test.go`
(partially), and `killswitch_e2e_test.go` (partially); the rest were surveyed by
targeted `grep` for `exec.Command`, `t.Parallel`, `t.Setenv`, `testing.Short`,
`copySelfDomain` and `copyNonSelfHostingDomain`. B3's audit will need the full read.
