# Спецификация Ведения И Управления Контрактными Требованиями В Jira

Дата подготовки: 2026-07-02

Источник требований: `contract_reqs.xlsx`, лист `ФТ `

## 1. Назначение

Эта спецификация описывает, как вести в Jira контрактные требования проекта AICAD и связывать их с продуктовой поставкой, UAT, ПСИ, дефектами, сценариями проверки и доказательствами приемки.

Документ одновременно является:

- спецификацией Jira-модели;
- инструкцией для руководителя проекта;
- постановкой задач для агентов и скриптов, которые выполняют рутинный импорт, сверку, трассировку и подготовку отчетов под контролем руководителя проекта.

Ключевой принцип:

```text
Контрактные требования отвечают на вопрос: что мы обязаны закрыть по договору.
Product Capability / Epic / Story отвечают на вопрос: как мы проектируем и поставляем продукт.
```

Эти структуры не должны совпадать один-в-один. Между ними должна быть трассировка many-to-many.

### 1.1 Как Руководителю Проекта Использовать Этот Документ

Руководитель проекта использует эту модель как операционный контур приемки:

1. держит `Contract Requirement` как стабильный договорный реестр;
2. управляет `Requirement Stage Slice` как конкретными объектами UAT/ПСИ по релизам;
3. контролирует, что каждый формальный slice имеет реализацию, сценарий проверки, evidence и понятный acceptance status;
4. поручает агентам рутинную сверку, импорт, поиск gaps и подготовку отчетов;
5. утверждает только смысловые решения: что считается покрытым, что входит в ПСИ, что переносится, что требует change request или отдельного решения.

Практический режим работы:

```text
РП ставит цель проверки -> агент готовит dry-run -> РП/PO/BA/QA review -> агент применяет только утвержденные изменения -> агент формирует verification report.
```

Агент не должен самостоятельно принимать продуктовые или договорные решения. Его зона ответственности - аккуратно собрать данные, предложить кандидаты, выполнить согласованные действия и показать расхождения.

### 1.2 Принцип Безопасной Автоматизации

Любое действие агента делится на четыре режима:

| Режим | Что делает агент | Можно ли менять Jira |
| --- | --- | --- |
| `Read-only audit` | Читает Jira/Excel/Confluence и строит отчет | Нет |
| `Dry-run proposal` | Формирует список proposed changes | Нет |
| `Approved update` | Применяет только утвержденные изменения | Да, строго по утвержденному списку |
| `Verification` | Проверяет результат после изменений | Нет, кроме исправления технических ошибок по отдельному разрешению |

Запрещено объединять `Dry-run proposal` и `Approved update` в один шаг.

## 2. Исходная Модель Требований

Файл `contract_reqs.xlsx` содержит 246 строк требований.

Основные колонки:

| Колонка | Смысл |
| --- | --- |
| `Раздел` | Верхний раздел договора |
| `Модуль` | Договорный модуль |
| `Функциональный блок/компонент` | Более детальная договорная группировка |
| `Номер требования` | Стабильный идентификатор требования |
| `Требование` | Текст требования |
| `Релиз 1` | Стадия требования в релизе 1 |
| `Релиз 2` | Стадия требования в релизе 2 |
| `Релиз 3` | Стадия требования в релизе 3 |
| `Релиз 4` | Стадия требования в релизе 4 |
| `Комментарий` | Уточнение объема или интерпретации |

В релизных колонках используются стадии:

| Стадия | Значение |
| --- | --- |
| `ТС` | Тестовый сценарий, уровень PoC / проверка подхода |
| `MVP` | Вводится в опытную эксплуатацию / UAT, но не входит в формальную ПСИ-приемку |
| `ФР` | Функциональный релиз, готовый к промышленной эксплуатации и ПСИ |
| `ФР 1` | Первый формальный функциональный срез для требования |
| `ФР 2` | Второй формальный функциональный срез для требования |

По текущему файлу получается 580 непустых release-stage срезов:

| Релиз | Непустых стадий |
| --- | ---: |
| Релиз 1 | 148 |
| Релиз 2 | 172 |
| Релиз 3 | 159 |
| Релиз 4 | 101 |

Эти 580 срезов имеет смысл создать как отдельные Jira issues, потому что именно к ним удобно привязывать UAT, ПСИ-сценарии, evidence, gaps, дефекты и решения о приемке.

## 3. Целевая Jira-Модель

### 3.1 Issue Types

Рекомендуется создать два issue type.

| Issue Type | Количество | Назначение |
| --- | ---: | --- |
| `Contract Requirement` | 246 | Стабильная строка договорного требования |
| `Requirement Stage Slice` | около 580 | Конкретная стадия конкретного требования в конкретном релизе |

Дополнительно можно использовать существующие или новые issue types:

| Issue Type | Назначение |
| --- | --- |
| `PSI Scenario` или обычный `Task` с `Work Type = PSI Scenario` | Подготовка сценария ПСИ |
| `UAT Scenario` или обычный `Task` с `Work Type = UAT Scenario` | Подготовка сценария опытной эксплуатации |
| `Acceptance Evidence` или `Task` | Сбор доказательств приемки |
| `Acceptance Gap` или `Bug`/`Task` | Разрыв между требованием и реализацией/приемкой |

На первом этапе отдельные issue types `PSI Scenario`, `UAT Scenario`, `Acceptance Evidence` можно не создавать. Достаточно использовать `Task` с полем `Work Type`.

### 3.2 Почему Stage Slice Должен Быть Отдельным Issue

Одна строка договора может проходить через несколько релизов в разном объеме:

```text
REQ-008
  Release 1: ТС
  Release 2: MVP
  Release 3: ФР
```

Если оставить только одно issue `Contract Requirement`, будет непонятно:

- к какой стадии относится ПСИ-сценарий;
- какой объем уже был в UAT;
- какая часть требования входит в формальную приемку;
- какие Epic/Story закрывают именно MVP, а какие ФР;
- где есть acceptance gap по конкретному релизу.

Поэтому рабочей единицей управления приемкой становится:

```text
Requirement Stage Slice = Contract Requirement + Release + Stage
```

Пример:

```text
CR-008 Анализ применения правил в проекте
  CRS-008-R1-TS
  CRS-008-R2-MVP
  CRS-008-R3-FR
```

### 3.3 Управленческая Логика Для Руководителя Проекта

Руководитель проекта управляет не отдельными задачами, а прохождением contract slices через контрольные состояния:

```text
Imported -> Scoped -> Linked to Implementation -> Scenario Ready -> Verified -> Accepted / Rework / Deferred
```

На каждом release/acceptance review руководитель проекта должен видеть:

- какие slices входят в текущий релиз;
- какие из них являются формальным объемом ПСИ;
- какие имеют linked Epics/Stories;
- какие не имеют сценариев UAT/ПСИ;
- какие имеют open bugs/gaps;
- какие готовы к приемке;
- какие требуют решения PO/BA/QA/Delivery.

Главные управленческие вопросы:

| Вопрос | Где должен быть ответ |
| --- | --- |
| Что мы обязаны принять по договору в релизе? | `Requirement Stage Slice` с `Contract Release` и `Formal Acceptance Scope` |
| Что уже можно отдавать в опытную эксплуатацию? | `MVP` slices с `UAT Scenario Status` и linked Epics/Stories |
| Что входит в ПСИ? | `ФР/ФР1/ФР2` slices с `Formal Acceptance Scope = Yes` |
| Чем это реализуется? | Links from slice to Epic/Story |
| Чем это проверяется? | Links from slice to UAT/PSI tasks |
| Что мешает приемке? | Linked Bugs/Gaps + `Acceptance Risk` |
| Какие решения нужны сейчас? | Release Pack / Acceptance Decision Log |

### 3.4 Контрольные Ворота

Работу с контрактными требованиями нужно вести через gates. Агент может подготовить данные для gate, но решение фиксирует руководитель проекта или назначенный владелец.

| Gate | Когда | Что проверяется | Кто утверждает |
| --- | --- | --- | --- |
| `Gate 0: Model Ready` | До импорта | Issue types, fields, link policy, permissions | Jira Admin + РП |
| `Gate 1: Import Approved` | До создания issues | Dry-run counts, sample rows, naming, natural keys | РП + BA Lead |
| `Gate 2: Scope Confirmed` | Перед планированием релиза | Какие slices входят в релиз, какие требуют UAT/ПСИ | РП + PO/BA/QA |
| `Gate 3: Coverage Ready` | До активной реализации/приемки | Есть links to Epic/Story или зафиксирован gap | РП + Delivery Leads |
| `Gate 4: Scenario Ready` | До UAT/ПСИ | Есть UAT/PSI scenarios и ответственные | РП + QA Lead + BA Lead |
| `Gate 5: Acceptance Decision` | После проверки | Accepted / Rework / Deferred, evidence, open issues | РП + PO/QA/Customer-facing owner |

### 3.5 Еженедельный Цикл Управления

Минимальный цикл для руководителя проекта:

1. Агент формирует weekly coverage snapshot.
2. РП смотрит только Red/Yellow slices и slices без links/scenarios.
3. BA Lead подтверждает корректность интерпретации requirements.
4. Delivery Leads подтверждают связь с Epics/Stories.
5. QA Lead подтверждает готовность UAT/PSI сценариев.
6. РП фиксирует решения: принять, вернуть на доработку, отложить, связать, создать gap, запросить уточнение.
7. Агент применяет утвержденные Jira changes и формирует verification report.

Итогом недели должен быть не общий статус “работаем”, а список конкретных изменений:

```text
Accepted slices
New risks
New gaps
Missing implementation links
Missing PSI/UAT scenarios
Decisions required
Approved Jira updates
```

## 4. Иерархия И Связи

### 4.1 Не Использовать Parent Link Для Контрактной Трассировки

`Parent Link` лучше оставить для продуктовой иерархии:

```text
Product Capability -> Epic
```

Контрактная трассировка должна идти через issue links и поля.

### 4.2 Связь Contract Requirement И Stage Slice

Рекомендуемый вариант:

```text
Requirement Stage Slice -- is stage of --> Contract Requirement
Contract Requirement -- has stage --> Requirement Stage Slice
```

Почему issue link лучше sub-task:

- stage slice должен свободно связываться с Epic, Story, Task, Bug, PSI/UAT-сценариями;
- stage slice может иметь собственный workflow приемки;
- stage slice должен попадать в release/acceptance boards;
- sub-task модель слишком техническая и хуже подходит для управленческого объекта.

### 4.3 Связь Stage Slice С Продуктовой Реализацией

Использовать issue links:

| Link Type | Откуда | Куда | Смысл |
| --- | --- | --- | --- |
| `is implemented by` / `implements` | Requirement Stage Slice | Epic / Story | Реализация закрывает стадию требования |
| `is verified by` / `verifies` | Requirement Stage Slice | PSI/UAT Task | Сценарий или проверка подтверждает стадию |
| `is evidenced by` / `evidences` | Requirement Stage Slice | Evidence Task / Confluence page | Доказательство приемки |
| `is blocked by` / `blocks` | Requirement Stage Slice | Bug / Task / Risk | Стадия не может быть принята |
| `has gap` / `is gap for` | Requirement Stage Slice | Acceptance Gap | Есть разрыв реализации/требования/приемки |

Если создание кастомных link types требует администрирования, на переходный период можно использовать стандартные `relates to`, но обязательно хранить смысл связи в поле `Trace Link Role`.

## 5. Поля

### 5.1 Поля Для Contract Requirement

| Поле | Тип | Обязательность | Пример |
| --- | --- | --- | --- |
| `Contract Req ID` | text | Да | `8` |
| `Contract Section` | select/text | Да | `Сквозные требования` |
| `Contract Module` | select/text | Да | `Аналитика действий и событий` |
| `Functional Block` | text | Нет | `Каталог правил` |
| `Requirement Text` | paragraph | Да | Текст требования |
| `Contract Comment` | paragraph | Нет | Уточнение из файла |
| `Source File` | text/url | Да | `contract_reqs.xlsx` |
| `Source Sheet` | text | Да | `ФТ ` |
| `Source Row` | number | Да | `4` |
| `Source Version` | text/date | Да | Версия выгрузки |
| `Requirement Owner` | user | Да | PO/BA owner |
| `Overall Coverage Status` | select | Да | Not Started / Partial / Covered / At Risk / Accepted |
| `Overall Acceptance Risk` | select | Да | Green / Yellow / Red |

### 5.2 Поля Для Requirement Stage Slice

| Поле | Тип | Обязательность | Пример |
| --- | --- | --- | --- |
| `Contract Req ID` | text | Да | `8` |
| `Requirement Slice ID` | text | Да | `REQ-008-R2-MVP` |
| `Contract Release` | select | Да | `Release 2` |
| `Contract Stage` | select | Да | `MVP` |
| `Formal Acceptance Scope` | select/boolean | Да | Yes / No |
| `UAT Scope` | select/boolean | Да | Yes / No |
| `PSI Scenario Status` | select | Да для ФР/ФР1/ФР2 | Not Needed / Not Started / Draft / Reviewed / Ready / Executed / Passed / Failed |
| `UAT Scenario Status` | select | Да для MVP | Not Needed / Not Started / Draft / Ready / Executed / Accepted / Rework |
| `Implementation Coverage` | select | Да | None / Partial / Covered / Blocked |
| `Verification Coverage` | select | Да | None / Scenario Draft / Ready / Executed / Passed / Failed |
| `Acceptance Status` | select | Да | Not Started / In Review / Accepted / Rework / Deferred |
| `Acceptance Owner` | user | Да | Ответственный за приемку |
| `Evidence Link` | url/text | Нет | Confluence/Jira attachment |
| `Acceptance Risk` | select | Да | Green / Yellow / Red |
| `Risk Reason` | paragraph | Да, если Yellow/Red |
| `Linked Product Area` | select | Нет | Редактор проектов |
| `Linked Capability` | issue link/text | Нет | Product Capability |

### 5.3 Правила Для Formal Acceptance Scope И UAT Scope

| Contract Stage | UAT Scope | Formal Acceptance Scope |
| --- | --- | --- |
| `ТС` | Optional | No |
| `MVP` | Yes | No |
| `ФР` | Yes | Yes |
| `ФР 1` | Yes | Yes |
| `ФР 2` | Yes | Yes |

## 6. Naming Convention

### 6.1 Contract Requirement Summary

```text
[REQ-008] Возможность анализа применения правил в проекте
```

### 6.2 Requirement Stage Slice Summary

```text
[REQ-008][R2][MVP] Возможность анализа применения правил в проекте
[REQ-008][R3][ФР] Возможность анализа применения правил в проекте
```

### 6.3 PSI/UAT Tasks

```text
[REQ-008][R3][ПСИ] Подготовить сценарий проверки анализа применения правил
[REQ-008][R2][UAT] Провести опытную эксплуатацию анализа применения правил
```

## 7. Workflow

### 7.1 Workflow Для Contract Requirement

`Contract Requirement` не должен проходить delivery workflow. Это договорный reference object.

Рекомендуемый workflow:

```text
Imported -> Reviewed -> Active -> Fully Accepted -> Closed
                    -> Change Required
```

Смысл статусов:

| Статус | Смысл |
| --- | --- |
| `Imported` | Требование импортировано из файла |
| `Reviewed` | Текст и метаданные сверены |
| `Active` | По требованию есть активные stage slices |
| `Fully Accepted` | Все обязательные ФР/ФР1/ФР2 slices приняты |
| `Change Required` | Нужна договорная/аналитическая интерпретация |
| `Closed` | Требование закрыто в рамках проекта |

### 7.2 Workflow Для Requirement Stage Slice

Рекомендуемый workflow:

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

Дополнительные статусы:

```text
Deferred
Blocked
Not Applicable
```

Для `ТС` и `MVP` часть статусов ПСИ может пропускаться:

```text
ТС: Imported -> Scope Review -> Scenario Ready -> Executed -> Accepted
MVP: Imported -> Scope Review -> Implementation In Progress -> Ready For UAT -> In UAT -> Accepted
ФР/ФР1/ФР2: ... -> Ready For PSI -> In PSI -> Accepted
```

Если Jira workflow на первом этапе сложно менять, можно оставить простой workflow issue, а стадии приемки хранить в полях `Acceptance Status`, `PSI Scenario Status`, `UAT Scenario Status`.

## 8. Как Работать С ПСИ И UAT

### 8.1 Для Стадии `ТС`

Цель: проверить подход, сценарий, PoC или техническую реализуемость.

Минимальные связанные объекты:

- `Requirement Stage Slice`;
- Task на тестовый сценарий или PoC;
- ссылка на результат проверки / Confluence page;
- решение: продолжать / уточнить / не применимо.

### 8.2 Для Стадии `MVP`

Цель: ввести функциональность в опытную эксплуатацию.

Минимальные связанные объекты:

- linked Epic/Story, которые реализуют MVP;
- UAT scenario task;
- evidence по результатам опытной эксплуатации;
- acceptance gaps, если есть;
- решение о переходе к ФР.

`MVP` не должен автоматически считаться формально принятым по ПСИ.

### 8.3 Для Стадий `ФР`, `ФР 1`, `ФР 2`

Цель: формальная поставка в объем ПСИ.

Минимальные связанные объекты:

- linked Epic/Story, которые закрывают формальный объем;
- PSI scenario task;
- evidence выполнения сценария;
- bugs/gaps, блокирующие приемку;
- итоговый статус `Accepted` или `Rework`.

## 9. Трассировка

### 9.1 Обязательные Направления Трассировки

Для каждого `Requirement Stage Slice` должны быть видны:

1. исходное `Contract Requirement`;
2. релиз и стадия;
3. связанные Epics/Stories реализации;
4. связанные UAT/PSI сценарии;
5. связанные bugs/gaps;
6. acceptance status;
7. evidence.

Для каждого Epic/Story должны быть видны:

1. какие contract stage slices он закрывает;
2. какие из них входят в ПСИ;
3. какие находятся только в MVP/UAT;
4. где есть gaps или open bugs.

### 9.2 Связь С Product Capability

Product Capability не должен заменять Contract Requirement.

Правильная модель:

```text
Product Capability
  -> Epic
    -> Story

Contract Requirement
  -> Requirement Stage Slice
    -> linked Epic/Story/Task/Bug
```

Связь между этими деревьями many-to-many.

## 10. Импорт Из Excel

### 10.1 Шаги Импорта

1. Прочитать `contract_reqs.xlsx`, лист `ФТ `.
2. Для каждой строки создать или обновить `Contract Requirement`.
3. Для каждой непустой release-stage ячейки создать или обновить `Requirement Stage Slice`.
4. Связать каждый stage slice с parent requirement через issue link `is stage of`.
5. Заполнить поля релиза, стадии, formal scope, UAT scope.
6. Добавить label `contract-req-import`.
7. Добавить label версии импорта, например `contract-req-import-2026-07-02`.
8. Сформировать dry-run отчет до применения изменений.

### 10.2 Идемпотентность

Импорт должен быть идемпотентным: повторный запуск не создает дубли.

Ключи сопоставления:

| Issue Type | Natural Key |
| --- | --- |
| `Contract Requirement` | `Contract Req ID` |
| `Requirement Stage Slice` | `Requirement Slice ID` |

Примеры:

```text
REQ-008
REQ-008-R1-TS
REQ-008-R2-MVP
REQ-008-R3-FR
```

### 10.3 Что Не Должен Делать Импорт

Первичный импорт не должен:

- автоматически связывать requirements с Epics без review;
- менять статусы существующих delivery issues;
- удалять issues, если требование исчезло из файла;
- перезаписывать ручные комментарии приемки;
- закрывать gaps/bugs.

### 10.4 Инструкция Агенту На Импорт

Агент, выполняющий импорт, получает от руководителя проекта:

- путь к актуальному `contract_reqs.xlsx`;
- дату/версию источника;
- подтверждение целевых issue types;
- список полей и link policy;
- режим запуска: `read-only`, `dry-run` или `approved update`.

Агент должен выполнить:

1. прочитать Excel;
2. проверить наличие обязательных колонок;
3. посчитать количество `Contract Requirement`;
4. посчитать количество `Requirement Stage Slice`;
5. проверить уникальность `Contract Req ID`;
6. сформировать natural keys для всех stage slices;
7. найти потенциальные дубли в Jira;
8. подготовить dry-run отчет;
9. дождаться утверждения;
10. применить изменения только из утвержденного списка;
11. сформировать verification report.

Dry-run отчет должен содержать:

| Раздел | Что показать |
| --- | --- |
| `Input Summary` | файл, лист, строк, дата обработки |
| `Create Plan` | сколько requirements и slices будет создано |
| `Update Plan` | сколько существующих issues будет обновлено |
| `Skip Plan` | что будет пропущено и почему |
| `Warnings` | пустые поля, неожиданные стадии, дубли, странные цепочки |
| `Sample` | 10-20 примеров будущих issues |
| `Approval Checklist` | что должен подтвердить РП перед применением |

Verification report должен содержать:

| Раздел | Что показать |
| --- | --- |
| `Applied Changes` | создано/обновлено/пропущено |
| `Errors` | ошибки API/Jira validation |
| `Reconciliation` | совпадает ли Jira с Excel после импорта |
| `Links Created` | сколько связей `is stage of` создано |
| `Next Actions` | какие ручные решения нужны |

### 10.5 Approval Checklist Для Руководителя Проекта

Перед разрешением `approved update` руководитель проекта проверяет:

1. количество импортируемых требований совпадает с ожиданием;
2. количество stage slices совпадает с непустыми release-stage ячейками;
3. naming convention устраивает команду;
4. стадия `MVP` не попала в формальный scope ПСИ;
5. стадии `ФР/ФР1/ФР2` попали в formal acceptance scope;
6. natural keys не создают дубли;
7. тестовая выборка issues читается понятно;
8. агент не планирует менять delivery issues, Epics, Stories, Bugs или их статусы.

Если хотя бы один пункт не подтвержден, импорт остается в `dry-run`.

## 11. Доски И Отчеты

### 11.1 Contract Coverage Dashboard

Назначение: показать покрытие договорных требований.

Основные срезы:

- requirements by section/module;
- stage slices by release/stage;
- slices without implementation links;
- slices without verification links;
- formal acceptance slices without PSI scenario;
- accepted / at risk / rework by release.

Примеры JQL:

```jql
project = AICAD
AND issuetype = "Requirement Stage Slice"
AND "Implementation Coverage" = None
ORDER BY "Contract Release", "Contract Req ID"
```

```jql
project = AICAD
AND issuetype = "Requirement Stage Slice"
AND "Formal Acceptance Scope" = Yes
AND "PSI Scenario Status" in ("Not Started", "Draft")
ORDER BY "Contract Release", "Contract Req ID"
```

### 11.2 Release Acceptance Board

Назначение: управлять UAT/ПСИ по релизу.

Фильтр:

```jql
project = AICAD
AND issuetype = "Requirement Stage Slice"
AND "Contract Release" = "Release 2"
ORDER BY "Contract Stage", "Acceptance Risk" DESC, "Contract Req ID"
```

Колонки:

```text
Scope Review
Implementation Planned
Implementation In Progress
Ready For UAT
In UAT
Ready For PSI
In PSI
Accepted
Blocked / Rework
```

Swimlanes:

```text
Red risk
Yellow risk
Formal Acceptance Scope = Yes
MVP / UAT
ТС
```

### 11.3 PSI Preparation Board

Назначение: управлять подготовкой сценариев ПСИ.

Фильтр:

```jql
project = AICAD
AND (
  issuetype = "Requirement Stage Slice"
  AND "Formal Acceptance Scope" = Yes
  OR issuetype = Task
  AND "Work Type" = "PSI Scenario"
)
ORDER BY "Contract Release", priority DESC, updated ASC
```

Срезы:

- formal slices without PSI scenario;
- PSI scenario draft;
- PSI scenario ready;
- PSI executed;
- PSI failed / rework.

### 11.4 Requirement Traceability Matrix

Назначение: показать двустороннюю трассировку.

Рекомендуется строить внешним скриптом в Excel/HTML, потому что Jira dashboard плохо показывает many-to-many coverage.

Строки:

```text
Contract Req ID
Requirement Slice ID
Release
Stage
Requirement Text
Linked Capability
Linked Epic
Linked Story
PSI/UAT Scenario
Open Bugs
Acceptance Status
Risk
Evidence
```

## 12. Автоматизация И Скрипты

### 12.1 Скрипт Импорта

Функции:

- dry-run импорт требований из Excel;
- создание/обновление `Contract Requirement`;
- создание/обновление `Requirement Stage Slice`;
- создание issue links `is stage of`;
- отчет о новых/измененных/пропущенных issues.

### 12.2 Скрипт Coverage

Функции:

- считать implementation coverage по linked Epics/Stories;
- считать verification coverage по linked UAT/PSI tasks;
- считать bugs/gaps по linked issues;
- обновлять поля `Implementation Coverage`, `Verification Coverage`, `Acceptance Risk`;
- формировать coverage matrix.

### 12.3 Скрипт Release Pack

Функции:

- сформировать пакет к UAT/ПСИ форуму;
- показать stage slices релиза;
- выделить formal acceptance scope;
- показать missing PSI scenarios;
- показать Red/Yellow risks;
- сформировать список решений.

### 12.4 Скрипт Drift Detection

Функции:

- сравнить текущую Jira-модель требований с новым `contract_reqs.xlsx`;
- найти измененный текст требования;
- найти измененные релизные стадии;
- найти пропавшие/новые требования;
- не применять изменения без review.

## 13. Постановка Задач Для Агентов

Этот раздел описывает рутинные работы, которые можно отдавать агентам или скриптам под контролем руководителя проекта.

Общее правило:

```text
Агент готовит данные и выполняет утвержденные действия.
Руководитель проекта принимает смысловые решения.
```

### 13.1 Агент 1: Import Agent

Назначение: первично загрузить требования и stage slices из Excel в Jira.

Входы:

- `contract_reqs.xlsx`;
- список issue types;
- список custom fields;
- link policy;
- режим запуска: `dry-run` или `approved update`.

Действия:

1. прочитать Excel;
2. создать natural keys `REQ-xxx` и `REQ-xxx-Rn-STAGE`;
3. проверить дубли;
4. подготовить dry-run CSV/Markdown;
5. после утверждения создать/обновить issues;
6. связать slices с parent requirements;
7. сформировать verification report.

Выходы:

- import dry-run report;
- import changes CSV;
- created/updated issue list;
- errors/warnings list;
- reconciliation report.

Нельзя без утверждения:

- создавать issues в Jira;
- менять существующие requirements;
- менять Epics/Stories/Tasks/Bugs;
- менять статусы.

### 13.2 Агент 2: Traceability Candidate Agent

Назначение: предложить связи между `Requirement Stage Slice` и Epics/Stories.

Входы:

- Jira issues `Requirement Stage Slice`;
- текущие Epics/Stories;
- Confluence pages, если используются как источник спецификаций;
- словарь Product Area / Capability.

Действия:

1. сопоставить требования и Epics/Stories по ключевым словам, модулю, Product Area, Confluence links;
2. выделить кандидаты связей с confidence level;
3. показать, какие slices не имеют кандидатов;
4. показать, какие Epics/Stories не покрывают contract slices;
5. подготовить таблицу на review.

Выходы:

| Поле | Смысл |
| --- | --- |
| `Requirement Slice ID` | Stage slice |
| `Candidate Epic/Story` | Предлагаемая реализация |
| `Link Role` | implements / partially implements |
| `Confidence` | High / Medium / Low |
| `Reason` | Почему связь предложена |
| `Needs Human Review` | Да/Нет |

Нельзя без утверждения:

- создавать issue links;
- ставить `Implementation Coverage = Covered`;
- закрывать gaps.

### 13.3 Агент 3: PSI/UAT Scenario Agent

Назначение: найти slices, для которых нужны сценарии UAT/ПСИ, и подготовить задачи на их создание.

Входы:

- `Requirement Stage Slice`;
- `Contract Stage`;
- `Formal Acceptance Scope`;
- текущие Tasks/Docs/Confluence pages.

Действия:

1. найти `MVP` slices без UAT scenario;
2. найти `ФР/ФР1/ФР2` slices без PSI scenario;
3. предложить задачи на подготовку сценариев;
4. сгруппировать сценарии по модулю/функциональному блоку;
5. выделить потенциально объединяемые сценарии.

Выходы:

- список missing UAT scenarios;
- список missing PSI scenarios;
- proposed scenario tasks;
- grouping by module/block;
- вопросы к BA/QA.

Нельзя без утверждения:

- создавать сценарии как Jira tasks;
- помечать сценарий `Not Needed`;
- менять `PSI Scenario Status` или `UAT Scenario Status`.

### 13.4 Агент 4: Coverage And Risk Agent

Назначение: регулярно считать покрытие, риски и готовность к приемке.

Входы:

- requirement slices;
- links to Epics/Stories/Tasks/Bugs;
- statuses;
- acceptance fields;
- evidence links.

Действия:

1. считать `Implementation Coverage`;
2. считать `Verification Coverage`;
3. находить open bugs/gaps;
4. выделять stale/rework slices;
5. предлагать `Acceptance Risk`;
6. формировать weekly coverage snapshot.

Выходы:

- coverage matrix;
- Red/Yellow risk list;
- slices without implementation;
- slices without verification;
- formal slices not ready for PSI;
- release readiness summary.

Нельзя без утверждения:

- менять `Acceptance Status = Accepted`;
- менять формальный scope;
- скрывать Red risk через автоматическое снижение риска.

### 13.5 Агент 5: Release Pack Agent

Назначение: готовить пакет к UAT/ПСИ/release acceptance форуму.

Входы:

- release number;
- requirement slices релиза;
- linked Epics/Stories;
- linked UAT/PSI tasks;
- bugs/gaps;
- evidence.

Действия:

1. собрать все slices релиза;
2. разделить `ТС`, `MVP`, `ФР/ФР1/ФР2`;
3. выделить formal acceptance scope;
4. показать готовые/неготовые slices;
5. подготовить decision queue;
6. сформировать управленческий отчет.

Выходы:

```text
Release Scope Summary
Formal PSI Scope
UAT Scope
Accepted Slices
Rework Slices
Deferred Slices
Missing Scenarios
Open Bugs/Gaps
Decisions Required
```

Нельзя без утверждения:

- переносить slices между релизами;
- менять acceptance decision;
- закрывать release risks.

### 13.6 Агент 6: Drift And Change Control Agent

Назначение: контролировать расхождение между договорным Excel и Jira.

Входы:

- новая версия `contract_reqs.xlsx`;
- текущий Jira registry;
- previous import snapshot.

Действия:

1. сравнить требования по `Contract Req ID`;
2. найти изменения текста;
3. найти изменения release-stage ячеек;
4. найти новые/пропавшие требования;
5. подготовить change impact report.

Выходы:

- added requirements;
- removed requirements;
- changed requirement text;
- changed release stages;
- affected stage slices;
- required management decisions.

Нельзя без утверждения:

- удалять Jira issues;
- переписывать исходный текст требования;
- менять релизный scope;
- закрывать old slices.

### 13.7 Агент 7: Jira Hygiene Agent

Назначение: поддерживать чистоту Jira-модели требований.

Действия:

- найти Contract Requirements без slices;
- найти slices без parent requirement link;
- найти duplicate `Requirement Slice ID`;
- найти slices без owner;
- найти formal slices без PSI status;
- найти accepted slices без evidence;
- найти Bugs, блокирующие приемку, но не связанные со slice.

Выходы:

- hygiene defects list;
- severity;
- proposed cleanup owner;
- proposed safe fixes;
- items requiring human decision.

Нельзя без утверждения:

- удалять дубли;
- менять ownership;
- закрывать accepted/rework/deferred;
- менять evidence links.

### 13.8 Единый Формат Поручения Агенту

Руководитель проекта ставит задачу агенту в таком формате:

```text
Цель:
Режим: read-only / dry-run / approved update
Источник данных:
Scope:
Что считать успехом:
Что нельзя делать:
Формат результата:
Кто утверждает следующий шаг:
```

Пример:

```text
Цель: подготовить dry-run импорт contract_reqs.xlsx в Jira.
Режим: dry-run.
Источник данных: /Users/.../contract_reqs.xlsx, лист ФТ.
Scope: только Contract Requirement и Requirement Stage Slice.
Что считать успехом: отчет с 246 requirements и 580 slices, без дублей natural keys.
Что нельзя делать: не создавать issues, не менять Jira.
Формат результата: Markdown summary + CSV proposed changes.
Кто утверждает следующий шаг: руководитель проекта и BA Lead.
```

### 13.9 Definition Of Done Для Агентских Работ

Работа агента считается завершенной, если:

1. есть входной snapshot;
2. есть понятный отчет;
3. есть список ошибок и предупреждений;
4. есть список предложенных или примененных изменений;
5. все изменения можно воспроизвести;
6. нет скрытых действий вне scope;
7. руководитель проекта может принять решение без ручного раскопа Jira.

## 14. Роли И Ответственность

| Роль | Ответственность |
| --- | --- |
| PO / Product Architect | Интерпретация требования, связь с capabilities/epics, acceptance decisions |
| BA Lead | Проверка корректности импорта, детализация требований, UAT/PSI сценарии |
| QA Lead | Стратегия проверки, статусы verification coverage, дефекты приемки |
| PMO | Дисциплина статусов, readiness отчетность, release acceptance board |
| Delivery Leads | Связь implementation issues с requirement slices |
| Jira Admin | Issue types, fields, screens, workflows, link types, permissions |

## 15. Правила Дисциплины

1. Нельзя закрывать `Requirement Stage Slice` как `Accepted`, если нет acceptance/evidence.
2. `ФР/ФР1/ФР2` slice должен иметь PSI scenario или явное решение `Not Needed` с причиной.
3. `MVP` slice должен иметь UAT scenario или явное решение `Not Needed` с причиной.
4. Epic/Story, заявленные в релиз, должны быть связаны хотя бы с одним requirement stage slice или иметь объяснение, почему это внутренний enabler/debt.
5. Bug, блокирующий приемку, должен быть связан с relevant stage slice.
6. Если требование закрывается частично, это отражается на уровне slice, а не через двусмысленный статус всего requirement.
7. Контрактное требование не редактируется свободно в Jira без фиксации source version и причины.
8. Агентские изменения в Jira выполняются только после dry-run и утверждения.
9. Каждый массовый update должен иметь verification report.
10. Любое автоматическое изменение статуса приемки запрещено без явного решения РП/PO/QA.

## 16. Порядок Внедрения

### Этап 1. Подготовка Модели

1. Создать issue types `Contract Requirement` и `Requirement Stage Slice`.
2. Создать поля из раздела 5.
3. Создать link types или утвердить временную link policy.
4. Настроить screens для удобного просмотра.
5. Подготовить dry-run импорт.
6. Назначить владельца каждого агентского пакета работ.
7. Согласовать шаблон dry-run и verification report.

### Этап 2. Первичный Импорт

1. Загрузить 246 `Contract Requirement`.
2. Загрузить около 580 `Requirement Stage Slice`.
3. Связать stage slices с requirements.
4. Сформировать отчет по импорту.
5. BA/PO проверяют выборку по 10-20 требований из разных модулей.
6. РП утверждает переход от import dry-run к approved update.

### Этап 3. Трассировка С Реализацией

1. Начать связывать stage slices с текущими Epics/Stories.
2. Для ближайшего релиза пройти все `ФР/ФР1/ФР2` slices.
3. Выделить slices без implementation coverage.
4. Выделить slices без UAT/PSI сценариев.
5. Запустить release acceptance board.
6. Агент формирует candidate links, но issue links создаются только после review.

### Этап 4. Приемочная Дисциплина

1. Ввести правило: новый релизный Epic должен указывать affected requirement slices.
2. Ввести weekly coverage review.
3. Запустить PSI preparation board.
4. Автоматически считать risk/coverage поля.
5. Формировать acceptance pack к каждому ПСИ.
6. Проводить weekly agent-assisted review: агент готовит отчет, РП фиксирует решения, агент применяет утвержденные изменения.

## 17. Минимальный Набор Для Первого Запуска

Чтобы не перегрузить Jira, первый запуск можно сделать так:

1. Issue types:
   - `Contract Requirement`;
   - `Requirement Stage Slice`.
2. Обязательные поля для `Contract Requirement`:
   - `Contract Req ID`;
   - `Contract Section`;
   - `Contract Module`;
   - `Requirement Text`;
   - `Source Row`.
3. Обязательные поля для `Requirement Stage Slice`:
   - `Requirement Slice ID`;
   - `Contract Req ID`;
   - `Contract Release`;
   - `Contract Stage`;
   - `Formal Acceptance Scope`;
   - `UAT Scope`;
   - `Acceptance Status`;
   - `Acceptance Risk`.
4. Link:
   - `Requirement Stage Slice` -> `Contract Requirement`.
5. Первый отчет:
   - release/stage coverage;
   - formal slices without PSI scenario;
   - slices without linked Epic/Story;
   - Red/Yellow risks.

Остальные поля и автоматизации можно добавлять после первого цикла review.

## 18. Итоговая Рекомендация

Для AICAD целевая модель должна быть такой:

```text
Contract Requirement
  stable contractual obligation

Requirement Stage Slice
  release-specific acceptance and coverage object

Product Capability / Epic / Story
  product delivery structure

PSI/UAT/Evidence/Bug/Gap
  verification and acceptance evidence
```

Это позволит:

- не ломать продуктовую структуру Epics;
- видеть договорное покрытие в обе стороны;
- управлять ПСИ по конкретным стадиям требований;
- отделить опытную эксплуатацию от формальной приемки;
- видеть, какие требования закрыты частично;
- готовить релизные acceptance packs без ручного раскопа Jira.
