<!-- LEGACY (Python-era) audit/review — describes a point-in-time snapshot of the Python prototype (pytest/spec/tools references below are historical, not current instructions); see README.md for the current Go CLI. -->

# Perf-инвестигация: gen_spec / pytest / LAND-циклы — 2026-07-10

Исследование производительности (read-only, без правок кода). Машина: Windows 10, Python 3.12.3,
venv `spec/.venv`. Замеры делались при частично активной параллельной агентской нагрузке (#108
редактировал `tests/test_apply_proposal_batch_stress.py` во время замеров — учтено ниже).
Зона #108 (`apply_proposal --batch` на 30+ узлах) намеренно НЕ покрыта.

## Базовые числа

| Что | Замер |
|---|---|
| `pytest -q` полный | **72.84s**, 1279 passed / 2 skipped; топ-30 длительностей = 32.4s (44%) |
| суммарно по фазам (`--durations=0`) | call = 45.7s, **setup = 20.6s (28% wall)** |
| `gen_spec.py` полный | 4.49s / **3.88s** (два прогона подряд, тёплый) |
| `gen_spec.py --docs-only` | **1.78s** |
| `what_now.py` | 1.37s |
| `emit_cipher.py` (хук каждого хода) | **1.38s** |
| `confront.py` | 0.44s |
| `diagnose(g)` in-process, тёплые кэши | **0.63s** |
| `load_content_graph()` | 9ms первый / 3ms кэшированный (293 узла) |
| pytest-оверхед: 1 маленький файл / collect-only весь suite | 3.98s / 4.75s |

---

## Категория 1 — генерация (gen_spec.py, ~3.9s тёплый)

### F1. diagnose() выполняется 3 раза за один прогон, self-граф грузится ДВАЖДЫ как разные объекты — P1, S
cProfile: `what_now.diagnose` ×3 = 3.15s из 6.7s профилированного main (~1.9s реального времени).
Вызовы: (1) `compute_cipher_lines` → self-граф через `load_content_graph()`
(кэш `_CONTENT_GRAPH_CACHE` по пути); (2) `_domain_pulse(hotam-dev)`; (3) `_domain_pulse(hotam-spec-self)`
→ **тот же self-граф, но через независимый `gen_spec._DOMAIN_GRAPH_CACHE`** (gen_spec.py:4230),
т.е. другой объект. Мемоизация `all_violations` по `id(g)` (проверено: повторный вызов на том же
объекте = 0.000s) не срабатывает — self-граф диагностируется дважды с нуля.
**Фикс:** `_load_domain_graph` для активного/pinned домена делегирует в `load_content_graph()`
(или общий ключ по resolved-пути). **Экономия ~0.6s/прогон (~15%).**

### F2. build_shared_tool_docs / build_shared_thinking_docs вызываются по 2 раза — P1, S
Два call-site с одинаковыми default-аргументами: `_write_shared_tool_docs` (запись
spec/docs/tools/*.md) и `_render_embedded_tools_block` (блок CLAUDE.md); аналогично для thinking.
Итог: **66 вызовов `_capture_tool_help`** (33 тула × 2, каждый — importlib-загрузка модуля тула),
1.03s; thinking-докстринги собираются дважды (~0.8s профилированного). `_memo_block` покрывает
только двойной рендер внутри одного call-site, не кросс-функциональный дубль.
**Фикс:** мемоизация результата по default-аргументам (тот же паттерн, что `_BLOCK_MEMO`).
**Экономия ~0.9s/прогон (~23%).**

### F3. check_section_anchors_known — 0.4-0.5s на КАЖДЫЙ all_violations в каждом процессе — P2, S/M
Профиль одного тёплого diagnose: из 1.04s — `check_section_anchors_known` = **0.478s (46%)**:
ast.walk по всем `src/hotam_spec/*.py` (38.5k узлов) на каждый вызов; парс кэширован
(`_cached_parse_path`), но обход и извлечение docstring-токенов — нет. Платят: gen_spec (×2-3),
what_now, **emit_cipher (каждый ход оператора)**, closure, attention.
**Фикс:** (а) in-process мемо результата по (path, mtime) — S; (б) персистентный индекс токенов
рядом с enforcer-index.json — M. **Экономия ~0.4s на каждый diagnose-процесс;
emit_cipher 1.38s → ~0.9s.**

Суммарно F1+F2+F3: gen_spec ~3.9s → **~2.0-2.3s (−40-45%)**; эффект каскадный — см. F7, F10.

---

## Категория 2 — тесты (72.8s)

### F4. Autouse-фикстура тянет tmp_path во все 1279 тестов: ~11s чистого setup-оверхеда — P1, S/M
`tests/conftest.py::_isolate_runtime_dir` (autouse) запрашивает `tmp_path` для каждого теста.
Микробенч на этой машине: 300 тривиальных тестов **1.27s без tmp_path vs 3.82s с tmp_path**
= ~8.5ms/тест (Windows: создание нумерованных каталогов + retention-логика tmp_path_factory).
1279 × 8.5ms ≈ **10.9s**; согласуется с общим setup = 20.6s (28% wall).
**Фикс:** session-scoped базовый каталог + дешёвый `os.mkdir` пер-тест внутри фикстуры
(семантика изоляции сохраняется; тесты, сами берущие tmp_path, не затронуты).
**Экономия ~8-10s (~12-14% suite). Лучшее соотношение эффект/усилие в тестах.**

### F5. Subprocess-тесты аргпарс-отказов — интерпретатор ради exit code — P3, S
`test_tool_create_entity_type::test_refuses_missing_args` 0.92s, `test_tool_create_domain::test_refuses_missing_args`
0.76s — спавнят `sys.executable` только чтобы проверить argparse-ошибку. In-process `main([])` +
`pytest.raises(SystemExit)` даёт то же за ~10ms. **Экономия ~1.5s.** Остальные subprocess-тесты
(root_crystal 2×1.9s, portability e2e 2.9+2.3s, hooks 1.28s, tick 1.27s) проверяют именно
env-наследование subprocess — конвертация теряет смысл теста; они автоматически ускорятся с F1-F3.

### F6. Верифицированный НЕ-проблемный паттерн: 90 прямых load_content_graph() в тестах — не трогать
Замер: 9ms первый вызов / 3ms из кэша (кэш intra-process по пути уже есть в graph.py). 90 вызовов
≈ 0.3s на весь suite. Перевод на session-фикстуру `active_graph` (existing, 11 файлов используют)
сэкономит <0.5s — **не стоит усилий**, гипотеза из задания не подтвердилась.

### F7. Крупнейшие одиночные тесты — производные от скорости gen_spec — P2 (через F1-F3)
`test_gen_spec_is_byte_idempotent` 4.87s (2 in-process прогона gen_spec), session-фикстура
`gen_spec_snapshot` 3.94s (1 прогон; уже оптимизация задачи #46), portability e2e 5.4s,
root_crystal 3.3s, template 2.1s. Суммарно gen_spec-driven ≈ **17-19s из 72.8s**. Прямых правок
не нужно: F1-F3 срежут ~40% этого блока (≈6-7s).

### F8. Разброс 70→210s — внешняя конкуренция, не деградация suite — P3, S
`.runtime/run-durations.jsonl`: типично 63-88s, пики 99/120/135/172/209s коррелируют с
параллельной агентской активностью (одновременные T2/gen_spec-подпроцессы других LAND-циклов).
Монотонной деградации нет (последние: 63, 69, 72s). Побочная находка: в журнале есть записи
**3.3s и 7.8s** — частичные прогоны прошли порог `_FULL_SUITE_THRESHOLD=100` (вероятно T1 с
большим списком node-id). Они портят статистику журнала. **Фикс:** сверять collected с реальным
полным числом тестов (или порог ~1000). Усилие S.

---

## Категория 3 — LAND-цикл (apply_proposal / gate / closure)

### F9. Сколько раз грузится граф за один apply: ~8-9 построений в 4 процессах — но это НЕ главная цена
In-process: валидация (1) + closure (1). Subprocess gen_spec pass 1: content-граф + 2 домена
(с дублем self, F1) ≈ 3; pass 2: ещё ≈ 3; pytest verify: 1+. Однако само построение графа дёшево
(~10ms) — реальная повторная цена это **diagnose/all_violations на процесс** (0.6-1.6s) и
интерпретаторные старты. Вывод: гнаться за «один парс» не нужно; давить F1/F3 и F10.

### F10. Pass 1 (--docs-only) полностью избыточен в типовом self-host случае — P1, S
apply_proposal.py:3320-3351: pass 1 (`gen_spec --docs-only`, 1.78s) + pass 2 (полный gen_spec,
3.9s). Собственный комментарий кода: «When the applied domain IS the pinned domain … the result
is identical to the old single pass». Типовое лендинг-действие — self-host домен == pin.
**Фикс:** `if applied_domain == pinned_domain: skip pass 1`. **Экономия ~1.8s на каждый LAND**
(плюс F1-F3 срежут pass 2 до ~2.2s). Итого одиночный apply: ~10-12s → ~6-7s.

### F11. Пол T1-верификации = ~4s pytest-оверхеда независимо от числа node-id — контекст, фиксить нечего дёшево
`pytest -q tests/test_project_paths.py` = 3.98s (интерпретатор + плагины + conftest + collect).
Т.е. T1 с 1-3 node-id всё равно стоит ~4-6s. Это архитектурный пол subprocess-верификации;
единственный дешёвый рычаг — не запускать pytest дважды (не наблюдалось) и F10.

### F12. Персистентный enforcer-index: (а) загрязнение tmpdir-записями, (б) инвалидация всего индекса любым тестовым файлом — P3, S
`.runtime/enforcer-index.json` (97KB) содержит **10 мусорных записей** `C:\...\Temp\tmp*` —
`_save_persistent_scan` вызывается безусловно (enforcer_resolution.py:322), хотя guard
«canonical dir only» стоит только на ЧТЕНИИ (:202). Каждое сохранение переписывает весь JSON.
Плюс fingerprint = (max mtime, count) по всем 115 test_*.py — правка ЛЮБОГО тестового файла
(в марафоне — постоянно) инвалидирует скан целиком → повторный AST-скан **0.6-1.5s** в первом
diagnose-процессе после каждой правки теста. **Фикс:** зеркальный guard на save (S); опционально
пофайловый fingerprint (M). Экономия: устранение периодических +1s к what_now/emit_cipher/gen_spec.

---

## Приоритеты (эффект / усилие)

| # | Находка | Экономия | Усилие |
|---|---|---|---|
| 1 | F4 tmp_path autouse → лёгкая изоляция | ~8-10s suite (−12-14%) | S/M |
| 2 | F2 мемо shared tool/thinking docs | ~0.9s каждый gen_spec (−23%) + ~3s suite | S |
| 3 | F1 дедуп self-графа/diagnose в gen_spec | ~0.6s каждый gen_spec (−15%) + ~2s suite | S |
| 4 | F10 скип pass 1 при applied==pinned | ~1.8s каждый LAND | S |
| 5 | F3 мемо/индекс section-anchors | ~0.4s на каждый diagnose-процесс; emit_cipher-хук 1.38→0.9s каждый ход | S/M |
| 6 | F12 guard на save + пофайловый fingerprint | периодические +1s к тулам | S |
| 7 | F5 argparse-тесты in-process | ~1.5s suite | S |
| 8 | F8 порог журнала скорости | гигиена калибровки | S |

Ожидаемый совокупный итог: gen_spec **3.9 → ~2.1s**, полный suite **~73 → ~58-60s**,
одиночный LAND **~10-12 → ~6-7s**, хук каждого хода **1.38 → ~0.9s**.
