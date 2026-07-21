# Review 6 response plan — стабилизация к релизу (HEAD d66547f)

**Date:** 2026-07-14
**Source:** шестое независимое ревью; оценки Применяемость 8.1 / Простота 7.3 / Развиваемость 8.2 / Поддерживаемость 7.7 (−0.3!) / Удобство-для-агентов 8.6 / Общая 8.0. Вердикт: «для пилота — уже применимо; для production/release нужно закрыть P0/P1». Дополнительно вобраны 3 находки финального @fl-ревью волны 5 (N1 blocked_on, N2 propose -h, N3 битые ссылки INDEX).

## Verification (против HEAD d66547f, до планирования)

| # | Утверждение ревью | Вердикт |
|---|---|---|
| 1 | P0: обычный `go test ./...` красный — `t.TempDir()` на Windows живёт под `C:\Users\Computer\AppData\Local\Temp`, а маркер-поиск вверх по дереву находит чужие `domains/`/`.claude/`/`CLAUDE.md` в home | CONFIRMED воспроизведением: `TestRepoRootForDomain_NoProjectRootFallsBackToDomainDir` + `TestExternal_InitGenSpecOutsideAnyProject_BareDomain` падают без чистого TMP/TEMP. Это ровно та контаминация, из-за которой вся сессия работает с TMP=D:\ai_dev\_clean_tmp — но тесты обязаны быть герметичными сами, а не полагаться на дисциплину запускающего. |
| 2 | P1: смена профиля full→consumer оставляет старые thinking/*.md и Planned tool-pages на диске (30 в stdout, 90 реально) | CONFIRMED структурно: genSpec только пишет, никогда не удаляет. |
| 3 | P1: root crystal устаревает после `hotam land` без `--claude-md` (флаг опционален, агент его не передаст) | CONFIRMED (известное поведение; в этой сессии оркестратор дважды ловил эту самую пропущенную регенерацию у суб-агентов). |
| 4 | P1: consumer-кристалл ссылается на несуществующие пути (thinking/, и др.) | CONFIRMED: свежий init-project → CLAUDE.md содержит 2 ссылки на docs/gen/thinking, каталога нет. |
| 5 | P1: несогласованные флаги — `propose stakeholder` использует `--domain` для поля объекта и `--domain-dir` для графа; apply-proposal/land требуют явный `--domain` вопреки active-domain у остальных; README говорит «fifteen» при 17 реализованных; `--profile`/`confront --proposal` не документированы; QUICKSTART-CONSUMER ведёт через старый init; REPO-MAP tools hand-maintained | CONFIRMED по всем пунктам (grep + чтение repomap_data.go, который сам признаётся в hand-maintained). |
| 6 | Consumer REQUIREMENTS.md ~27KB для одного seed-требования (44 tool-derived + полная методология) | CONFIRMED по построению BuildRequirements (безусловные секции). |

Из @fl-ревью волны 5 (уже верифицированы там):
- N1: `blocked_on` невозможно очистить через proposal (нет clear-сентинела) — односторонний клапан новой burn-down метрики.
- N2: `hotam propose requirement -h` не работает (`-h` не в isBoolFlag → kind-сканер глотает `requirement`).
- N3: закоммиченные INDEX.md ссылаются на 2 untracked-файла (план волны 5 + чекпоинт) — dead links в свежем клоне.

## Решения по скоупу

- Пункты 1-6 приоритетов ревью — В РАБОТУ (задачи ниже).
- Пункт 7 (шум inspect: 249 кандидатов) — В РАБОТУ последней задачей волны (advisory-качество, не блокер).
- Пункт 8 (release/tag/binary) — ОТЛОЖЕН: resolver дважды явно отклонял тег в этой сессии; поднимем отдельно после закрытия P0/P1, решение за resolver.

## Задачи волны 6 (последовательно, /crush суб-агенты, коммит после каждой)

### T-a (#131, P0) — герметичность тестов от домашнего окружения
Тесты, полагающиеся на «изолированный» t.TempDir(), должны быть герметичны на ЛЮБОЙ машине. Направление: тестовый хелпер, создающий действительно изолированный корень (подавление маркер-поиска через HOTAM_SPEC_PROJECT_ROOT-инъекцию невозможно — тесты проверяют именно фейл резолвера; вместо этого — точка инъекции глубины/границы поиска или использование корня диска вне пользовательского профиля + явная проверка предусловия «вверх по дереву нет маркеров» с t.Skip и ЧЕСТНЫМ сообщением, если найдены). Плюс аудит всех тестов с той же уязвимостью. Критерий: обычный `go test ./...` зелёный на этой машине БЕЗ чистого TMP.

### T-b (#132) — закоммитить файлы, на которые ссылаются INDEX (N3)
Тривиально: `git add docs/reviews/2026-07-13-review5-response-plan.md docs/checkpoints/2026-07-13-1910.md` + этот план. Оркестратор делает сам, без суб-агента.

### T-c (#133, P1) — очистка stale-файлов при смене профиля
genSpec после записи удаляет generator-owned файлы в docs/gen/, не входящие в текущий written-набор (thinking/*.md, tools/*.md, atoms-*.md и пр. — только известные генератору категории, никогда чужие файлы). Тест: full→consumer реально сокращает диск до consumer-набора; consumer→full восстанавливает.

### T-d (#134, P1) — land автоматически обновляет root crystal
Если project root разрешается и на нём лежит CLAUDE.md (или .hotam-spec-project-маркер), land/propose --land регенерируют кристалл БЕЗ явного --claude-md. Флаг остаётся как override. Закрывает класс «graph свежий, crystal старый» навсегда.

### T-e (#135, P1) — профильные ссылки в кристалле + link-existence test
Кристалл consumer-профиля не должен ссылаться на thinking/ и прочие не-генерируемые в consumer пути. Тест: каждый относительный путь, упомянутый в сгенерированном кристалле, существует на диске (для обоих профилей).

### T-f (#136, P1) — унификация флагов + active-domain для apply-proposal/land
`--domain` везде = путь к целевому графу; `propose stakeholder` получает `--stakeholder-domain` для поля объекта (breaking для никем ещё не используемой команды — допустимо). apply-proposal/land переходят на resolveDomain (active-domain chain) вместо жёсткого требования --domain. Плюс подсказка N5: `--domain <имя>` без разделителя → «did you mean domains/<имя>?».

### T-g (#137, P1) — документация из registry: README/QUICKSTART/REPO-MAP
README: «fifteen»→актуальное число ИЗ registry или drift-тест на число; добавить --profile и confront --proposal. QUICKSTart-CONSUMER.md переписать через init-project. REPO-MAP tools-секцию — генерировать из methodology.Tools (repomap_data.go сам признаётся, что hand-maintained и уже дрейфанул).

### T-h (#138) — clear-сентинел для blocked_on (N1)
По образцу `<clear>` у enforced_by: явный способ очистить blocked_on через UPDATE-proposal, с валидацией и тестом (иначе burn-down метрика не замыкает жизненный цикл feature-blocked→closeable-now).

### T-i (#139) — починить -h/-help в propose (N2)
`h`/`help` в isBoolFlag (или учёт в kind-сканере propose); per-kind справка по флагам работает. Тест на `hotam propose requirement -h`.

### T-j (#140, P2) — сжать consumer REQUIREMENTS.md
В consumer-профиле: без 44 tool-derived requirements, без полной методологической энциклопедии — требования домена + краткий контракт + ссылки. Full-профиль без изменений.

### T-k (#141) — снизить шум inspect
249 кандидатов / 53 над порогом, почти все lexical. Направление: поднять вклад структурных эвристик в ранжирование, review порогов, возможно per-heuristic лимиты в дефолтном выводе. Advisory-инструмент — аккуратно, без потери реальных сигналов (регрессионный пин по known-conflict ground truth уже есть).

## Исполнение
Последовательные /crush суб-агенты (кроме T-b — сам оркестратор), независимая верификация после каждой задачи (build/vet/gofmt/test -race -count=1 БЕЗ чистого TMP после T-a — это станет новым критерием, all-violations оба домена, идемпотентность gen-spec), коммит после каждой, пуш и финальное @fl-ревью в конце по явной команде.
