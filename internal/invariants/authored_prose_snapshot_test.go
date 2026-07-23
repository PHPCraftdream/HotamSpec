package invariants

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// --- Process.Why / Step.Why coverage (task #331's original scope) ---------

// TestCheckAuthoredProseSnapshot_FiresOnGpsmSmShapedSentence reproduces the
// REAL positive this check exists for: gpsm-sm's actual stale Process.Why
// sentence shape (a "ТЕКУЩЕЕ ПОЛОЖЕНИЕ" marker phrase co-occurring with an
// ISO date) — task #331's design consult's confirmed contradiction case.
func TestCheckAuthoredProseSnapshot_FiresOnGpsmSmShapedSentence(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Processes: []ontology.Process{
			{
				ID:  "PR-gpsm-ott-delivery",
				Why: "27 из 32 ФТ прошли P-G1. ТЕКУЩЕЕ ПОЛОЖЕНИЕ на 2026-07-21.",
			},
		},
	}
	got := checkAuthoredProseSnapshot(g)
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 violation for the gpsm-sm-shaped snapshot sentence, got %d: %+v", len(got), got)
	}
	if got[0].Check != "check_authored_prose_snapshot" {
		t.Errorf("Check = %q, want check_authored_prose_snapshot", got[0].Check)
	}
	if got[0].ID != "PR-gpsm-ott-delivery" {
		t.Errorf("ID = %q, want PR-gpsm-ott-delivery", got[0].ID)
	}
}

// TestCheckAuthoredProseSnapshot_FiresOnTallyCoOccurringWithOwnStageToken
// covers fire condition (b): a "N из M" / "N of M" tally co-occurring with a
// stage token from the domain's OWN declared gate_stage_order — even
// without any marker phrase or ISO date.
func TestCheckAuthoredProseSnapshot_FiresOnTallyCoOccurringWithOwnStageToken(t *testing.T) {
	t.Parallel()
	domainDir := gateSignoffFixture(t, `["P-G0", "P-G1"]`)
	g := &ontology.Graph{
		DomainDir: domainDir,
		Processes: []ontology.Process{
			{
				ID:  "PR-x",
				Why: "27 из 32 требований прошли P-G1 в этой волне.",
			},
		},
	}
	got := checkAuthoredProseSnapshot(g)
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 violation for tally+stage-token co-occurrence, got %d: %+v", len(got), got)
	}
}

// TestCheckAuthoredProseSnapshot_FiresOnStepWhy proves the scope also
// covers Step.Why (not only Process.Why), naming the process id plus the
// step in the violation's ID for traceability.
func TestCheckAuthoredProseSnapshot_FiresOnStepWhy(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Processes: []ontology.Process{
			{
				ID: "PR-x",
				Steps: []ontology.Step{
					{Name: "review", Why: "As of 2026-07-21, current status: blocked."},
				},
			},
		},
	}
	got := checkAuthoredProseSnapshot(g)
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 violation for the Step.Why snapshot sentence, got %d: %+v", len(got), got)
	}
	if got[0].ID != "PR-x / step review" {
		t.Errorf("ID = %q, want %q", got[0].ID, "PR-x / step review")
	}
}

// TestCheckAuthoredProseSnapshot_TrueNegatives asserts the three explicit
// false-positive-avoidance shapes the design consult named — this check
// must NOT fire on ordinary prose that merely contains a date or a count
// with no snapshot claim attached. These are as important as the positive
// tests: the whole point of a NARROW, ADVISORY-ONLY lint is staying
// low-noise across 300+ existing graph nodes, never becoming noise itself.
func TestCheckAuthoredProseSnapshot_TrueNegatives(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		why  string
	}{
		{
			name: "resolved-on-date-no-marker-no-tally",
			why:  "This tension was resolved on 2026-07-22.",
		},
		{
			name: "contract-due-date-no-marker-phrase",
			why:  "The vendor contract is due 2026-09-15.",
		},
		{
			name: "four-waves-no-tally-vs-stage-token",
			why:  "This process runs in четыре волны, each independently reviewed.",
		},
	}
	// A domain-declared gate_stage_order IS present here (unlike the bare
	// no-DomainDir cases above) so the "четыре волны" true-negative is
	// actually exercised against condition (b)'s stage-token lookup, not
	// merely skipped because no stage vocabulary was ever declared.
	domainDir := gateSignoffFixture(t, `["P-G0", "P-G1"]`)
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := &ontology.Graph{
				DomainDir: domainDir,
				Processes: []ontology.Process{{ID: "PR-x", Why: tc.why}},
			}
			got := checkAuthoredProseSnapshot(g)
			if len(got) != 0 {
				t.Errorf("expected 0 violations (true negative) for why=%q, got %d: %+v", tc.why, len(got), got)
			}
		})
	}
}

// TestCheckAuthoredProseSnapshot_NoOpWhenNoProcesses is the honest no-op: a
// graph with zero Process nodes and no manifest.json goals/charter never
// fires, regardless of anything else.
func TestCheckAuthoredProseSnapshot_NoOpWhenNoProcesses(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{}
	got := checkAuthoredProseSnapshot(g)
	if len(got) != 0 {
		t.Errorf("expected 0 violations for a graph with no Processes, got %d: %+v", len(got), got)
	}
}

// TestCheckAuthoredProseSnapshot_TallyWithoutDeclaredStageOrderIsNoOp
// proves condition (b) never fires for a domain that has not declared
// gate_stage_order at all (g.DomainDir == "", the in-memory-fixture shape) —
// there is no domain-declared stage vocabulary to check a tally against, so
// a bare tally alone (no marker phrase, no ISO date) must not fire.
func TestCheckAuthoredProseSnapshot_TallyWithoutDeclaredStageOrderIsNoOp(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Processes: []ontology.Process{
			{ID: "PR-x", Why: "27 из 32 требований прошли P-G1 в этой волне."},
		},
	}
	got := checkAuthoredProseSnapshot(g)
	if len(got) != 0 {
		t.Errorf("expected 0 violations when no gate_stage_order is declared, got %d: %+v", len(got), got)
	}
}

// TestAuthoredProseSnapshotWarnings_ExportedWrapperMatchesInternalCheck
// proves the exported entry point cmd/hotam calls
// (AuthoredProseSnapshotWarnings) produces the identical result the internal
// check does — it is a pure pass-through, never registered into the All
// registry (so it is never double-counted in invariants.AllViolations).
func TestAuthoredProseSnapshotWarnings_ExportedWrapperMatchesInternalCheck(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Processes: []ontology.Process{
			{ID: "PR-x", Why: "ТЕКУЩЕЕ ПОЛОЖЕНИЕ на 2026-07-21."},
		},
	}
	got := AuthoredProseSnapshotWarnings(g)
	want := checkAuthoredProseSnapshot(g)
	if len(got) != len(want) || len(got) != 1 {
		t.Fatalf("AuthoredProseSnapshotWarnings = %+v, want %+v", got, want)
	}
}

// TestCheckAuthoredProseSnapshot_NeverRegisteredInAllRegistry is the
// advisory-band contract itself: check_authored_prose_snapshot must never
// appear in the All registry, so it can never surface in
// invariants.AllViolations / block `hotam all-violations`'s exit code / block
// internal/proposal/apply.go's proposal gate — mirrors HonoredSkipWarnings'
// identical never-registered contract.
func TestCheckAuthoredProseSnapshot_NeverRegisteredInAllRegistry(t *testing.T) {
	t.Parallel()
	for _, inv := range All.All() {
		if inv.Name == "check_authored_prose_snapshot" {
			t.Fatalf("check_authored_prose_snapshot must NOT be registered in All (advisory-only, mirrors HonoredSkipWarnings) — found it registered")
		}
	}
}

// --- manifest.json goals/charter coverage (task #333's extension) ---------

// manifestGoalsCharterFixture writes a manifest.json (carrying the given
// goals/charter/gate_stage_order raw JSON fragments) plus a minimal
// graph.json, and returns the domain directory — mirrors writeParentFixture
// / gateSignoffFixture's identical "write manifest.json + graph.json, return
// dir" shape. goalsJSON is the raw JSON array literal for "goals" (e.g.
// `["a", "b"]`); charter is the raw string value (empty string omits the
// field entirely, matching DomainPresentation's own optional-field
// contract); gateStageOrderJSON is the raw JSON array literal for
// "gate_stage_order" (empty string omits the field).
func manifestGoalsCharterFixture(t *testing.T, goalsJSON, charter, gateStageOrderJSON string) string {
	t.Helper()
	tmp := t.TempDir()
	domainDir := filepath.Join(tmp, "domains", "testdomain")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("MkdirAll domainDir: %v", err)
	}
	manifest := `{"purpose": "test domain", "parent": null`
	if goalsJSON != "" {
		manifest += `, "goals": ` + goalsJSON
	}
	if charter != "" {
		manifest += `, "charter": ` + charter
	}
	if gateStageOrderJSON != "" {
		manifest += `, "gate_stage_order": ` + gateStageOrderJSON
	}
	manifest += `}`
	if err := os.WriteFile(filepath.Join(domainDir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("WriteFile manifest.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(domainDir, "graph.json"), []byte(`{"schema_version":3}`), 0o644); err != nil {
		t.Fatalf("WriteFile graph.json: %v", err)
	}
	return domainDir
}

// TestCheckAuthoredProseSnapshot_FiresOnManifestGoalsPreTask329Shape
// reproduces gpsm-sm's REAL pre-#329 goals text shape (reconstructed from
// this session's own history: task #329's fix reworded a hardcoded "32/32
// SIGNED" snapshot out of gpsm-sm's manifest goals) — a marker phrase
// co-occurring with an ISO date, this time in manifest.json's "goals"
// rather than Process.Why. Proves the SAME two predicates now also scan
// goals entries.
func TestCheckAuthoredProseSnapshot_FiresOnManifestGoalsPreTask329Shape(t *testing.T) {
	t.Parallel()
	preTask329Goal := `"Провести 32 ФТ через Planning pipeline; ТЕКУЩЕЕ ПОЛОЖЕНИЕ на 2026-07-20: 32 из 32 SIGNED на P-G1 — продвинуть к P-G2."`
	domainDir := manifestGoalsCharterFixture(t, `[`+preTask329Goal+`]`, "", `["P-G0", "P-G1", "P-G2"]`)
	g := &ontology.Graph{DomainDir: domainDir}
	got := checkAuthoredProseSnapshot(g)
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 violation for the pre-#329-shaped goals snapshot text, got %d: %+v", len(got), got)
	}
	if got[0].Check != "check_authored_prose_snapshot" {
		t.Errorf("Check = %q, want check_authored_prose_snapshot", got[0].Check)
	}
	if got[0].ID != "manifest.json goals[0]" {
		t.Errorf("ID = %q, want %q", got[0].ID, "manifest.json goals[0]")
	}
}

// TestCheckAuthoredProseSnapshot_TrueNegativeOnRealPostTask329GpsmSmGoals is
// the true-negative counterpart: gpsm-sm's REAL, CURRENT (post-#329-reworded)
// goals text — copied verbatim from
// D:\ai_dev\prat\PRAT-hotam\domains\gpsm-sm\manifest.json (read-only
// inspection performed during this task, per its constraints) — must NOT
// fire. The rewording replaced the hardcoded tally+date with a pointer to
// the live DOMAIN-MAP/FAQ projection, exactly the fix this check exists to
// push domains toward.
func TestCheckAuthoredProseSnapshot_TrueNegativeOnRealPostTask329GpsmSmGoals(t *testing.T) {
	t.Parallel()
	realGoals := []string{
		"Провести 32 ФТ (R-FR-01..R-FR-32, SETTLED) через Planning pipeline методологии prat (P-G0→P-G4) до передачи в исполнение (R-gpsm-project); P-G1 полностью пройден — см. живой статус в DOMAIN-MAP/FAQ (гейт-строка **gates** и assert-проверяемый orientation_faq) — продвинуть ФТ по конвейеру за P-G1 к P-G2 и далее",
		"Пройти ревью ИБ по обработке VIN/госномеров, ПДн и внешним API на этапе SDR (A-ib-review-pending) — структура ревью смоделирована типизированным SecurityReview (spec/model/security_review.go), но реальное ревью ИБ Заказчика остаётся событием внешнего мира вне модели",
		"Prod-интеграция VIN (R-FR-02) после готовности API-контракта подрядчика 2026-09-15 (A-vin-contractor-august HOLDS); до этого adapter/mock + собственный кэш (R-FR-03, R-FR-05)",
	}
	goalsJSON := `["` + realGoals[0] + `", "` + realGoals[1] + `", "` + realGoals[2] + `"]`
	domainDir := manifestGoalsCharterFixture(t, goalsJSON, "", `["P-G0", "P-G1", "P-G2", "P-G3", "P-G4"]`)
	g := &ontology.Graph{DomainDir: domainDir}
	got := checkAuthoredProseSnapshot(g)
	if len(got) != 0 {
		t.Errorf("expected 0 violations for gpsm-sm's real post-#329 goals text (true negative), got %d: %+v", len(got), got)
	}
}

// TestCheckAuthoredProseSnapshot_FiresOnManifestCharter proves the charter
// field is scanned with the identical predicates as goals — a single string
// field, not a list.
func TestCheckAuthoredProseSnapshot_FiresOnManifestCharter(t *testing.T) {
	t.Parallel()
	charter := `"По состоянию на 2026-07-21 этот домен обрабатывает VIN-запросы."`
	domainDir := manifestGoalsCharterFixture(t, "", charter, "")
	g := &ontology.Graph{DomainDir: domainDir}
	got := checkAuthoredProseSnapshot(g)
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 violation for the snapshot-shaped charter text, got %d: %+v", len(got), got)
	}
	if got[0].ID != "manifest.json charter" {
		t.Errorf("ID = %q, want %q", got[0].ID, "manifest.json charter")
	}
}

// TestCheckAuthoredProseSnapshot_TrueNegativeOnRealGpsmSmAndPratCharters is
// the true-negative counterpart for charter: both real consumer manifests'
// charter text this task validated against
// (D:\ai_dev\prat\PRAT-hotam\domains\gpsm-sm\manifest.json and
// domains\prat\manifest.json, read-only) — durable "what kind of thing this
// domain is" descriptions, no dates, no tallies — must NOT fire.
func TestCheckAuthoredProseSnapshot_TrueNegativeOnRealGpsmSmAndPratCharters(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		charter string
	}{
		{
			name:    "gpsm-sm",
			charter: "Этот домен — код-спек-тест модель требований проекта, а не развёрнутая система; внешние стороны (Автокод, ИБ Заказчика, Telegram) представлены мок-объектами с методами, которые одновременно тестируют логику и производят документацию.",
		},
		{
			name:    "prat",
			charter: "Этот домен — сама методология PRAT как исполняемая код-спек-тест модель (объекты/поля/методы/тест-кейсы, генерирующие доки), а не запущенный в проде инструмент; она проверяет и документирует сама себя.",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			domainDir := manifestGoalsCharterFixture(t, "", `"`+tc.charter+`"`, "")
			g := &ontology.Graph{DomainDir: domainDir}
			got := checkAuthoredProseSnapshot(g)
			if len(got) != 0 {
				t.Errorf("expected 0 violations for %s's real charter text (true negative), got %d: %+v", tc.name, len(got), got)
			}
		})
	}
}

// TestCheckAuthoredProseSnapshot_ManifestScanNoOpWhenDomainDirEmpty proves
// the manifest goals/charter scan is an honest no-op (never a panic, never a
// false negative masquerading as coverage) for an in-memory fixture graph
// with no DomainDir — there is no manifest.json to read at all. Mirrors
// condition (b)'s identical DomainDir=="" guard.
func TestCheckAuthoredProseSnapshot_ManifestScanNoOpWhenDomainDirEmpty(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{}
	got := checkAuthoredProseSnapshot(g)
	if len(got) != 0 {
		t.Errorf("expected 0 violations for a graph with no DomainDir, got %d: %+v", len(got), got)
	}
}
