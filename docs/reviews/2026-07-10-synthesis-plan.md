# Синтез пятилинзового ревью 2026-07-10 — план исполнения

Источники (детальные отчёты):
- `2026-07-10-lens-1-redundancy.md` — избыточность/дублирование (8 находок)
- `2026-07-10-lens-2-simplification.md` — архитектурное упрощение под J1/J3 (8 находок)
- `2026-07-10-lens-3-usability.md` — cold-start юзабельность (5 находок)
- `2026-07-10-lens-4-applicability.md` — переносимость/честность доков (4 блока)
- `2026-07-10-lens-5-quality.md` — техдолг/качество (7 блоков)

Контекст: прошлая волна упрощения (#82–98, план `2026-07-09-simplification-plan.md`)
выполнена полностью и перепроверена линзой 1 — все заявленные сносы/слияния реальны.
Новое позиционирование (J1, коммит `b2c58c8`): ядро — «память и дисциплина для
человек+LLM-флота»; борьба с противоречиями — свойство. J3: Conflict-граф периферия
(270 Req : 8 Conflict, все DECIDED).

## Кросс-валидированные темы (нашли независимо ≥2 линзы)

1. **uv-зависимость не убрана** (L3-F1 + L5-#5): README/CONTRIBUTING/PR-template +
   докстринги 17 инструментов подают `uv run` как единственную форму; prerequisite
   нигде не заявлен. Стьюард ранее явно просил убрать обязательность uv.
2. **Спавн-трассировка мертва на практике** (L2-#3 + L4-#4): spawn-log содержит
   1 тестовую запись при десятках реальных host-спавнов за марафон; 4 параллельных
   механизма трассировки, ведётся только DG-файлы. Заявленное ядро («дисциплина
   флота») — единственная зона систематического расхождения правила и практики.
3. **Закрываемый enforcement-долг** (пульс P0 + L5-#3 + L4-#4): из 6 closeable —
   два закрываются бесплатно (enforcer-тесты уже написаны, не подключены в графе).
4. **A-ontology-transfers не заякорен** (L4-#3): риск №1 проекта живёт только в
   ревью-доках — нарушение собственного R-anchor-everything.

---

## Этап A — Гигиена кода: баг emit_cipher + флаки-щели + дедуп-остатки
**[механика, S×7, один проход, один-два коммита]** · Источник: L1-#1..#3,#5,#6 + L5-#2

1. **БАГ** `spec/tools/emit_cipher.py:25-35` — обходит единый резолвер
   `resolve_active_domain()` (читает pin-файл напрямую, игнорирует env
   `HOTAM_SPEC_ACTIVE_DOMAIN`) — нарушение R-active-domain-pin-not-alphabetical,
   реальное расхождение счётчика «other domains open». Плюс: мёртвый параметр
   `text`, инлайновый sys.path-пролог вместо `_bootstrap` (единственный в tools/).
2. Флаки-щель `spec/tests/test_root_crystal_follows_pin.py:158-162` — безусловный
   `pop()` env-переменной вместо save/restore.
3. `spec/tests/test_pending_proposal_archive.py:88` — `time.sleep(0.05)` → явный
   `os.utime` для mtime-порядка.
4. `_load_graph(*, demo)` — 5 байт-похожих копий (attention.py:44,
   audit_atomicity.py:58, confront.py:209, what_now.py:703, audit_tensions.py:405)
   → один общий хелпер (место — по R-shared-tools-in-spec-tools).
5. apply_proposal.py:803/975/2444 — тройка байт-идентичных
   `_find_{requirement,operator,assumption}_call` → общий хелпер; убрать мёртвый
   `source_lines` из `_find_requirements_tuple_end` (:868) и его прокидывание.
6. sys.path-стреглеры в 5 тест-файлах (список в L1-отчёте).
7. pyproject `[project.scripts]`: легаси `hotam-gate`/`hotam-gate-status`
   (hotam-land покрывает) — убрать после проверки отсутствия ссылок.
8. (опц.) Неиспользуемые dev-deps cosmic-ray/z3-solver — оставить, но
   задокументировать «на вырост» комментарием; удалять только если тривиально.

## Этап B — Доки python-first: uv-свип + точность README/QUICKSTART/PROPOSAL-REFERENCE
**[механика, S-M, один проход по докам+докстрингам]** · Источник: L3-F1..F5 + L5-#5 + L1-#4 + L4-#3

1. uv-свип: README Quick start (19 вхождений), CONTRIBUTING.md, PR-template →
   python-first (`.venv`/`python`), uv — опциональная альтернатива одной строкой.
   Build-backend `uv_build` в этом проходе НЕ трогать (отдельное решение — в
   вопросы стьюарду).
2. Докстринги 17 инструментов `uv run python …` → `python …` → regen
   (`gen_spec.py` перепишет docs/tools/*.md).
3. F2: README.md:124 «regenerate the crystal» — AI-жаргон в human-секции → нейтрально.
4. F4: легенда анкер-нотации перед первым `R-…` (README.md:~71).
5. F5: QUICKSTART:32/35 mkdir-vs-existing-project; голая ссылка :147; упомянуть
   лимит подъёма 5 уровней в секции R1-R6.
6. README «50+ check_*» при фактических ~120 → формулировка без хрупкого числа.
7. Таблица 7 концептов README: честно пометить frozen/self-only типы
   (EntityType/Process/Goal — по L4).
8. PROPOSAL-REFERENCE.md: добавить enum-словари (status; семантика
   PROSE|STRUCTURAL|ENFORCED; enforceability c INHERENTLY_PROSE; relation-kinds)
   + анти-дрифт тест (сверка перечня полей per kind с proposal.py).

## Этап C — Граф ⇄ J1: якорь риска + бесплатный enforcement + centerpiece-дрейф
**[механика + короткий PRESENT перед LAND, S-M]** · Источник: L4-#3,#4 + L5-#3 + L2-#1

1. Землить `A-ontology-transfers` как UNCERTAIN Assumption через apply_proposal
   (owner — существующий stakeholder, вероятно framework-author; проверить по
   графу). PRESENT JSON стьюарду перед LAND.
2. Enforcement-апгрейды (Requirement-proposal update): R-crystal-carries-short-form
   → ENFORCED (enforcer `test_short_form.py` уже существует),
   R-delegation-is-a-file → ENFORCED (`test_tool_delegate.py` уже существует, на
   него уже ссылается другое ENFORCED-правило — рассинхрон бухгалтерии);
   опционально третий — R-project-root-not-hardcoded (~1 ч по L5). Если механизм
   update не поддержан писателем — честно остановиться и доложить.
3. `spec/src/hotam_spec/conflict.py:1` — канон-докстринг «the centerpiece» →
   переформулировать под J1 (первоклассный connector-узел; работа с противоречиями
   — свойство дисциплины) → regen thinking-доков.
4. LOCATE-порядок в шаблонах CLAUDE.md (mediation loop): «реестр сначала,
   TENSIONS потом» — синхронно в оба шаблона + regen.
5. Зафиксировать вопрос стьюарду (не решать): 2 PROSE-правила — кандидаты в
   INHERENTLY_PROSE (имена в L5-отчёте).

## Этап D — Консолидация инструментов: context-цепочка, CLI-диспетчеры, setup_hooks
**[механика, M]** · Источник: L2-#4..#6 + L1-#7

1. Context-цепочка: context.py + context_producer.py + setup_context_hook.py →
   один модуль/диспетчер (правила R-unmeasured-* не трогаются — только код).
2. CLI-диспетчер для малоиспользуемых (audit_tensions: 2 записи следа,
   mark_revisit_evaluated: 5, spawn_log_isolation_status: 0) по прецеденту
   land.py; 35 → ~30 файлов; решить судьбу их pip entry-points явно.
3. setup_hooks.py / setup_context_hook.py — слить общую settings-JSON обвязку.
4. ENTITIES.md (368Б заглушка) и DECISIONS.md (пустой M-реестр) — условная
   материализация (не писать пустые файлы). Это НЕ отклонённая I3-консолидация.
5. (опц., PRESENT) CONFRONT-advisory внутрь apply_proposal — петля 6 видимых
   шагов → 4; меняет текст петли в шаблоне → diff стьюарду перед LAND.

## Этап E — [PRESENT] Дисциплина флота: спавн-трассировка 4→1 + attention-источники
**[решение стьюарда + механика после решения, M]** · Источник: L2-#3 + L4-#4

1. Инвентаризация 4 механизмов трассировки (spawn-log jsonl, DG-файлы, land-log,
   ticket-history) с фактическим следом каждого; варианты консолидации A/B/C со
   стоимостью — PRESENT стьюарду, решение за ним.
2. После решения: апдейт затронутых правил (R-task-spawn-log-runtime и смежные)
   через петлю, снос лишних швов.
3. Новые attention-источники (механика, не ждёт решения по п.1): протухшие
   открытые тикеты; незакрытые DG старше порога; mutating-спавн без
   worktree-изоляции (поверх R-spawn-log-carries-isolation).

## Этап F — Adoption-prep: обкатка --batch + AST-контракт потребителя
**[механика, S-M]** · Источник: L4-#2,#4

1. Стресс-обкатка `apply_proposal.py --batch`: синтетический batch 30+ узлов на
   копии графа (tmp), замер и починка найденного. Подготовка к плану стьюарда
   (prat: 32 узла, gpsm: 15 — из PLAN-hotamspec-adoption.md).
2. AST-контракт consumer graph.py (`requirements = (...)`): проверить полноту
   диагностики при несоответствии, задокументировать контракт в
   PROPOSAL-REFERENCE/QUICKSTART.
3. Устаревший docstring domains/hotam-dev/graph.py: guard блокирует hand-edit
   всего файла — зафиксировать как открытый вопрос (нужен инструмент или разовая
   санкция стьюарда), НЕ править втихую.

---

## Порядок и зависимости

```
A (код-гигиена) → B (доки) → C (граф+regen)
                  B → D (инструменты)
                       D → E [PRESENT] (спавн-дисциплина)
                       D → F (adoption-prep)
```

Сериализация A→B→C и B→D — из-за файловых пересечений (spec/tools/*.py,
docs/gen regen). E и F после D независимы друг от друга; E ждёт слова стьюарда
по консолидации швов (механическая часть п.3 — не ждёт).

Методология исполнения (установлена в этой сессии): sx/sxx-агент исполняет →
координатор ЛИЧНО верифицирует (полный T2 + `git status` чист после прогона +
детерминизм regen при затронутом gen_spec) → независимое oh-ревью → коммит
координатором. Пуш — только по явному слову стьюарда.

## Вопросы стьюарду (копятся, не таски)

- Build-backend `uv_build` → нейтральный (hatchling/setuptools)? (L3-F1/L5-#4)
- 2 PROSE-правила → INHERENTLY_PROSE? (L5-#3, имена в отчёте)
- Мораторий на новые Conflict-атомы в self-hosting? (L2-#1)
- D1-рескоуп: Goal(1)/Process(1) в extension-слой? (L2-#2; D1 уже откладывался)
- Эскалация отложенных D3 (бинарный enforcement) и E1 (компактный индекс)? (L2-#8)

## Что сознательно НЕ вошло (уже отклонено/отложено ранее — Recently rejected)

Распилы монолитов gen_spec/apply_proposal (отклонены трижды: 6.2b, #91, #92 —
консервативные альтернативы уже сделаны); радикальная кристалл-диета E4;
полная docs/gen-консолидация I3; Entity-снос (стьюард решил оставить);
первый внешний домен (стьюард: «сам решу когда пора»).
