# AUTHORED-SPEC-CONTRACT.md — контракт authored executable specification

Статус: действующий контракт, **версия 2** (расширение под сценарно-генерируемую спеку,
волны W1–W3.1 движка; дизайн-документ сценарного слоя — `../PLAN-scenario-generated-spec.md`).
Версия 1 (контракт authored-ссылок `implemented_by`/`verified_by`, дизайн-документ
`../PLAN-authored-spec-discipline.md`) сохранена в §1–§7 и §9; сценарный слой — в §8.
Заменяет отменённый контракт генерации кода (`gen-code`, история сохранена в git; см. §9 "История").
Область: правила, по которым HotamSpec является дисциплиной построения и проверки **authored
executable specification** — не генератором кода. Агент, поняв предметную область, пишет
код-спеку (модели → поля → методы → сценарные тесты, генерирующие нормативный текст) **руками**;
движок держит структурный пол и генерирует только производные проекции документации.

## 0. Переворот в одной фразе

HotamSpec — **не генератор кода**, а **дисциплина построения и проверки authored executable
specification**. Фреймворк не пишет модель за агента: он **заставляет агента оставить проверяемый
результат** (реальные ссылки, реальные тесты) и генерирует лишь **производные проекции**
(`REQUIREMENTS.md`, `MODELS.md`, `TRACEABILITY.md`, `COVERAGE.md`, `UNENFORCED.md`) — не сам код.

Рекурсивно: сам HotamSpec построен так же — но с ЗАЯВЛЕННОЙ асимметрией. Его engine-код
(`internal/ontology`, `internal/proposal`, ...) authored руками; домен `hotam-spec-self` связывает
движковые требования с этим реальным кодом через те же `implemented_by`/`verified_by` поля и держит
над собой СТРУКТУРНЫЙ пол (реальные символы/тесты/ссылки, `check_self_requirements_match_registry`
+ RAC-A/B). НО домен НЕ объявляет `discipline: "full"` в `manifest.json` и потому НЕ opt-in в ПОЛНУЮ
сценарно-генерируемую спеку, которую требует от consumer-доменов (`check_settled_requires_scenario`
и смежные гейты — honest no-op без opt-in). Это осознанный долг, не тайное исключение: на 2026-07-31
только 43 из 259 SETTLED требований домена не имеют носителя (ни `enforced_by`, ни `INHERENTLY_PROSE`,
ни `implemented_by`+`verified_by`) — остальные проходят через движковый `enforced_by` (180) или честно
помечены `INHERENTLY_PROSE` (36). Замер задачи #402: временный flip `discipline:"full"` даёт ровно 43
нарушения `check_settled_requires_scenario` (не ~289 из ранних оценок — те не учитывали, что `enforced_by`
сам по себе удовлетворяет гейт). Закрыть долг — снабдить носителем эти 43 и провести сценарную
миграцию, а не переключить флаг.

## 1. Authored `spec/` слой

```
domains/<d>/
  manifest.json            # purpose/goals/director (authored, hand-edited)
  graph.json               # требования/оси/процессы/конфликты (через mediation loop)
  spec/                     # AUTHORED код-спека — пишется РУКАМИ, движок НЕ перезаписывает
    model/                 # объектная модель: агрегаты, value-objects, поля, инварианты
    application/           # операции/сценарии (бизнес-действия)
    policy/                # политики/гейты/предикаты
    tests/                 # содержательные executable-тесты (рядом с кодом — тех. выбор Go-идиомы)
    go.mod
  docs/gen/                # ГЕНЕРИРУЕМЫЕ проекции (движок перезаписывает)
    REQUIREMENTS.md MODELS.md TRACEABILITY.md UNENFORCED.md COVERAGE.md PIPELINE.md
```

Ключевое различие: `spec/` — **авторский, не перезаписывается** движком. `docs/gen/` —
**производный, всегда безопасно перезаписать**. Тесты по умолчанию раскладываются РЯДОМ с кодом
(`spec/model/risk_test.go` возле `spec/model/risk.go`, Go-идиома: same-package тест видит
unexported конструкторы/инварианты); `spec/tests/` — для кросс-модельных сценариев. Для резолвера
раскладка безразлична — `verified_by` несёт явный путь.

Признак самого HotamSpec как self-hosting-домена (`hotam-spec-self`): здесь "authored spec/" — это
сам движок (`internal/`, `cmd/`) относительно корня репозитория (там, где лежит `go.mod`), а не
поддиректория `spec/` внутри `domains/hotam-spec-self/` (резолюция корня — см. §5).

## 2. Схема связи: requirement → code → test

Каждое требование несёт два ОРТОГОНАЛЬНЫХ опциональных поля (`internal/ontology/requirement.go`,
`ImplementedBy`/`VerifiedBy`):

```json
{
  "id": "R-risk-owner-required",
  "enforcement": "ENFORCED",
  "implemented_by": ["spec/model/risk.go:NewRisk"],
  "verified_by":    ["spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"]
}
```

- **`implemented_by`** — где требование ВОПЛОЩЕНО: путь-квалифицированная ссылка `file:symbol` на
  authored-код (функция, метод или тип). Отвечает на вопрос "ГДЕ это реализовано".
- **`verified_by`** — где требование ДОКАЗАНО: путь-квалифицированная ссылка `file:test` на реально
  запускаемый `func TestXxx(t *testing.T)`. Отвечает на вопрос "ГДЕ это доказано".

Два разных механизма, никогда не сливаются в одно поле и не считаются алиасами друг друга
(R-spec-link-embodied-vs-proven). Символ, воплощающий требование без теста, который его доказывает
(или наоборот), — неполная связь: `ENFORCED` требует ОБЕ половины вместе
(R-enforced-requires-enforcer-or-authored-link).

`verified_by` — не переименование старого `enforced_by`: `enforced_by` остаётся отдельным полем
для движкового механизма (голые идентификаторы — `check_x` резолвится через реестр инвариантов
`invariants.All.Get`, `TestY` — через repo-wide AST-скан `gate.TestFuncNames` над `internal/` и
`cmd/`). `implemented_by`/`verified_by` — path-qualified ссылки, резолвящиеся парсингом ОДНОГО
названного файла в доменном `spec/` (или, для self-hosting домена, в engine-репозитории — см. §5).
Два раздельных поля, не два имени одной вещи.

### Формат `implemented_by`

`<path>:<symbol>` — путь относительно домена (или engine-корня для self-hosting), затем имя
top-level функции, метода (`Type.Method` квалифицирует получателя явно; голое имя матчит любой
получатель) или типа. Пример: `spec/model/risk.go:NewRisk`,
`internal/ontology/lifecycle.go:Lifecycle`.

### Формат `verified_by`

`<path>:<TestName>` — путь, затем имя реального `func TestXxx(t *testing.T)`. Имя ОБЯЗАНО начинаться
с `Test`. Пример: `spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner`.

## 3. Гейт готовности

Требование **может быть SETTLED без кода** — это честный roadmap-долг (намерение зафиксировано, не
скрытый пробел). Переход в **`ENFORCED` требует настоящих носителей** — ЛИБО движкового механизма
(`enforced_by` непуст), ЛИБО authored-механизма (`implemented_by` И `verified_by` оба непусты).
Дизъюнктивный гейт: ни один из двух путей не является обязательным по отдельности, обязательно
наличие хотя бы одного (`check_enforced_requires_enforcer_or_authored_link`,
R-enforced-requires-enforcer-or-authored-link). Никаких `ENFORCED` без носителя.

### 3.1 Манифест домена: поле `profile`

Манифест домена (`manifest.json`) может нести необязательное поле
`"profile"` — именованный профиль, который при загрузке (`internal/loader`)
раскрывается в набор флагов манифеста. Сейчас известен один профиль:
`"atoms"` — `discipline: "full"`, `requirements_authority: "code"`,
`self_executing_atoms: true`, `gen_profile: "consumer"`. Правила: профиль
раскрывается в одном месте при загрузке; явно указанный в манифесте флаг
всегда побеждает значение профиля; неизвестное имя профиля — ошибка загрузки.
Домены без поля `profile` (в том числе все существующие) загружаются без
изменений. `conformance.rule_cases` профилем не задаётся — он нужен только
для семантики `WithCase`, а не для обнаружения `Fact`/`Holds`-методов.

## 4. Послойная прогрессия покрытия (общее → частное)

Покрытие authored `spec/` идёт послойно, тем же принципом, что и founding домена
(R-authored-spec-layer-progression, refines R-domain-founded-in-wave-order):

1. **Модели** — сначала ВСЕ модели домена (скелет объектной модели целиком: какие
   агрегаты/сущности/value-objects есть), прежде чем детализировать хотя бы одну.
2. **Поля** — затем поля каждой модели.
3. **Методы** — затем операции/методы: ВСЕ взаимодействия моделей (поведение, инварианты, ошибки),
   не только одной модели.
4. **Тесты** — затем содержательные executable-тесты, доказывающие требования против этого кода.

Агент НЕ ведёт одно требование от модели до теста целиком, прежде чем начать следующее — он
завершает КАЖДЫЙ слой по всему домену, прежде чем перейти к следующему слою. `COVERAGE.md`
(§7) отслеживает, на каком слое домен находится сейчас.

## 5. Каталог механических проверок (структурный пол)

Движок проверяет МЕХАНИЧЕСКИ (`internal/invariants/authored_links.go`,
R-authored-spec-links-mechanically-checked), шесть проверок:

1. **`check_implemented_by_symbol_resolvable`** — каждая непустая запись `implemented_by`
   разбирается на `file:symbol`, и символ реально объявлен (функция/метод/тип) в названном файле.
2. **`check_verified_by_test_resolvable`** — каждая непустая запись `verified_by` разбирается на
   `file:test`, имя начинается с `Test`, и это реальная `func TestXxx(t *testing.T)` в названном
   файле.
3. **`check_verified_by_test_has_teeth`** — резолвящийся `verified_by`-тест НЕ пуст и НЕ состоит
   только из `t.Log`/`t.Logf` — анти-вакуумность: настоящая assertion (`t.Error*`/`t.Fatal*`/
   `require.*`/`assert.*`) или ветвление (`if`/`for`/`switch`/`range`).
4. **`check_verified_by_test_no_skip`** — резолвящийся `verified_by`-тест НЕ содержит безусловный
   top-level `t.Skip`/`t.Skipf` (условный skip внутри рантайм-проверки — нормальная Go-идиома, не
   флагуется).
5. **`check_verified_by_no_unrelated_reuse`** — одна и та же запись `verified_by`, формально
   процитированная ≥2 требованиями, не связанными между собой через `Relations`
   (`refines`/`depends_on`/`replaces`), — reuse-детектор: один тест не может честно доказывать
   несвязанные claim'ы.
6. **`check_enforced_requires_enforcer_or_authored_link`** — каждое `SETTLED`+`ENFORCED` требование
   несёт реальный носитель через движковый ИЛИ authored-путь (§3).

Все шесть — NO-OP для требования с пустыми `implemented_by`/`verified_by` (оба поля опциональны).

Резолвер (`internal/gate/spec_resolver.go`, `ResolveSpecSymbol`/`ResolveSpecTest`) выполняет
ЦЕЛЕВОЙ парсинг ОДНОГО названного файла — никогда repo-wide скан, потому что каждая запись несёт
свой собственный путь. Корень резолюции — `gate.SpecRootForGraph(g)`: `g.DomainDir` для обычного
домена, либо корень engine-репозитория (найденный подъёмом до ближайшего `go.mod`) для self-hosting
домена (`g.SelfHosting`).

**Staleness**: изменение authored-модели, осиротившее ссылку (`spec/...:Symbol` больше не
существует), обнаруживается тем же `check_implemented_by_symbol_resolvable`/
`check_verified_by_test_resolvable` при следующем прогоне — ссылка становится ложной в момент,
когда код изменился, независимо от статуса требования.

## 6. Граница честности: структурный пол vs зеркальный аудит

Движок держит СТРУКТУРНЫЙ пол: цитируемый символ/тест реально существует, резолвится по имени в
authored `spec/`-дереве домена, не пуст (has teeth), не содержит безусловный skip, не переиспользован
между несвязанными требованиями, не осиротел после правки кода (R-structural-floor-vs-mirror-audit).

Движок **НЕ МОЖЕТ** механически доказать, что тест *семантически* доказывает требование — это
остаётся **зеркальным аудитом**: человек или LLM читает пару «текст требования ↔ authored код+тест»
БЕЗ домысливания и БЕЗ лексических/семантических эвристик, зашитых в движок. Дисциплина:

- Движок гарантирует «код и тест реальны и связаны».
- Зеркало-аудит сертифицирует «они действительно про ЭТО требование».

Структурные проверки НИКОГДА не выдаются и не полагаются заменой зеркального аудита; семантическое
суждение зеркального аудита НИКОГДА не кодируется как лексическая/ключевая эвристика внутри слоя
инвариантов графа (`internal/invariants`). Соблазн свернуть семантическую сертификацию в движок
«ещё одной check_*-проверкой» — именно тот отказ, против которого стоит это требование.

## 7. Генерируемые проекции (вместо кода)

Каждый документ, описывающий authored `spec/`-дерево домена, — СГЕНЕРИРОВАННАЯ проекция,
собираемая `hotam gen-spec` из графа + резолвера `spec/`, и НИКОГДА не пишется/не правится руками
(R-authored-spec-projections-are-derived):

- **`REQUIREMENTS.md`** — каталог требований (как сейчас). Построена.
- **`MODELS.md`** — обзор authored-моделей: структуры/поля/методы/инварианты, извлечённые из
  `spec/model` через `go/ast` (`internal/generator/models.go`, `BuildModels`). Построена.
- **`TRACEABILITY.md`** — requirement → `implemented_by` (file:symbol) → `verified_by` (file:test)
  (`internal/generator/traceability.go`, `BuildTraceability`). Построена.
- **`COVERAGE.md`** — какие `SETTLED` требования имеют реальные носители (authored
  `implemented_by`+`verified_by`, ре-резолвится тем же резолвером, что и `TRACEABILITY.md`; либо
  движковый `enforced_by`) против честного roadmap-долга против постоянной дисциплины
  (`INHERENTLY_PROSE`), и на каком слое (модели/поля/методы/тесты, §4) домен находится сейчас — слой
  читается тем же `go/ast`-сканом, что и `MODELS.md` (`internal/generator/coverage.go`,
  `BuildCoverage`). Построена (task #239/F4 remediation, 2026-07-17).
- **`UNENFORCED.md`** — burn-down; сейчас покрывает движковый `enforced_by`-долг, authored-эпохи
  burn-down (по `implemented_by`/`verified_by`) — часть того же roadmap-долга, что и `COVERAGE.md`.
- **`SPEC.md`** — нормативный ТЕКСТ спеки: под каждым требованием — claim (из графа) +
  Given/When/Then/Value-нарратив РЕАЛЬНОГО прогона `verified_by`-теста, записанный через рекордер
  `hotamspec` (§8). Единственная проекция, требующая реального исполнения тестов (`hotam gen-spec
  --spec`, opt-in per run); устаревание энфорсится `check_spec_md_current` (§8.3). Не пишется руками.

`spec/` — единственный артефакт в этом пайплайне, авторский руками и НИКОГДА не перезаписываемый
движком; `docs/gen/` — генерируемая сторона, ВСЕГДА безопасно перегенерировать поверх.

## 8. Сценарный слой: `hotamspec` → генерируемый SPEC.md + гейты обязательности (W1–W2)

Расширение контракта под сценарно-генерируемую спеку (дизайн-документ —
`../PLAN-scenario-generated-spec.md`, волны W1–W7). Тест авторской спеки несёт ДВОЙНУЮ службу:
(1) ассертит поведение через вызовы реальных методов (как и в §2–§5), и (2) является ИСТОЧНИКОМ
нормативного текста спеки. Текст — производная проекция, никогда не первоисточник.

### 8.1 Рекордер `hotamspec` (W1.1)

Канонический исходник рекордера живёт в движке (`internal/recorder/canon/hotamspec.go`) и
ВЕНДОРИТСЯ в consumer-домен как однофайловый пакет `<domainDir>/spec/hotamspec/hotamspec.go`
командой `hotam vendor-recorder` — без cross-module `replace` (уважая долг NEW-2-bis, тем же
паттерном, каким вендорится кристалл-маркдаун). Инвариант `check_recorder_current`
sha256-сверяет вендоренную копию с каноном (после strip banner) и флагует дрифт.

API (имена сверены с каноном и реальным пилотом `domains/prat/spec/model/brd_package_test.go`):

```
s := hotamspec.NewScenario(t, "R-brd-integrity-zero-blockers", "BRD sign-off requires zero blockers")
s.Given("a BRD package with one outstanding blocker", "rule_id", "ac-orphan")
s.When("SignOffP_G3 is called")
s.Then("sign-off is rejected with ErrBrdHasBlockers", errors.Is(err, ErrBrdHasBlockers))
s.Eq("blocker_count is", p.BlockerCount(), 1)
s.Value("blocker_count", p.BlockerCount())
```

Выбор между `Then` и `Eq` — по природе факта: `Eq(label, got, want)` — для
фактов-ЗНАЧЕНИЙ («blocker_count is 1»): ассертит равенство через канонический
`renderValue` обеих сторон и записывает `StepThen` с desc = `label + " " +
renderValue(got)` — значение в тексте приходит из ИСПОЛНЕНИЯ, а не из руки
автора (нельзя написать «born 1990» в тексте, пока код ассертит 1987); при
несовпадении зовёт `t.Errorf` (non-fatal, как `Then`), называя label, got и
want. `Then(desc, cond)` — для булевых предикатов («sign-off is rejected»),
где значения в тексте нет.

**Строгость `Eq`.** После разыменования не-nil указателей `got` и `want` должны иметь ОДИН И ТОТ ЖЕ
динамический тип И одинаковый канонический рендер: `Eq("x", 1, 1.0)` падает (int vs float64), два разных
struct-типа с одинаковым рендером — тоже. Удобство для констант: у untyped-константы в рантайме нет
типа, поэтому `want` предопределённого типа по умолчанию (bool/int/int32/float64/string)
приводится к типу `got`, если `got` — базовый вид того же семейства (bool; любой int/uint;
float32/float64; string) и приведение без потерь (round-trip, без overflow/усечения). Так
`Eq("year", BirthYear(1987), 1987)` и `Eq("kind", Kind("a"), "a")` проходят; между семействами
(int ↔ float) приведения нет.

**Дефолтный `When`.** `hotamspec.NewScenario(t, id, title, hotamspec.WithWhen("инициализация"))` —
убирает повтор `s.When(...)` в каждом сценарии файла. Если тест не вызвал `When` до первого
`Then`/`Eq`/`Value`, рекордер вставляет `StepWhen(desc)` ровно перед ним — шаги (и SPEC.md)
идентичны явному `s.When(desc)` в этой точке. Явный `When` всегда приоритетнее (дубля нет); без
опции поведение прежнее.

**Зубы `Then`/`Eq` (детектор).** Вызовы `.Then`/`.Eq` считаются assertion (teeth) ТОЛЬКО если
тест-файл импортирует пакет с путём `hotamspec` или оканчивающимся на `/hotamspec` (любой
алиас, кроме `_`); `.Then(...)` на постороннем типе в файле без рекордера — не зубы. Остальные
правила (`t.Error*`/`Fatal*`/`Fail*`, `require.*`/`assert.*`) без изменений.

Обычный `go test` — ЧИСТЫЕ АССЕРТЫ: `Then` зовёт `t.Errorf` ровно как руко-написанный тест
(удаление всех вызовов `hotamspec.*` кроме `Then` ничего не ломает — рекордер строгий надмножество
над «тест всё ещё ассертит»); артефакт НЕ пишется. Record-режим включается ТОЛЬКО движком —
env `HOTAM_RECORD_DIR` (`hotamspec.RecordDirEnv`), выставляемый во время `hotam gen-spec --spec`;
тогда каждый `Scenario` сериализует записанные шаги в canonical JSON-артефакт
(`{req_id, test, title, steps[{kind, desc, values}], verdict}`). Детерминизм: kv хранятся ordered
slice (не map — map-order hazard закрыт структурно), значения рендерятся канонично (float через
`strconv.FormatFloat` 'g' -1, pointer разыменовывается — никогда адрес, error через `.Error()`),
никакого wall-clock/random в артефакте.

### 8.2 Legacy one-language `SPEC.md` — scenario-generated narrative (W1.3)

This subsection describes the existing one-language `NewScenario` path.
Multilingual atom views and explicit rule/case rendering are specified in §13.
For this legacy path, `hotam gen-spec --spec` executes each `verified_by` test
through `go test` (`gate.RunVerifiedByTestRecording`) and builds the scenario
narrative with `gate.BuildSpec` / `BuildSpecFromRows`. By default, `gen-spec`
does not write this `SPEC.md`; generation is opt-in per run. Under each
requirement the narrative combines the graph claim with the real recorded
Given/When/Then/Value steps.

The `docs/gen/` files are mechanically regenerated, not hand-edited, but their
ordinary normative prose is authored `Claim`/`Why`/`Context` projected from
the graph. Only their structure (tables, IDs, statuses, links and counters) is
generated. Prior measurements (task #402) found authored prose at ~12% of
`docs/gen/*.md` on hotam-dev and ~49% on hotam-spec-self; one authored claim
was ~54% of self-hosted `REQUIREMENTS.md`. In this legacy path, `SPEC.md` is
the projection augmented by machine-recorded scenario steps; it is a derived
view, never the source of truth.

### 8.2.1 Путь requirements-as-code: оцифровка вместо prose

На пути `requirements_authority: code` нет prose/verbatim-первоисточника. Утверждение владельца
оцифровывается целиком: атомы и методы модели (значения, отношения между ними — «по должности X,
по факту Y», «проекты чьи и где», «по Торе, не будучи евреем» — а не пары разрозненных значений)
плюс сценарии, которые порождают текст. Сгенерированное предложение — единственный текст;
оно зеркалит код (`Eq` для значений, `Then` для предикатов, `WithWhen` вместо повторяющегося
`When`) и не добавляет фактов сверх слов владельца. Потерянное или схлопнутое при оцифровке
дописывается кодом, а заменённый/снятый факт уходит в `REJECTED` с `replaces` (граф append-only).
`Signoff.verbatim` на этом пути не используется.

### 8.3 Scenario-proof and freshness gates (W2.1–W2.4) — independent triggers

Four invariants govern the legacy scenario-generated path. The scenario/model
obligations use the domain's explicit `discipline: "full"` trigger;
`check_spec_md_current` instead owns freshness when the legacy `SPEC.md` is
present. No check borrows a different check's trigger.

- **`check_settled_requires_scenario`** (W2.1) — in a `discipline: full`
  domain, each SETTLED requirement without an engine `enforced_by` carrier
  must have `implemented_by`, `verified_by` and at least one scenario-narrated
  test. Without `discipline: full` this obligation is a no-op.
- **`check_scenario_executes_impl`** (W2.2) — coverage-proof: движок гоняет `verified_by`-тест с
  `-coverprofile` и проверяет, что строки `implemented_by`-символа исполнены ИМЕННО этим тестом
  (>0 покрытых statement'ов) — закрывает главный вектор форжинга артефакта тестом, не зовущим
  реальные методы.
- **`check_spec_md_current`** (W2.3) — устаревший SPEC.md (на диске не совпадает с тем, что
  regen-дало бы из текущего графа + сценариев) = violation, по образцу byte-identical гейтов
  `gen-spec`. NO-OP, пока SPEC.md не закоммичен вообще (честное «сценарный слой ещё не подключён»,
  тот же honest-no-op контракт, что у `check_recorder_current`).
- **`check_model_complete`** (W2.4) — послойная полнота как ЗАВЕРШЁННОСТЬ, не временной порядок
  (D5): у привязанной к дисциплине модели поля документированы, методы привязаны, сценарии есть.
  Гейт не видит историю коммитов; механически проверяет конечную полноту.

Граница честности (§6) сохраняется буквально: движок держит структурный пол (реальные вызовы,
coverage-proof, byte-identical SPEC.md), зеркальный аудит сертифицирует семантическую адекватность
— семантическое суждение НИКОГДА не кодируется как лексическая/ключевая эвристика внутри слоя
инвариантов графа (`internal/invariants`). Миграция домена на `discipline: full` — ratchet:
остаточное «inherently prose» видно в `COVERAGE.md`, не прячется тихим исключением.

## 9. Самоисполняемые атомы

Домен включает режим явно: `"self_executing_atoms": true` в `manifest.json`.
Существующий сценарный режим других доменов не меняется; `NewScenario` остаётся
для многошаговых сценариев.

Атом — именованный метод без аргументов. Метод значения содержит один `return`
с одним выражением. Его фраза — первая строка doc-комментария; заголовок метода
вроде `BirthYear —` в эту фразу не включается. Текст требования выводится из
фразы и реально исполненного значения: «Год рождения — 1987.».

Здесь описан legacy одноязычный Fact-режим; языковые блоки и rule/case режим
описаны в §13.

```go
// год рождения
func (h Human) BirthYear() BirthYear { return h.born }

func TestBirthYear(t *testing.T) {
    hotamspec.Fact(t, Init().BirthYear, BirthYear(1987))
}
```

`Fact` исполняет переданный метод один раз, проверяет значение и возвращает
`Evidence`. Замыкания, анонимные и свободные функции не являются субъектами.
В артефакте сохраняются нормализованный `Subject` метода и исполненное значение;
суффикс `-fm`, указатель получателя и параметры generic-типа не меняют идентичность.

Отношение — отдельный `bool`-метод; оно проверяется через
`Holds(t, predicate, evidence...)`. Доказательства — результаты `Fact`, не
повторные исполнения методов. По умолчанию ожидается `true`; отрицательное
отношение проверяется с `hotamspec.Expect(false)` без анонимной обёртки.
В методе отношения разрешены ветвления.

ID по умолчанию — `R-<receiver-kebab>-<method-kebab>`; явное переопределение
в реестре сопоставляется по методу в `implemented_by`. Остальные ссылки
выводятся: `implemented_by` — объединение субъектов, `verified_by` — исполнивший
их тест. `atom_defaults` манифеста задаёт owner, status, why, created_at и
settled_at; owner по умолчанию берётся из director, status — SETTLED.
Реестр хранит только REJECTED и исключения. Переименование с новым выводимым
ID требует явного REJECTED старого требования и `replaces` у нового.
Несовпадающий ручной заголовок не подменяет выведенный текст, а является ошибкой.

Пакеты поддоменов находятся в `spec/model/<sub>/`; обход рекурсивный.
Порядок атомов — авторский нарратив тестов, а не позиция метода в исходниках:
пакеты обходятся как раньше, внутри пакета тест-файлы и `TestXxx` функции —
в порядке исходников, внутри теста — в порядке вызовов `Fact`/`Holds`.
Отношение `Holds` стоит на позиции своего вызова, поэтому `Fact`-доказательства,
вложенные в его аргументы, идут сразу после отношения. Атом,
доказанный в нескольких тестах, берёт самую раннюю позицию; ручной `DeclOrder`
не нужен.
`docs/gen/SPEC.md` — индекс пакетных страниц `docs/gen/spec/<pkg>.md`.
Если inline-требования consumer-кристалла превышают 6000 символов, вместо
пустоты выводятся ссылки на пакеты со счётчиками.

Запись производится пакетным `go test -json`: родной кэш Go переигрывает
канонические артефакты из stdout для неизменённых пакетов, изменённые пакеты
исполняются заново. Собственного дискового кэша вердиктов нет.
Проверки фразы, атомарности и единственного субъекта структурные;
семантическое доказательство остаётся за зеркальным аудитом (§6).

## История: переход от gen-code

Ранняя версия этого контракта (`GEN-CODE-CONTRACT.md`, git-история сохраняет полный текст)
описывала `hotam gen-code` — генератор, порождавший Go-модели/lifecycle-методы/тесты ИЗ графа
домена. Решение resolver (2026-07-16, `PLAN-authored-spec-discipline.md` §1, §12) отменило эту
генерацию:

- **`t.Log("no structural atom")` в 10 из 22 требований prat** — под генерацией это была честная
  пометка «генератор не может выдумать структуру», но всегда-зелёный тест создавал лишь видимость
  исполнимого требования, не покрытие.
- **Лексический матч ≠ смысл** — генератор угадывал структуру по словам claim'а; правильно не
  угадывать, а авторски выражать.
- **`internal/generator/gocode` удалён вместе с командой `gen-code`** — демотирование до
  необязательного bootstrap-прототипа было отклонено resolver'ом (пока `gen-code` жив, существует
  троянский путь доверия: воскрешённый каталог всегда-зелёных `t.Log`-заглушек резолвер принял бы
  за настоящие энфорсеры). `internal/gate`'s `testFuncRoots()` не имеет ветки, доверяющей
  сгенерированному коду.

Машинерия проверки (атом-классификатор, coverage-audit, резолвер) не пропала — она развернулась с
«генерировать» на «верифицировать authored-код» (§5-§6 этого документа).

## 10. Каталог вопросов оператора → проекции

Субстрат (граф + `gen-spec`) отвечает хорошо на одни классы вопросов оператора и структурно не
отвечает на другие. Существующие `docs/gen/*.md` систематически являются **памятью для изменения**
(confront/trace/status: «что нарушено», «почему отклонено раньше», «что сейчас в долге») и
систематически НЕ являются **памятью для понимания** (обзор: «как это устроено целиком»,
стадийность/pipeline) — два разных режима работы с одним и тем же графом, не два случайных примера.

| Класс вопроса | Пример | Отвечающая проекция | Статус |
|---|---|---|---|
| **orient** — «что сейчас важно» | «что делать прямо сейчас, какой топ-приоритет» | Domain Map / manifest (`hotam what-now`, DOMAIN-MAP-блок CLAUDE.md) | построена |
| **overview** — «как это устроено целиком» (стадии/pipeline) | «какие этапы у методологии PRAT», «как данные текут от заявки до релиза» | `PIPELINE.md` — собирается из Process-узлов + EntityType + pipeline-ссылок между сущностями (R-domain-overview-projection) | построена |
| **confront** — «что это нарушает / с чем конфликтует» | «конфликтует ли эта идея с уже принятым» | `REQUIREMENTS.md` (+ `hotam confront`) | построена |
| **trace** — «почему было отклонено раньше» | «мы уже пробовали так делать?» | `HISTORY.md` (anti-relitigation) | построена |
| **status** — «что сейчас в долге» | «что заявлено, но не гарантировано кодом» | `what-now` / `UNENFORCED.md` | построена |
| **authored coverage** — «на каком слое домен, что реально доказано» | «сколько требований имеют настоящий implemented_by+verified_by» | `COVERAGE.md` (§7) — carrier-разбивка + слой; `TRACEABILITY.md`/`MODELS.md` дают детальную навигацию по отдельным требованиям/объектам | построена |

Явное правило: **если субстрат отвечает на класс вопроса ХУЖЕ, чем чтение исходного prose-документа
руками — это обнаружимый дефект покрытия проекций, а не приемлемая деградация.** Появление нового
класса вопросов, на который ни одна существующая проекция не отвечает, — повод завести для него
строку в этой таблице и (если это реальная потребность) SETTLED-требование по образцу
R-domain-overview-projection, а не молча оставить пробел.

## 11. Code-authority: требования как Go-код (`sync-domain`)

§1–§7 описывают домен, чьи требования живут в `graph.json` и попадают туда через mediation loop
(`hotam apply-proposal`/`land` над JSON-профсами). Альтернатива (task #365–#367, RAC2): домен
объявляет `"requirements_authority": "code"` в `manifest.json` и авторитативно держит требования
как Go-литералы в своём `spec/requirements.go` (через зеркальный registry), а граф — их проекция.

Механика (consumer-домен):
1. `hotam vendor-ontology --domain <path>` — вендорит минимальное зеркало `Requirement`+`Registry`
   из `internal/ontology/canon` в `<domain>/spec/hotamontology/`.
2. `hotam scaffold-registrydump --domain <path>` — пишет `<domain>/spec/registrydump/main.go`,
   печатающий `json.Marshal(Requirements.All())`.
3. Автор пишет требования как `[]ontology.Requirement` в `<domain>/spec/requirements.go`.
4. `hotam sync-domain --domain <path>` — читает Go-registry (через `go run ./registrydump`),
   dry-run по умолчанию, `--confirm-hash` для записи, зеркалируя Go→graph (тот же handshake и
   gate-order, что у `sync-self`). `requirements_authority: "code"` затем блокирует
   `apply-proposal`/`land` от ручного авторинга Requirement/Rejection для этого домена.

For this authorized code projection, the engine uses
`loader.LoadGraphForCodeProjection(path)`, gated to self-hosting or
`requirements_authority: "code"`. Ordinary `loader.LoadGraph` remains strict.
It retains strict wire, manifest and core-graph validation, deferring only
stale Requirement-conformance checks. After the source-to-graph projection,
callers must run `loader.ValidateGraph` before `loader.WriteGraph` persists it.

Self-hosting-домен `hotam-spec-self` использует ту же механику через `sync-self` (его Go-registry —
`internal/selfspec/requirements_*.go`, §self-hosting lock). Направление ВСЕГДА Go→graph (код —
первоисточник), никогда graph→Go: это тот же принцип «authored spec, не генератор кода», что и §9.

## 12. Наблюдения, находки и источники

`hotam evidence --domain <path> --json --write` executes checks and saves one
shared `docs/gen/evidence.json` plus `EVIDENCE.md` and `FINDINGS.md` views;
localized bundles use the corresponding language-suffixed views. This reports
observations, not graph mutation or publication of a successful normative
SPEC. On discrepancy, files are still saved before nonzero exit. A passing
test remains visible when a sibling in the package fails. Atom-domain reports
also collect real `spec/model` artifacts before the first successful
`sync-domain`, when the graph may still be empty.

`Fact` records actual/expected. `WithInput(value)` and
`WithContext(ArtifactContext{Implementation, ImplementationVersion,
SpecVersion, Profile, Target, Operation, Producer, Components})` add explicit
input and caller-supplied context; versions and components are never guessed.
`Target`, `Operation` and `Producer` distinguish selection/provenance. The
runtime `Components` list is separately recorded from manifest compositions;
only values actually supplied by the producer are stored, in deterministic
order.

`Observed(value, Observe(name, input, actual, expected)...)` remains the
generic nested-summary API. `Bytes`, `Text`, `Integer`, `Float64Bits`,
`Scalar`, `Diagnostic` and `Object` create exact typed payloads for raw
observations. The method runs once; `Fact` compares the underlying value and
preserves the nested comparisons. A failed nested comparison makes the real
`testing.T` fail even when its summary matches `want`.

В `manifest.json` список `specification_sources` задаёт объекты
`{id, path, version, sha256}`. Относительный path считается от домена,
для self-hosting — от его Go module root. Абсолютный path задаётся явно.
Версия — авторская декларация; SHA-256 проверяется по фактическим байтам.
`Requirement.SourceLinks` содержит `{source_id, anchor}`; anchor — существующий
Markdown heading или диапазон `Lx-Ly`/`Lx`. Проверка существования и хэша
структурная: она не доказывает, что метод семантически покрывает пункт.
Старые свободные `SourceRefs` не превращаются в подтверждённые источники.

Отчёт различает `verified`, `discrepancy`, `unsupported_recommendation`,
`unreachable`, `unverified`. Первые два состояния вычисляются из исполнения.
Авторская `Coverage`-декларация содержит status/rationale/profile; для
unreachable нужны обоснование, профиль и source links, для неподдерживаемой
рекомендации — обоснование и source links. Такая декларация не подавляет
реально наблюдённое расхождение. Неисполненное или недоступное — не verified.

Finding preserves the norm, method, test, case, comparison, profile, target,
producer and component provenance with a content-addressed ID. Its initial
status is unreviewed: the framework does not decide whether the specification,
model or implementation is at fault. `hotam findings list|show|review` stores
separate human review in `docs/reviews/finding-reviews.json`. Review requires
kind (`specification_issue`, `model_issue`, `implementation_issue`,
`needs_review`), status (`open`, `resolved`), rationale and decision-ref. It
does not change observations or verdicts. Locale selection does not affect
finding identity or transfer a review; source/case changes follow the existing
fingerprint rules. This is not a verdict cache.

`confront` обозначает лексические совпадения как `lexical_suspicion`;
общие implementation/source links или Relations усиливают их до
`linked_suspicion`, но не устанавливают противоречие. `formal_conflict`
возникает из явного unresolved Conflict carrier, перечисляющего оба
требования. Только такой carrier блокирует запись без зафиксированного
решения; opposite-marker слова сами по себе не блокируют и не порождают
ложную acknowledgement history.


## 13. Atomic multilingual and conformance specifications

This section extends the plain authored-spec path without replacing it. One
graph, requirement identity, source index, test snapshot, evidence store and
review store serve every language. Language views and multiple cases are not
separate requirement copies. Structural presence, case execution and corpus
pass do not establish semantic equivalence, exhaustive input coverage or
complete conformance.

### 13.1 Language and source-text contract

`manifest.json` may declare `languages`, `default_language` and
`conformance`. Their Go fields are `loader.DomainManifest.Languages`,
`DefaultLanguage` and `Conformance`. The resolved `Graph` carries
`Languages`, `DefaultLanguage`, `RenderLanguage` and `Conformance` as
invocation-local state; `RenderLanguage` is a view on the shared graph, not a
second graph.

- Without `languages`, the existing one-language mode and output names remain
  unchanged. A one-code list is also one-language mode; its default may be
  omitted or equal that sole code. Plain doc comments remain valid.
- A multi-language list must be non-empty, unique, path-safe, supported and
  paired with a `default_language` in the list. The current service catalog
  supports `en`, `ru` and `zh`. Unsupported or invalid codes are errors;
  generators do not choose an OS/terminal/doc-language guess.
- A fixed service-template key missing from a selected locale is a typed
  missing-translation error. There is no English fallback. Catalogs cover
  framework/tool descriptions and fixed rendering templates only; authored
  claims, history, review rationale, quotes, SDK output, code and identifiers
  remain authored or raw.

For multilingual atoms the sole comment syntax is `>>>>> lang=<code>` in the
method's Go doc comment. Each declared language has exactly one non-empty
phrase block; malformed markers, text outside blocks, unknown/duplicate codes,
empty blocks or a missing declared language are errors with file/method/line/
language context. A plain unmarked phrase is not a multilingual fallback. Do
not add alternate tags, IDs,
headings, tables, templates or translation dictionaries to ordinary method
comments.

Bool atom methods (`func (...) T bool` in fact/holds mode) may author a
negation phrase: a doc line after the main phrase starting with `not: ` (in
multilingual mode, its own `not: ...` line inside each language block, paired
with that block's phrase). A `not:` line is never part of the main phrase;
misplaced, empty or duplicate `not:` lines are errors. Claim rendering for
bool steps: value `true` renders the bare phrase (`Фраза.`); value `false`
renders the authored negation phrase (`Not-фраза.`); a `false` verdict
without a `not:` phrase for the language is an error — the wording is never
auto-generated. Non-bool value-facts keep the `Фраза — значение.` form. For
`Holds`, the claim is only the predicate's phrase; evidence methods remain in
the artifact and SPEC observations, never concatenated into the claim.

Rendered values are localized the same way. In a multilingual domain, an
executed value that matches a typed string constant of the subject method's
return type is substituted with that constant's translation: the constant's
doc comment carries the same `>>>>> lang=<code>` blocks, and each projection
uses its own block text. Constant blocks follow the same strictness as
method phrases: unknown, duplicate, empty or missing declared languages are
errors with file/constant/line/language context. A matched string constant
without any language blocks is itself an error ("string constant value
without translation in a multilingual domain") — the English projection never
silently contains Russian values. Values that must not be translated mark
that intent with a single `>>>>> lang=*` block in the constant's doc; it must
be the only block, and the raw value renders verbatim in every language.
Numbers, bools (which keep their own `not:` semantics) and values with no
matching constant are unchanged; in one-language domains nothing of this
applies and constants need no blocks.

The source index retains each language's phrase and source position. One
`Requirement` keeps one ID, method/test links and relation graph. Its typed
`ClaimTexts` (`claim_texts`, `ontology.LocalizedText`) carries the derived
claim for every declared language. A rule claim is its authored doc norm; a
value-fact claim combines the doc phrase with the executed value. `Claim` is
mechanically the `default_language` entry, not a second manual translation or
fallback. In single-language payloads, absent localized fields stay absent. Editing a
non-default translation is preserved in source→graph diffs and confirmation
hashes. Switching only the render locale does not alter identity, verdict,
coverage or finding review; source edits still follow the content-addressed
identity rules.

For multiple languages HotamSpec renders `docs/gen/SPEC.<lang>.md` and
`docs/gen/spec/<lang>/<pkg>.md`. Other generated human-readable views use the
same language suffix; each index links to shards in its own locale. Machine
data (`graph.json`, `evidence.json`, review storage) is shared. The standard
`CLAUDE.md` uses the default language; additional localized boot views are
`CLAUDE.<lang>.md`, without a duplicate default-language copy.

Within a multilingual bundle, other emitted views use the language suffix in
`docs/gen/<NAME>.<lang>.md`. URI/code identifiers and shared machine-data files
are never translated.

The output inventory is shared by rendering, freshness and cleanup. Every
view and shard is rendered before publication; a config/translation/render
error does not publish a partial new bundle. Existing recovery mechanisms
apply, but multi-file publication is not a filesystem-wide atomic snapshot.
On language add/remove, reverse transition or default change, only obsolete
generator-owned files are removed; authored files and durable review notes are
never cleanup targets.
`check_language_bundle_complete` owns source-language block completeness and
`check_language_outputs_current` owns localized-output freshness whenever a
language list is explicitly declared, including one code (where legacy
filenames remain unchanged). It compares generated localized docs and
language-aware boot crystals. It requires the full SPEC index/shard bundle in
multilingual mode; an explicit single locale follows the existing/legacy SPEC
promise. EVIDENCE/FINDINGS views are freshness-checked only when the shared
`docs/gen/evidence.json` or a known evidence view is already materialized; the
check does not make running `hotam evidence --write` a new obligation. The
legacy `check_spec_md_current` and `check_domain_claude_md_current` are no-ops
for explicit languages. This post-processing checker reuses the phase-one
violation/evidence snapshot, not a second source or test execution. Legacy
opt-ins do not activate either language obligation.

### 13.2 Rule atoms, cases and execution

`atom_kind` absent/empty preserves legacy fact mode; only an explicit `"rule"`
selects stable authored norm semantics. Neither a `bool` method nor a large
number of tests changes the kind implicitly. The ordinary `Fact`/`Holds`
contract continues to derive a value fact from phrase plus real value and
rejects conflicting values for one fact ID. In rule mode the authored doc
phrase is the stable norm; per-case values belong to case/evidence records and
never rewrite that norm.

`Requirement`'s optional JSON fields are `claim_texts`, `atom_kind`, `cases`,
`clause_links`, `strength`, `applicability` and `precedence` (all omitted when
empty).

`conformance.rule_cases: true` permits recorder rule/case mode; it is
independent of language configuration and older triggers, and does not enable
a scan of every model method. `self_executing_atoms` remains the explicit atom
discovery trigger. A domain using automatic `sync-domain` rule discovery must
declare both; neither one alone grants the other's behavior. `WithCase` on
`Fact` or `Holds` selects the authored-rule mode, retains the bound method's
requirement ID, and requires a non-empty case ID. Graph-aware processing
rejects the case mode when `rule_cases` is not enabled. Explicit graph
`CaseDefinition` declarations trigger their own conformance audit without
implicitly scanning methods. No new assertion DSL or per-case graph copy is
introduced.

`CaseContext` carries `ID`, `AtomIDs`, `Profile`, `Target`, `Operation`,
`Producer`, `Fixtures`, `Conditions`, `Sides` and `Selection`. It also accepts
optional runtime-only `Expected *TypedValue`, a shared oracle for a case that
compares multiple properties.

The persistent `CaseDefinition` includes typed `Input` and `Expected` as
independent case/oracle values plus selection metadata. Its file-qualified
`Test` link is derived from a real execution and addresses that case only; it
is not a `Requirement.VerifiedBy` alias and does not mark the atom verified.
Explicit graph/report cases may use a `CaseDefinition.Test` reference without
scanning model packages. Normal test-driven discovery projects `WithInput` and
the test's explicit `want`/`Expect` into a new case descriptor; these are
authored test data, never derived from SUT actual. A registry/proposal may
declare a case descriptor directly, but if so recorded declarations must
agree; discovery reports mismatch and does not overwrite explicit values.

The recorder `CaseContext` omits `Test` and `Input`: it obtains the test
identity from the real test/subtest and records `case_input` from the supplied
input. Its runtime `Expected` is omitted from the case object and deep-copied
only to the artifact's root `case_expected`; when absent, that root value comes
from `Fact`'s explicit `want` or `Holds`' `Expect`, never from actual output.
Each observation's `RawExpected` remains its own property-comparison
expectation, not the shared oracle. Runtime `case_input` / `case_expected` are
separate from the authored `CaseDefinition.Input` / `.Expected` metadata.

The pipeline associates the descriptor with the bound method's requirement
identity; optional `AtomIDs` allow one shared case to address additional
atoms. `Operation`, `Target` and `Producer` are explicit selection/provenance,
not inferred from a result. Reusing one case ID with conflicting metadata is
an error; compatible evidence can share one descriptor and one parse/
execution.
Explicitly compatible cases tied to the same file-qualified test justify
shared `verified_by` without inventing a `Relations` edge. Atom execution
checks reuse the invocation snapshot and require a passing recorded method
subject matching each implementation link; legacy scenarios retain their
test-specific coverage proof.


Cases can link several `AtomIDs`, while one atom can have many cases. Keep
independent conditions as separate atoms when each is independently
checkable. Case ID is stable across languages and actual results. Test,
input, expectation, profile, target, producer, fixture and composition changes
remain addressable in diff/fingerprint/evidence; they do not mint language
copies of the normative atom.

### 13.3 Source-clause inventory and corpus

`ConformanceConfig` has independent fields `RuleCases`, `Clauses`, `Profiles`
and `Compositions`. Rule recording is activated by `RuleCases`; clauses,
profiles and compositions bring only the checks for their own declared data.
Enabling `languages` does not make any of those inventories mandatory, and
legacy opt-ins do not activate them.

Each `SourceClause` has stable `ID`, `SourceLinks`, `Sides`, `Strength` and
optional `Applicability`. A `ClauseLink` on a requirement identifies
`ClauseID` and `Side`; the mapping is many-to-many. Source links remain typed
references to manifest-declared `{id, path, version, sha256}` pins and real
anchors. Their byte/hash/version/anchor checks establish source identity and
traceability, not semantic correspondence or a complete list of norms.
Clause IDs are authored stable keys, not heading text, translation, method
name or line count. Explicit sides prevent a passing happy path from standing
for conditions it did not exercise.

`CaseDefinition.Fixtures` carries `FixtureRef{ID, Category, Path, Role,
SHA256, RawBytes}`. All five string fields and `RawBytes` are required;
`SHA256` is 64 hexadecimal characters, and `RawBytes` is an explicit wire
boolean serialized as `false` or `true` (`true` marks raw-byte input). `Role`
is `input`, `expected`, `canonical`, `error` or `extra`.
Relative paths resolve from the domain specification root; an absolute path
must be explicit. The digest is checked against exact file bytes.
`corpus.Read(root, ref)` reads and validates the reference without interpreting
categories, decoding an unknown file format, or running a second test language.
Raw-byte fixtures stay bytes before UTF-8 processing; categories are generic
domain data, not Ktav-specific engine enums. A fixture is provenance/evidence,
never an independent requirement or compliance by itself.

### 13.4 Typed observations and exact comparison

Readable `Input`/`Actual`/`Expected` strings remain projections. Typed
`ontology.ObservedValue` distinguishes `text`, `bytes`, `bool`, `integer`,
`float`, `object`, `null` and `diagnostic`; `Fields` contain typed nested
values. `kind: "bool"` requires the typed JSON `bool` payload (`Bool *bool`):
a non-nil pointer preserves `false`, while an absent payload is not false.
Boolean values are not encoded in `Text` or `ScalarKind`. Bytes carry an
encoding and exact base64 payload; integers are decimal strings; binary64
`FloatBits` are 16 hexadecimal IEEE-754 bits. Diagnostic data keeps `Code`,
optional `Class`/`Reason`/one-based `Line` and optional half-open byte
`Span{Start, End}`.

At the recorder boundary the raw-payload mirror is `hotamspec.TypedValue`,
including `Bool *bool` with the same `bool` JSON field and absent-versus-false
semantics. It is distinct from the existing generic
`hotamspec.ObservedValue[V]` summary wrapper, and `Observed(value,
observations...)` remains unchanged. `Bytes([]byte)`, `Text(string)`,
`Integer(string)`, `Float64Bits(float64)`, `Scalar(kind string, value any)`,
`Diagnostic(DiagnosticValue)` and `Object(map[string]TypedValue)` construct
typed raw values; `Scalar("bool", false)` uses `Bool`, not text. Raw
observations supplement, not replace, readable values.
Missing fields stay absent, distinct from zero, empty or null. In particular,
invalid UTF-8 is never round-tripped through a replacement-character string.

Typed equality is exact structural identity: no implicit tolerance,
normalization or coercion. `Float64Bits` distinguishes signed zero and NaN
payloads; Integer and Text are different types. Error code/class/reason,
line and byte span are distinct fields. Only record positions and properties
the producer actually supplied; do not invent diagnostics or an ordering for
map data that transport did not preserve.

### 13.5 Strength, applicability, profiles and precedence

`Strength` expresses authored `MUST`, `SHOULD` or `MAY`, not an execution
verdict. `Applicability{Operations, Profiles, Features}` is evaluated from
the declared selected operation/case/profile context: non-empty classes are
ANDed, values within Operations or Profiles are ORed, and every declared
Feature is required. Empty applicability is universal. Do not derive it from
a failed output. A non-applicable selection is reported as such, not as
verified or unreachable.

`Profile` has stable `ID`, `Operations`, `Features`, `IntegerMin`,
`IntegerMax`, `FloatDomain`, `Rounding`, optional `PreservesOrder` and
string-valued `Capabilities`. Profile/capability declarations describe
selection context, not an observed implementation result. Authored
`unsupported_recommendation`, `unreachable` and `unverified` qualifications
require rationale; unsupported/unreachable also require source links, and
unreachable requires an explicit profile. They do not erase observed
discrepancy.

`PrecedenceLink{Target, Scope, Applicability}` is a distinct typed ordering
relation. It is not method/source file order and is not `depends_on`. A
strict cycle within a declared scope is structurally invalid. The priority
norm itself still needs an atom, and a behavioral priority finding requires
an actually observed competition witness: matched conditions and selected
branch are preserved only when the producer recorded them. A happy path for
each branch separately does not prove which rule wins when several match.
No backend trace is invented.

### 13.6 Composition, audit and reports

`Composition{ID, Components}` describes a selected implementation
composition. Each `Component` carries `ID`, free declared non-empty `Role`,
`Version`, `SHA256` and `Measured`. Runtime evidence separately records
caller-supplied `ArtifactContext.Target`, `.Operation`, `.Producer` and
`.Components []Component`; the recorder stores only the component metadata
actually passed by the test and orders that list deterministically. Manifest
declarations do not fabricate runtime measurements. A direct SDK target and
an adapter-adjusted/composed target remain distinct observations. A mismatch
or missing measurement is reported; no component is automatically blamed.

`ontology.ValidateConformance(g)` validates declaration structure.
`conformance.Audit(g, executions)` reports structural assessments relative to
the explicit clause/profile/case/composition inventory. The report distinguishes
clause decomposition/source/sides, atoms/methods, cases/execution/verdict,
profile qualifications, precedence and component provenance. Its issue IDs
include `clause_missing_decomposition`, `clause_missing_side`,
`clause_missing_source`, `atom_missing_method`, `atom_missing_cases`,
`case_missing_execution`, `case_unverified`, `case_discrepancy`,
`case_target_mismatch`, `case_profile_mismatch`, `case_metadata_conflict`,
`source_unverified`, `profile_not_applicable`, `profile_unsupported`,
`profile_unreachable`, `profile_unverified`, `precedence_cycle`,
`precedence_unwitnessed`, `precedence_discrepancy`, `composition_unmeasured`
and `composition_fingerprint_changed`. A real failure/discrepancy takes
precedence over applicability and authored qualifications.

The named self-hosted structural carriers are `check_language_bundle_complete`
(source-language block completeness, only when more than one language is
declared), `check_language_outputs_current` (the complete expected localized
output bundle whenever a language list is explicitly declared) and
`check_conformance_audit` (declared rule/case/conformance data through
structural validation and the audit API). That audit is triggered only by
`conformance.rule_cases`, declared conformance clauses/profiles/compositions,
or requirement `atom_kind`, `cases`, `clause_links`, `strength`,
`applicability` or `precedence` fields. `check_language_outputs_current` is the
stable postprocess registry hook in `internal/invariants/language_freshness.go`;
the actual document comparison is wired by
`cmd/hotam/language_freshness_wiring.go:checkLanguageOutputsCurrentReal`.
The postprocessor shares phase-one violation/evidence state, and localized
checks own their outputs rather than inheriting duties from old opt-ins.
`hotam all-violations` reports invariant violations and
`hotam evidence --json --write` emits observations/audit data without making a
failed execution green. `RequirementResult` and findings retain
`ClaimTexts`/`AtomKind` and case/profile/target/producer/typed values in one
shared evidence model. Locale selection does not change raw comparisons or
finding identity; changed source or case data continues to follow normal
fingerprint/review rules.

No check in this contract proves that a translation has the same meaning as
another, that an inventory contains every natural-language norm, that a
corpus is exhaustive, that passing observed cases prove universal behavior,
or that a particular implementation layer is to blame. Those are human
semantic review decisions, not claims of structural conformance.

### 13.7 Implemented API and command boundaries

- `gate.NewAtomSourceIndex(specRoot)` preserves the legacy/plain one-language
  meaning. `gate.NewAtomSourceIndexForGraph(g)` returns a graph-configured
  index using invocation-local languages, source root and `Conformance.RuleCases`.
  `(*gate.AtomSourceIndex).Resolve(subject)` returns an `AtomSource` with
  legacy `Phrase`, localized `Phrases`, per-language `PhrasePositions`, AST
  method and source positions. `(*gate.AtomSourceIndex).DeriveClaims(artifact)`
  returns `(ontology.LocalizedText, error)`; `DeriveClaim` returns the selected
  primary/default phrase. `selfspec.DiscoverAtoms` scans with
  `self_executing_atoms`; rule artifacts additionally require
  `conformance.rule_cases`. Each source package is recorded once regardless
  of locale count.
- Exported `gate.AtomArtifact` decodes recorder `req_id`, `test`, `mode`,
  `title`, `verdict` and `steps`, plus `Case *AtomCaseContext` and typed
  `CaseInput` / `CaseExpected`. `AtomArtifact.CaseDefinition(fileQualifiedTest)`
  projects declared recorder metadata, input and independent expected value;
  it never uses the SUT actual as an oracle.
- `gate.CollectAtomExecutionSnapshot(g)` collects package runs, source data and
  case tests once. Discovery, SPEC rows, evidence and atom checks consume that
  snapshot; adding a locale does not execute the package again.
  `gate.CollectSpecRowsFromSnapshot(g, snapshot)` derives shared rows;
  `gate.CollectSpecRows(g)` remains the standalone convenience path.
  `gate.BuildSpecFromRowsForLanguage(g, rows, language)` strictly renders one
  view without execution. `gate.BuildSpecDocumentsFromRows(g, rows)` renders
  all locale views from those rows and returns a path/content map or error.
  The legacy `gate.BuildSpecFromRows(g, rows) string` retains its signature
  and panics with the original render error rather than returning fallback or
  empty content. `generator.BuildLocalizedDocuments(g, domainName, repoRoot,
  today)` returns the other localized document paths/content. `docbundle.NewLayout`
  centralizes safe names for documents, SPEC indexes/shards, crystals and
  owned-output cleanup.
  Localized renderer keys under `docs/gen/` are domain-relative; keys under
  `framework/` are project-relative. `docbundle.ResolveOutputPath` confines both
  inventories and rejects separately published paths; publication refuses
  authored destination content before writing the bundle.
- The localization leaf exposes `localization.Supported(language)`,
  `localization.Text(language, sourceTemplate, args...)` and
  `localization.Lookup(language, sourceTemplate)`. `Lookup` returns a typed
  `*localization.MissingTranslation`; strict bundle builders convert missing
  fixed keys into errors. These APIs translate fixed service/catalog text
  only, never completed documents or authored data.
- `ontology.ValidateConformance(g)` validates declaration structure;
  `conformance.Audit(g, executions)` assesses recorded cases relative to the
  explicit inventory; `corpus.Read(root, ref)` validates and reads exact
  fixture bytes. `hotam evidence --json --write` exposes observations/audit
  reports; `hotam all-violations` exposes structural invariant failures.
- Self-hosted carriers live in the hand-maintained `internal/selfspec`
  registry. Project them with `hotam sync-self` in dry-run mode, inspect the
  proposed diff/hash, then use the printed hash with `--confirm-hash` to
  synchronize `graph.json`. Do not hand-edit that graph.

