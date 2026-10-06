# Ревью фреймворка: что улучшить, доработать, изменить

Дата: 2026-10-05
База: `19f3160` (самоисполняемые атомы, многоязычность, conformance, evidence/findings).
Метод: замеры по репозиторию, прогон `go vet` / `go test -timeout 60m ./...` (зелёные), `all-violations` на `hotam-spec-self` и `apps/Human`, живая проверка многоязычности на копии Human (ru+en, смена языка по умолчанию, возврат к одному языку, негативные сценарии).

Приоритеты: **P0** — нарушение жёсткого правила или обещания; **P1** — мешает принципу «требования как код» или работе с большой спекой; **P2** — заметная шероховатость; **P3** — гигиена.

## Что уже хорошо

- Принцип реализован: `Fact`/`Holds` — метод, doc-фраза и одна строка теста дают текст требования; ID, `implemented_by`, `verified_by` выводятся; атомарность проверяется инвариантами.
- Проверено на Human: смена 1987 → 1988 правкой только `Init` и `want` распространяется в SPEC и CLAUDE.
- Многоязычность строгая: пропуск, дубль, неизвестный язык, текст вне блоков, метки в одноязычном домене — отказ без подмены другим языком; устаревшие языковые выходы ловит `check_language_outputs_current`; смена языка по умолчанию и удаление языка чистят старые файлы.
- Тесты и `go vet` зелёные; самохостинг — 0 нарушений.

## Замеры

| Что | Значение |
|---|---|
| Go-код | 130 290 строк (≈ 55 % — тесты) |
| Крупнейшие пакеты (не тесты) | invariants 10,8k · cmd/hotam 9,6k · generator 9,1k · selfspec 8,1k · gate 8,0k |
| Команды CLI / инварианты / поля manifest | 28 / 130 / 27 |
| Полный `go test` | ≈ 25 мин; cmd/hotam 1250 с, gate 531 с, generator 516 с, invariants 464 с, selfspec 428 с |
| `all-violations` самохостинга / Human | 44 с / 2,4 с |
| Самохостинг: SETTLED / с `verified_by` | 264 / 9 (ENFORCED 189 через `enforced_by`, PROSE 55, STRUCTURAL 20) |
| Human: свой код / вендоренный код | 236 / 2235 строк |
| Корневой CLAUDE.md / Human CLAUDE.md | 24,7k / 4,8k символов |
| Документы автора | CONTRACT 877 строк, QUICKSTART 553, PROPOSAL-REFERENCE 912, CHANGELOG 2834 |

## P0

### P0-1. Локальные пути машины в репозитории
Новые `docs/PLAN-atomic-conformance-spec.md` и `docs/PLAN-atomic-multilingual-spec.md` содержали абсолютный путь до внешнего репозитория ktav-lang (3 места). Исторические — в `docs/checkpoints/**`, `docs/reviews/**`, `CHANGELOG.md` (пути вида `D:\…`, `C:\Users\…`). Нарушает правило «не писать частные пути машины в репозиторий».
**Сделать:** заменить на относительные или описательные ссылки («исходник спецификации Ktav»); добавить проверку (тест или pre-commit), которая ищет абсолютные пути Windows/home в отслеживаемых файлах.

## P1

### P1-1. Фреймворк не живёт по собственному принципу
Самохостинг (`hotam-spec-self`) не включает ни `discipline`, ни `requirements_authority: code`, ни `self_executing_atoms`. Из 264 SETTLED только у 9 есть `verified_by`; формулировки пишутся руками в `internal/selfspec/requirements_*.go`, доказательство — через имена `check_*`. Принцип «метод несёт свой текст» доказан на 15 фактах Human, но не на самом фреймворке.
**Сделать:** выбрать пилотную подсистему (рекордер или локализация) и перевести её требования на `Fact`/`Holds`; явно записать, какие классы требований остаются в виде инвариантов `check_*` и почему.

### P1-2. Медленный цикл разработки
Полный прогон ≈ 25 минут, `all-violations` самохостинга 44 с. При таком цикле агенты и люди пропускают проверку или ждут.
**Сделать:** быстрый слой (`go test -short` < 2 мин) с выносом e2e/бинарных прогонов под `testing.Short()` или build tag; профилировать `cmd/hotam` и `gate`; общий собранный бинарник для e2e-тестов пакета.

### P1-3. Комбинаторика переключателей manifest
Поведение задают `discipline`, `requirements_authority`, `claim_authority`, `public_surface_authority`, `scenario_authority`, `self_executing_atoms`, `conformance.rule_cases`, `languages`/`default_language`, `gen_profile`, `require_provenance`. Симптом: `QUICKSTART-CONSUMER.md` в одном абзаце требует `conformance.rule_cases: true` для обнаружения методов, а в следующем говорит, что `self_executing_atoms` работает без него (и Human работает без него).
**Сделать:** именованные профили (например `"profile": "atoms"`), которые разворачиваются в согласованный набор флагов; `init-project` создаёт актуальный профиль; матричный тест допустимых сочетаний; исправить противоречие в QUICKSTART.

### P1-4. Многоязычность не переводит значения
В английском виде: «Sex — М», «Role — фронтенд-разработчик», «Projects owner — я». Переводится только doc-фраза метода; строковые константы-значения остаются на языке автора.
**Сделать:** локализуемые значения — например, doc-блоки `>>>>> lang=` у констант/типов-перечислений или метод `Text(lang)`; отказ при неполном покрытии значений, как для фраз.

### P1-5. Текст для `bool` и отношений
«Живёт по Торе — true», «По факту занят тем, что подразумевает должность — false»; claim отношения склеивает доказательства в одну строку («… — false. The role implies — … Work practice — …»). Противоречит цели «минимальный читаемый текст».
**Сделать:** для `bool` — утверждение при `true` и авторская отрицательная фраза при `false` (вторая строка doc или `>>>>> not`); доказательства отношения — отдельным списком, не частью claim.

## P2

### P2-1. Порядок требований не по коду
В CLAUDE.md Human список начинается с хобби, год рождения четвёртый: порядок задаётся именами файлов.
**Сделать:** порядок по графу владения от корневого объекта (`Init`) или по порядку методов в объявлении типа.

### P2-2. Вендоринг тяжёлый и дрейфует
На каждый домен 2235 строк вендоренного кода против 236 своих; после обновления движка Human получил 3 нарушения (`check_recorder_current`, `check_ontology_vendor_current`, `check_domain_claude_md_current`).
**Сделать:** одна команда `hotam upgrade --domain` (перевендорить всё + регенерация + проверка); рассмотреть импорт рекордера как Go-модуля вместо копии; разделить рекордер на ядро (`Fact`/`Holds`) и conformance-часть, чтобы малым доменам не тащить всё.

### P2-3. Шероховатости многоязычности в реестре
- Включение второго языка требует переводов для всех REJECTED-записей истории (`claim_texts is required for every requirement in a multilingual domain`).
- Для явных записей `Claim` дублирует `ClaimTexts[default]`; пустой `Claim` не выводится из переводов, а берётся старый из графа — смена языка по умолчанию требует ручной правки каждой записи.
**Сделать:** выводить `Claim` из `ClaimTexts[default_language]`; для REJECTED допускать один язык (исторический) с пометкой.

### P2-4. Монолитные файлы
`internal/localization/localization.go` 1873, `internal/gate/test_exec.go` 1839, `cmd/hotam/gen_spec.go` 1656, `internal/recorder/canon/hotamspec.go` 1645 (26 экспортируемых функций, 22 типа), `internal/gate/spec_build.go` 1523.
Каталог локализации — карта, где ключ — английский исходный шаблон: правка формулировки в генераторе тихо ломает перевод до первого прогона этого пути рендера.
**Сделать:** разнести по ответственности; каталог — по идентификаторам шаблонов или со статическим тестом «каждый шаблон, переданный в `localization.Text`, есть во всех каталогах».

### P2-5. Объём документации автора
CONTRACT 877 строк, QUICKSTART 553, PROPOSAL-REFERENCE 912; корневой CLAUDE.md 24,7k символов. Новичку-потребителю нужен один короткий путь.
**Сделать:** QUICKSTART — один сквозной пример «метод → тест → sync → текст» на первой странице, остальное — ссылками; проверить бюджет корневого CLAUDE.md.

### P2-6. Процесс и статусы
Коммит `19f3160` — 206 файлов, +27,6k строк, три независимые темы в одном коммите: история и откат по темам невозможны. Статусы в `PLAN-atomic-*.md` — «реализация не начата» при реализованном коде.
**Сделать:** коммит на тему/фазу; обновлять статус плана в том же коммите, что и реализацию.

## P3

### P3-1. Мелочи генерации
- Двойное `spec` в пути шарда: `docs/gen/spec/en/spec/model.md`.
- В шапке документов `reader: (unresolved-reader)` у потребителя.

### P3-2. Долги самохостинга из `hotam what-now`
6 SETTLED закрываемых (PROSE/STRUCTURAL при ENFORCEABLE); конфликт `C-d20cf537` (`reviewability-vs-code-authority`) без движения; 40 требований просрочены на ревью, 24 ни разу не ревьюились.

### P3-3. Старый worktree #23
`.claude/worktrees/agent-a5efcd28f4546ff38` сохранён ради идеи инвалидации скомпилированных бинарников. Нужно решение: оформить идею задачей и удалить worktree.

## Цикл 2 (2026-10-06)

База: `bf6dbc4` (23 коммита 19f3160..bf6dbc4 закрывают находки цикла 1).
Метод: замеры по репозиторию; `go build ./...` (ok, 9 с) и `go vet ./...` (чисто); `go test -count=1 -short ./...` — дважды: 3м51с холодный, 2м40с тёплый, все пакеты ok; узкие прогоны по gate / selfspec / localization / loader / repohygiene; `hotam all-violations` — hotam-spec-self: 0 нарушений, hotam-dev: 0; `hotam what-now` self-домена; `hotam req show` по трём атомам локализации; ручная запись артефактов рекордера (`HOTAM_RECORD_DIR` во временный каталог ОС, репозиторий не тронут — `git status` чист). `apps/Human` не отслеживается git'ом, поэтому потребительские сценарии проверялись на тестах движка и двух закоммиченных доменах. Нумерация новых находок продолжает нумерацию задач цикла 1 (там заняты P1-1b, P1-6, P1-7, P2-7, P3-4, P3-5).

### Статус находок цикла 1

| Находка | Статус | Доказательство |
|---|---|---|
| P0-1 пути машины | закрыта | guard-тест `internal/repohygiene/repohygiene_test.go` зелёный (`go test -run TestNoPrivateMachinePathsInTrackedFiles ./internal/repohygiene/` → ok); по регэкспам guard'а в docs/checkpoints/, CHANGELOG.md, PLAN-atomic-*.md — 0 совпадений; остатки путей только в задокументированных allowlist-исключениях (frozen-история графа и proposals) |
| P1-1 не живёт по принципу | закрыта (дыра пилота → P1-8) | manifest self-домена: `self_executing_atoms` + пакет `internal/localization`; 3 атома `canon.Holds`/`canon.Fact` (internal/localization/atoms_test.go), в графе ENFORCED с реальными implemented_by/verified_by; какие классы остаются `check_*` и почему — docs/AUTHORED-SPEC-CONTRACT.md:951-952 |
| P1-2 медленный цикл | частично | 52 тест-файла с `testing.Short()`, внешний e2e скипается (cmd/hotam/external_e2e_test.go:52); полный `-short`: 3м51с холодно / 2м40с тепло — цель «< 2 мин» не достигнута (→ P3-8); тяжелейшие в тёплом прогоне: cmd/hotam 152с, selfspec 82с, gate 46с (у selfspec самый медленный одиночный тест 2.4с — время в сборке, не в тестах); против ≈25 мин полного прогона в цикле 1 — ускорение ~7-9x |
| P1-3 комбинаторика флагов | закрыта | `"profile": "atoms"` разворачивается при загрузке, неизвестный профиль — ошибка (internal/loader/profile.go); init-project пишет профиль; противоречие QUICKSTART устранено: «`rule_cases` нужен ONLY для `WithCase`» (docs/QUICKSTART-CONSUMER.md:227-228) |
| P1-4 значения не переводятся | закрыта | internal/gate/atom_source_values.go: `>>>>> lang=` у типизированных констант, `>>>>> lang=*` — verbatim; неполное покрытие языка и непереведённая константа в многоязычном домене — отказ (TestAtomValueIncompleteLanguageCoverageRejected, TestAtomValueUntranslatedStringConstantRejectedInMultilingualDomain — PASS) |
| P1-5 bool и отношения | закрыта | `not:`-фразы с валидацией дублей и порядка (internal/gate/atom_source_phrases.go:142-151); evidence убран из claim (internal/gate/atom_source_claims.go:29); edge-тесты selfspec — PASS |
| P2-1 порядок не по коду | закрыта | порядок по нарративу авторского теста (internal/selfspec/atom_derive.go:94-96); одноимённые методы разных типов — по фактическим call-site'ам (bfd5665); TestDiscoverAtomsFollowsTestNarrativeOrder, TestDiscoverAtomsOrdersSameNamedMethodsByCallOrder — PASS |
| P2-2 вендоринг | закрыта | `hotam upgrade -domain` (cmd/hotam/upgrade.go): перевендор рекордера/онтологии/registrydump + регенерация; banner-less (hand-modified) `registrydump/main.go` не перезаписывает (upgrade.go:144); рекордер и онтология — вендоренные копии под `check_recorder_current`/`check_ontology_vendor_current`, их перезапись при отличии — по контракту; вариант «рекордер как Go-модуль» не реализован (остаётся опцией) |
| P2-3 i18n-реестр | закрыта | пустой Claim выводится из `claim_texts[default]` (internal/selfspec/sync.go:119, 177); REJECTED-история одноязычна; смена языка по умолчанию ловится проекцией кода + exact-match валидацией (internal/loader/code_projection_test.go: переходы add/remove/default — PASS) |
| P2-4 монолиты | в основном | localization.go 1873→83 (каталоги разнесены по языкам, по 900), test_exec.go 1839→628, gen_spec.go 1656→1138, spec_build.go 1523→938; статический AST-тест покрытия каталога (TestCatalogCoversAllTextLookupTemplates) — PASS; canon/hotamspec.go (1645) не делился — он вендорится одним файлом, это часть контракта вендоринга (см. P2-2) |
| P2-5 объём документации | закрыта | QUICKSTART-CONSUMER.md 553→301 строка; Part 1 — один сквозной проверенный путь (init-project → Fact → sync-domain → текст), остальное свёрнуто ссылками в AUTHORED-SPEC-CONTRACT |
| P2-6 процесс и статусы | закрыта | в PLAN-atomic-*.md статусы «реализация не начата» отсутствуют; 23 коммита после базы — по одной теме на коммит |
| P3-1 мелочи генерации | закрыта | шарды плоские: `docs/gen/spec/internal/localization.md` (двойного `spec/en/spec` нет нигде в domains/**); шапка — `reader: domain-user`, `unresolved-reader` не встречается |
| P3-2 долги самохостинга | частично | 4 из 6 закрыты в ENFORCED (docs/reviews/2026-10-06-self-debt-decisions.md §3), 2 — честно не закрыты с объяснением и предложением резолверу; конфликт C-d20cf537 по-прежнему DETECTED — материалы и варианты подготовлены (§1 того же документа), ждёт решения человека; ревью-долг: 27 никогда (было 24), 40 просрочено |
| P3-3 worktree #23 | закрыта | каталога `.claude/worktrees/agent-*` на диске нет; идея реализована как код: content-hash инвалидация кэша бинарников на путях verdict и recording (internal/gate/compile_cache.go, 234ad7b) |

### Новые находки

### P1-8. Артефакты `canon.Holds`/`canon.Fact` не несут человеческого текста — ложный вечный STALE и машинное «доказательство» в SPEC
`atomScenario` строит сценарий только из qualified-символа метода (internal/recorder/canon/hotamspec.go:1393) — заголовок артефакта пустой: прогон атомных тестов с `HOTAM_RECORD_DIR` даёт `"title": ""` и шаг `desc: "github.com/...localization.Catalog.Supported true"`. Следствия: (а) свежий вывод claim строится из title артефакта (internal/selfspec/claim_derive.go:214-227), получает пустую строку, и `hotam req show` по всем трём пилотным атомам отдаёт `proof state: STALE` (internal/selfspec/requirement_state.go:100-109: тесты прошли, но committed Claim ≠ «свежему выводу») — сигнал дрейфа ложный и вечный, до PROVEN атом добраться не может; (б) SPEC-шард рендерит доказательство машинной строкой — docs/gen/spec/internal/localization.md:22: «`github.com/PHPCraftdream/HotamSpec/internal/localization.Catalog.Supported` — true» вместо предложения, вопреки обещанию SPEC.md (нарратив из реальных прогонов, не hand-written). При этом `all-violations` = 0: proof state считается только по запросу карточки (internal/query/show.go:90), инвариантов поверх него нет — реальный дрейф неотличим от этого ложного.
**Сделать:** рекордеру — класть в атомный артефакт человеческий заголовок (фраза метода: параметр `Holds`/`Fact`, штамп при discovery или разбор doc-комментария); в claim_derive — пустой title трактовать как «наррации нет» (`ok=false` → PROVEN), а не как дрейф; SPEC-шарду — рендерить доказательство атома из фразы/claim, а не из qualified-символа; surfaced-ость: показывать STALE-атомы в `what-now`/`UNENFORCED.md`, пока инварианта нет.

### P3-6. Guard машинных путей исключает каталоги целиком — новые файлы не проверяются
`allowSuffix` (internal/repohygiene/repohygiene_test.go:32-40) exempt'ит префиксы `proposals/`, `domains/hotam-spec-self/proposals/`, `domains/hotam-spec-self/docs/gen/`, `graph.json` — не только конкретные замороженные исторические файлы. Каталог proposals/ пополняется (task226, task236…): новый файл там с машинным путём guard молча пропустит — ослабляет обещание P0-1 «ищет абсолютные пути в отслеживаемых файлах».
**Сделать:** сузить allowlist до поимённого перечня замороженных файлов; всё, что не перечислено, сканируется на общих основаниях.

### P3-7. Fixpoint gen-spec: граница 3 итерации без ошибки при не-сходимости; список readers захардкожен дважды
Имена двух crystal-reading проверок повторяются в двух местах: internal/invariants/crystal_readers.go:14 и фильтр cmd/hotam/gen_spec_fixpoint.go:27. Добавление третьей проверки только в один список даст склейку pre-write и post-write нарушений. Граница `iteration < 3` (gen_spec_fixpoint.go:23) по исчерпании возвращает `nil` — несошедшийся кристалл запишется молча, без диагностики.
**Сделать:** единый источник списка readers (один экспортируемый набор имён, используемый обоими местами); при выходе по границе без сходимости — ошибка или явная диагностика в выводе gen-spec.

### P3-8. Быстрый слой `-short` выше цели 2 минуты
Полный `go test -short ./...`: 3м51с холодно, 2м40с тепло; основное время — сборка тестовых бинарников (cmd/hotam 152 с, selfspec 82 с при самом медленном одиночном тесте 2,4 с).
**Сделать:** измерить, что именно собирается в `-short` у cmd/hotam и selfspec (`go test -short -json`, время сборки против времени тестов); убрать из `-short` пути, которые компилируют вложенные бинарники/домены, или переиспользовать один собранный бинарник на пакет.

## Цикл 3 (2026-10-06)

База: `340c685`; diff: `9c22615..HEAD` (3 коммита). `go build ./...` и `go vet ./...` завершились с кодом 0, без вывода. Отдельные `go test -count=1 -short` успешны: `cmd/hotam` 28,238 с; `internal/selfspec` 48,995 с; `internal/gate` 5,923 с; `internal/localization` 0,366 с; `internal/repohygiene` 0,789 с. Узкие проверки: `go test -count=1 -short -run '^TestCrystalFixpoint' ./cmd/hotam` — ok, 0,915 с; `go test -count=1 -run TestRequirementState_ProvenWhenEmptyAtomTitleFallsBackToSourcePhrase ./internal/selfspec` — ok, 0,811 с. `go test ./...` и агрегированный `go test -count=1 -short ./...` по ограничению не запускались: полное время до 2 минут независимо не подтверждено. Запись CHANGELOG о 77 с в тёплом режиме не является измерением этого прогона.

Структурный просмотр затронутых объектов и методов, документации, мультиязычного рендеринга и добавленных пропусков `-short` дополнял команды; это не аудит универсального соответствия. Команды проверки ограничены перечисленными пакетами и тестами; `go test ./...` и агрегированный `go test -count=1 -short ./...` не запускались.

### Статус находок цикла 2

| Находка | Статус | Доказательство |
|---|---|---|
| P1-8 артефакты атомов без человеческого текста | частично | Исправлены proof-state и человекочитаемый текст пилота; мультиязычная регрессия — P1-9. |
| P3-6 allowlist guard путей | закрыта | `internal/repohygiene/repohygiene_test.go:24-52`; точный `allowFiles`, guard проходит. |
| P3-7 инварианты Crystal fixpoint | закрыта | `internal/invariants/crystal_readers.go:9-18` и `cmd/hotam/gen_spec_fixpoint.go:161-199`; общие `CrystalReaderCheckNames`, три узких теста проходят. |
| P3-8 цель быстрого слоя | частично | 38 существующих guard'ов (24 cmd, 11 self, 3 gate), все в первой инструкции, плюс новый proof-тест — 39. Только условные пропуски; тело вне short не изменено. Отдельные пакеты зелёные, агрегированный target по времени не измерен. |

### Новые находки

### P1-9. SPEC сохраняет исходный язык фраз атома при рендеринге другого языка

Реальный временный unit-тест `TestReviewCycle3EvidenceUsesRequestedLanguage` (удалён после проверки) создал in-memory AtomSourceIndex: languages ru/en, default ru; фразы предиката Русский предикат/English predicate и evidence Русское свидетельство/Evidence; артефакт holds с Verdict pass и шагами true/7. Вызвал `humanizeAtomSteps(index, atom, claim)`, затем `BuildSpecFromRowsForLanguage(graph, rows, "en")` с ClaimTexts для обоих языков. Авторские методы/рекордер в этой фикстуре не запускались: проверялся путь рендера записанного артефакта. Команда завершилась exit 1 из-за отсутствия Evidence — 7. Ниже выдержка реального вывода:

```text
go test -count=1 -run '^TestReviewCycle3EvidenceUsesRequestedLanguage$' ./internal/gate
FAIL TestReviewCycle3EvidenceUsesRequestedLanguage
**Claim:** English predicate.
- Then Русский предикат — true. — **held**
- Given Русское свидетельство — 7.
FAIL .../internal/gate
```

Причина: `internal/gate/spec_build.go:812,820,839-862` и `internal/gate/atom_source_claims.go:112-119` выбирают исходный язык; `internal/gate/spec_render.go:475-486` форматирует описание без локализации, а `internal/gate/spec_shards.go:133-169` повторно использует эти строки. Затронуты и Then, и Given; оба проявились в одном и том же пути вывода.

**Сделать:** формировать локализованные claim/evidence для каждого языка рендера; сохранять структурированные локализуемые шаги вместо замороженного первичного `Desc`. Добавить тесты ru/en, перестановки default language, holds/rule и failure-сценариев, включая текстовые значения.

### P3-9. Вводный текст SPEC называет действующий check будущим

Наблюдаемое устаревшее обещание в `domains/hotam-spec-self/docs/gen/spec/internal/localization.md:6`: «a future `check_spec_md_current`». Устаревший шаблон исходного текста находится в `internal/gate/spec_render.go:75`; фактическая регистрация check — `internal/invariants/spec_md_current.go:221-255`. Оба запуска `go run ./cmd/hotam all-violations --domain domains/hotam-spec-self` и `go run ./cmd/hotam all-violations --domain domains/hotam-dev` завершились без нарушений. Это остаток старого текста, а не изменение цикла 2.

**Сделать:** обновить вводный текст и эквиваленты в каталоге языков, затем регенерировать поддерживаемые проекции; не редактировать сгенерированные документы вручную.

`go run ./cmd/hotam req show <id> --domain domains/hotam-spec-self` для `R-catalog-supported`, `R-catalog-translated` и `R-missing-translation-message` показал `proof state: PROVEN`; `domains/hotam-spec-self/docs/gen/spec/internal/localization.md:22,32,42` содержит человеческие Then-строки. Оба запуска `all-violations` дали `0 violations — graph clean`. `what-now` self-домена: `C-d20cf537` без движения, 33 feature-blocked, 27 никогда не ревьюились, 40 просрочены; старую P3-2 не дублируем.

Итог цикла: две новые находки — P1-9 и P3-9; новых P0/P2 нет. Ревью не завершено до исправления находок.

## Цикл 4 (2026-10-06)

База: `c0873a1`; просмотрен diff `70d4c69..HEAD` и `git show --stat c0873a1`. Метод: проверка путей полной SPEC и шардов, legacy `Subject`/`Value`, ошибок атомов, смены default language и границы кэширования; сверка с проекциями доменов. `go build ./...` — exit 0; `go vet ./internal/gate ./internal/localization ./internal/repohygiene` — exit 0, без вывода. `go test -count=1 -short ./internal/gate ./internal/localization ./internal/repohygiene` — exit 0: gate 2,656 с, localization 0,848 с, repohygiene 2,987 с. Полный `go test ./...` и агрегированный `go test -short ./...` не запускались.

Дополнительное воспроизведение — временный `TestReviewCycle4FailureContextAndCacheRefutation`, подключённый через Go overlay из `.tmp-agent/`: `go test -count=1 -overlay=.tmp-agent/overlay.json -run '^TestReviewCycle4FailureContextAndCacheRefutation$' -v ./internal/gate` — PASS, 0,015 с. Это проверка AST-деривации и рендера синтетического RawJSON/RecordingResult, а не запуск авторского bool-метода. Контроли: false с `not:` и не-bool failure; повторная сборка строк из одних raw-байтов при default ru/en и рендер каждого языка. Временные материалы удалены после проверки. Три `req show` для `R-catalog-supported`, `R-catalog-translated`, `R-missing-translation-message` показали `proof state: PROVEN`.

### Статус находок цикла 3

| Находка | Статус | Доказательство |
|---|---|---|
| P1-9 язык Then/Given атомов | частично | Локализация исправлена: `humanizeAtomSteps` формирует карты всех языков (`internal/gate/spec_build.go:845-882`), рендер выбирает язык (`internal/gate/spec_render.go:478-520`), шарды используют тот же путь (`internal/gate/spec_shards.go:133-169`). Зелёный пакет gate включает `TestSpecLocalizedAtomSteps`: ru/en, оба default language, fact/holds/rule, переводимые константы, failure и шарды; legacy Subject/Value проверен отдельным тестом. Однако новый отказ на failed bool ухудшает диагностический SPEC — P1-10. |
| P3-9 будущий check в вводном тексте | закрыта | `internal/gate/spec_render.go:75`, каталоги ru/zh и поддерживаемые SPEC-шарды больше не называют `check_spec_md_current` будущим. Проверка localization и её каталога проходит; поиск старой английской фразы в domains не дал совпадений. |

Подозрение о потере `Texts` из-за `json:"-"` не подтверждено на действующем пути: кэш хранит исходные RawJSON, а не гуманизированные SpecRow (`internal/gate/test_exec_recording.go:193-195,274-284`); после декодирования тексты выводятся заново (`internal/gate/spec_build.go:770-828`). Временный тест повторно собрал строки из одинаковых raw-байтов с default ru/en, проверил наличие обоих языков в `Texts` и непустой правильный текст во всех четырёх рендерах. Сериализации SpecRow в рабочих потребителях не найдено; прямой JSON-roundtrip внутреннего шага действительно теряет карту, но это не используемый здесь путь кэша. `json:"-"` сам по себе не новая находка.

### Новые находки

### P1-10. Провал bool-атома без `not:` блокирует весь диагностический SPEC

**Наблюдалось:** для мультиязычного `holds` с verdict fail и наблюдаемым false без `not:` результат теряет failed-artifact и рендер возвращает только ошибку деривации. Фактический вывод временного теста:

```text
missing-not outcome passed=false failedArtifacts=0 problem="" sourceError="../../.tmp-agent/cycle4-fixture/spec/model/value.go:4: Box.Value language \"ru\": bool atom evaluated to false without a `not:` negation phrase"
missing-not bundle documents=nil; single document length=0
positive control sourceError=<nil> failedArtifacts=1
cache default=ru render=ru document length=2387
cache default=ru render=en document length=1541
cache default=en render=ru document length=2387
cache default=en render=en document length=1541
PASS
```

Механизм подтверждён кодом и воспроизведением: `humanizeAtomSteps` принудительно меняет verdict на pass для деривации (`internal/gate/spec_build.go:846-852`), но проверка bool false требует отрицательной фразы (`internal/gate/atom_source_claims.go:84-85`). `atomRecordingOutcome` возвращается с sourceError до добавления failed-artifact (`internal/gate/spec_build.go:823-830`); ошибка переносится в строку (`:310-313`), весь документ останавливается (`internal/gate/spec_render.go:169-173`), CLI возвращает `gen-spec: render SPEC bundle: …` (`cmd/hotam/gen_spec.go:432-434`). Ошибка честно называет false/отсутствие `not:`, но SPEC не сохраняет TestValue/наблюдение как failed-artifact и не показывает остальные успешные требования. Это регрессия диагностической проекции, не утверждение о потере verdict во всех иных API. Контроль с `not:` сохраняет failed-artifact и локализованные ОШИБКА/FAILED; обычный числовой провал также не блокирует деривацию.

**Сделать:** разделить валидацию нормативного текста и представление наблюдаемого провала; сохранять идентичность теста/атома, наблюдаемое false и диагностику вместе с успешными соседними атомами. Не ослаблять обязательность переводов и не подменять отсутствующий язык первичным. Добавить негативные bool-сценарии без `not:` для ru/en, обоих default language, полного рендера и шардов. Это поддерживает `R-authored-spec-projections-are-derived`, а не заменяет реальный провал ложным подтверждением.

### P3-10. На HEAD проекции обоих доменов не проходят проверку свежести

**Наблюдалось:** каждый отдельный запуск `go run ./cmd/hotam all-violations --domain domains/hotam-spec-self` и `go run ./cmd/hotam all-violations --domain domains/hotam-dev` завершился exit 1. Ниже существенные поля вывода без локальных путей машины:

```text
check_engine_docs_fingerprint_current: stamped 9a1c83988b8b6da9; current e65dda38e474797b
check_domain_claude_md_current: generated portion does not match a fresh run
2 violation(s) found
exit status 1
```

Штамп `9a1c83988b8b6da9` находится в `domains/hotam-spec-self/docs/gen/ENGINE-VERSION.md:9` и `domains/hotam-dev/docs/gen/ENGINE-VERSION.md:9`. Fingerprint хэширует относительные имена и байты всех обычных файлов трёх пакетов, включая testdata (`internal/gate/engine_fingerprint.go:30-76,109-139`); в diff изменился `internal/generator/testdata/fixture/SPEC.md`, поэтому изменение нельзя считать внешним для этого алгоритма. Проверка сравнения — `internal/invariants/engine_version_current.go:108-134`.

Причина не списана на временную фикстуру: независимый расчёт тем же алгоритмом по tracked blob'ам HEAD и по файлам на диске дал одинаковый результат:

```text
HEAD tracked hash: e65dda38e474797b
disk tracked hash: e65dda38e474797b
different contents: []
extra files: []
CRLF files: 0
```

`.tmp-agent/` вне трёх хэшируемых пакетов. Несовпадение кристаллов наблюдалось отдельно; что обе диагностики имеют одну причину, не доказано. `what-now` показывает эти два STRUCTURE-сигнала; прежний конфликт `C-d20cf537` и ревью-долг не переоформляются как новые находки.

**Сделать:** регенерировать поддерживаемые проекции и кристаллы обоих доменов, затем повторить `all-violations` на каждом и убедиться, что именно эти сигналы исчезли (`R-verify-closure-per-action`). Не редактировать сгенерированные документы вручную.

Итог: две новые находки — P1-10 и P3-10; новых подтверждённых P0/P2 нет. Локализация P1-9 на штатных путях исправлена, P3-9 закрыта; цикл ревью не завершён до устранения диагностической регрессии и проверки свежести проекций.

## Цикл 5 (2026-10-06)

База: `c409ba3` (HEAD ветки review-5; диапазон `28d9239..HEAD` — один коммит). Метод: разбор `git show --stat c409ba3` и путей рендера (`internal/gate/spec_build.go`, `spec_render.go`, `spec_shards.go`); `go build ./...` — exit 0; `go vet ./internal/gate` — exit 0 без вывода; `go run ./cmd/hotam all-violations` по обоим доменам (вывод ниже); `go run ./cmd/hotam what-now --domain domains/hotam-dev`. Проверка отображения проваленного атома — временный e2e-тест (`TestReviewCycle5FailedBoolRendersReadableSPEC`, подключён overlay'ем из `.tmp-agent/`, удалён после проверки): фикстура с реальным запуском рекордера — проваленный bool-атом без `not:` и проходящий сосед; многоязычный домен ru+en (рендер обоих языков и полный `BuildSpecDocumentsFromRows`), одноязычный домен, strict-контроль проходящей ветки. Полный `go test ./...` не запускался (по ограничению); outcome-уровень для обоих default language покрыт добавленным в c409ba3 тестом.

### Статус находок цикла 4

| Находка | Статус | Доказательство |
|---|---|---|
| P1-10 провал bool-атома без `not:` блокирует SPEC | закрыта | `atomRecordingOutcome`: проваленная ветка больше не ставит `sourceError` — при недоступной деривации сохраняются записанные шаги артефакта (`internal/gate/spec_build.go:821-831`), проходящая ветка строга как прежде (`internal/gate/spec_build.go:810-819`); добавлен outcome-тест на default ru/en (`internal/gate/atom_pipeline_test.go:190-239`). Временный e2e-тест (PASS, удалён): в многоязычном домене бандл рендерится, в документах ru и en провал показан читаемо — локализованное пояснение («Запись артефака `TestFlag` не прошла; его формулировка не показана как проверенная.» / «Recorded artifact `TestFlag` did not pass; …») плюс наблюдаемое значение (строка `- …Box.Flag — false` в ru, `- Given …Box.Flag false` в одноязычном), проходящий сосед рендерится локализованно рядом (`- Тогда Значение равно семи — 7. — **удержано**` / `- Then The value equals seven — 7.`), проваленная формулировка нигде не показана как проверенная; одноязычный домен — то же самое; strict-контроль: проходящий атом с пропущенным языком `en` по-прежнему даёт sourceError (`missing language block`), бандл отказывается рендериться. |
| P3-10 проекции не проходят проверку свежести | закрыта | `go run ./cmd/hotam all-violations --domain domains/hotam-spec-self` → `0 violations — graph clean`, exit 0; `go run ./cmd/hotam all-violations --domain domains/hotam-dev` → `0 violations — graph clean`, exit 0. Штампы переписаны на `e65dda38e474797b` в `domains/hotam-dev/docs/gen/ENGINE-VERSION.md:9` и `domains/hotam-spec-self/docs/gen/ENGINE-VERSION.md:9`. |

Регрессий не обнаружено: обязательность переводов не ослаблена (пропуск языка у проходящего атома — по-прежнему отказ), проваленная формулировка не подменяется успешной.

### Новые находки

### P3-11. AGENT-CONTEXT.md hotam-dev на HEAD перечисляет несуществующие нарушения; свежесть этого файла не покрыта ни одной проверкой

**Наблюдалось:** `domains/hotam-dev/docs/gen/AGENT-CONTEXT.md:18-19` (закреплён в c409ba3) подаёт как текущие top actions два [P1] STRUCTURE-сигнала: `check_domain_claude_md_current` («does not match what a fresh … run produces right now») и `check_engine_docs_fingerprint_current` («stamped 9a1c83988b8b6da9, but the current engine's fingerprint is e65dda38e474797b»). На том же HEAD оба сигнала ложны: `domains/hotam-dev/docs/gen/ENGINE-VERSION.md:9` в том же коммите штампован `e65dda38e474797b`, оба прогона `all-violations` дают 0, а живой `go run ./cmd/hotam what-now --domain domains/hotam-dev` показывает только [P0] enforcement-gradient и [P5] LATENT_CONNECTOR ×2 — обоих [P1]-сигналов нет. Файл — снимок середины коммита, оставшийся после закрытия P3-10, и он противоречит соседним файлам того же коммита: агент, загружающийся из AGENT-CONTEXT.md, видит две ложные [P1]-тревоги. Свежесть файла не проверяет никто: в `internal/invariants/` имени AGENT-CONTEXT нет вовсе (`grep -rn AGENT-CONTEXT internal/invariants/` — пусто); `check_engine_docs_fingerprint_current` покрывает ENGINE-VERSION.md, `check_domain_claude_md_current` — CLAUDE.md, а AGENT-CONTEXT.md/live-state пишутся из снимка нарушений «фазы один» до перезаписи проекций (`cmd/hotam/gen_spec.go:207-230,582`) и пост-проверки не имеют — поэтому устаревший файл незаметно проходит `all-violations`.

**Сделать:** перегенерировать `domains/hotam-dev` (`hotam gen-spec --domain domains/hotam-dev`), чтобы AGENT-CONTEXT.md перестал называть несуществующие нарушения; устранить корень — формировать снимок для AGENT-CONTEXT.md/live-state после фиксации штампа и кристалла (или включить их в fixpoint) и добавить механическую проверку свежести AGENT-CONTEXT.md, чтобы «0 violations» не соседствовал с файлом, перечисляющим нарушения.

Итог: P1-10 и P3-10 закрыты по существу, регрессий нет; одна новая находка — P3-11 (гигиена: устраняется одной регенерацией и одной проверкой); новых P0–P2 нет.

## Цикл 6 (2026-10-06)

База: `46d3936` (HEAD ветки review-6; диапазон `3f468af..HEAD` — один коммит). Метод: разбор `git show --stat HEAD` и дифа фикса P3-11 (`cmd/hotam/gen_spec.go`, `cmd/hotam/gen_spec_fixpoint.go`, `internal/invariants/agent_context_current.go`, `internal/invariants/publication_snapshot.go`, `cmd/hotam/agent_context_current_wiring.go`, `internal/selfspec/requirements_crystal.go`, `cmd/hotam/gen_spec_agentcontext_test.go`); идемпотентность — по два последовательных прогона `go run ./cmd/hotam gen-spec --domain domains/hotam-spec-self --claude-md CLAUDE.md --spec` и `go run ./cmd/hotam gen-spec --domain domains/hotam-dev --claude-md domains/hotam-dev/CLAUDE.md`, после каждого `git status --porcelain` и `git diff --stat` пусты; `go run ./cmd/hotam all-violations` по обоим доменам → `0 violations — graph clean`, exit 0; `go run ./cmd/hotam what-now` по обоим доменам совпадает с live-state/AGENT-CONTEXT; полный `go test ./...` не запускался (по ограничению), запускались узкие: добавленный фиксом `TestGenSpec_AgentContextConvergesInOnePass` — PASS; три временных зондирующих теста (подключались в cmd/hotam из `.tmp-agent/`, удалены после проверки): `TestTmpReview6AgentContextDatePin` (реальные домены, оба), `TestTmpReview6StaleStampFalseCrystal` (фикстура с подменой штампа), `TestTmpReview6LiveStateTamper`. Флага `--today` у `all-violations` нет (`flag provided but not defined: -today`, exit 2), поэтому дата-зависимость проверялась тестами, а не CLI.

### Статус находки цикла 5

| Находка | Статус | Доказательство |
|---|---|---|
| P3-11 AGENT-CONTEXT.md hotam-dev перечисляет несуществующие нарушения; свежесть файла не покрыта | закрыта | `git diff 3f468af..HEAD`: из `domains/hotam-dev/docs/gen/AGENT-CONTEXT.md` удалены ровно два ложных [P1]-сигнала (`check_domain_claude_md_current`, `check_engine_docs_fingerprint_current`); AGENT-CONTEXT обоих доменов теперь рендерится из того же publication-снимка, что live-state и кристалл (`cmd/hotam/gen_spec.go:820`, `cmd/hotam/gen_spec_fixpoint.go:107` — раньше файл сам вычислял `AllViolations` по недописанному диску); свежесть закрыта `check_agent_context_md_current` (`internal/invariants/agent_context_current.go`; реальная логика в `cmd/hotam/agent_context_current_wiring.go:80-147`; якорь `R-agent-context-md-current`, `internal/selfspec/requirements_crystal.go:78-95`); сходимость в один проход зафиксирована тестом фикса `TestGenSpec_AgentContextConvergesInOnePass` (PASS: подменённый штамп чинится одним genSpec-проходом, второй прогон байт-идентичен по всему сгенерированному дереву, `allViolationsAsOf` — 0); идемпотентность и «0 нарушений» обоих доменов — выше; ручная правка AGENT-CONTEXT.md ловится (временный тест: перезапись файла мусором → ровно 1 нарушение `check_agent_context_md_current`). |

Регрессий не обнаружено: публикация-снимок для live-state не смягчён, честные no-op сохранены (файла нет — no-op, `cmd/hotam/agent_context_current_wiring.go:93-95`; граф без `DomainDir` — no-op, `:81-83`; явные языки уходят в `check_language_outputs_current`, `:88-90`).

### Ответы на контрольные вопросы цикла

- **Идемпотентность** — подтверждена: два прогона gen-spec обоих доменов (с `--spec` для hotam-spec-self) не меняют дерево ни после первого, ни после второго.
- **Свежесть локализованных AGENT-CONTEXT** — покрыта: у доменов с явными языками корневой AGENT-CONTEXT не пишется (`cmd/hotam/gen_spec.go:818-820`), локализованный рендерится в составе бандла (`internal/generator/localized_documents.go:354`) и побайтно сравнивается `check_language_outputs_current` (`cmd/hotam/language_freshness_wiring.go:276-285`) — с оговоркой P3-13 о флейворе сравнения.
- **Дата** — пин «как на дату файла» работает только для файлов, случайно несущих `--today` в тексте freshness-адвизори; для hotam-dev это не так → новая P2-7. Сами счётчики «(as of …)» и freshness-сообщения прикрепляются к дате файла и стареют молча, но самофактурирующе: строка помечена датой, живое значение даёт `hotam due`; контент, протекающий через снимок нарушений фазы-1 (top actions), от пина не зависит и дрейфует честно.

### Новые находки

### P2-7. check_agent_context_md_current краснеет от календарной даты для файлов без `--today` в тексте — hotam-dev ложно «протухнет» уже на следующий день после генерации

**Наблюдалось:** пин «судить файл как на записанную в нём дату» реализован регэкспом `agentContextTodayRE` (`--today (\d{4}-\d{2}-\d{2})`, первое вхождение; `cmd/hotam/agent_context_current_wiring.go:78,108-110`). Дата попадает в AGENT-CONTEXT.md только через сообщения freshness-адвизори «run `hotam due --today %s`» (`internal/diagnose/freshness_signals.go:38-39,50-51`). У hotam-spec-self такие [P7]-строки есть (`domains/hotam-spec-self/docs/gen/AGENT-CONTEXT.md:19-20`) — регэксп находит дату, и временный тест даёт 0 нарушений и при `today=2026-10-06`, и при `today=2030-01-01`. У hotam-dev freshness-адвизори нет (SETTLED несут будущие `review_after`, например `domains/hotam-dev/graph.json:77` — `2027-01-12`), `--today` в его AGENT-CONTEXT.md не встречается ни разу — регэксп не совпадает, проверка рендерит с `time.Now()` (`cmd/hotam/agent_context_current_wiring.go:45`), и строка счётчиков «OVERDUE 0 (as of 2026-10-06)» (`internal/generator/agentcontext.go:163,174`) на любую дату позже сравнивается против рендера с другим «(as of …)». Тот же временный тест: hotam-dev — 0 нарушений при `2026-10-06`, **1** при `2030-01-01` («…does not match what a fresh `hotam gen-spec` run produces right now…»). Автономный `all-violations` передаёт `time.Now()` и флага `--today` не имеет, поэтому уже на следующий после генерации день `go run ./cmd/hotam all-violations --domain domains/hotam-dev` перестаёт быть зелёным при неизменном дереве — ровно то «краснеет каждую полночь», которое коммит 46d3936 объявил решённым. Тест-сьют структурно не видит пробел: у фикстур без review-метаданных freshness-адвизори есть всегда, а дату тесты передают явно через `allViolationsAsOf`, никогда — wall clock. Гейты land/sync-self самосогласованы (пишут и проверяют одной датой, `cmd/hotam/all_violations.go:139-145`), краснеет именно автономный all-violations.

**Сделать:** убрать зависимость пина от случайного присутствия `--today` в тексте: писать в AGENT-CONTEXT.md явный служебный штамп даты генерации (или исключить today-зависимые строку счётчиков и freshness-сообщения из сравниваемого рендера), либо добавить `all-violations` флаг `--today` с прокидыванием в `AllViolationsAsOf`; добавить тест «файл домена без freshness-адвизори остаётся зелёным на следующий день».

### P3-12. docs/gen/live-state.md (не-локализованный) по-прежнему не покрыт проверкой свежести

**Наблюдалось:** P3-11 называла пару «AGENT-CONTEXT.md/live-state» без пост-проверок; фикс добавил проверку только для AGENT-CONTEXT.md. В `internal/invariants/` имя live-state не встречается вовсе (`grep -rn "live-state" internal/invariants/` — пусто; из инвариантов файл не читает никто — только генератор, docbundle-инвентарь и тесты). Временный тест: после чистого `genSpec` полная перезапись `docs/gen/live-state.md` мусором → `allViolationsAsOf` = **0 нарушений**; та же перезапись соседнего AGENT-CONTEXT.md → 1 нарушение `check_agent_context_md_current`. Ручная правка или дрейф live-state.md проходит `all-violations` молча — та же дыра класса P3-11, в соседнем файле.

**Сделать:** распространить `check_agent_context_md_current` на live-state.md (тот же publication-рендер, тот же снимок) или завести одну проверку пары «live-state.md + AGENT-CONTEXT.md».

### P3-13. Сравнительный рендер `check_domain_claude_md_current` и `check_language_outputs_current` использует полный флейвор фазы-1, а gen-spec пишет publication-флейвор — ложное «кристалл устарел» при живом disk-нарушении

**Наблюдалось:** genSpec пишет кристалл, live-state и AGENT-CONTEXT из publication-снимка — диск-проекционные нарушения фазы-1 выброшены, кроме SPEC (`internal/invariants/publication_snapshot.go:65-79`; `cmd/hotam/gen_spec.go:190,198`), а `checkDomainClaudeMDCurrentReal` рендерит эталон с нефильтрованным priorViolations фазы-1 (`cmd/hotam/claude_md_current_wiring.go:152`), `check_language_outputs_current` — то же для всего локализованного бандла, включая локализованные live-state/AGENT-CONTEXT (`cmd/hotam/language_freshness_wiring.go:113-114`, `internal/generator/localized_documents.go:351-354`). Пока любая disk-проекция нарушена (штамп ENGINE-VERSION, claim-scenario), эталон содержит STRUCTURE-сигнал, записанный файл — нет. Временный тест на фикстуре: чистый `genSpec`, затем подмена штампа без регенерации → `allViolationsAsOf`: `check_engine_docs_fingerprint_current`=1 (честное), **`check_domain_claude_md_current`=1 (ложное)**, `check_agent_context_md_current`=0 (новая проверка в том же состоянии зелёная — её Why прямо называет такой файр недопустимым, `internal/invariants/agent_context_current.go:52-57`); повторный `genSpec` переписал CLAUDE.md побайтно идентично (sha256 совпал) — вердикт «does not match what a fresh run produces» был ложным. Асимметрия существовала и до 46d3936 (publication-флейвор записи появился раньше: `cmd/hotam/gen_spec.go:190,198` идентичны в `3f468af`), коммит её не создал — но применил правильный принцип только к одному из трёх артефактов. Для `check_language_outputs_current` механизм тот же по коду (фикстурного прогона локализованного домена не делалось).

**Сделать:** скармливать обоим сравнительным рендерам тот же publication-флейвор (`invariants.PublicationViolationsFromPhaseOne`), что и записи, — выбор флейвора вынести в одно место рядом с `publication_snapshot.go`.

Итог: P3-11 закрыта по существу, регрессий нет; новых находок три — P2-7 (ложное срабатывание новой проверки от календарной даты на hotam-dev), P3-12 (live-state.md без проверки свежести), P3-13 (флейвор-асимметрия запись/сравнение у кристалла и локализованного бандла).

## Цикл 7 (2026-10-06)

**База:** HEAD 2a07ce9 (дерево чистое). **Метод:** `go build ./...`, `go vet` (cmd/hotam, internal/invariants, internal/generator) — чисто; идемпотентность gen-spec обоих доменов (`go run ./cmd/hotam gen-spec --domain domains/hotam-spec-self --claude-md CLAUDE.md --spec`; `go run ./cmd/hotam gen-spec --domain domains/hotam-dev --claude-md domains/hotam-dev/CLAUDE.md`; `git status --porcelain` до/после — пусто оба раза); `all-violations` обоих доменов — «0 violations — graph clean»; пробы дат и строк через временный тест в cmd/hotam (использовал `allViolationsAsOf` и `generator.RenderClaudeMDFromTemplate` на реальных графах, удалён после ревью); узкий прогон новых тестов репозитория `go test -run 'TestClaudeMDCurrent_UsesPublicationFlavor_ComparativeRender|TestLiveStateCurrent|TestGenSpec_AgentContextConvergesInOnePass' ./cmd/hotam` — ok.

### Статус находок цикла 6

| Находка | Статус | Факт закрытия |
|---|---|---|
| P2-7 (AGENT-CONTEXT краснеет от календарной даты) | закрыта | дата берётся из первого совпадения `(as of YYYY-MM-DD)` либо `--today YYYY-MM-DD` (`cmd/hotam/agent_context_current_wiring.go:55-68,117-120`); в AGENT-CONTEXT обоих доменов теперь безусловная строка счётчиков «… (as of 2026-10-06)» (`domains/hotam-dev/docs/gen/AGENT-CONTEXT.md:23`, `domains/hotam-spec-self/docs/gen/AGENT-CONTEXT.md:24`); проба `allViolationsAsOf` по обоим доменам: 0 нарушений на 2026-10-06 и 2026-11-01 |
| P3-12 (live-state.md без проверки свежести) | закрыта | новая `check_live_state_md_current` (`internal/invariants/live_state_current.go`, реальная логика в `cmd/hotam/live_state_current_wiring.go`), якорь R-live-state-md-current (SETTLED/ENFORCED, `internal/selfspec/requirements_crystal.go:97-115`); тесты: перезапись мусором даёт ровно 1 нарушение, дата-пин — `TestLiveStateCurrent_StampedDatePin`, `TestLiveStateCurrent_GarbageYieldsExactlyOneViolation` (pass) |
| P3-13 (флейвор-асимметрия запись/сравнение) | закрыта | единый селектор `invariants.PublicationViolationsFromPhaseOne` теперь скармливают все сравнительные рендеры: кристалл (`cmd/hotam/claude_md_current_wiring.go:142,157`), AGENT-CONTEXT и live-state (`cmd/hotam/live_state_current_wiring.go:121`), локализованный бандл и кристаллы (`cmd/hotam/language_freshness_wiring.go:57,122,427,460`); пиннинг — `TestClaudeMDCurrent_UsesPublicationFlavor_ComparativeRender` (подмена штампа ENGINE-VERSION при чистом остальном даёт ровно 1 честное fingerprint-нарушение и 0 ложных про кристалл/AGENT-CONTEXT; pass) |

Штамп `- **generated:** <дата>` присутствует только в `docs/gen/live-state.md` обоих доменов (`:3`); в CLAUDE.md/AGENT-CONTEXT его нет — кристалл и AGENT-CONTEXT используют нестампованный блок (`internal/generator/livestate.go:53-62,88,177-186`). Регрессий не найдено: gen-spec обоих доменов идемпотентен, `all-violations` зелёный, честные no-op и publication-флейвор на месте.

### Ответ на отдельный вопрос цикла: зависит ли кристалл от календарной даты

Да, зависит — и это оставшаяся находка. Проба на реальных, полностью чистых деревьях: `allViolationsAsOf` даёт 0 нарушений на 2026-10-06 и 2026-11-01, но ровно 1 `check_domain_claude_md_current` на обоих доменах на 2027-06-01 при неизменном дереве (live-state и AGENT-CONTEXT при этом молчат — их пины работают). Механизм локализован рендером кристалла на две даты через `generator.RenderClaudeMDFromTemplate`: различие ровно одна строка — блок DOMAIN-MAP, `- **open actions** — 3 (top: …)` превращается в `- **open actions** — 4 …`, когда ближайший будущий `review_after` графа пересекается (`domains/hotam-dev/graph.json:77` — `2027-01-12`; в hotam-spec-self их 701). Проверка сравнивает как на календарное «сегодня» (`cmd/hotam/claude_md_current_wiring.go:55`), сам кристалл дату генерации не несёт, а флаг `--today` у `hotam all-violations` отсутствует (`cmd/hotam/all_violations.go` — флага нет; `allViolationsAsOf` доступен только тестам), т.е. в день пересечения ближайшего review_after автономный `all-violations` на неизменном дереве ложно краснеет, и «полечить» его нечем, кроме регенерации.

### Новые находки

### P2-8. check_domain_claude_md_current краснеет на неизменном дереве в день пересечения ближайшего review_after — пина даты у кристалла нет и задать дату вручную нельзя

**Наблюдалось:** пробы описаны выше (1 ложное нарушение `check_domain_claude_md_current` на обоих реальных доменах на 2027-06-01; строка-виновник `- **open actions**` в DOMAIN-MAP, смена 3→4). Штамповать дату в видимой части кристалла нельзя — тогда CLAUDE.md будет переписываться ежедневно (это осознанно исключено в `internal/generator/livestate.go:54-57`), поэтому наивное копирование пина P2-7/P3-12 не подходит.

**Сделать:** дать сравнительному рендеру кристалла честный пин без штампа в видимом тексте — например: (а) неявный HTML-комментарий-штамп в сгенерированной части (`<!-- generated: YYYY-MM-DD -->`), по которому `checkDomainClaudeMDCurrentReal` судит как на дату файла (по аналогии с `agentContextStampedDate`/`liveStateStampedDate`), либо (б) пин от штампа того же прогона в `docs/gen/live-state.md` (файл уже несёт `- **generated:**` и пишется тем же `genSpec`), либо (в) флаг `--today` у `hotam all-violations` с прокидыванием в `AllViolationsAsOf`. В любом варианте — тест «кристалл домена с будущим review_after остаётся зелёным на день пересечения при неизменном дереве».

Находок P0/P1/P3 нет. Гигиена: машинных путей в отслеживаемых файлах нет (только синтетика в тестах `internal/generator/relpath_test.go` и описание старой находки в самом отчёте); счётчики после добавления инварианта и требования сходятся (269 SETTLED, 345 узлов, `Other (103)`, R-live-state-md-current [E] в AGENT-CONTEXT `:42`); мусорных файлов нет; `.tmp-agent` удалён, временный тест удалён.

Цикл ревью завершён: P2-7, P3-12, P3-13 закрыты по существу, регрессий нет; одна новая находка P2-8.

## Цикл 8 (2026-10-06)

**База:** HEAD c2f3fcb (дерево чистое). **Метод:** `go build ./...`, `go vet ./cmd/hotam` — чисто; идемпотентность gen-spec обоих доменов (`go run ./cmd/hotam gen-spec --domain domains/hotam-spec-self --claude-md CLAUDE.md --spec`; `go run ./cmd/hotam gen-spec --domain domains/hotam-dev --claude-md domains/hotam-dev/CLAUDE.md`; `git status --porcelain` до/после — пусто оба раза); `all-violations` обоих доменов — «0 violations — graph clean»; пробы через временный тест в cmd/hotam на фикстуре `p3_12Fixture` (кристалл + `allViolationsAsOf`, файл удалён после ревью, дерево чистое); узкий прогон `go test -run 'TestCrystalCurrent_PinnedToGenerationDate|TestLiveStateCurrent|TestAgentContextCurrent' ./cmd/hotam` — ok. Полный `go test ./...` не запускался (зелёный на HEAD по условиям цикла).

### Статус находок цикла 7

| Находка | Статус | Факт закрытия |
|---|---|---|
| P2-8 (кристалл краснеет от календарной даты в день пересечения review_after) | закрыта | `checkDomainClaudeMDCurrentReal` теперь судит кристалл как на дату генерации: `domainGenerationDate` (`cmd/hotam/generation_date.go:21-29`) читает `- **generated:** <дата>` из `docs/gen/live-state.md`, иначе `(as of <дата>)` из `docs/gen/AGENT-CONTEXT.md`, и подменяет `today` сравнительного рендера (`cmd/hotam/claude_md_current_wiring.go:138-142`); домены без штампа остаются на календарном дне. Пиннинг: `TestCrystalCurrent_PinnedToGenerationDate` (`cmd/hotam/live_state_current_test.go:196-225`, pass). Пробы: (1) на минимальном fixture-домене `allViolationsAsOf(dir, "2030-01-01")` после генерации на 2026-10-05 — 0 нарушений `check_domain_claude_md_current`; (2) реальный дрейф ловится: добавление требования в graph.json без регенерации даёт нарушение «…does not match what a fresh `hotam gen-spec --claude-md` run produces right now…» на 2026-10-06; (3) оба реальных домена идемпотентны, `all-violations` = 0 на обоих. Регрессий нет |

### Остаточный риск consumer-домена без штампа (принятый)

Проба на минимальном consumer-домене (`gen_profile: "consumer"`, непустой граф, `docs/gen/live-state.md` не пишется, `domainGenerationDate` → `ok=false`): `allViolationsAsOf` на 2030-01-01 НЕ даёт `check_domain_claude_md_current` — кристалл consumer-профиля не зависит от пульса свежести (нет DOMAIN-MAP open-actions для одиночного домена), поэтому ложного красного при смене календарного дня не возникает. Сформулированный в фикс-коммите остаток («localized/empty/consumer остаются календарно-зависимыми») для кристалла consumer-профиля не воспроизводится как ложное красное; за локализованными доменами (`check_language_outputs_current`) остаток фиксируется как принятый, без воспроизведения.

### Новые находки

Находок P0–P3 нет, цикл ревью завершён.

Гигиена: машинных путей в отслеживаемых файлах нет (единственное вхождение — синтетические фикстуры `D:\dev\proj` в тесте `internal/generator/relpath_test.go:12,31,34,40-41` — это тестовые данные, не утечка); мусорных файлов нет, `.tmp-agent` и временный тест удалены, `git status --porcelain` чист; счётчики и проекции свежие (gen-spec обоих доменов прошёл без изменений дерева; штамп `2026-10-06` в `docs/gen/live-state.md` обоих доменов; 269 SETTLED / 345 узлов / `Other (103)` в AGENT-CONTEXT). `what-now` показывает только известное: P3-2 (конфликт C-d20cf537 ждёт решения человека) и advisory по review-freshness (29 never-reviewed / 40 overdue — известный, сознательно отложенный пункт заполнения freshness-метаданных).

Дополнение оркестратора: пробы на реальных доменах, которых не хватало ревью. Временный тест в `cmd/hotam` (удалён) вызвал `allViolationsAsOf` для `domains/hotam-dev` и `domains/hotam-spec-self` на датах 2026-11-01, 2027-06-01 и 2030-01-01 — 0 нарушений на каждой дате обоих доменов (до фикса P2-8 на 2027-06-01 было по одному `check_domain_claude_md_current`). Вывод ревью о consumer-профиле принят как «не воспроизведено», не как доказанное отсутствие зависимости.
