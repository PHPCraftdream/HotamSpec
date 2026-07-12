# Линза 1: Онтология — типовая система

fxx-agent, read-only, 2026-07-05. Граф: 257 req / 8 conflicts / 14 assumptions / 1 operator / 1 goal / 1 process (hotam-spec-self); 8 req / 2 conflicts / 1 EntityType "wave" + 9 instances (hotam-dev).

---

## КРИТИЧНО

### K1. Anti-relitigation стоит на em-dash-регэкспе: 6 из 33 REJECTED-линий уже потеряны.

Типизированного ребра `replaces` нет — RELATION_KINDS = {supports, refines, depends_on} (`spec/src/hotam_spec/requirement.py:86`). Линия замещения живёт как проза в `why`, а RECENTLY-REJECTED генерится матчем строки `"REJECTED — REPLACES"` (`spec/tools/gen_spec.py:4064`). В графе 26 em-dash и 6 double-dash `"REJECTED -- REPLACES"` — эти 6 (включая R-operator-backend-protocol, phi-cap, M18-partition) **молча выпали** из анти-релитигационного блока.

Вектор: Relation kind `replaces` (или поле `replaced_by: tuple[str,...]` у REJECTED) + ProposedRejection.replaced_by; генерация RECENTLY-REJECTED/HISTORY/confront.py из рёбер, не из прозы. Инвариант: REJECTED с «REPLACES» в прозе, но без ребра — Violation.

### K2. Провенанс решений структурно ТЕРЯЕТСЯ.

(a) AssumptionTransition: `decided_by` обязателен на валидации, но writer пишет в граф только status + reason. Подпись остаётся в gitignored JSON. R-trust-anchor-mechanism неаудитируем из субстрата.

(b) HELD→DECIDED **стирает Variant-ы**: у C-be22cdd1 в графе `variants=()`, а выбранный V-unfreeze-entity-projection жив только внутри строки DECIDED(...). Логика writer'a перезаписывает kwarg пустым кортежем. implies/costs не-выбранных вариантов уничтожены.

(c) **Во всей онтологии нет ни одного timestamp**: ни settled_at, ни decided_at. UNCERTAIN-aging на деле меряет fan-out (≥5 зависимых), а не возраст.

Вектор: Decision/Signoff frozen-рекорд {decided_by, date, verbatim, instrument: personal|DEL-n, chosen_variant?}.

### K3. Дыра биекции write-path vs R-no-hand-edit-graph: у половины коллекций нет механического пути записи.

Нет ProposedOperator(create)/ProposedGoal/ProposedProcess/ProposedEntityInstance и нет обновления Conflict.members. wave-EntityInstance в hotam-dev — доказуемо ручной вброс. Обещание «Conflict переживает churn членов — обновляются только рёбра members» **не имеет механизма**. У DECIDED C-c3911f28 ОБА member — REJECTED-надгробия, и ничто это не подсвечивает.

Вектор: добить kinds до биекции; ProposedConflictMemberUpdate; инвариант «member is REJECTED в не-REJECTED конфликте».

## ВАЖНО

### V1. Assumption обходит keystone §Lifecycle.

Плоский frozenset ASSUMPTION_STATES + отдельный check; таблица переходов и подписной замок — прозой в докстринге. Пять имён для одного понятия: Requirement.status / Conflict.lifecycle / Goal.lifecycle / EntityInstance.state / Assumption.status.

Вектор: ASSUMPTION_LIFECYCLE как Lifecycle-константа; Transition.requires_signoff: bool.

### V2. Payload-в-строке: OPEN(question) / DECIDED(rationale) / HELD(reason) / REVISIT_WHEN(cond).

Парсинг strip("()") ломается на скобке внутри вопроса; rationale не запрашиваем; prefix_states-хак существует ТОЛЬКО ради этого. Канал ревизита раздвоен: REVISIT_WHEN(cond) И revisit_marker.

Вектор: state = чистый enum + типизированные поля рядом; prefix_states умирает.

### V3. Delegation (pending) — строить СТОИТ, но кандидат воспроизводит болезни K2/K3.

Нет ссылки на DEL-n, нет даты, нет lifecycle, scope дублирует child_op.scope.

### V4. Ticket: движок есть, граница не типизирована — уже гниёт.

links[] не валидируются; assignee — пятый неанкерённый неймспейс акторов. Держать вне графа верно (R-task-vs-action-distinct-altitudes), но граница обязана быть типизированной.

### V5. Налог на допуск типа — корневая причина пропущенных типов.

Новый тип стоит: dataclass + поле + ~5 check_* + kind + AST-writer + рендер + тесты. Именно поэтому Ticket/Decision/Delegation остаются вне онтологии.

## ИНТЕРЕСНО

- I1. Goal ≅ Assumption.IMPLEMENTS — волеизъявление смоделировано дважды.
- I2. Инвентарь спящей машинерии (Process 1 инстанс, depends_on 2 ребра, scope 0 непустых scope при 230 строках, m_tag 4 инварианта на пустой реестр).
- I3. Identity неоднородна: Conflict.id хрупок к правке context; Requirement — ручной слаг без graph-wide uniqueness; реестра анкер-префиксов нет; V4/V5-коллизия в V-неймспейсе; DEL- сквотнут тикетом.
- I4. R-no-observation-type держится, но стоки текут: датированные наблюдения — в why, журнал revisit — мутация прозы, калибровочные константы — неанкерённые допущения.

## ТОП-3 СУПЕРВЕКТОРА

**S1. Decision/Signoff frozen-рекорд** — один тип-пейлоад, прикрепляемый везде, где подписывает человек. Закрывает K2 целиком.

**S2. NODE_SCHEMAS-реестр** (kind → prefix, коллекция, ref-поля, lifecycle) с генерацией typed-anchor/dangling/lifecycle-чеков и скелетов Proposed*. Режет налог на допуск типа на порядок; закрывает K3; реестр префиксов первоклассен.

**S3. replaces-ребро + member-update + сохранение Variant** — типизация линии жизни знаний. Закрывает K1; confront→графовая трассировка; «Conflict переживает churn» получает механизм.

Рекомендации по сопутствующим: Delegation — строить (с поправками V3); Ticket — не в граф, типизировать границу; Wave — здоров как EntityType, чинить write-path инстансов; Decision — рекорд, не узел; Evidence/Observation — отказ сохранить, стоки залатать; Metric — не тип, константы-пороги анкерить как данные/Assumption; Risk — отсутствие здорово.
