# Спецификация Управленческих Jira-Досок И Acceptance-Досок AICAD

Дата подготовки: 2026-07-02

## Назначение

Эта спецификация описывает управленческие доски/отчеты Jira, которые должны помочь видеть не только занятость команд, но и состояние потока фич, контрактного покрытия, UAT/ПСИ, релизной готовности, дефектов и hygiene-дисциплины.

Доски не заменяют командные execution-доски FE/BE/BA/QA. Их задача - дать PMO, PO, архитектору продукта, тимлидам и QA lead подготовленные управленческие срезы без ручного просмотра десятков задач.

После добавления модели контрактных требований доски делятся на два слоя:

| Слой | Доски | Зачем |
| --- | --- | --- |
| Product / Delivery | `Flow Control`, `Epic Delivery`, `Milestone / Deadline Control`, `Release Readiness`, `QA / Defect Strategy`, `Issue Hygiene` | Управлять продуктовой поставкой, сроками и рабочим потоком |
| Contract / Acceptance | `Contract Coverage`, `Release Acceptance`, `PSI / UAT Preparation` | Управлять договорным покрытием, опытной эксплуатацией и ПСИ |

## Общие Принципы

1. Управленческие доски должны строиться на полях, а не на свободных labels и префиксах в summary.
2. Labels допустимы только для временных флагов: `cleanup-resolution`, `watch-release-risk`, `migration-workstream`, `temporary-review`.
3. `[FE]`, `[BE]`, `[BA]` в названии issue можно оставить на переходный период, но целевая модель должна использовать поле `Workstream`.
4. Поток фич должен смотреться на уровне `Epic -> Story`, а FE/BE/BA/QA задачи должны быть детализацией исполнения.
5. Контрактные требования не должны становиться Epic. Для них используется отдельная ось: `Contract Requirement -> Requirement Stage Slice`.
6. `Requirement Stage Slice` является рабочей единицей UAT/ПСИ и связывается с Epic/Story/Task/Bug через issue links.
7. Любая доска должна иметь понятное действие по каждой проблемной карточке: принять, вернуть, заблокировать, уточнить, отложить, закрыть, связать, переназначить.

## Минимальные Поля Для Досок

| Поле | Где использовать | Зачем |
| --- | --- | --- |
| `Product Area` | Epic | Где проявляется продуктовый срез |
| `Epic Type` | Epic | Vision / Discovery / R&D / MVP / Delivery / Hardening / Enabler |
| `Product Theme` | Epic | Сквозная тема: AI, Rules, Permissions, Performance |
| `Workstream` | Task, Sub-task, Bug, Story | BA / SA / FE / BE / QA / DevOps / AT |
| `Release Readiness` | Epic, Story, Bug | Candidate / At Risk / Ready / Deferred |
| `Release Mode` | Epic | Atomic / Incremental / Feature Flag / Backend First / No Prod Release |
| `Target Milestone` | Epic, Story | Веха поставки, например customer UAT milestone |
| `UAT Deadline` | Epic, Story | Конкретная дата, к которой фича должна быть готова к UAT |
| `Commitment Level` | Epic, Story | Candidate / Committed / Stretch / Deferred |
| `Forecast Delivery Date` | Epic, Story | Текущий прогноз готовности |
| `Schedule Risk` | Epic, Story | Green / Yellow / Red |
| `Schedule Risk Reason` | Epic, Story | Почему срок под риском |
| `Milestone Owner` | Epic, Story | Кто отвечает за контроль вехи |
| `Product Acceptance` | Epic, Story | Not started / In review / Accepted / Rework |
| `Acceptance Owner` | Epic, Story | Кто принимает результат |
| `Blocked Reason` | Все рабочие issue | Почему работа стоит |
| `Blocker Type` | Все рабочие issue | Dependency / Access / Requirement / Environment / Decision / Defect |
| `Blocked Owner` | Все рабочие issue | Кто отвечает за снятие блокера |
| `Unblock Target Date` | Все рабочие issue | Когда ожидается снятие блокера |
| `QA Category` | Bug, QA findings | Product bug / Regression / Acceptance gap / Test data / Environment |
| `Linked Requirement Stage Slice` / legacy `FT Requirement IDs` | Epic, Story | Покрытие договорных требований; новое управление ведется через linked slices |

## Минимальные Поля Для Contract / Acceptance Слоя

| Поле | Где использовать | Зачем |
| --- | --- | --- |
| `Contract Req ID` | Contract Requirement, Requirement Stage Slice | Стабильный номер требования из договора |
| `Requirement Slice ID` | Requirement Stage Slice | Natural key вида `REQ-008-R2-MVP` |
| `Contract Release` | Requirement Stage Slice | Релиз 1 / 2 / 3 / 4 |
| `Contract Stage` | Requirement Stage Slice | ТС / MVP / ФР / ФР1 / ФР2 |
| `Formal Acceptance Scope` | Requirement Stage Slice | Входит ли slice в формальный объем ПСИ |
| `UAT Scope` | Requirement Stage Slice | Входит ли slice в опытную эксплуатацию |
| `Implementation Coverage` | Requirement Stage Slice | None / Partial / Covered / Blocked |
| `Verification Coverage` | Requirement Stage Slice | None / Scenario Draft / Ready / Executed / Passed / Failed |
| `PSI Scenario Status` | Requirement Stage Slice, PSI Task | Not Needed / Not Started / Draft / Reviewed / Ready / Executed / Passed / Failed |
| `UAT Scenario Status` | Requirement Stage Slice, UAT Task | Not Needed / Not Started / Draft / Ready / Executed / Accepted / Rework |
| `Acceptance Status` | Requirement Stage Slice | Not Started / In Review / Accepted / Rework / Deferred |
| `Acceptance Risk` | Requirement Stage Slice | Green / Yellow / Red |
| `Evidence Link` | Requirement Stage Slice, Evidence Task | Ссылка на Confluence/attachment/evidence |
| `Work Type` | Task | PSI Scenario / UAT Scenario / Acceptance Evidence / Acceptance Gap |

## Целевая Статусная Модель И Workflows

Целевые workflows должны расширять текущую Jira-модель, а не ломать ее одномоментно. На переходном этапе можно сохранить существующие статусы (`Open`, `To Do`, `Change`, `In Progress`, `IN REVIEW`, `Ready For Testing`, `Ready to testing`, `In Testing`, `ON HOLD`, `Postpone`, `Postponed`, `Closed`) и постепенно добавить недостающие статусы/поля.

Главное изменение: добавить явный статус `Blocked`.

### Blocked vs Deferred / Postponed

`Blocked` и `Deferred` нельзя смешивать.

| Состояние | Когда использовать | Как обсуждать |
| --- | --- | --- |
| `Blocked` | Команда не может начать или продолжить работу из-за препятствия | Ежедневно, в приоритетном порядке на планерке |
| `Deferred` / `Postponed` | Работа осознанно вынесена из текущего фокуса или релиза | На planning/release/priority forum |
| `ON HOLD` | Переходный текущий статус, который нужно разнести на `Blocked` или `Deferred` | Через hygiene review |

Правила для `Blocked`:

1. `Blocked` требует `Blocked Reason`, `Blocker Type`, `Blocked Owner` и next action.
2. `Blocked` не должен использоваться как парковка низкого приоритета.
3. Все `Blocked` issues рассматриваются на ежедневной планерке до обычного WIP.
4. Если у задачи нет действия по разблокировке, это не `Blocked`, а `Deferred` / `Postponed` / scope decision.
5. `Blocked` старше 3 рабочих дней должен попадать в escalation queue.

JQL для ежедневного review:

```jql
project = AICAD
AND resolution IS EMPTY
AND (
  status = Blocked
  OR "Blocked Reason" IS NOT EMPTY
  OR status = "ON HOLD"
)
ORDER BY priority DESC, updated ASC
```

### Общий Mapping Текущих Статусов

| Текущий статус | Целевое состояние | Комментарий |
| --- | --- | --- |
| `Open`, `To Do` | Ready / To Do | Очередь готовой или почти готовой работы |
| `Ready for development` | Ready for Development | Для Story/Task, которые можно брать в реализацию |
| `Change` | Scope Review / Draft / Change Required | Нужна интерпретация по типу issue |
| `In Progress` | In Progress / In Delivery / In Fix | Активная работа |
| `IN REVIEW`, `Approval` | Review | BA/SA/design/code/product review |
| `Ready For Testing`, `Ready to testing` | Ready for Testing / Ready for Retest | Дубли нужно унифицировать |
| `In Testing` | In Testing / Retest / In UAT / In PSI | Проверка или приемка |
| `ON HOLD` | Blocked или Deferred | Требует разборки |
| `Postpone`, `Postponed` | Deferred / Parked | Не ежедневный blocker |
| `Closed`, `Complete`, `Rejected` | Done / Closed | Требует корректного `resolution` |

### Workflow Для Epic

Epic является управляемым product/delivery/acceptance slice, а не контейнером команды.

```text
Draft / Framing
-> Ready for Decomposition
-> In Delivery
-> In Acceptance
-> Ready for Release
-> Done / Closed
```

Боковые ветки:

```text
Blocked
Deferred / Parked
Change Required
```

Минимальные правила:

- `Ready for Decomposition` требует `Product Area`, `Epic Type`, owner и понятный scope.
- `In Delivery` требует хотя бы одну активную Story/Task или явное решение, почему Epic идет без декомпозиции.
- `In Acceptance` требует acceptance owner и связанный acceptance context.
- `Ready for Release` требует `Release Readiness` и отсутствие Red schedule/acceptance risk.

### Workflow Для Story

Story является единицей продуктового поведения, которую можно проверить и принять.

```text
Ready for Development
-> In Progress
-> Review
-> Ready for Testing
-> In Testing
-> Product Acceptance
-> Done
```

Боковые ветки:

```text
Blocked
Deferred
Rework
```

Минимальные правила:

- Story не должна считаться готовой, если закрыты только FE/BE tasks, но нет QA/acceptance.
- `Product Acceptance` требует acceptance owner.
- Story, связанная с customer UAT deadline, должна иметь `Target Milestone` / `UAT Deadline`.

### Workflow Для BA/SA Tasks

BA/SA задачи часто не должны проходить через `Ready for Testing`. Их результат - спецификация, сценарий, решение, декомпозиция, acceptance criteria.

```text
To Do
-> In Analysis
-> In Review
-> Ready for Development / Ready for Acceptance Prep
-> Done
```

Боковые ветки:

```text
Blocked
Deferred
Rework
```

Если отдельные статусы `In Analysis` / `Ready for Acceptance Prep` пока не созданы, на переходном этапе:

| Целевое состояние | Временный текущий статус |
| --- | --- |
| `In Analysis` | In Progress |
| `In Review` | IN REVIEW |
| `Ready for Development` | Ready for development |
| `Done` | Closed / Complete |

Правило: BA/SA task не надо искусственно гонять в `Ready for Testing`, если результатом является документ/анализ, а не тестируемый инкремент продукта.

### Workflow Для BE/FE Tasks И Sub-tasks

BE/FE задачи описывают инженерную реализацию внутри Story/Epic.

```text
To Do
-> In Progress
-> Code Review / Review
-> Ready for Testing
-> Done
```

Опционально, если команда явно ведет dev-testing:

```text
Ready for Testing -> In Testing -> Done
```

Боковые ветки:

```text
Blocked
Deferred
Rework
```

Минимальные правила:

- `Ready for Testing` означает, что реализация готова к проверке QA/acceptance, а не просто “разработчик закончил у себя”.
- BE/FE task должен иметь `Workstream`.
- Если task блокирует Story, блокер должен быть поднят на Story или связан через blocker link.

### Workflow Для QA / UAT / PSI / Evidence Tasks

Эти задачи обеспечивают проверку, приемку и evidence, особенно для `Requirement Stage Slice`.

```text
To Do
-> Drafting / Preparation
-> Review
-> Ready
-> Executing
-> Evidence Needed
-> Done
```

Боковые ветки:

```text
Blocked
Rework
Deferred
```

Если отдельные статусы пока не созданы, временный mapping:

| Целевое состояние | Временный текущий статус |
| --- | --- |
| `Drafting / Preparation` | In Progress |
| `Review` | IN REVIEW |
| `Ready` | Ready for Testing |
| `Executing` | In Testing |
| `Done` | Closed / Complete |

Правило: PSI/UAT/evidence task должен быть связан с `Requirement Stage Slice`.

### Workflow Для Bug

Bug должен проходить через triage и retest, а не смешиваться с обычными FE/BE tasks.

```text
Open
-> Triage
-> Ready for Fix
-> In Fix
-> Ready for Retest
-> Retest
-> Done
```

Боковые ветки:

```text
Blocked
Deferred / Won't Fix
Reopen
```

Если отдельного `Triage` нет, использовать поле `QA Category` / `Root Cause` и статус `Open`.

Минимальные правила:

- High/Highest bug требует `Release Readiness Impact` или `Acceptance Impact`.
- Bug, блокирующий UAT/ПСИ, должен быть связан с `Requirement Stage Slice`.
- `Done` требует retest или явное решение `Won't Fix` / `Deferred`.

### Workflow Для Contract Requirement

`Contract Requirement` - договорный reference object, он не должен проходить delivery workflow.

```text
Imported
-> Reviewed
-> Active
-> Fully Accepted
-> Closed
```

Боковая ветка:

```text
Change Required
```

### Workflow Для Requirement Stage Slice

`Requirement Stage Slice` - release-specific объект приемки.

```text
Imported
-> Scope Review
-> Implementation Planned
-> Implementation In Progress
-> Ready For UAT
-> In UAT
-> Ready For PSI
-> In PSI
-> Accepted
```

Боковые ветки:

```text
Blocked
Rework
Deferred
Not Applicable
```

Для `ТС` и `MVP` часть ПСИ-статусов может пропускаться:

```text
ТС: Imported -> Scope Review -> Scenario Ready -> Executed -> Accepted
MVP: Imported -> Scope Review -> Ready For UAT -> In UAT -> Accepted
ФР/ФР1/ФР2: ... -> Ready For PSI -> In PSI -> Accepted
```

### Как Внедрять Без Поломки Текущих Workflows

1. Сначала добавить поля `Workstream`, `Blocked Reason`, `Blocker Type`, `Blocked Owner`, `Unblock Target Date`.
2. Добавить статус `Blocked` в существующие workflows, не удаляя `ON HOLD` сразу.
3. На досках отобразить `Blocked` отдельной колонкой/swimlane.
4. Провести hygiene-разбор `ON HOLD`: разнести issues в `Blocked` или `Deferred`.
5. Для BA/SA, BE/FE, QA/UAT/PSI задач сначала использовать `Workstream` и `Work Type`, а не создавать много issue types.
6. Разделение workflow по issue types делать постепенно: сначала Epic/Story/Bug/Contract Slice, затем Task branches.
7. Только после 2-3 недель стабильного использования вводить validators/automation.

## 1. Flow Control Board

### Цель

Показывать фактический поток работ по пайплайну: где WIP, где ожидание, где тестирование, где зависание и где Jira не отражает реальность.

Это основная операционная доска для weekly flow review.

### Основные Пользователи

PMO, delivery lead, тимлиды FE/BE/BA/QA, PO/архитектор продукта как наблюдатель.

### Scope

Все unresolved рабочие issue, кроме верхнеуровневых reference/registry объектов.

`Requirement Stage Slice` не должен по умолчанию попадать на операционную delivery-доску, иначе поток разработки смешается с приемочным реестром. На Flow Control Board попадают только реальные работы по ним: UAT/PSI tasks, evidence tasks, bugfix tasks, analysis tasks.

Базовый JQL:

```jql
project = AICAD
AND resolution IS EMPTY
AND issuetype not in ("Contract Requirement", "Requirement Stage Slice")
ORDER BY Rank ASC
```

### Колонки

| Колонка | Статусы |
| --- | --- |
| `Backlog` | Backlog |
| `Ready / To Do` | Open, To Do, Ready for development |
| `Blocked` | Blocked, ON HOLD на переходном этапе |
| `In Progress` | In Progress |
| `Review` | IN REVIEW, Approval |
| `Ready for Testing` | Ready For Testing, Ready to testing |
| `In Testing` | In Testing |
| `Deferred / Parked` | Postpone, Postponed |
| `Done` | Closed, Complete, Rejected |

Рекомендация: унифицировать `Ready For Testing` и `Ready to testing`; унифицировать `Postpone` и `Postponed`.

### Swimlanes

Приоритет:

1. `Blocked`
2. `High / Highest priority`
3. `Bugs`
4. `Feature work`
5. `Technical / Enabler work`

После внедрения поля `Workstream` можно добавить quick filters:

```jql
Workstream = FE
Workstream = BE
Workstream = BA
Workstream = QA
```

### Card Layout

Показывать на карточке:

- issue type;
- assignee;
- priority;
- `Workstream`;
- Epic Link / Parent;
- `Blocked Reason`;
- linked `Requirement Slice ID`, если issue является UAT/PSI/evidence/gap работой;
- age in current status, если доступно через Jira/automation.

### Метрики

| Метрика | Как считать |
| --- | --- |
| WIP by column | Count unresolved by board column |
| Stale WIP | Active statuses + `updated <= -14d` / `updated <= -30d` |
| Blocked count | Status = Blocked / ON HOLD или `Blocked Reason is not EMPTY` |
| Blocked aging | Blocked older than 1/3/5 working days |
| Ready for Testing age | Ready For Testing / Ready to testing older than N days |
| Testing bottleneck | In Testing count and age |
| UAT/PSI work WIP | Tasks with `Work Type` in UAT/PSI/Evidence/GAP by status |

### Правила Дисциплины

- Issue не должен висеть в `In Progress` без обновлений больше 7-10 дней без причины.
- `Blocked` требует `Blocked Reason`, `Blocker Type`, `Blocked Owner` и next action.
- Все `Blocked` issues рассматриваются на daily planning до обычного WIP.
- `Postpone` / `Postponed` не должны использоваться как blocker; это deferred/parked scope.
- `Done / Closed` требует `resolution`.
- `Ready for Testing` требует хотя бы минимального test/acceptance context.
- UAT/PSI/evidence/gap task должен быть связан с `Requirement Stage Slice`.

## 2. Epic Delivery Board

### Цель

Показывать поток фич и продуктовых срезов: какие Epic активны, какие Story внутри них движутся, где есть риски приемки, где нет договорной трассировки, какие delivery slices зависли.

Эта доска нужна, чтобы смотреть не на отдельные `[FE]`/`[BE]` задачи, а на поставку фич.

### Основные Пользователи

PO, архитектор продукта, PMO, delivery lead, тимлиды.

### Scope

`Epic` и `Story`, связанные через Epic Link, плюс видимость contract coverage через linked `Requirement Stage Slice`.

Базовый JQL для Epic-level board:

```jql
project = AICAD
AND issuetype = Epic
AND resolution IS EMPTY
ORDER BY Rank ASC
```

Hygiene JQL:

```jql
project = AICAD
AND issuetype = Epic
AND resolution IS EMPTY
AND ("Product Area" IS EMPTY OR "Epic Type" IS EMPTY)
```

### Колонки Для Epic

| Колонка | Смысл |
| --- | --- |
| `Draft / Framing` | Epic создан, но не готов к декомпозиции |
| `Ready for Decomposition` | Есть scope и owner, можно резать |
| `In Delivery` | Активная реализация |
| `In Acceptance` | Фича проверяется и принимается |
| `Ready for Release` | Принято, ожидает релиза |
| `Done / Closed` | Завершено |
| `Parked` | Отложено |

Если новые статусы Epic пока не внедрены, использовать поле `Epic Lifecycle State`.

### Группировки

Основная группировка:

```text
Product Area -> Epic
```

Срезы:

- `Product Area`;
- `Epic Type`;
- `Product Theme`;
- `Release Mode`;
- `Acceptance Owner`;
- linked `Requirement Stage Slice`;
- `Implementation Coverage`;
- `Formal Acceptance Scope`.

### Card Layout

Показывать:

- `Epic Type`;
- `Product Area`;
- `Product Theme`;
- `Acceptance Owner`;
- `Release Mode`;
- `Product Acceptance`;
- linked Stories count / open child count.
- linked requirement slices count;
- formal acceptance slices count;
- open acceptance gaps count.

### Метрики

| Метрика | Зачем |
| --- | --- |
| Epics without Product Area / Epic Type | Контроль управленческой разметки |
| Active Epics by Product Area / Product Theme | Видимость фокуса команды |
| Epics by Epic Type | Баланс discovery / delivery / hardening / enabler |
| Epics without Acceptance Owner | Риск неуправляемой приемки |
| Epics without linked Requirement Stage Slice | Риск потери договорного покрытия |
| Formal slices by Epic | Какие Epics входят в ПСИ |
| MVP/UAT slices by Epic | Какие Epics идут в опытную эксплуатацию |

### Правила Дисциплины

- Новый delivery Epic должен иметь `Epic Type`, `Product Area`, `Acceptance Owner`.
- Epic не должен быть контейнером FE/BE/QA работ; он должен быть фазой или product slice.
- Epic без активных дочерних задач должен попадать в cleanup queue.
- Delivery Epic, заявленный в релиз, должен иметь links на affected `Requirement Stage Slice` или явное объяснение, почему это внутренний enabler/debt.

## 3. Milestone / Deadline Control Board

### Цель

Контролировать сроки по тем Epic/Story, которые имеют внешние или управленческие вехи, особенно вехи заказчика для UAT.

Эта доска нужна именно потому, что спринты в текущем процессе не являются надежным инструментом контроля сроков: есть переходящий WIP, задачи перетекают между спринтами, а ценность sprint-based аналитики низкая. Поэтому сроковые обязательства фиксируются не через Sprint, а через milestone/deadline поля на Epic/Story.

### Основные Пользователи

Руководитель проекта, PMO, PO/архитектор продукта, delivery lead, BA lead, QA lead, тимлиды.

### Scope

Epics и Stories, для которых есть целевая веха, UAT deadline или commitment.

Базовый JQL:

```jql
project = AICAD
AND resolution IS EMPTY
AND issuetype in (Epic, Story)
AND (
  "Target Milestone" IS NOT EMPTY
  OR "UAT Deadline" IS NOT EMPTY
  OR "Commitment Level" in ("Committed", "Candidate")
)
ORDER BY "UAT Deadline" ASC, "Schedule Risk" DESC, priority DESC
```

Вариант для ближайших 30 дней:

```jql
project = AICAD
AND resolution IS EMPTY
AND issuetype in (Epic, Story)
AND "UAT Deadline" <= endOfDay("+30d")
ORDER BY "UAT Deadline" ASC, "Schedule Risk" DESC
```

### Колонки

| Колонка | Смысл |
| --- | --- |
| `Candidate` | Может войти в milestone, но commitment еще не подтвержден |
| `Committed` | Обязательство по вехе подтверждено |
| `In Delivery` | Реализация идет |
| `In QA / Acceptance Prep` | Проверка, UAT/ПСИ сценарии, evidence готовятся |
| `Ready For UAT` | Можно передавать в опытную эксплуатацию |
| `Delivered To UAT` | Передано в UAT |
| `At Risk` | Прогноз или состояние не укладываются в дату |
| `Deferred` | Осознанно вынесено из milestone |

Если отдельный workflow не создается, колонки можно строить через `Commitment Level`, `Release Readiness`, `Product Acceptance`, `Schedule Risk` и обычный status.

### Swimlanes

1. `Schedule Risk = Red`
2. `Schedule Risk = Yellow`
3. `Commitment Level = Committed`
4. `UAT Deadline <= 14d`
5. `Target Milestone`

### Card Layout

Показывать:

- Epic/Story key;
- summary;
- `Product Area`;
- `Epic Type`;
- `Target Milestone`;
- `UAT Deadline`;
- `Forecast Delivery Date`;
- `Commitment Level`;
- `Schedule Risk`;
- `Schedule Risk Reason`;
- `Milestone Owner`;
- linked `Requirement Stage Slice`;
- open blockers / high bugs count.

### Метрики

| Метрика | Зачем |
| --- | --- |
| Committed items by milestone | Что обещано к конкретной вехе |
| Red/Yellow items by milestone | Где нужен управленческий фокус |
| Due in 14/30 days | Ближайшие сроковые риски |
| Forecast later than UAT Deadline | Прогнозное опоздание |
| Committed without linked requirement slices | Риск потери договорного контекста |
| Scope growth after commitment | Распухание объема после обещания |
| Ready for UAT vs Committed | Реальная готовность к передаче заказчику |

### Правила Дисциплины

- Sprint не считается сроковым обязательством, если нет `Target Milestone` / `UAT Deadline`.
- `Committed` требует `UAT Deadline`, `Milestone Owner`, `Acceptance Owner` и понятный scope.
- Если `Forecast Delivery Date` позже `UAT Deadline`, `Schedule Risk` должен быть Yellow/Red.
- `Schedule Risk = Red` требует next action, owner и дату решения.
- Epic/Story с customer UAT deadline должен быть связан с affected `Requirement Stage Slice`, если это договорный scope.
- Изменение `UAT Deadline` или перевод `Committed -> Deferred` требует явного управленческого решения.
- Milestone review смотрит не все задачи, а только committed/candidate items, Red/Yellow risks и ближайшие 14/30 дней.

### Отличие От Sprint

| Sprint | Milestone / Deadline |
| --- | --- |
| Execution bucket команды | Управленческое обязательство по сроку |
| Может содержать переходящий WIP | Должен иметь дату и owner |
| Не гарантирует готовность к UAT | Фиксирует готовность к UAT/передаче |
| Полезен для короткого ритма | Полезен для контроля обязательств |
| Не является scope contract | Может быть связан с contract slices |

## 4. Contract Coverage Dashboard

### Цель

Показывать договорное покрытие в обе стороны: какие требования и release-stage slices уже связаны с реализацией, какие готовятся к UAT/ПСИ, где нет implementation links, verification links или evidence.

Это не delivery-доска. Это управленческий реестр покрытия договора.

### Основные Пользователи

Руководитель проекта, PMO, PO/архитектор продукта, BA lead, QA lead, delivery leads.

### Scope

`Contract Requirement` и `Requirement Stage Slice`.

Базовый JQL:

```jql
project = AICAD
AND issuetype in ("Contract Requirement", "Requirement Stage Slice")
ORDER BY "Contract Req ID" ASC
```

Основной рабочий JQL для slices:

```jql
project = AICAD
AND issuetype = "Requirement Stage Slice"
ORDER BY "Contract Release" ASC, "Contract Stage" ASC, "Contract Req ID" ASC
```

### Срезы

| Срез | Зачем |
| --- | --- |
| `Contract Release` | Что относится к конкретному договорному релизу |
| `Contract Stage` | ТС / MVP / ФР / ФР1 / ФР2 |
| `Formal Acceptance Scope` | Что входит в ПСИ |
| `UAT Scope` | Что идет в опытную эксплуатацию |
| `Implementation Coverage` | Есть ли связанная реализация |
| `Verification Coverage` | Есть ли проверка/сценарий/evidence |
| `Acceptance Risk` | Где нужно управленческое внимание |

### Очереди / Saved Filters

| Queue | JQL |
| --- | --- |
| `Slices without implementation` | `project = AICAD AND issuetype = "Requirement Stage Slice" AND "Implementation Coverage" in ("None", "Partial")` |
| `Formal slices without PSI scenario` | `project = AICAD AND issuetype = "Requirement Stage Slice" AND "Formal Acceptance Scope" = "Yes" AND "PSI Scenario Status" in ("Not Started", "Draft")` |
| `MVP slices without UAT scenario` | `project = AICAD AND issuetype = "Requirement Stage Slice" AND "Contract Stage" = "MVP" AND "UAT Scenario Status" in ("Not Started", "Draft")` |
| `Red acceptance risk` | `project = AICAD AND issuetype = "Requirement Stage Slice" AND "Acceptance Risk" = "Red"` |
| `No evidence` | `project = AICAD AND issuetype = "Requirement Stage Slice" AND "Acceptance Status" = "Accepted" AND "Evidence Link" IS EMPTY` |

### Dashboard Gadgets

- Filter Results: Red/Yellow slices.
- Two-dimensional statistics: `Contract Release` x `Contract Stage`.
- Pie chart: `Implementation Coverage`.
- Pie chart: `Verification Coverage`.
- Filter Results: formal slices without PSI scenario.

### Card / Table Layout

Показывать:

- `Requirement Slice ID`;
- `Contract Req ID`;
- summary;
- `Contract Release`;
- `Contract Stage`;
- `Formal Acceptance Scope`;
- `Implementation Coverage`;
- `Verification Coverage`;
- `Acceptance Status`;
- `Acceptance Risk`;
- linked Epics/Stories count;
- linked UAT/PSI tasks count;
- `Evidence Link`.

### Метрики

| Метрика | Зачем |
| --- | --- |
| Slices by release/stage | Понимание договорного объема |
| Formal slices without PSI scenario | Риск неподготовленной ПСИ |
| MVP slices without UAT scenario | Риск неподготовленной опытной эксплуатации |
| Slices without implementation links | Риск отсутствия поставки |
| Accepted slices without evidence | Риск формальной приемки |
| Red/Yellow slices | Фокус руководителя проекта |

### Правила Дисциплины

- `Requirement Stage Slice` без связи с `Contract Requirement` является hygiene defect.
- `ФР/ФР1/ФР2` slice должен иметь `Formal Acceptance Scope = Yes`.
- `MVP` slice должен иметь `UAT Scope = Yes` и `Formal Acceptance Scope = No`.
- `Accepted` требует evidence.
- `Covered` требует linked Epic/Story или явно зафиксированной причины.

## 5. Release Readiness Board

### Цель

Показывать, что реально может попасть в ближайший релиз, что под риском, что принято, что требует QA/acceptance/release decision.

Доска должна заменить ручное прыгание по задачам перед релизом.

### Основные Пользователи

PO, PMO, QA lead, release manager, delivery lead, тимлиды.

### Scope

Epics, Stories, Bugs и release-blocking Tasks, которые имеют `fixVersion` или `Release Readiness`.

`Requirement Stage Slice` не является основным объектом этой доски, но должен быть виден через linked slices и contract coverage summary. Для управления самими slices используется `Release Acceptance Board`.

Базовый JQL:

```jql
project = AICAD
AND resolution IS EMPTY
AND (
  fixVersion is not EMPTY
  OR "Release Readiness" in (Candidate, "At Risk", Ready, Deferred)
)
ORDER BY priority DESC, updated ASC
```

Пока `fixVersion` не используется, временный JQL:

```jql
project = AICAD
AND resolution IS EMPTY
AND status in ("Ready For Testing", "Ready to testing", "In Testing", "IN REVIEW")
ORDER BY priority DESC, updated ASC
```

### Колонки

| Колонка | Смысл |
| --- | --- |
| `Candidate` | Может войти в релиз, но еще не принято |
| `At Risk` | Есть blocker, QA risk, scope risk или acceptance gap |
| `In QA / Acceptance` | Проверяется |
| `Ready` | Готово к релизу |
| `Deferred` | Осознанно вынесено |
| `Released / Done` | Выпущено / закрыто |

### Обязательные Поля

- `fixVersion` или release marker;
- `Release Readiness`;
- `Target Milestone` / `UAT Deadline`, если issue связано с customer UAT вехой;
- `Commitment Level`;
- `Schedule Risk`;
- `Product Acceptance`;
- `QA Category` для bugs/test findings;
- `Acceptance Owner`;
- `Known Issues` или linked blockers, если есть.
- linked `Requirement Stage Slice`, если issue закрывает договорный scope.

### Quick Filters

```jql
"Release Readiness" = "At Risk"
"Product Acceptance" != "Accepted"
issuetype = Bug
priority in (Highest, High)
fixVersion = "<target release>"
"Release Mode" = Atomic
```

Фильтры по linked `Requirement Stage Slice` в стандартной Jira могут быть ограничены. Если нет ScriptRunner/JQL-плагина, такие срезы лучше строить скриптом и записывать агрегаты в поля вроде `Linked Formal Slices Count` / `Acceptance Risk`.

### Метрики

| Метрика | Зачем |
| --- | --- |
| Ready vs At Risk | Go/No-Go signal |
| Bugs blocking release | Качество релизного кандидата |
| Candidate without Acceptance Owner | Риск зависания приемки |
| Candidate without fixVersion | Релизная дисциплина |
| Accepted slices | Что реально можно выпускать |
| Release candidates without contract slice links | Риск потери договорной трассировки |
| Candidates linked to Red acceptance slices | Что нельзя выпускать без решения |
| Committed milestone items at risk | Что угрожает UAT-вехам |

### Правила Дисциплины

- В релиз попадают не задачи, а принятые product slices.
- `Release Readiness = Ready` допускается только при понятном QA/acceptance state.
- Для `Release Mode = Atomic` нельзя выпускать часть Epic без отдельного решения.
- `Deferred` требует короткой причины.
- Релизный Epic/Story должен иметь links на affected `Requirement Stage Slice`, если он закрывает договорный scope.
- Если Epic/Story имеет `Commitment Level = Committed`, он должен отображаться и на `Milestone / Deadline Control Board`.

## 6. Release Acceptance Board

### Цель

Показывать, какие `Requirement Stage Slice` конкретного релиза готовы к UAT/ПСИ, какие уже приняты, какие требуют rework, где не хватает реализации, сценариев, evidence или управленческого решения.

Эта доска является главным инструментом руководителя проекта для подготовки и прохождения UAT/ПСИ.

### Основные Пользователи

Руководитель проекта, PMO, PO/архитектор продукта, BA lead, QA lead, delivery leads.

### Scope

`Requirement Stage Slice` выбранного контрактного релиза.

Базовый JQL для релиза:

```jql
project = AICAD
AND issuetype = "Requirement Stage Slice"
AND "Contract Release" = "Release 2"
ORDER BY "Acceptance Risk" DESC, "Contract Stage" ASC, "Contract Req ID" ASC
```

Для формального ПСИ:

```jql
project = AICAD
AND issuetype = "Requirement Stage Slice"
AND "Contract Release" = "Release 2"
AND "Formal Acceptance Scope" = "Yes"
ORDER BY "Acceptance Risk" DESC, "Contract Req ID" ASC
```

### Колонки

| Колонка | Смысл |
| --- | --- |
| `Scope Review` | Slice импортирован, но объем/интерпретация еще подтверждаются |
| `Implementation Planned` | Понятно, чем реализуется, но работа еще не завершена |
| `Implementation In Progress` | Реализация идет |
| `Ready For UAT` | Можно отдавать в опытную эксплуатацию |
| `In UAT` | Проходит опытную эксплуатацию |
| `Ready For PSI` | Готово к формальной проверке |
| `In PSI` | Проходит ПСИ |
| `Accepted` | Принято |
| `Blocked / Rework` | Есть блокер, gap или повторная работа |
| `Deferred` | Осознанно перенесено |

Если отдельный workflow пока не настроен, колонки строятся по `Acceptance Status`, `Implementation Coverage`, `Verification Coverage`, `PSI Scenario Status`, `UAT Scenario Status`.

### Swimlanes

1. `Acceptance Risk = Red`
2. `Acceptance Risk = Yellow`
3. `Formal Acceptance Scope = Yes`
4. `Contract Stage = MVP`
5. `Contract Stage = ТС`

### Card Layout

Показывать:

- `Requirement Slice ID`;
- `Contract Stage`;
- `Formal Acceptance Scope`;
- `UAT Scope`;
- `Implementation Coverage`;
- `Verification Coverage`;
- `PSI Scenario Status`;
- `UAT Scenario Status`;
- `Acceptance Status`;
- `Acceptance Risk`;
- `Acceptance Owner`;
- linked Epic/Story count;
- open bugs/gaps count;
- `Evidence Link`.

### Метрики

| Метрика | Зачем |
| --- | --- |
| Slices by acceptance column | Динамика приемки |
| Formal slices ready for PSI | Готовность к ПСИ |
| Formal slices blocked/rework | Риски приемки |
| MVP slices ready for UAT | Что можно отдавать в опытную эксплуатацию |
| Accepted slices with evidence | Формальная доказательность |
| Red/Yellow aging | Где риск не закрывается |

### Правила Дисциплины

- `Ready For PSI` требует `Formal Acceptance Scope = Yes`, связанный PSI scenario и implementation coverage.
- `Ready For UAT` требует UAT scenario или явное решение `Not Needed`.
- `Accepted` требует evidence и отсутствие blocking bugs/gaps.
- `Deferred` требует reason и decision owner.
- Red slice всегда должен иметь next action и owner.

## 7. PSI / UAT Preparation Board

### Цель

Показывать подготовку сценариев UAT/ПСИ и evidence по контрактным slices: что нужно написать, проверить, согласовать, выполнить и приложить к приемке.

Это рабочая доска для BA/QA/PMO, которая разгружает release acceptance review.

### Основные Пользователи

BA lead, QA lead, PMO, руководитель проекта, acceptance owners.

### Scope

Tasks с `Work Type` в `PSI Scenario`, `UAT Scenario`, `Acceptance Evidence`, `Acceptance Gap`, а также связанные `Requirement Stage Slice` без сценариев.

Базовый JQL:

```jql
project = AICAD
AND resolution IS EMPTY
AND (
  "Work Type" in ("PSI Scenario", "UAT Scenario", "Acceptance Evidence", "Acceptance Gap")
  OR (
    issuetype = "Requirement Stage Slice"
    AND (
      "Formal Acceptance Scope" = "Yes"
      OR "Contract Stage" = "MVP"
    )
  )
)
ORDER BY "Contract Release" ASC, priority DESC, updated ASC
```

### Колонки

| Колонка | Смысл |
| --- | --- |
| `Need Scenario` | Сценарий нужен, но задача/документ не создан |
| `Drafting` | Сценарий готовится |
| `Review` | Сценарий проверяется BA/QA/PO |
| `Ready` | Сценарий готов к выполнению |
| `Executing` | UAT/ПСИ выполняется |
| `Evidence Needed` | Нужны доказательства/протокол |
| `Done` | Сценарий/evidence готовы |
| `Blocked` | Нужен decision или есть blocker |

### Quick Filters

```jql
"Work Type" = "PSI Scenario"
"Work Type" = "UAT Scenario"
"Work Type" = "Acceptance Evidence"
"Work Type" = "Acceptance Gap"
"Contract Release" = "Release 2"
"Acceptance Risk" in ("Red", "Yellow")
```

### Метрики

| Метрика | Зачем |
| --- | --- |
| Formal slices without PSI scenario | Готовность к ПСИ |
| MVP slices without UAT scenario | Готовность к опытной эксплуатации |
| Scenario tasks by status | Рабочий прогресс BA/QA |
| Evidence missing after execution | Формальный риск приемки |
| Acceptance gaps by module | Где слабые места требований/реализации |

### Правила Дисциплины

- PSI/UAT task должен быть связан с `Requirement Stage Slice`.
- Сценарий без linked slice не считается управляемым.
- Evidence task должен иметь ссылку на Confluence/attachment/protocol.
- Acceptance gap должен быть связан с slice и, если нужно, с Bug/Epic/Story.

## 8. Issue Hygiene Board

### Цель

Показывать, где Jira врет или мешает аналитике: закрытые без resolution, задачи без Epic/Parent, stale WIP, отсутствие владельца, пустой release marker, некорректные статусы, ошибки контрактной трассировки и приемочного evidence.

Это не delivery-доска, а доска регулярной чистки.

### Основные Пользователи

PMO, Jira/process owner, delivery lead, тимлиды.

### Scope

Все issues AICAD, попавшие в hygiene-фильтры.

### Очереди / Swimlanes

| Queue | JQL |
| --- | --- |
| `Closed without resolution` | `project = AICAD AND status in (Closed, Done, Resolved) AND resolution IS EMPTY` |
| `Resolved outside done` | `project = AICAD AND resolution IS NOT EMPTY AND status not in (Closed, Done, Resolved, Complete)` |
| `Epic missing management fields` | `project = AICAD AND issuetype = Epic AND resolution IS EMPTY AND ("Product Area" IS EMPTY OR "Epic Type" IS EMPTY)` |
| `Issue without Epic` | `project = AICAD AND resolution IS EMPTY AND issuetype not in (Epic, "Contract Requirement", "Requirement Stage Slice", Sub-task, sTask, sBug, sDoc, sDesign) AND "Epic Link" IS EMPTY` |
| `Stale active WIP` | `project = AICAD AND resolution IS EMPTY AND status in ("In Progress", "IN REVIEW", "Ready For Testing", "Ready to testing", "In Testing") AND updated <= -30d` |
| `Blocked without reason` | `project = AICAD AND resolution IS EMPTY AND status in (Blocked, "ON HOLD") AND "Blocked Reason" IS EMPTY` |
| `Blocked without owner` | `project = AICAD AND resolution IS EMPTY AND status in (Blocked, "ON HOLD") AND "Blocked Owner" IS EMPTY` |
| `Blocked older than 3 working days` | `project = AICAD AND resolution IS EMPTY AND status in (Blocked, "ON HOLD") AND updated <= -3d` |
| `No release marker` | `project = AICAD AND resolution IS EMPTY AND fixVersion IS EMPTY` |
| `Committed without UAT deadline` | `project = AICAD AND resolution IS EMPTY AND issuetype in (Epic, Story) AND "Commitment Level" = "Committed" AND "UAT Deadline" IS EMPTY` |
| `Overdue committed item` | `project = AICAD AND resolution IS EMPTY AND issuetype in (Epic, Story) AND "Commitment Level" = "Committed" AND "UAT Deadline" < startOfDay()` |
| `Red schedule risk without reason` | `project = AICAD AND resolution IS EMPTY AND issuetype in (Epic, Story) AND "Schedule Risk" = "Red" AND "Schedule Risk Reason" IS EMPTY` |
| `Contract slice without parent requirement` | `project = AICAD AND issuetype = "Requirement Stage Slice" AND "Contract Req ID" IS EMPTY` |
| `Formal slice without PSI status` | `project = AICAD AND issuetype = "Requirement Stage Slice" AND "Formal Acceptance Scope" = "Yes" AND "PSI Scenario Status" IS EMPTY` |
| `MVP slice without UAT status` | `project = AICAD AND issuetype = "Requirement Stage Slice" AND "Contract Stage" = "MVP" AND "UAT Scenario Status" IS EMPTY` |
| `Accepted slice without evidence` | `project = AICAD AND issuetype = "Requirement Stage Slice" AND "Acceptance Status" = "Accepted" AND "Evidence Link" IS EMPTY` |
| `Acceptance work link audit` | `project = AICAD AND issuetype = Task AND "Work Type" in ("PSI Scenario", "UAT Scenario", "Acceptance Evidence", "Acceptance Gap")` |

### Колонки

| Колонка | Смысл |
| --- | --- |
| `Detected` | Нарушение найдено |
| `Assigned for Cleanup` | Назначен ответственный |
| `In Cleanup` | Исправляется |
| `Needs Decision` | Нужно решение PO/PMO/lead |
| `Cleaned` | Исправлено |

Если отдельный workflow для hygiene не нужен, можно использовать dashboard/filter results вместо Kanban-доски.

### Метрики

| Метрика | Зачем |
| --- | --- |
| Hygiene defects by type | Где основная боль |
| Aging of hygiene defects | Как долго Jira остается недостоверной |
| Defects created vs cleaned weekly | Дисциплина ведения Jira |
| Epics with Product Area / Epic Type | Прогресс управленческой разметки |
| Committed items with valid deadline fields | Достоверность срокового контроля |
| Blocked with reason/owner | Достоверность ежедневного blocker-review |
| Contract slices with complete links | Достоверность приемочной трассировки |
| Accepted slices with evidence | Формальная доказательность |

### Правила Дисциплины

- Hygiene review 1 раз в неделю, 15-30 минут.
- Не обсуждать содержание фич, только корректность Jira-состояния.
- Если issue фактически завершен, Jira должна быть закрыта корректно.
- Если issue не нужен, он должен быть закрыт/отложен с причиной.
- Contract/acceptance hygiene review должен проводиться перед каждым UAT/ПСИ форумом.
- `Accepted` без evidence всегда попадает в cleanup queue.

## 9. QA / Defect Strategy Board

### Цель

Показывать не просто список багов, а качество продукта и стратегию тестирования: где концентрируются дефекты, какие gaps в приемке, какие areas нестабильны, что блокирует релиз, UAT или ПСИ.

### Основные Пользователи

QA lead, PO, delivery lead, тимлиды, PMO.

### Scope

Все Bug issues, test findings, acceptance defects и defects/gaps, связанные с `Requirement Stage Slice`.

Базовый JQL:

```jql
project = AICAD
AND issuetype = Bug
ORDER BY priority DESC, updated DESC
```

Открытые:

```jql
project = AICAD
AND issuetype = Bug
AND resolution IS EMPTY
ORDER BY priority DESC, updated ASC
```

### Колонки

| Колонка | Статусы |
| --- | --- |
| `New / Open` | Open |
| `Triage` | Open + `QA Category is EMPTY` или отдельный status Triage |
| `Ready for Fix` | Ready for development / To Do |
| `In Fix` | In Progress |
| `Ready for Retest` | Ready For Testing / Ready to testing |
| `Retest` | In Testing |
| `Done` | Closed / Done |
| `Deferred / Won't Fix` | Postponed / resolution = Won't Fix / Deferred |

### Поля

| Поле | Зачем |
| --- | --- |
| `QA Category` | Product bug / Regression / Acceptance gap / Test data / Environment / Requirement ambiguity |
| `Product Area` | Где проявляется дефект |
| Epic Link / linked Story | К какому Epic/Story относится |
| `Severity` или priority policy | Техническая/продуктовая критичность |
| `Detected By` | QA / Customer / BA / Dev / Automated test |
| `Root Cause` | Requirement gap / Design gap / Implementation / Regression / Data / Environment |
| `Release Readiness Impact` | Blocks release / At risk / No release impact |
| `Requirement Slice ID` / linked slice | К какой договорной стадии относится дефект |
| `Contract Release` | Какой релиз/ПСИ затрагивает дефект |
| `Acceptance Impact` | Blocks PSI / Blocks UAT / Rework / No acceptance impact |

### Quick Filters

```jql
priority in (Highest, High)
"QA Category" = "Acceptance gap"
"Root Cause" = "Requirement gap"
"Release Readiness" = "At Risk"
"Acceptance Impact" in ("Blocks PSI", "Blocks UAT")
status in ("Ready For Testing", "Ready to testing", "In Testing")
updated <= -14d AND resolution IS EMPTY
```

### Метрики

| Метрика | Зачем |
| --- | --- |
| Open bugs by Product Area | Где болит продукт |
| Bugs by QA Category | Что именно ломается |
| Bugs by Root Cause | Что улучшать в процессе |
| Reopened / retest failures | Качество исправлений |
| Aging by priority | Риск накопления критичных дефектов |
| Bugs linked to Epic/Story | Traceability качества |
| Bugs linked to requirement slices | Traceability приемки |
| Bugs blocking PSI/UAT | Приемочный риск |
| Acceptance gaps by Contract Module | Где требования/сценарии требуют доработки |

### Правила Дисциплины

- Bug без `QA Category` должен быть triaged.
- Bug без связи с Epic/Story не должен попадать в релизный review как понятный риск.
- High/Highest bug требует явного release impact.
- Acceptance gap должен возвращаться не только в bugfix, но и в улучшение acceptance criteria/test strategy.
- Bug, блокирующий UAT/ПСИ, должен быть связан с `Requirement Stage Slice`.
- `Acceptance Impact = Blocks PSI` требует owner и next action.

## Рекомендуемый Порядок Внедрения

### Неделя 1

1. Создать недостающие поля delivery-слоя: `Workstream`, `Product Area`, `Epic Type`, `Release Readiness`, `Acceptance Owner`, `Blocked Reason`, `Blocker Type`, `Blocked Owner`, `Unblock Target Date`, `Target Milestone`, `UAT Deadline`, `Commitment Level`, `Forecast Delivery Date`, `Schedule Risk`, `Schedule Risk Reason`, `Milestone Owner`.
2. Создать недостающие поля contract/acceptance-слоя: `Contract Req ID`, `Requirement Slice ID`, `Contract Release`, `Contract Stage`, `Formal Acceptance Scope`, `UAT Scope`, `Implementation Coverage`, `Verification Coverage`, `Acceptance Status`, `Acceptance Risk`.
3. Создать issue types `Contract Requirement` и `Requirement Stage Slice`.
4. Добавить статус `Blocked` в существующие workflows без удаления текущего `ON HOLD`.
5. Создать или утвердить link policy: `Requirement Stage Slice` связывается с Contract Requirement, Epic/Story, UAT/PSI tasks, Bugs/Gaps через issue links.
6. Подготовить dry-run импорт `contract_reqs.xlsx`; в Jira пока не писать без approval.

### Неделя 2

1. После review выполнить approved import: 246 `Contract Requirement` и около 580 `Requirement Stage Slice`.
2. Запустить `Contract Coverage Dashboard`.
3. Запустить `Issue Hygiene Board` с contract/acceptance hygiene filters.
4. Начать проставлять `Workstream` вместо зависимости от `[FE]`, `[BE]`, `[BA]`.
5. Начать заполнение `Epic Type`, `Product Area`, `Acceptance Owner`.

### Неделя 3

1. Запустить `Epic Delivery Board`.
2. Запустить `Flow Control Board` как weekly flow review.
3. Запустить `Milestone / Deadline Control Board`.
4. Разметить Epic/Story, у которых есть customer UAT deadlines: `Target Milestone`, `UAT Deadline`, `Commitment Level`, `Schedule Risk`.
5. Запустить candidate linking `Requirement Stage Slice -> Epic/Story` в dry-run режиме.
6. Для ближайшего релиза пройти все `ФР/ФР1/ФР2` slices и выделить formal PSI scope.
7. Ввести ежедневный blocker-review: `Blocked` / `ON HOLD` с reason, owner, next action.
8. Начать использовать `Release Readiness` для ближайшего релиза.

### Неделя 4

1. Запустить `Release Readiness Board`.
2. Запустить `Release Acceptance Board` для ближайшего контрактного релиза.
3. Запустить `PSI / UAT Preparation Board`.
4. Запустить `QA / Defect Strategy Board` с привязкой bugs/gaps к requirement slices.
5. Перевести review-форумы на prepared queues.
6. Добавить Jira automation/validators только там, где дисциплина уже повторно нарушается.

### Постоянный Цикл

1. Агент/скрипт формирует weekly coverage snapshot.
2. Агент/скрипт формирует milestone/deadline snapshot.
3. РП смотрит Red/Yellow slices, Red/Yellow schedule risks, ближайшие 14/30 дней и missing links/scenarios/evidence.
4. PO/BA/QA/Delivery подтверждают решения.
5. Агент применяет только approved Jira updates.
6. Формируется verification report.

## Важное Ограничение

Не стоит сразу пытаться сделать идеальную Jira. Первый этап - получить управляемую наблюдаемость:

- где фича;
- где работа;
- где сроковое обязательство;
- где блокер;
- где приемка;
- где договорное требование;
- где UAT/ПСИ-сценарий;
- где evidence;
- где релизный риск;
- где Jira врет.

Когда эти вопросы становятся видимыми без ручного раскопа, можно усиливать workflow и автоматизацию.

Важное техническое замечание: стандартные Jira dashboards плохо показывают many-to-many трассировку и агрегаты linked issues. Для contract coverage, linked counts, Red/Yellow risk и acceptance matrix нужен дополнительный скриптовый слой, который считает агрегаты и при необходимости пишет их обратно в Jira custom fields. Jira после этого используется как источник фактов и витрина.
