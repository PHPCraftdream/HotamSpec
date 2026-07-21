# Линза 4 (2026-07-10) — Применимость / переносимость за пределы self-hosting

Независимый аудит. Линза: что из 228 SETTLED переносится наружу, что честно
задокументировано, что укреплять под новое позиционирование J1/J3 («память и
дисциплина для человек+LLM-флота»; Conflict — периферийная механика).
Контекст: первый внешний домен (#94/J2) сознательно отложен стьюардом —
здесь только оценка готовности и рисков, не старт работ.

Источники: `domains/hotam-spec-self/docs/gen/REQUIREMENTS.md` (роster 270+ строк),
`domains/hotam-dev/graph.py`, `README.md`, `docs/QUICKSTART-CONSUMER.md`,
`docs/reviews/lens-4-roi.md`, `docs/reviews/2026-07-09-simplification-backlog.md` (J1–J3),
`domains/hotam-spec-self/docs/gen/UNENFORCED.md`, `spec/pyproject.toml`,
план стьюарда `D:/ai_dev/prat/PLAN-hotamspec-adoption.md` (прочитан как контекст, не исполнялся).

---

## 1. Доля self-hosting-специфичного vs универсального в 228 SETTLED

Оценка по полному роster'у (классификация по трём корзинам; граница размыта,
числа — порядок величины, не бухгалтерия):

| Корзина | Доля | Примеры |
|---|---|---|
| **A. Спецификация самого фреймворка** (как устроен ЭТОТ репозиторий: генерация, гейты, хуки, layout) | **~60–65% (≈140–150 атомов)** | `R-glossary-sync-fails-dead`, `R-land-tier-trace-skips-dry-run`, `R-domain-has-manifest`, `R-claude-md-template-driven`, `R-enforcement-perimeter-visible`, `R-run-speed-guarded`, все 33 `R-tool-*`, вся секция FRAMEWORK-INVARIANTS (112 атомов) |
| **B. Агентная дисциплина** (как LLM-оператор работает с ЛЮБЫМ репо — ядро J1-позиционирования) | **~20–25% (≈45–55)** | `R-boot-reload-three-facts`, `R-speak-by-reference`, `R-crystallize-before-split`, `R-delegation-conclusions-only`, `R-delegation-is-a-file`, `R-task-spawn-log-runtime`, `R-parallel-mutating-agents-use-worktree`, `R-user-request-decomposed-to-tickets`, `R-work-within-launch-dir`, `R-agent-conduct-is-rules-not-tests` |
| **C. Универсальная методология графа** (применимо к любому бизнес-домену) | **~12–15% (≈25–35)** | `R-resolver-distinct-from-owners`, `R-conflict-min-two-members`, `R-decided-needs-human-signoff`, `R-axis-controlled-vocab`, `R-rejected-preserved-not-deleted`, `R-held-carries-variants`, `R-signoff-preserved-in-substrate`, `R-requirement-claim-is-atomic`, `R-no-hand-edit-graph`, `R-anchor-everything`, `R-ai-presents-not-decides` |

**Ключевое переосмысление оценки «60–70% не перенесутся» (lens-4-roi):**
цифра верна, но это не дефект переносимости — это **категориальная ошибка
измерения**. Корзина A и не должна «переноситься»: это продукт-спецификация
самого фреймворка (аналог его собственного SDLC). Потребительский домен
стартует с ПУСТОГО графа и пишет свои атомы; из 228 ему «переезжает» не
контент, а: (а) код фреймворка — и корзина C переезжает почти целиком как
**check_\*-инварианты, работающие на любом графе** (resolver≠owner,
min-two-members, signoff, axis-vocab — это поведение фреймворка, не текст,
который надо пере-выводить); (б) корзина B — как дисциплина оператора,
независимая от домена. Реальный непроверенный риск ýже, чем «70% правил»:
**хватает ли 8 типов онтологии и инвариантного набора для не-мета-контента,
и выживает ли enforcement-градиент вне репозитория с кодом** (см. §2).

## 2. Уроки hotam-dev — первый шаг от self-hosting уже сделан и уже дал данные

`domains/hotam-dev/graph.py`: 9 Requirement, **2 Conflict**, 2 Assumption,
2 Stakeholder, 2 Axis, 1 EntityType (wave) + 9 EntityInstance. Уроки:

1. **Онтология вместила домен без единой строчки нового кода фреймворка** —
   позитивный сигнал переносимости типов (в пределах ещё-одного-мета-домена).
2. **Обвал enforcement-градиента: 2/9 ENFORCED (22%) против 198/228 (87%) в self.**
   Первый же шаг от self-hosting подтвердил прогноз lens-4-roi §1: как только
   claim перестаёт быть про код этого репо, он честно оседает в
   PROSE/STRUCTURAL (`R-commit-follows-review`, `R-push-only-on-request`,
   `R-worktree-parallel-permitted` — все INHERENTLY_PROSE). В реальном
   бизнес-домене доля ENFORCED будет ближе к hotam-dev, чем к self, и главная
   гордость («198/228 ENFORCED») наружу не продаётся. План стьюарда это уже
   учитывает (PLAN §2.4, Q2: «на первой волне — все PROSE»).
3. **Conflict снова периферия: 9:2 (self — 271:8).** Причём вторая
   Conflict-нода решена вердиктом «здесь нет противоречия» — механика
   конфликта была поднята для не-конфликта. Обе — про механику разработки,
   не про столкновение интересов стейкхолдеров. Подтверждает J3.
4. **Дырки протокола всплыли и были закрыты именно вторым доменом**: docstring
   graph.py (строки 13–17) фиксирует, что Stakeholder/Axis/Assumption пришлось
   hand-seed'ить, т.к. Proposed-kinds не существовало. Сейчас
   `R-proposed-stakeholder-kind-exists` SETTLED и `create_axis` есть — т.е.
   **петля «новый домен → пробел → починка» работает**, но docstring
   hotam-dev теперь частично устарел (утверждает отсутствие ProposedStakeholder).
5. **Хрупкая связка для потребителя**: docstring (строки 19–24) требует, чтобы
   graph.py любого домена держал ТОЧНУЮ AST-форму `requirements = (...)` —
   иначе apply_proposal не найдёт куда дописывать. Это контракт на
   потребительский файл, задокументированный только в docstring чужого домена.
6. **EntityType получил первое реальное применение** (журнал волн) — но как
   append-only лог, а не как lifecycle-машина; заморозка Entity-аспекта
   (`R-speculative-aspects-frozen`) остаётся оправданной.
7. **A-single-human-wears-all-hats HOLDS** — честно записано, что вся
   role-separation (resolver≠owner, dev-resolver vs pipeline-operator) проверена
   только структурно: один человек носит все шляпы. Социальная механика
   стюардства (самая универсальная часть корзины C) не проверена вообще.

## 3. Честность документации

### Что уже честно (лучше, чем ожидалось после 2026-07-05)

- **README-теглайн уже перепозиционирован под J1**: «Executable memory and
  discipline for a human + LLM-agent fleet»; противоречивые требования —
  «one of its properties, not its whole purpose». Conflict-периферийность (J3)
  отражена структурно: глоссарий из 6 терминов, агентная машинерия отделена
  («skip it if you're just using... CLI discipline»).
- README и QUICKSTART-CONSUMER разделяют «Required for any team» vs
  «Optional: AI operator» — правильная лестница входа.
- QUICKSTART честен про PyPI («once published»); 27 `hotam-*` entry points
  реально существуют в `spec/pyproject.toml` — consumer-путь не вымышлен.
- **Сам граф честен**: `A-bootstrap-self-applies` = IMPLEMENTS (стремление,
  не факт); `R-speculative-aspects-frozen` (Entity/федерация/рекурсия заморожены
  «до реальной бизнес-потребности»); UNENFORCED.md отделяет closeable debt (6)
  от inherent discipline (24); 39 REJECTED сохранены с REPLACES.
- Жёсткие ревью (lens-4-roi и пр.) закоммичены в `docs/reviews/` — редкая честность.

### Что создаёт ложное впечатление «готово к продакшену»

1. **Главный пробел — по собственному закону фреймворка**: риск
   «онтология проверена только на себе» (`A-ontology-transfers`) существует
   ТОЛЬКО в review-доках (`lens-4-roi.md`, backlog J2) и **не приземлён как
   Assumption-нода**. R-anchor-everything: important-yet-invisible → typed
   anchored node. Риск №1 проекта невидим для what_now/attention/confront —
   фреймворк не ест свой собственный корм в самом важном месте.
   (Только регистрация допущения — не старт #94; решения не предвосхищает.)
2. **README нигде не говорит** «онтология проверена только на двух
   мета-доменах; внешних потребителей ноль». «Adopt: your domain in 15
   minutes» читается как проверенный путь; end-to-end он пройден только
   изнутри (тестами portability), ни разу — реальным внешним потребителем.
3. **README рукописный и уже дрейфует**: «50+ check_* functions» (реально
   ~120+ по Concept Map), «Structural invariants» — в проекте, чья главная
   claim — «drift structurally impossible», рукописный README без anti-drift
   меты — ирония, заметная снаружи.
4. Витринная таблица «Framework concepts» продаёт все 7 типов ровным списком,
   хотя Process/Goal/Operator/Entity — по сути frozen/self-only (Process: 1
   инстанс, Goal: 1, Operator: 1). Потребитель не узнает, что рабочая
   поверхность — Requirement/Conflict/Assumption/Axis/Stakeholder.

## 4. Что укреплять в первую очередь под «память+дисциплина» (J1/J3), не дожидаясь внешнего домена

Приоритет — то, что усиливает подтверждённую ценность (реестр + анти-дрифт +
анти-релитигация) и не тянет Conflict-механику:

1. **Приземлить `A-ontology-transfers` как Assumption (UNCERTAIN)** с
   зависимыми атомами — риск №1 попадает в сенсориум (uncertain-assumptions
   surface, P4) и перестаёт жить только в ревью. Дёшево, полностью в духе
   `R-uncertain-assumptions-surface`.
2. **UNENFORCED-трекинг: добавить измерение «переносимости»**. Сегодня
   burn-down меряет только ENFORCED-долю self-домена. Дешёвое расширение:
   пометить каждый SETTLED атом корзиной (framework-spec / agent-discipline /
   domain-universal — одно поле или префикс-конвенция) и дать в UNENFORCED/
   отдельной проекции срез «что из этого — экспортируемое предложение».
   Сейчас категории из §1 существуют только в головах ревьюеров.
3. **what_now/attention для «памяти», не только для графа**: сигналы уже
   лучшие в классе для graph-долга, но «память флота» живёт и в tickets/,
   delegations/, spawn-log, land-log. Атом `R-attention-registry` уже даёт
   расширяемый реестр — добавить источники: устаревшие open-тикеты (возраст),
   незакрытые DG-делегации, mutating-спавны без worktree (tool есть —
   `spawn_log_isolation_status`, в attention не влит). Это прямое усиление
   «дисциплины» без единого нового типа онтологии.
4. **Закрыть 6 closeable-долгов** (UNENFORCED): особенно
   `R-project-root-not-hardcoded` и `R-framework-suite-domain-independent` —
   оба буквально про переносимость; их энфорсеры = дешёвая страховка
   consumer-пути до появления потребителя.
5. **Анти-дрифт для README**: либо генерировать числовые факты README из
   графа (как CLAUDE.md), либо мета-тест «README не противоречит счётчикам».
   Плюс один честный абзац «Status: validated on two meta-domains;
   no external domain yet» — дешевле всего покупает доверие.
6. **Зафиксировать consumer-контракт graph.py** (AST-форма
   `requirements = (...)`) как атом + check при загрузке чужого графа с
   внятной ошибкой — сейчас первый внешний потребитель, переформатировавший
   файл ruff'ом/black'ом, получит молчаливый отказ apply_proposal.
7. **Обновить устаревший docstring hotam-dev/graph.py** (утверждение об
   отсутствии ProposedStakeholder) — мелочь, но это единственный пример
   «второго домена», по которому будущий потребитель будет учиться.

## 5. Оценка готовности к будущему внедрению (по PLAN-hotamspec-adoption, только чтение)

План стьюарда (prat → gpsm-sm) реалистично калиброван: Фаза 0 (portability)
частично уже съедена (`project_root()` R1–R6 описан в QUICKSTART, entry
points в pyproject есть); «все RULE-узлы PRAT сперва PROSE» (Q2) совпадает с
уроком hotam-dev §2.2. Два места, где план встретит трение, предсказуемое
уже сейчас: (а) enforcement-градиент почти не даст ENFORCED вне кода —
ценность там будут делать STRUCTURAL-проверки YAML-артефактов, для которых
у фреймворка пока нет ни одного примера; (б) объём Фазы 4 «32 ФТ механическим
переносом» упрётся в стоимость per-proposal цикла (lens-4-roi §3: 4–6 JSON +
гейт на конфликт) — batch-режим apply_proposal существует (`--batch`), но
на потоке из 32+15 узлов ни разу не обкатан. Оба пункта — аргументы в пользу
рекомендаций §4.2 и §4.4, а не в пользу немедленного старта #94.

---

**Итог линзы.** Переносимость как «доля правил» — ложная метрика: ~60–65%
атомов и не должны переноситься (спецификация самого фреймворка), реальный
экспорт — это код-инварианты корзины C + дисциплина корзины B, и именно они
совпадают с новым позиционированием J1. hotam-dev уже дал главный
эмпирический факт (обвал ENFORCED 87%→22% за один шаг от self-hosting) и
доказал петлю «новый домен вскрывает пробел → пробел чинится». Документация
после 2026-07-05 стала заметно честнее (теглайн, лестница входа, IMPLEMENTS),
но риск №1 до сих пор не существует как узел собственного графа — это самое
дешёвое и самое показательное укрепление из всех возможных.
