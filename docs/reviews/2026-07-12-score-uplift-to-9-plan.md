# План: поднятие всех оценок ревью до 9/10

Источник: исследование агента `@fm` (2026-07-12), запущенное после волны P0×6/P1×7/P2×5
(коммит `c23b3f8`) и независимого ревью `@om`. Все факты ниже получены прямыми
командами на репозитории (`go test`/`go vet`/`gofmt`, `hotam` CLI, скрипты по
`graph.json`) — не с чужих слов.

## 0. Текущие оценки (после волны P0/P1/P2, по ревью `@om`)

| Область | Было | Стало | Цель |
|---|---|---|---|
| Онтология графа | 8 | 8 | 9 |
| Структурная защита от противоречий | 8 | 8 | 9 |
| Качество Go-порта | 7 | 8 | 9 |
| Простота сопровождения | 4 | 6 | 9 |
| Удобство агента | 3 | 7 | 9 |
| Актуальность требований | 2 | 4 | 9 |
| Применимость внешним проектам | 3 | 7 | 9 |
| Итоговое соответствие цели | 5 | 7 | 9 |

## 1. Собранная фактическая база

| Факт | Значение | Как проверено |
|---|---|---|
| Требований всего / SETTLED / REJECTED / DRAFT | 275 / 232 / 41 / 2 | парсинг `domains/hotam-spec-self/graph.json` |
| SETTLED со stale Python-контентом в claim/why/summary | 155 / 232 | regex-скан (`.py`, pytest, tools/, gen_spec, hotam_spec., spec/agents, dataclass…) |
| SETTLED со stale-контентом включая enforcement/evidence/source_refs | 184 / 232 (79%) | то же |
| Stale в других узлах | conflicts 7/8, assumptions 7/16, hotam-dev 5/9 | то же |
| enforced_by записей у SETTLED | 412: 125 `check_*` (валидны), **287 (70%) — ссылки на несуществующие Python-тесты** (233 pytest node-id вида `test_x.py::test_y`, 54 голых `.py`) | классификация enforced_by |
| Следствие: `hotam gate` | fail-closed почти на любом якоре: `gate R-anchor-everything` → `confident: false … could not be resolved to a Go test function` | запуск CLI |
| Инвариант на enforced_by | `check_enforced_by_resolvable` проверяет ТОЛЬКО `check_*`-имена; `checkEnforcedByTestHasTeeth` — **no-op в Go-порте** → 287 битых ссылок молча проходят | `internal/invariants/enforcement.go:52-103` |
| `hotam due --today 2026-07-12` | OVERDUE: none, NEVER-REVIEWED: none — backfill выставил `review_after` в 2026-12-30…2027-01-11, цикл ревью молчит ещё ~6 месяцев при 79% stale контента | CLI + скан дат |
| `hotam inspect` без лимита | 1388 кандидатов, 1121 (81%) со score ≤ 4; 1387 из 1388 — одна эвристика lexical_claim_overlap; флага `--min-score` нет | CLI `--json --limit 0`; `cmd/hotam/inspect.go:29` |
| `hotam what-now` | топ-16 сигналов — один и тот же тип [P7] «REJECTED без структурного replaces edge» (16 узлов) | CLI |
| GraphIndex | используется только в `internal/diagnose/finding.go` и `internal/query/context.go:36`; линейный `ontology.RequirementByID` остался в `internal/query/show.go:79`, `internal/query/context.go:139`, `internal/proposal/mutate.go:239`; doc-comment `internal/ontology/index.go:5-12` преувеличивает покрытие | grep |
| gofmt | 22 файла не отформатированы (`gofmt -l internal cmd`) | gofmt |
| CI / Makefile / golangci | отсутствуют | ls |
| Coverage | loader 68.2%, generator 70.3%, paths 71.2%, ontology 80.3%; invariants 97%, methodology/registry 100% | `go test -cover` |
| go vet / go test | чисто, все пакеты зелёные; 0 TODO/FIXME в коде | запуск |
| CLAUDE.md | 22 464 B (цель < 15 KB). Крупнейшие блоки: Tool reference 6 358 B (28/40 записей — «Not ported»), Methodology 4 767 B | скан по секциям |
| Python-наследие в Go-коде | `internal/paths/project_root.go` резолвит корень через pyproject.toml `[tool.hotam-spec]`; `docs/development/ROADMAP.md` целиком описывает Python-эру | чтение |
| go.mod vs remote | `github.com/PHPCraftdream/HotamSpecGo` vs remote `…/HotamSpec.git` → `go install …@latest` невозможен; тегов релизов нет | cat go.mod, git remote |
| Механика массового обновления | `ProposedRequirement` имеет полноценный UPDATE-путь (claim/why/enforced_by/evidence/review-поля); батч-режима нет — один proposal на файл на один `land` | чтение mutate.go/apply.go |

## 2. Диагноз по 8 областям

### A. Онтология графа — 8 → 9
Причина: 16 REJECTED-узлов декларируют REPLACES-преемника только прозой (не machine-traversable — те же 16 сигналов [P7] в what-now); 7/8 conflicts и 7/16 assumptions содержательно описывают Python-систему; 2 DRAFT висят без движения.
Меры: 16 `ProposedRejection` с `replaced_by`; актуализация conflicts/assumptions в волне контент-рефреша; решить судьбу 2 DRAFT.
Проверка: `hotam what-now` без P7 replaces-сигналов; все 40 REJECTED-«REPLACES known» имеют структурное ребро; stale-grep по conflicts/assumptions = 0.

### B. Структурная защита от противоречий — 8 → 9
Причина: статус ENFORCED у 205/232 требований не подкреплён проверяемо — 70% enforced_by ведут на несуществующие тесты, инвариант это не ловит, gate fail-closed.
Меры: (1) перепривязать данные (см. E), (2) расширить `check_enforced_by_resolvable`: `Test*` — против реального скана тестового корпуса (`gate.BuildCheckToTestsMap`), `.py`/pytest-node-id — violation. Порядок обязателен: инвариант после чистки данных.
Проверка: битая фикстура fires; `all-violations` = 0 на обоих доменах; `hotam gate` confident:true на 10 выборочных якорях.

### C. Качество Go-порта — 8 → 9
Причина: 22 файла не проходят gofmt; pyproject.toml-резолюция корня — Python-наследие в Go-коде; GraphIndex внедрён на 2 из 5 мест, doc-comment завышает охват; module path расходится с remote; coverage 68–71% у слабых пакетов.
Меры: gofmt всего дерева + CI-гейт; довнедрить GraphIndex, исправить doc-comment; сделать маркер-файл основным путём резолюции корня, pyproject — явный legacy-fallback; coverage слабых пакетов ≥80%; module path (см. G).
Проверка: `gofmt -l internal cmd` пусто; `go test -race ./...` зелёный в CI; coverage loader ≥80, generator ≥78, paths ≥80; grep `RequirementByID(` вне ontology = 0.

### D. Простота сопровождения — 6 → 9
Причина: нет CI/Makefile/линтера — «зелёность» проверяется только вручную; одноразовый `cmd/seed-due` живёт в дереве; ROADMAP.md описывает несуществующую Python-систему; CONTRIBUTING говорит «branch from master», ветка — main.
Меры: GitHub Actions (build + gofmt-check + vet + test -race + идемпотентность gen-spec); Makefile с канон-командами; переписать ROADMAP; удалить `cmd/seed-due`; поправить CONTRIBUTING.
Проверка: `make check` воспроизводит всё, что делает CI; grep Python-артефактов по docs/development = 0.

### E. Актуальность требований — 4 → 9 (главный фронт)
Причина — три слоя: (1) контент: 155/232 SETTLED с Python-текстом, 184/232 включая enforcement/evidence; (2) enforcement-данные: 287/412 битые Python-ссылки; (3) процесс: freshness формально зелёный, но backfill проставил ревью без факта, `review_after` кластером на конец 2026 → будущий «review cliff» разом на ~232 требования.

**Стратегия — три волны:**
- **Волна A (механическая):** перепривязка enforced_by на реальные Go-тесты через `gate.BuildCheckToTestsMap`; без Go-эквивалента — честный даунгрейд ENFORCED→STRUCTURAL/PROSE. Скрипт → proposals → land батчами.
- **Волна B (содержательная):** переписывание claim/why/summary/evidence у 155 требований, 8–10 параллельных батчей по разделам конституции, каждый батч — resolver-ревью. Каждый UPDATE ставит `last_reviewed_at=today` и разнесённый `review_after` (устраняет review cliff). REJECTED не переписывать — только replaces-рёбра (область A).
- **Волна C (процессная):** регламент — review-mark только после фактической сверки, backfill запрещён (новое требование R-review-mark-requires-substantive-review).
- **Инфраструктурная предпосылка:** batch-режим `hotam land` — иначе ~184 отдельных приземления.

Проверка: stale-grep по SETTLED = 0; enforced_by без `.py` = 100%; `due --json` показывает `review_after`, размазанный по месяцам; первое настоящее ревью по циклу демонстрируемо.

### F. Удобство агента — 7 → 9
Причина: CLAUDE.md 22.4KB vs цель 15KB — 6.4KB съедает Tool reference (28/40 «Not ported»); what-now забит 16 однотипными сигналами; inspect без `--min-score` (81% шума); CONFRONT-шаг медиационного цикла помечен «not yet ported, scan by hand».
Меры: компакция генератора (Tool reference: ported полностью, not-ported — одна сводная строка; Methodology — заголовки-ссылки); `inspect --min-score N` (default ≈5) + сводка; группировка однотипных сигналов в what-now; портировать `confront` (переиспользовать claimTokens/markerHits из inspect.go).
Проверка: `wc -c CLAUDE.md` ≤ 15360 на всех трёх зеркалах; what-now топ-20 содержит ≥3 разных типа сигналов; `hotam confront` работает e2e; default-вывод inspect ≤ 40 строк с честным счётчиком подавленных.

### G. Применимость внешним проектам — 7 → 9
Причина: `go install` не работает из-за расхождения module path с remote; релизов/тегов нет; QUICKSTART сам признаёт «no published package or go install target yet».
Меры: переименовать module path в `github.com/PHPCraftdream/HotamSpec` (требует явного одобрения стьюарда); tag v0.1.0; CI-релиз бинарников; обновить QUICKSTART.
Проверка: с чистой машины после push+tag: `go install …@v0.1.0` даёт рабочий `hotam init`/`gen-spec`.

### H. Итоговое соответствие цели — 7 → 9
Причина: производная от E и B — система, чьё самоописание на 79% рассказывает о другой системе и чей enforcement-статус не верифицируем, не соответствует собственной цели.
Меры: закрытие E и B — основной рычаг; далее полный зелёный проход (all-violations 0, gate confident, due осмысленный, CLAUDE.md в бюджете).

## 3. Задачи по волнам (TaskList)

Заведены в TaskList сессии как #46–#63 (см. там же для live-статуса и blockedBy). Внутри волны
задачи безопасны для параллельного запуска (нет пересечений файлов/полей graph.json); между
волнами — реальная зависимость по данным/файлам, не только формальный порядок.

- **Волна 0** (соло, фундамент): #46 P0-1 Batch-режим land/apply-proposal
- **Волна 1** (параллельно, независимые правки): #47 P0-5 replaces-рёбра · #48 P1-2 inspect --min-score · #49 P1-3 группировка what-now · #50 P1-7 GraphIndex доводка · #51 P2-1 paths marker-first · #52 P2-3 чистка docs/development
- **Волна 2** (соло, данные): #53 P0-2 перепривязка enforced_by (blockedBy #46)
- **Волна 3** (параллельно): #54 P0-3 инвариант enforced_by (blockedBy #53) · #55 P0-4 контент-рефреш 155 требований (blockedBy #46, #53) · #56 P1-4 порт confront
- **Волна 4** (параллельно, после стабилизации контента): #57 P1-1 компакция CLAUDE.md (blockedBy #55, #56, #47) · #58 P2-2 удалить seed-due (blockedBy #55) · #59 P2-5 регламент freshness-ревью (blockedBy #55)
- **Волна 5** (последняя код-трогающая, сериализовано): #60 P2-4 coverage (blockedBy #51, #57) · #61 P1-5 gofmt+CI+Makefile (blockedBy #47,#48,#49,#50,#51,#52,#54,#55,#56,#57,#60)
- **Волна 6** (соло, требует явного одобрения стьюарда перед стартом): #62 P1-6 module path rename + v0.1.0 (blockedBy #61)
- **Волна 7** (финал): #63 P2-6 hotam version + релизные бинарники в CI (blockedBy #61, #62)

Особый момент по данным (не выражается через `blockedBy`, контролируется при исполнении):
задачи #47/#53/#55/#59 все пишут в один и тот же `domains/*/graph.json` через `hotam land`.
Черновики proposal JSON можно готовить параллельно, но сами вызовы `land` по одному домену
должны идти строго последовательно — иначе гонка на файле и `graph.lock`.

## 4. Честная оценка достижимости 9/10

- **Реально достижимо кодом/данными:** Онтология, Структурная защита, Качество Go-порта, Простота сопровождения, Удобство агента — при выполнении плана это ~9/10 без натяжек.
- **Актуальность требований:** 8–9 достижимо, но это крупнейшая статья трудозатрат (~184 требования содержательного ревью + 287 enforcement-перепривязок). Риск: после честного даунгрейда фиктивных ENFORCED цифра «205/232» временно упадёт — это правильно, но ревью нужно явно показать, что падение = рост честности.
- **Применимость внешним проектам:** 9 упирается в module rename + публикацию (решение стьюарда) и, по строгому ревью, может держаться на 8 без фактического внешнего использования.
- **Итоговое соответствие цели:** производная; закрытие E и B вероятно даёт 8–9, но гарантированной 9 нет — часть оценки зависит от прожитого времени/использования, не только от объёма работы.

Структурно недостижимого нет, но для «Актуальность требований» и «Итоговое соответствие цели» честный потолок ближайшей волны — уверенные 8 с траекторией на 9.
