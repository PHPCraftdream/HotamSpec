# Внешнее ревью 2026-07-10 — CONFRONT-разбор и план улучшений

Источник: внешнее продуктовое ревью (передано стьюардом). Все ключевые факты
проверены координатором против репозитория ПЕРЕД составлением плана — все
проверенные утверждения подтвердились дословно, включая номера строк.

## CONFRONT-классификация

### A. Подтверждено и НОВОЕ (пять линз 2026-07-10 это пропустили)

1. **P0 упаковка**: `spec/src/hotam_spec/cli/_path_setup.py` сам признаёт —
   «In a pip-installed-consumer scenario, tools/ is NOT shipped... CLI wrappers
   are best-effort». QUICKSTART обещает 26 команд после `pip install
   hotam-spec` — для чистого (не-editable) wheel это не работает. Линзы не
   поймали, потому что e2e ставит пакет через `pip install -e`.
2. **E2E не покрывает `hotam-apply-proposal`** — ноль упоминаний в
   `test_e2e_consumer_subprocess.py`; центральная операция не проверена.
3. **Rollback отсутствует** — `apply_proposal.py` печатает «auto-revert is not
   implemented in P3. Inspect the diff and revert manually».
4. **Verify consumer-записи гоняет тесты фреймворка** (`cwd=_SPEC_ROOT`) —
   проверяется Hotam-Spec, а не граф потребителя.
5. **Freshness-поля мертвы**: `created_at` — 2/270, `settled_at` — 15/228;
   полей `last_reviewed_at`/`evidence`/`source_refs` нет; сигналов устаревания
   от дат нет.
6. **Байт-детерминизм ≠ смысловая актуальность**: `REQUIREMENTS.md:6` до сих
   пор говорит `spec/content/graph.py`; источник — захардкоженные строки
   `gen_spec.py:8,415,497,680` и docstring `__init__.py:70`.
7. **UX-пробел**: нет show/list/search/patch/context; апдейт одного поля
   требует пересылки полного объекта (испытано нами же в Этапе C).

### B. Подтверждено, но УЖЕ сделано/заякорено

- Позиционирование «делает противоречия явными» — сделано (J1, `b2c58c8`).
- A-ontology-transfers UNCERTAIN — наш узел (Этап C); пилоты = решение
  стьюарда, отложено.
- confront=лексический — честно задокументировано (R-machine-check-syntactic);
  `machine_check` — seam для будущего семантического слоя.
- Экспериментальные пометки типов — сделано (Этап B).
- --batch N-кратный regen — починено (Этап F).

### C. Конфликтует с ранее принятыми решениями → только PRESENT стьюарду

1. **Per-node JSON хранилище** ↔ отклонённый `R-rdf-store` + предпосылка
   «the store IS the Python code» (R-rules-as-data [E],
   R-crystallize-knowledge-to-code). Аргументы ревью реальны (AST-хрупкость —
   Этап F, отсутствие partial-patch — Этап C), но смена станового хребта —
   кандидат в Conflict-узел (ось executable-store-vs-declarative-store),
   решение только стьюарда.
2. **Минимальное ядро 5 понятий / плагины** ≈ D1-рескоуп (дважды отложен,
   уже в «Вопросах стьюарду» синтез-плана).

### D. Мелкие неточности ревью

«42 tool-модуля» включает `_private`-хелперы (публичных ~30 после Этапа D);
оценки N/10 субъективны, но согласуются с нашими линзами.

---

## План улучшений — Этапы G-L

Методология исполнения (установлена в сессии): sx-агент исполняет →
координатор ЛИЧНО верифицирует (полный T2 + `git status` чист после прогона +
поведенческая проверка + детерминизм regen при затронутом gen_spec) → коммит
координатором. Ревью oh/fh — по рискованности этапа. Пуш — только по явному
слову стьюарда. `domains/*/graph.py` — только через apply_proposal.py.

### Этап G — [P0] Wheel: упаковка + чистый quickstart с нуля

Источник: внешнее ревью #1. Цель: `pip install hotam-spec` из НАСТОЯЩЕГО
wheel (не editable) даёт работающие команды.

1. Решить механику упаковки tools/ в wheel: (a) force-include spec/tools как
   package-data / подпакет `hotam_spec._tools`, (b) перенос тел инструментов в
   пакет с tools/ как тонкими шимами, (c) иное — агент исследует, предлагает
   минимально-инвазивный вариант, честно останавливается если требуется
   архитектурное решение стьюарда. ВНИМАНИЕ: build-backend сейчас `uv_build` —
   его замена лежит в «Вопросах стьюарду»; если вариант упаковки возможен без
   смены бэкенда — предпочесть его.
2. Собрать wheel, поставить в чистый venv, пройти ВЕСЬ
   docs/QUICKSTART-CONSUMER.md буквально (create-domain → what-now →
   stakeholders → axis → requirements → conflict → pulse).
3. Оформить как гейтированный e2e-тест (по прецеденту
   HOTAM_SPEC_RUN_E2E_SUBPROCESS), проверяющий wheel-путь, не editable.

### Этап H — [P0] Consumer write-path: e2e apply-proposal + rollback + in-process check

Источник: внешнее ревью #2,#3. blockedBy: G (общий e2e-файл).

1. Расширить consumer-e2e: создание Requirement, апдейт (тот же id),
   Rejection, создание Conflict, отказ невалидного proposal (все через
   hotam-apply-proposal сабпроцессом).
2. Атомарный write-path: запись во временный файл + верификация → swap;
   либо backup+auto-restore при красном verify. Убрать «revert manually».
3. Для CONSUMER-графов (не self-hosting) заменить pytest-фреймворка на
   прямой in-process invariant check (`all_violations()` / conscience) —
   framework-тесты для чужого графа проверяют не то (ревью #2). Self-hosting
   LAND-гейт (T1/T2 pytest) НЕ трогать — он проверяет как раз то.
   Честная остановка, если выяснится конфликт с R-verify-closure-per-action /
   R-land-* правилами — тогда PRESENT вместо кода.

### Этап I — [P1] Stale-refs sweep + хвосты fh-ревью (F-1/F-2/F-3)

Источник: внешнее ревью #6 + docs/reviews/2026-07-10-fh-final-review-etap-a-f.md.
blockedBy: H (пересечение по apply_proposal.py).

1. Захардкоженные `spec/content/graph.py` в gen_spec.py (:8,:415,:497,:680) и
   `__init__.py:70` → актуальные `domains/<name>/graph.py` формулировки →
   regen; затем repo-wide grep spec/content + spec/agents на прочие стейлы
   (кроме легитимных legacy-fallback упоминаний в резолвере — их пометить
   «legacy fallback» явно, не удалять).
2. fh F-1: `--batch` tier-selection — форсировать T2, если ХОТЬ ОДИН элемент
   батча был созданием нового узла (или всегда T2 для --batch — выбрать и
   обосновать).
3. fh F-2: мёртвая строка test_apply_proposal_batch_stress.py:350.
4. fh F-3: смягчить «fails fast» для не-tuple RHS в PROPOSAL-REFERENCE.md
   (реально ловится позже, на regen/verify).

### Этап J — [P1] CLI UX: show / list / search / patch / context

Источник: внешнее ревью #4 (+ «Контекст для агента»). blockedBy: I.

1. Новый диспетчер (рабочее имя `hotam-req` или расширение review.py-прецедента):
   `list [--status S] [--owner O]`, `show R-x`, `search "текст"` (по
   claim/why/id, из графа, не из markdown).
2. `patch R-x --field value [...]`: читает текущий узел из графа, мержит
   изменения, генерирует ПОЛНЫЙ Requirement-proposal внутри и прогоняет через
   существующий apply_proposal-механизм — чистый UX-сахар, БЕЗ нового
   proposal-kind, БЕЗ изменения онтологии.
3. `context R-x [--depth N] [--json]`: требование + owner + assumptions +
   relations (обе стороны) + конфликты-членства + replaces-цепочка + enforcement.
   JSON-режим — для агентов (дешевле, чем 112-КБ REQUIREMENTS.md).
4. Тесты на каждую подкоманду; pip entry-point.

### Этап K — [P1-P2] Perf top-5 из perf-отчёта

Источник: docs/reviews/2026-07-10-perf-investigation.md. blockedBy: I
(пересечение gen_spec.py/apply_proposal.py).

1. gen_spec: тройной `diagnose()` → один (синхронизировать два кэша графа);
   двойные build_shared_*_docs вызовы → один. Цель ~3.9s → ~2.1s.
2. Suite setup: autouse-фикстура тянет tmp_path во все 1279+ тестов (~11s) —
   сузить scope. Цель −8-10s на прогон.
3. LAND: пропускать первый gen_spec-проход (--docs-only) когда
   applied==pinned. Цель −1.8s на каждый LAND.
4. Гард на запись в `.runtime/enforcer-index.json` (уже загрязнён
   tmpdir-записями) + починка fingerprint-инвалидации.
5. После каждого пункта — замер до/после в отчёте; R-run-speed-guarded
   пересчитtakes baseline санкционированным путём, если нужно.

### Этап L — [PRESENT] Триаж-вопросы стьюарду (разговор, НЕ делегируется)

Копятся из синтез-плана + внешнего ревью:

1. **Per-node JSON хранилище** (внешнее ревью, конфликт с R-rdf-store) —
   materialize как Conflict-узел или отклонить с записью?
2. **Ядро/плагины** (минимальное ядро 5 понятий; D1-рескоуп эскалация).
3. **Freshness-дизайн** (внешнее ревью #5): поля last_reviewed_at /
   review_after / evidence / source_refs — расширять ли онтологию Requirement;
   backfill created_at/settled_at из git-истории (мутация всех 270 узлов —
   нужна санкция).
4. build-backend `uv_build` → нейтральный (влияет на Этап G!).
5. 2 PROSE-правила → INHERENTLY_PROSE (lens-5).
6. Эскалация отложенных D3 (бинарный enforcement) и E1 (компактный индекс).

Тема пилотов/реальных внешних систем (ревью P3-10) сознательно исключена из
триажа по явному слову стьюарда 2026-07-10 («сам решу когда пора») — не
поднимается, пока стьюард не поднимет сам.

## Зависимости

```
G (wheel) → H (write-path) → I (stale+fh) → J (CLI UX)
                                         → K (perf)
L (PRESENT) — независим, по готовности стьюарда; п.4 влияет на G
```

## Что сознательно НЕ вошло

- Миграция на per-node хранилище и распил монолитов — только через Этап L
  (конфликт с прошлыми решениями, анти-релитигация).
- Семантический conflict-detection слой — после L/фидбэка пилотов (seam
  `machine_check` уже существует).
- Пилоты на внешних системах — решение стьюарда, вне плана.
