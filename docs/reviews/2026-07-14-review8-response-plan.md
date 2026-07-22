# Review 8 response plan — consumer-готовность и смысловые противоречия (HEAD 58e1c5b)

**Date:** 2026-07-14
**Source:** восьмое независимое ревью; оценки Применяемость 8.1 / Простота 7.4 / Развиваемость 8.4 / Поддерживаемость 8.3 / Удобство-для-агентов 8.0 / Структурная целостность 9.2 / **Семантическая непротиворечивость 6.4** / Общая 8.0. Вердикт: «пригоден для внутреннего применения; для широкого использования закрыть consumer-документацию, JSON-контракт и явную обработку смысловых противоречий».

## Verification (против HEAD 58e1c5b, до планирования — все 4 P1 воспроизведены)

| # | Утверждение ревью | Вердикт |
|---|---|---|
| 1 | P1: consumer tools/INDEX.md содержит ссылки на 27 Planned-страниц, которые consumer-профиль не пишет | CONFIRMED чтением кода: `BuildToolDocsIndex()` (tooldocs.go:116) не принимает профиль и всегда рендерит `[...](command.md)`-ссылки для Planned; genSpec под consumer пропускает запись Planned-страниц (задача #129/#133) → битые ссылки. |
| 2 | P1: consumer REPO-MAP.md и CLAUDE.md ссылаются на internal/ontology, internal/proposal, `go run ./cmd/hotam` — пути, которых в внешнем проекте нет | CONFIRMED воспроизведением: свежий init-project → REPO-MAP.md несёт 21 упоминание `internal/`, CLAUDE.md — 26 упоминаний `internal/`+`go run ./cmd/hotam`. |
| 3 | P1: `propose --land --json` печатает JSON и затем прозу в ОДИН stdout | CONFIRMED воспроизведением: stdout = JSON-документ + "applied Requirement..." + "regenerated 33 doc(s)" + "landed: ..."; stderr пуст. Любой JSON-парсер падает. |
| 4 | P1: смысловые противоречия садятся в SETTLED без следа решения | CONFIRMED воспроизведением: «export service must always encrypt records» и «...must never encrypt records» оба SETTLED, all-violations = 0. confront сработал (advisory), но ничего не потребовал. |
| 5 | UX: init (full) vs init-project (consumer) — разные дефолтные профили | CONFIRMED: init-project пишет gen_profile:consumer в manifest (задача #129); bare init не пишет ничего → resolve = full. |
| 6 | UX: справка/registry местами утверждают, что --domain обязателен | PARTIALLY — задача #136 обновила land/apply-proposal, но registry Purpose-строки (tools_data.go) и часть main.go help ещё говорят «(required)». |

## Решения по скоупу

- **В РАБОТУ (волна 8):** четыре P1 (задачи ниже, a-d), унификация профиля по умолчанию + устаревшие подсказки --domain (e), CLI propose для axis/assumption/conflict (f — прямо обслуживает P1-4: чтобы агент мог зафиксировать Conflict-узел одной командой, а не ручным JSON).
- **В РАБОТУ (волна 8, хвост):** генерация PROPOSAL-REFERENCE.md из Go-структур (g), архивация исторических reviews/checkpoints в archive/ (h) — п.4 и п.7 приоритетного плана ревью.
- **ОТЛОЖЕНО:** gate для не-Go проектов (manifest с test roots) — отдельная фича с дизайн-решениями, поднимем после этой волны отдельным решением resolver. Bootstrap-режим `--empty`/статус для R-domain-exists — мелочь, но затрагивает семантику метрик; отложено до решения resolver. Дробление claudemd.go/inspect.go/mutate.go по ответственности — рефакторинг без функциональной нужды прямо сейчас; отложено.

## Задачи волны 8 (последовательно, /crush суб-агенты — @sh при час-пике, коммит после каждой)

### R8-a (#144, P1) — consumer tools/INDEX.md без битых ссылок
`BuildToolDocsIndex(consumer bool)`: под consumer Planned-инструменты перечисляются БЕЗ markdown-ссылок (plain-текст списком имён) либо секция сворачивается в одну строку-счётчик; Implemented остаются ссылками (их страницы пишутся всегда). Тест: каждый `[...](X.md)`-линк в consumer tools/INDEX.md существует на диске (расширение существующего link-паттерна из claudemd_links_test.go на tools/INDEX.md).

### R8-b (#145, P1) — consumer-доки без внутренностей фреймворка
REPO-MAP.md (repomap_data.go: Framework-body секция + registry Purpose-строки с internal/-путями) и static-шаблоны кристалла (common.go:19, mediation loop «go run ./cmd/hotam gen-spec») становятся profile-aware: под consumer — только предметные требования, граф, applied tools и `hotam gen-spec` (установленный бинарь, не go run). Full-профиль байт-в-байт неизменен (проверка против обоих реальных доменов, read-only).

### R8-c (#146, P1) — контракт --json: ровно один JSON-документ в stdout
Все операционные сообщения (`applied...`, `regenerated N doc(s)`, `landed: ...`, confront-репорт) при --json уходят в stderr; stdout несёт ровно один JSON. proposeResult расширяется: количество regenerated docs, violations (список/счётчик). Аудит ВСЕХ команд с --json (status/what-now/gate/all-violations/propose/confront/req/brief) на тот же контракт. Тест: stdout при --json парсится как единственный JSON-документ (json.Decoder + проверка EOF).

### R8-d (#147, P1) — явное подтверждение смысловых противоречий при land
Дизайн по рекомендации ревью (не блокировать эвристикой, требовать зафиксированного решения человека): если confront-при-land находит SETTLED-кандидатов выше порога перекрытия, land требует ЛИБО `--ack-conflict <existing-C-id-or-new>` (сажает требование + создаёт/ссылается Conflict-узел со resolver), ЛИБО `--decision-ref <ссылка на решение>` (аудит-след в Signoff/history). Без одного из них land такого требования отказывает с сообщением, называющим конфликтующие анкеры. Порог и точная механика — предмет дизайна внутри задачи; принцип: R-ai-presents-not-decides сохраняется — решает человек, система лишь требует, чтобы решение было ЗАФИКСИРОВАНО. Тест: сценарий ревью (always/never encrypt) без ack падает, с ack садится + Conflict-узел существует.

### R8-e (#148) — унификация дефолтного профиля + устаревшие подсказки
`hotam init` тоже пишет gen_profile (consumer — как init-project; или наоборот, единое решение с обоснованием в коммите). Grep всех «(required)»-упоминаний --domain в main.go help и tools_data.go Purpose-строках, синхронизация с реальностью active-domain chain (задача #136).

### R8-f (#149) — hotam propose axis|assumption|conflict
Три новых propose-подвида по образцу requirement/rejection/stakeholder (задача #124): флаги → валидный proposal JSON → confront → write → optional --land. Conflict — приоритетный (обслуживает R8-d: агент фиксирует конфликт одной командой). Registry/README/drift-тесты обновляются (задача #137 сделала это структурным).

### R8-g (#150, P2) — PROPOSAL-REFERENCE.md из Go-структур
Генератор таблиц полей/enum из internal/proposal-типов (reflection или AST — решение внутри задачи), PROPOSAL-REFERENCE.md становится generated-артефактом с drift-тестом. ~23KB ручного дублирования уходит.

### R8-h (#151, P2) — архивация исторической документации
docs/reviews/ (34 файла) и docs/checkpoints/ (26) делятся на текущие + archive/ подкаталог, INDEX.md обоих каталогов обновляются (сохраняя задаче #130/#132 целостность ссылок — link-existence проверка). История не удаляется.

## Исполнение
Последовательные суб-агенты (/crush; при час-пике zai 08:00–12:00 — сразу @sh без ретраев), независимая верификация каждой задачи (build/vet/gofmt/test plain+race/all-violations оба домена/gen-spec идемпотентность), коммит после каждой, финальное @fl-ревью в конце волны. Пуш — только по явной команде resolver.
