<!-- LEGACY (Python-era) audit/review — describes a point-in-time snapshot of the Python prototype (pytest/spec/tools references below are historical, not current instructions); see README.md for the current Go CLI. -->

# Линза 3: Оператор-UX — трение и шум

fxx-agent, read-only, 2026-07-05. Кристалл 75674 chars, 128 записей boot-cite-log, 171 записей land-log.

---

## КРИТИЧНО

### 1. P6-канал лжёт и повторяется дословно каждый промпт

11 сигналов, ~2512 chars инжекта. 3 ложных (wave16-*.json «awaiting steward» — атомы посажены, файлы не заархивированы); 2 стухших (T-1/T-2 на сделанные волны); revisit C-8600b1b8 перевзвёлся через 2 дня. Дедупа нет, снапшота «что уже показывал» нет.

Фикс: (а) архивация посаженных pending (target_anchor в графе); (б) снапшот-дедуп: полный список при изменении, иначе счётчик.

### 2. Двойная проводка хуков (БАГ)

committed settings.json + settings.local.json исполняются ОБА: emit_cipher дважды, gen_spec дважды на SessionStart, boot-cite-log задвоен, + локальный PostToolUse context_producer (~2с) на каждый tool call.

Фикс: setup_hooks вычищает пересечения из local.

### 3. Boot-cite — ритуал с неподключённым прибором

115/128 (90%) cited:false. Писатель (Stop-хук) исправно пишет лог; читатель не вызывается НИЧЕМ — ни attention, ни what_now, ни reflection.

Фикс: либо снять хук, либо rate<50% → P0-Finding в reflection.

## ВАЖНО

### 4. Кристалл 75.7k: ~75% — справочники O(атомов)

Рабочее ядро ~9k. Рост +7.8k/день → warn 130k через ~7 дней. THINKING обрезаны мид-словом = иллюзия знания. REPO-MAP дублирует TOOLS. CONSTITUTION-обрезки 40% веса при нечитаемости. Оценка среза: 55-60% без потери силы (75.7k → 30-35k).

### 5. LAND-цикл: 5-8 шагов, T2 для каждого нового узла, 33% лендов красные

T2 в 128/171 (75%); pytest_ok=false в 56/171 (33%); closure_exit в 9/171 (5%); auto-revert «not implemented».

Фикс: транзакционный batch (all-or-nothing + revert), T1 для новых из enforced_by, closure по умолчанию.

### 6. Четыре канала одного пульса

LIVE-STATE + emit_cipher + diff_watch + what_now CLI.
Фикс: единственный пушер — emit_cipher; LIVE-STATE из резидента убрать; diff_watch фильтрует LIVE-STATE.

### 7. confront.py слеп к русскому

Токенизатор [a-z0-9]; русская формулировка против R-budget-measure → «likely novel, proceed».
Фикс: не-ASCII>30% → «lexical check inapplicable — confront manually».

### 8. Петля 6 шагов театральна для read-only

Фикс: 2 режима — full для мутаций, short-circuit для чтения.

## ИНТЕРЕСНО

- 9. Два трекера работ, оба полумертвы (тикеты и TaskList).
- 10. Порог revisit +10 SETTLED при темпе 8/день = перевзводится каждые 1.5 дня.
- 11. UNMEASURED-нытьё адресовано оператору, а не человеку.
- 12. proposals/pending/ содержит design-MD, py-скрипт, wave11/ (не flat *.json).

## ТОП-3 СУПЕРВЕКТОРА

**A. Дельта-сенсориум** — инжект только нового/изменившегося; стабильное — счётчик. Сигнал/шум ×10.

**B. Кристалл = семя + указатели** — 75.7k → 30-35k, рост-кривая плоская. Исполнение «дерева ссылок».

**C. Транзакционный LAND** — batch all-or-nothing + auto-revert + T1 из proposal + closure по умолчанию.

Мета-наблюдение: вся тщательность порогов вложена в детерминированную половину (substrate), а живая (что оператор видит каждый ход) отдана сырому повтору. Сенсориум не под наблюдением: 90% провала boot-cite и ложные pending не имеют ни узла, ни стьюарда.
