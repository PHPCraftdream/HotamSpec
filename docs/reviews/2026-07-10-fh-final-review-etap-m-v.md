<!-- LEGACY (Python-era) audit/review — describes a point-in-time snapshot of the Python prototype (pytest/spec/tools references below are historical, not current instructions); see README.md for the current Go CLI. -->

# Финальное ревью волны Этапов M-V — 2026-07-10/11 (агент fh, задача #125)

Независимое сквозное ревью десяти коммитов волны:
`2d8476f` (M) → `9e27b63` (N) → `63c0396` (O) → `09a2646` (P) → `d9f8a6b` (Q) →
`efa8511` (R) → `ebfb394` (S) → `ea0c294` (T) → `f8e465b` (U) → `880fce8` (V).

## Мета-верификация (всё прогнано лично)

- **T2 на HEAD (`880fce8`)**: 1332 passed, 4 skipped, 49.05s — GREEN. (U заявлял 1330/4; +2 теста добавил Этап V — сходится.)
- **`git status --short` после T2**: чисто (только заранее существовавший untracked `docs/checkpoints/2026-07-10-1632.md`, к волне не относится). Утечки активного домена в генераторах в этот раз НЕ было.
- **`gen_spec.py` поверх HEAD**: нулевой drift — закоммиченные генераты байт-в-байт равны свежей регенерации.
- **Wheel e2e лично**: `HOTAM_SPEC_RUN_E2E_SUBPROCESS=1 pytest tests/test_e2e_wheel_subprocess.py` — 1 passed, 92.5s.
- **Live break-тесты enforcer'ов (S, V, Q)**: подложил файл-нарушитель → оба скана S/V упали RED с именем файла-нарушителя в сообщении; probe-модуль `import hotam_spec.attention` в core → Q-ратчет RED; после удаления проб — всё GREEN. Ни один из трёх новых enforcer'ов не vacuous.

## Вердикт по коммитам

### Этап M (`2d8476f`) — GREEN
OPEN→SETTLED для `R-decided-by-verifiable-signature` c `enforceability=INHERENTLY_PROSE`. Весь контекст исходного OPEN (DEL-1, six-lens Enf#2/Vision#1, A-single-human-wears-all-hats) сохранён внутри нового `why` вместе с явным revisit-триггером (второй живой resolver). Дифф совпадает с сообщением; docs/gen (OPEN.md, UNENFORCED.md, REQUIREMENTS.md, check.md) согласованы.

### Этап N (`9e27b63`) — GREEN
`R-per-node-json-store` REJECTED с честно отличённым от R-rdf-store обоснованием и двумя явными revisit-триггерами (второй легитимный писатель графа; реальный не-Python потребитель). `build_graph_json()` детерминирован (sort_keys, LF, без таймстампов), подключён и в `main()`, и в `_process_domains()`; тесты пинят валидность, покрытие node_ids, детерминизм и анти-drift (committed == regenerated). Nits, не блокеры:
- `_TOOLS` в `spec/tests/test_graph_json_export.py:16` вычисляется и не используется (мёртвая строка).
- N-1: у REJECTED-с-рождения узла проставлен `settled_at="2026-07-10"` — по документированной семантике поля («дата ПОСЛЕДНЕГО перехода в SETTLED») узел, никогда не бывший SETTLED, носить его не должен. То же в Этапе U у `R-binary-enforcement-gradient`. Семантический шум, машинно ни на что не влияет.

### Этап O (`63c0396`) — GREEN, с одной реальной некритичной находкой (O-1)
Онтология (4 поля + `HistoryEntry`), писатель (diff→append) и синтаксический enforcer посажены согласованно; `history` корректно отвергается как proposal-ключ; `check_requirement_history_wellformed` — структура-только, с явной анти-театральной аргументацией (R-boot-cite-measured); тесты freshness покрывают и «молчит на валидном», и «стреляет на сломанном» (non-vacuous). Baseline invariants.py обновлён санкционированным путём (protected_baselines.json в том же коммите).

**O-1 (подтверждена лично в диффе): асимметрия diff-tracked полей.** `_history_tracked` в `spec/tools/apply_proposal.py` включает `settled_at`, но НЕ `created_at`; более того, UPDATE-цикл полей вообще не содержит `created_at` — писатель не способен изменить его при UPDATE. Оценка: асимметрия реальна, но её вред мал — `created_at` есть факт рождения узла и после backfill меняться не должен; читатель `history` увидит записи про settled_at и никогда про created_at, что слегка обманчиво, но не искажает данных. Решение исключить created_at из tracking (чтобы Этап P не заспамил history 269 записями) было бы уместно, если бы P шёл через писателя — а он через писателя не шёл (см. P), так что мотивировка задним числом неточна. Рекомендация: не чинить изолированно, а покрыть одним тикетом вместе с P-1 ниже.

O-2 (наблюдение, не завожу отдельно): `_abbrev(limit=40)` делает summary для длинных полей почти неинформативным — в живых узлах S/V записи вида `why: Resolver verdict 2026-07-05, verbatim: '…→Resolver verdict…'` фиксируют ЧТО поле менялось, но не что именно. Приемлемо (детали несёт git), но читателю history стоит об этом знать.

### Этап P (`09a2646`) — ОСОБЫЙ ВЕРДИКТ (см. три строки ниже) — содержательно GREEN
Проверено лично, построчно, по всему диффу (482 строки):

- **(a) Чистота splice — ПОДТВЕРЖДЕНА.** 0 удалённых строк; 482 добавленных; каждая добавленная строка машинно сматчена на форму `created_at="YYYY-MM-DD",` либо `settled_at="YYYY-MM-DD",` — исключений ноль. 269 `created_at` (совпадает с заявленными «269 nodes») + 213 `settled_at`. Никаких побочных изменений. Выборочная сверка дат с git-историей (4 случайных id): `R-reflection-predicates-first-class` → 3ea9bdc 2026-07-02 ✓; `R-enforceability-kind-declared` → 435bf18 2026-07-01 ✓; `R-glossary-sync-test` (REJECTED, без settled_at — корректно) → 2026-06-30 ✓; `R-agent-is-recursive-director` → 2026-06-30 ✓. Единственная выглядевшая аномалией пара (created 07-02, settled 07-10 у узла, SETTLED с рождения) при трассировке оказалась ДО-P артефактом: `settled_at="2026-07-10"` уже стоял в родительском коммите — это документированный re-stamp писателя из прошлой волны (зафиксирован ещё в ревью A-F), P его не трогал. Даты не выдуманы.
- **(b) Прецедент: приемлемое разовое исключение, НО тикет обязателен.** Backfill дат — разовая миграция данных, не граф-мутация по смыслу; splice детерминирован и верифицирован диффом; буква R-no-hand-edit-graph («правки через apply_proposal.py») нарушена мягко — собственный one-shot writer переиспользовал `_find_requirement_call` из самого apply_proposal.py, т.е. это mechanical writer, а не свободное редактирование. Однако: (1) корневая причина — реальный дефект официального писателя, а не только «ограничение»: UPDATE-путь `_apply_requirement_to_source` не умеет писать `created_at` вовсе И трактует proposal как полное новое состояние, перезатирая неуказанные why/assumptions/enforcement/enforced_by дефолтами — это делает любой «минимальный patch»-proposal деструктивным; (2) горькая ирония: за один коммит до P граф зафиксировал в `R-per-node-json-store` revisit-триггер «появится ВТОРОЙ легитимный писатель графа» — и второй писатель фактически появился в следующем же коммите, пусть и одноразовый; (3) скрипт удалён из gitignored `.runtime/` — воспроизводимость держится только на верифицируемости самого диффа (здесь достаточной, т.к. дифф чисто механический), но миграционные скрипты правильнее сохранять (например, под `spec/scripts/`).
- **(c) ВЕРДИКТ ПО ЭТАПУ P ОТДЕЛЬНОЙ СТРОКОЙ: GREEN как разовая миграция — данные верны, splice чист; прецедент НЕ считать нормой; завести тикет P-1: «apply_proposal.py UPDATE-путь: (i) не поддерживает created_at, (ii) перезатирает неуказанные поля дефолтами (нет patch-семантики), (iii) created_at исключён из history-tracking (O-1)» — это баг-класс писателя, а не только неудобство backfill'а.**

### Этап Q (`d9f8a6b`) — GREEN
- Классификация `scope_projection` как core проверена лично: `spec/src/hotam_spec/invariants.py:2215` — function-local `from hotam_spec.scope_projection import ...` внутри core-проверки. Классификация корректна; сам факт, что скан ПОПРАВИЛ первоначальную классификацию исполнителя, — признак того, что скан работает.
- Не-vacuous подтверждено дважды: встроенный negative control (`test_ratchet_catches_a_reversed_import`, включая relative-import форму) + мой live break-тест (probe-модуль в core с `import hotam_spec.attention` → RED с именем модуля, после удаления → GREEN).
- D1 третье откладывание честно оформлено: узла D1 нет — триггеры revisit записаны в `why` нового `R-core-periphery-import-ratchet`, ратчет закрывает именно риск отложенного решения (тихое обращение стрелки зависимостей).
- Nit Q-1: докстринг `test_core_modules_do_not_import_periphery` (строки 106-107) до сих пор перечисляет `scope_projection` среди periphery — остаток ДО-коррекционной классификации, противоречит `_PERIPHERY_MODULES` и модульному докстрингу. Одно-строчная чистка.

### Этап R (`efa8511`) — GREEN
`build_wheel.py`: populate→build→verify в одном процессе; `finally: populate_tools.clean()` гарантирует чистое дерево; на mismatch артефакт удаляется и exit 1; проверка «ровно один новый wheel» тоже с удалением. E2e-тест перепроведён на атомарный билдер и прогнан мной лично с env-флагом — GREEN за 92.5s. RELEASE NOTE в populate_tools.py направляет будущего релизёра на единственный путь. Наблюдение R-1 (не завожу): self-check сравнивает только КОЛИЧЕСТВО членов `_tools/*.py`, не множество имён — количественно совпавший, но состоящий из не тех файлов wheel прошёл бы; исторический сбой был именно «пустой _tools», так что проверка честна против реального вектора, но сравнение множеств имён стоило бы столько же строк.

### Этап S (`ebfb394`) — GREEN
`R-work-within-launch-dir` PROSE→ENFORCED через честно-частичный AST-скан (committed-code write vector — форма реального `--patch-global` инцидента; runtime-bash остаток явно оставлен prose-дисциплиной со ссылкой на R-agent-conduct-is-rules-not-tests). Границы эвристики задокументированы (no data-flow, `~/`-префикс vs прозаическое упоминание, пустой allowlist с обоснованием). Live break-тест лично: RED/GREEN. Приятно: этот UPDATE прошёл через писателя и оставил первый живой `HistoryEntry` в графе — механизм Этапа O работает не только в тестах.

### Этап T (`ea0c294`) — GREEN
`R-conflict-resolved-in-members-or-mediator` → `enforceability=INHERENTLY_PROSE`, closeable debt 3→2. Аргумент честен: суть правила («резолюция произошла в графе, а не в мире») требует сравнения графа с миром — не проверяемо кодом; проверяемый остаток уже покрыт `check_decided_has_rationale_or_derived` + signoff-проверками. Замечание T-1 (следствие O-2): при переписывании `why` дословная resolver-цитата 2026-07-05 сокращена, а `_abbrev`-обрезанный HistoryEntry старый текст фактически не сохраняет — полный прежний why доступен только через git. Ключевая цитата частично оставлена в новом why, так что информация не потеряна, но это иллюстрация предела 40-символьного summary.

### Этап U (`f8e465b`) — GREEN
D3: `R-binary-enforcement-gradient` REJECTED с сильным техническим обоснованием (ratchet-давление уже бинарно на оси enforceability; `is_closeable_debt()` игнорирует PROSE↔STRUCTURAL; ~50-точечный рефактор при нулевом сдвиге метрики) и revisit-триггером (второй нормативный потребитель STRUCTURAL-уровня). Анти-релитигационное ребро посажено СТРУКТУРНО: `R-enforcement-first-class` получил `Relation("replaces", "R-binary-enforcement-gradient")` — machine-traversable, как требует R-rejected-preserved-not-deleted. E1 declined без узла — корректно (никогда не был Requirement), решение зафиксировано в synthesis-plan с цифрами (~2900 символов из 150000 при headroom 122k). Nit: тот же `settled_at` у REJECTED-узла, что и N-1.

### Этап V (`880fce8`) — GREEN
`R-project-root-not-hardcoded` STRUCTURAL→ENFORCED. Эвристика узкая и честная: наказуема именно СВЯЗКА `__file__`-climb (depth>=2) + consumer-сегмент в одном выражении; shallow-climb для sys.path, deep-climb к `_tools`, и `project_root_or_raise()`-пути корректно в границах; allowlist из одного санкционированного резолвера с rationale. Negative control покрывает 4 плохих + 4 хороших формы, включая лево-ассоциативную цепочку `/ 'tickets' / 'backlog'`. Live break-тест лично: RED с именем файла, после удаления GREEN. Re-stamp `settled_at` 2026-07-09→2026-07-10 зафиксирован в HistoryEntry (документированное поведение писателя).

## Сводка находок (в бэклог, не блокеры)

1. **P-1 (главная; тикет обязателен)** — дефект UPDATE-пути `apply_proposal.py`: нет поддержки `created_at`, нет patch-семантики (неуказанные поля перезатираются дефолтами), `created_at` вне history-tracking (O-1). Именно этот дефект вынудил P идти в обход официального писателя.
2. **N-1/U** — `settled_at` на REJECTED-с-рождения узлах противоречит документированной семантике поля (2 узла).
3. **Q-1** — докстринг положительного теста ратчета всё ещё называет `scope_projection` periphery (1 строка).
4. **R-1** — member-count self-check wheel'а сравнивает количество, не множество имён.
5. **O-2/T-1** — 40-символьный `_abbrev` делает history-summary длинных полей малоинформативным; фиксирует факт, не содержание.
6. Мелочь: мёртвая переменная `_TOOLS` в `test_graph_json_export.py:16`.

## Итог

**GREEN по всей волне M-V.** Все десять коммит-сообщений прослежены до реальных диффов без расхождений; T2 на HEAD зелёный, дерево чисто, regen детерминирован, wheel e2e зелёный, все три новых enforcer'а (Q/S/V) доказанно не-vacuous живыми break-тестами. Особый пункт волны — Этап P — принят как чистая разовая миграция (splice верифицирован построчно, даты сверены с git-историей), с обязательным follow-up тикетом P-1 по писателю: прецедент «обходного механического writer'а» не должен повториться без починки UPDATE-пути.
