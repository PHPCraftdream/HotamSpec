# Линза 1 (повторный проход) — Избыточность и дублирование — 2026-07-10

Контекст: аудит ПОСЛЕ волны упрощения #82–#98. Прошлый отчёт: `lens-1-redundancy.md` (2026-07-05).

## Что из прошлой волны подтверждённо закрыто (перепроверено, не по отчётам)

- record_delegation.py + delegations.jsonl — удалены; осталась одна файловая система делегаций (delegate.py + _delegation_store.py).
- tick.py — удалён; what_now.py поглотил tick/render_tick.
- Три baseline слиты в `spec/tests/protected_baselines.json` + один `update_baseline.py` с общими `_update_hash_section`.
- cli/ — 30 обёрток сведены к однострочным вызовам фабрики `cli/_dispatch.py:make_main`; ticket/delegation/land — сабкомандные диспетчеры.
- sys.path-бойлерплейт в тестах: 92 файла → 5 (остатки — см. находку 6).
- emit_cipher больше НЕ парсит markdown регулярками — берёт цифры из `gen_spec.compute_cipher_lines(g)` напрямую.
- apply_proposal: пять `_find_*_tuple_end` — теперь однострочные делегаты общего `_find_module_tuple_end` (#92 выполнен).
- docs/demo — регенерирован (3a3903f), не мёртв. README / QUICKSTART-CONSUMER — чисто разведены по режимам (self-hosting vs pip-consumer), взаимные ссылки, дублирование минимально.

---

## Находка 1 — emit_cipher.py обходит единый резолвер активного домена + возит мёртвый компат-код

**Файл:** `spec/tools/emit_cipher.py:25-35` (свой `_PIN_FILE` + `_pinned_domain()`), `:37` , `:82-86`, `:13-18`.

R-active-domain-pin-not-alphabetical требует, чтобы РОВНО ОДНА функция — `hotam_spec.domain_resolution.resolve_active_domain()` — реализовывала порядок резолюции (env `HOTAM_SPEC_ACTIVE_DOMAIN` → pin-файл → алфавит). gen_spec.py:121 и apply_proposal.py:218 её используют. emit_cipher же читает `domains/.active-domain` напрямую и НЕ видит env-override: при выставленном `HOTAM_SPEC_ACTIVE_DOMAIN=X` `load_content_graph()` загрузит X, а `_other_domains_open()` исключит из счёта «других доменов» pin-файловый домен, а не X — счётчик расходится с цифрами пульса. Это не только дубль, но и латентный баг второго глаза сенсориума.

Там же три хвоста полу-мёртвого кода: (а) неиспользуемый параметр `text` в `_other_domains_open(text="")` «для обратной совместимости»; (б) `context = ""` — «трёх-цифирный» пульс навсегда эмитит две цифры, пустышка сохранена «for bit-identical output»; (в) инлайновый sys.path-пролог вместо `import _bootstrap` (единственный тул в tools/, не перешедший на _bootstrap вместе со всеми).

**Усилие:** S. **Приоритет:** ВЫСОКИЙ (расхождение поведения, не косметика).

## Находка 2 — `_load_graph(*, demo)` скопирован в 5 тулов

**Файлы:** `spec/tools/attention.py:44`, `spec/tools/audit_atomicity.py:58`, `spec/tools/confront.py:209`, `spec/tools/what_now.py:703` (вариант, возвращающий tuple с меткой), `spec/tools/audit_tensions.py:405-409` (инлайн в main).

Одинаковое тело: `if demo: sys.path.insert(tests); from fixtures.seed import seed_graph; return seed_graph(); return load_content_graph()`. Пять копий одного контракта «--demo → фикстура, иначе контент». Новый (или переживший волну) кандидат в `_bootstrap.py`-соседа или в `hotam_spec.graph.load_graph(demo=...)`.

**Усилие:** S (одна функция + 5 замен + ни одного изменения поведения). **Приоритет:** СРЕДНИЙ.

## Находка 3 — apply_proposal: тройка клонов `_find_*_call` пережила дедупликацию #92

**Файл:** `spec/tools/apply_proposal.py:803` (`_find_requirement_call`), `:975` (`_find_operator_call`), `:2444` (`_find_assumption_call`).

Побайтово одинаковая логика «walk AST → Call с именем конструктора → сверить kwarg id=» — отличается только строкой `"Requirement"/"Operator"/"Assumption"`. Докстринг `_find_assumption_call` сам признаётся: «Mirrors _find_requirement_call». #92 унифицировал tuple-end-финдеры, но эту тройку не тронул. (`_find_conflict_call:936` — легитимно другой: вычисляет identity из axis+context.) Плюс мёртвый параметр `source_lines` в `_find_requirements_tuple_end:868` — держится «for backward-compat call-site symmetry», т.е. ради симметрии с уже несуществующими сиблингами.

**Усилие:** S (`_find_ctor_call(tree, ctor_name, node_id)` + 3 делегата; убрать мёртвый параметр). **Приоритет:** СРЕДНИЙ-НИЗКИЙ.

## Находка 4 — PROPOSAL-REFERENCE.md: рукописный дубль схем proposal.py в репозитории, где закон — генерённые доки

**Файл:** `docs/PROPOSAL-REFERENCE.md:1-245`.

245 строк ручного поля-за-полем пересказа dataclass-ов `spec/src/hotam_spec/proposal.py` (497 строк) и `_validate_*` из apply_proposal.py — с явным дисклеймером «if this document and the code disagree, the code wins — please file an issue». Весь остальной репозиторий живёт по анти-дрифт закону (docs/gen byte-stable, meta-test regen==committed), а единственный consumer-facing справочник полей — единственный документ, дрейф которого НЕ ловится ничем, кроме глаз читателя. При этом машинное описание схем уже есть: `spec/src/hotam_spec/node_schemas.py` (312 строк). Три представления одних схем: dataclass-ы, node_schemas, рукописный markdown.

**Усилие:** M (рендерить required/optional-таблицы из node_schemas/proposal-датаклассов, прозу оставить рукописной). **Приоритет:** СРЕДНИЙ (дрейф здесь — вопрос времени, и бьёт именно по внешнему потребителю).

## Находка 5 — слияние в hotam-land вышло аддитивным: 3 CLI-команды на один land-log остались

**Файлы:** `spec/pyproject.toml` `[project.scripts]` (hotam-gate, hotam-gate-status, hotam-land), `spec/src/hotam_spec/cli/gate.py`, `cli/gate_status.py`, `cli/land.py`.

land.py осознанно оставил gate.py/closure.py как модули (import-identity + hash-pin — рационально, задокументировано в land.py:1-35). Но КОМАНДНАЯ поверхность не сократилась: потребитель по-прежнему видит и `hotam-gate`, и `hotam-gate-status`, и `hotam-land select|status|verify-closure` — три двери в один жизненный цикл, ровно то, что #87 хотел свести к одной. Ни deprecation-пометки в pyproject, ни в --help.

**Усилие:** S (удалить/задепрекейтить 2 entry-point-а — решение стьюарда, не механика). **Приоритет:** НИЗКИЙ-СРЕДНИЙ.

## Находка 6 — стреглеры sys.path в 5 тест-файлах

**Файлы:** `spec/tests/test_agent_map.py:31,196`, `test_attention_core.py:25-46` (три вставки в одном файле), `test_docs_gen.py:32,235`, `test_no_stale_m_table.py:31`, `test_root_crystal_follows_pin.py:146`.

После #84 conftest.py делает вставку src+tools централизованно, но пять файлов продолжают вставлять пути руками (частью внутри тестовых функций). Работает, но это ровно тот бойлерплейт, который волна объявила устранённым «в ноль».

**Усилие:** S. **Приоритет:** НИЗКИЙ (гигиена, дёшево).

## Находка 7 — два инсталлятора хуков с параллельной обвязкой settings-JSON

**Файлы:** `spec/tools/setup_hooks.py` (249 строк, пишет коммитимый `.claude/settings.json`) и `spec/tools/setup_context_hook.py` (194 строки, пишет личный `settings.local.json`).

Разделение по назначению обосновано (R-sensorium-committed vs личный контекст-мост), но оба независимо реализуют load/parse/backup/write/маркер-детекцию для settings-файлов Claude. Общий `_claude_settings.py`-хелпер убрал бы ~60-80 строк и один источник расхождений формата.

**Усилие:** S-M. **Приоритет:** НИЗКИЙ.

## Находка 8 (остаточная, известная) — монолиты не только не распилены, но подросли

**Файлы:** `spec/tools/gen_spec.py` — 5003 строки (было 4786 на прошлом ревью), `spec/tools/apply_proposal.py` — 3565.

Вдвоём — 55% tools/. #91 дал контракт тестов на промежуточную модель, #92 — точечную дедупликацию, но физический распил не состоялся, и gen_spec продолжает расти (+217 строк за волну упрощения). Фиксирую как остаточный долг с трендом вверх, не как новую находку.

**Усилие:** L. **Приоритет:** НИЗКИЙ сейчас (сознательно отложено), но тренд стоит мониторить числом.

---

## Резюме

| # | Находка | Усилие | Приоритет |
|---|---------|--------|-----------|
| 1 | emit_cipher: обход resolve_active_domain (env-pin игнорируется) + 3 хвоста мёртвого компата | S | высокий |
| 2 | `_load_graph(demo)` ×5 копий в тулах | S | средний |
| 3 | apply_proposal: тройка `_find_*_call` + мёртвый параметр | S | средний-низкий |
| 4 | PROPOSAL-REFERENCE.md — рукописный дубль схем без анти-дрифт защиты | M | средний |
| 5 | hotam-gate/gate-status/land — аддитивное «слияние» CLI-поверхности | S | низкий-средний |
| 6 | sys.path-стреглеры в 5 тестах | S | низкий |
| 7 | Два инсталлятора хуков без общего settings-хелпера | S-M | низкий |
| 8 | gen_spec 5003 (растёт) / apply_proposal 3565 — остаточный долг | L | низкий |

Волна #82–#98 реально сработала: все девять пунктов отчёта 2026-07-05 либо закрыты, либо осознанно отложены. Новая избыточность — мелкозернистая: копии приватных хелперов внутри tools/ (находки 1-3) — ровно тот слой, который не покрыт ни ratchet-ом атомарности (меряет check_*, не тулы), ни каким-либо структурным чеком. Единственная находка с поведенческой ценой — №1.
