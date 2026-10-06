# Справка: решения по долгам самохостинга (P3-2) — конфликт и ревью

Дата: 2026-10-06. Контекст: docs/reviews/2026-10-05-framework-review.md §P3-2. Резолвер: только человек; этот документ — материалы к решению, не само решение.

## 1. Конфликт C-d20cf537 (ось reviewability-vs-code-authority) — DETECTED

- Возник 2026-07-24 (task #350/RAC-B3), resolver: framework-reviewer, без движения с создания.
- Суть: для Requirement/Rejection на self-домене закрыт JSON-путь — авторство живёт в Go-литералах `internal/selfspec/requirements_*.go`, а резолвер на шаге PRESENT подписывает dry-run рендер `hotam sync-self` (FieldDiff), а не сам артефакт. Доверие сместилось с «одобряю файл, который вижу» на «доверяю рендеру, потому что байт-тождественность доказана тестами» (embed_test/merge_test/sync_test, `check_self_requirements_match_registry`). Члены конфликта: `R-ai-presents-not-decides`, `R-requirement-update-signoff-typed`.
- Открытый вопрос из контекста: обобщается ли паттерн «резолвер подписывает рендер, не источник» на consumer-домены, где резолвер — не программист (gpsm-pm читает prose-документы, не Go).

Варианты:
- (a) ACKNOWLEDGE + revisit_marker «REVISIT при первом consumer-домене с requirements_authority: code». Дёшево; конфликт остаётся видимым якорем, вопрос не теряется. Риск: без границы в маркере вопрос замалчивается.
- (b) DECIDED: записать границу применимости (паттерн легален только для programmer-resolver доменов; для остальных обязателен prose-рендер). Закрывает напряжение границей, без нового кода. Риск: границу придётся пересматривать при расширении.
- (c) Построить prose-рендер подписи (business-language FieldDiff в sync-self). Полный ответ на вопрос обобщения; дороже (генератор + тесты) и YAGNI, пока нет реального consumer-домена на этом пути (заморожено духом R-speculative-aspects-frozen).

Рекомендация: (a) сейчас, с формулировкой границы из (b) внутри revisit_marker; (c) — только по реальному триггеру. Механика: `hotam apply-proposal <json>` с ProposedConflictTransition для C-d20cf537. DETECTED→ACKNOWLEDGED — легальный переход (`resolver-acknowledge`). В JSON нужны `kind: "ConflictTransition"`, `conflict_id`, `new_lifecycle: "ACKNOWLEDGED"`; `decided_by`, `verbatim`, `reason` не требуются. Заполнить также `revisit_marker` условием возврата к вопросу. `hotam apply-proposal <json>` принимает один объект-предложение на файл; резолвер-человек один решает и подписывает.

## 2. Ревью-долг: 27 никогда + 40 просрочено

Отметки ревью (ProposedReviewMark) ставит только резолвер-человек: это его повторное подтверждение, что claim всё ещё верен; evidence должен называть, ЧТО перепроверено (файл/тест/документ).

Полный формат записи: один JSON-объект на файл: `{"kind":"ReviewMark","requirement_id":"R-…","reviewed_at":"YYYY-MM-DD","review_after":"YYYY-MM-DD","evidence":["..."]}`. `hotam apply-proposal` обрабатывает одно предложение-объект на файл, не массив; пакетный режим принимает отдельные JSON-файлы из папки (`--batch`).

Предложение пакетной прогонки: разбить на смысловые группы ниже; для каждой группы завести папку `proposals/wave16-review-marks/<group>/` и по одному файлу на требование (apply-proposal поддерживает один объект на файл плюс режим -batch <dir> — атомарное применение всех *.json из папки в порядке имён), затем apply по группе одной командой (-batch <dir>). Целевой review_after: +90 дней от 2026-10-06 (2027-01-04) — согласуется с существующим полугодовым окном и не создаёт нового завала.

Группа Н1 «never-reviewed: локализация/каталог» (5): R-catalog-supported, R-catalog-translated, R-language-bundle-complete, R-language-outputs-current, R-missing-translation-message.
Группа Н2 «never-reviewed: authored-spec дисциплина» (6): R-authored-spec-layer-progression, R-authored-spec-projections-are-derived, R-spec-link-embodied-vs-proven, R-source-clause-links-resolve, R-structural-floor-vs-mirror-audit, R-coverage-qualification-is-authored.
Группа Н3 «never-reviewed: freshness-проекции» (3): R-claude-md-current, R-engine-docs-fingerprint-current, R-specification-source-bytes-current.
Группа Н4 «never-reviewed: authority-owns-obligations» (4): R-opt-in-trigger-owns-its-own-obligations, R-public-surface-authority-owns-its-own-obligations, R-scenario-authority-owns-its-own-obligations, R-self-executing-atoms-own-structural-obligations.
Группа Н5 «never-reviewed: conformance/механизация» (4): R-conformance-audit-own-fields, R-requirement-generation-mechanized-by-registry, R-scenario-spec-obligations-mechanically-enforced, R-entity-type-realized-by-go-symbol-never-generated.
Группа Н6 «never-reviewed: регистр/онбординг» (2): R-speak-domain-register-by-default, R-orientation-faq-answerable.
Группа Н7 «never-reviewed: vendored-канон» (2): R-vendored-ontology-matches-engine-canon, R-vendored-recorder-matches-engine-canon.
Группа Н8 «never-reviewed: gate» (1): R-gate-signoff-single-carrier.

(O1) Просроченные 55д (review_after=2026-08-12), 21: R-active-loop-apply-tool, R-active-loop-protocol, R-agent-code-imports-framework, R-agent-declares-purpose, R-agent-has-docs-dir, R-agent-has-own-tools-dir, R-agent-is-a-directory, R-agent-is-recursive-director, R-agent-map-generated, R-agent-never-lost, R-agent-references-shared-docs, R-agent-scoped-constitution, R-anchor-everything, R-anchor-taxonomy, R-assumption-implements-state, R-assumption-transition-kind-exists, R-atomicity-ratchet-no-growth, R-attention-agent-agnostic-core, R-attention-registry, R-attention-superset-of-diagnose, R-audit-atomicity-tool.
(O2) Просроченные 24д (review_after=2026-09-12), 19: R-axis-gatekeeper-policy, R-backend-scope, R-bijection-r-to-enforcer, R-budget-measure, R-check-method-is-atomic, R-claude-md-consolidates-when-single-agent, R-commit-boundary-checkable, R-conflict-addressing-resolves-variables, R-content-free-no-examples, R-content-free-no-seed-graph, R-content-layout-evolution, R-context-bounded-delegation, R-core-imports-stdlib-or-hotam-spec-only, R-core-periphery-import-ratchet, R-critical-core-methodology, R-crystal-carries-mediation-loop, R-crystal-carries-recursion-seed, R-crystal-carries-short-form, R-crystallize-before-split.

Практический порядок: начать с Н1–Н8 (никогда не ревьюились — самый тёмный долг), затем O2, затем O1; для каждой записи evidence = один конкретный факт перепроверки (например «UNENFORCED.md/FRAMEWORK-INVARIANTS.md перечитан, claim соответствует enforcerу X»).

## 3. Сведение P3-2 (сделано агентом 2026-10-06, к сведению резолвера)

Закрыто в ENFORCED (4): R-tool-is-its-own-requirement (TestScanToolRequirements_ProjectsEveryEntryToRequirement), R-domain-owns-tools-and-agents (check_domain_dirs_lazy_materialized), R-project-name-hotam-spec (TestProjectIdentityHotamSpec), R-speculative-aspects-frozen (TestFrozenAspectsSnapshot + testdata baseline). Через sync-self (dry-run → --confirm-hash).

Не закрыто честно (2), объяснение:
- R-authored-spec-layer-progression: требование про ПОРЯДОК авторинга слоёв (models→fields→methods→tests) — временнАя дисциплина; конечное состояние spec/ не отличает нарушение порядка (правильный и неправильный порядок дают одно дерево). Механический enforcer — аудитор порядка по git-истории spec/-дерева; такого инструмента нет, а у self-домена spec/-дерева нет вовсе (тест был бы вакуумным). Предложение резолверу: поставить BlockedOn feature:spec-layer-order-auditor (переведёт в честный feature-blocked, снимет из closeable-now).
- R-presented-pending-decision-type: enforcer тривиален («в корне proposals/ нет файлов вне wave-папок»), но сегодня 7 файлов лежат в корне proposals/ (6 draft-*.json — не применённые черновики требований, и update-R-authored-spec-links-mechanically-checked-f1.json — применённый и оставленный). Агент не трогал чужие pending-артефакты (вне области задачи). Предложение резолверу: либо разложить эти файлы по wave-папкам (тогда enforcer можно писать и закрывать требование), либо amend требования под фактическую практику.
