<!-- LEGACY (Python-era) audit/review — describes a point-in-time snapshot of the Python prototype (pytest/spec/tools references below are historical, not current instructions); see README.md for the current Go CLI. -->

# Lens 3 — Usability / cold-start аудит (2026-07-10)

Роль: разработчик, впервые открывший репозиторий. Не знаю жаргона R-/C-/A-/OP-,
не читал внутренних доков. Сужу только по README.md, docs/QUICKSTART-CONSUMER.md,
docs/PROPOSAL-REFERENCE.md, сверяя каждую команду с кодом.

Вердикт: путь новичка **заметно лучше, чем типичный** для проекта такой плотности.
За ~2 минуты README отвечает "что это / зачем / кому"; тэглайн, глоссарий из 6
терминов и явная граница "всё про operators/crystals — внутренняя машинерия,
пропусти" — работают. Захардкоженного бейджа "944 passing" больше нет (проверено:
в README только license/python бейджи; строка 203 честно отсылает к `pytest -q`
за актуальным числом). QUICKSTART-CONSUMER сверен с кодом команда за командой —
фактических ошибок не найдено. Оставшееся трение — ниже, по убыванию тяжести.

---

## Находки

### F1. [СРЕДНЕЕ] README Quick start требует `uv`, нигде не заявленный как prerequisite

- **Где:** README.md:51 (`cd spec && uv sync`), :54, :57, :60, :63, :87, :92,
  :107 и далее — 19 вхождений `uv run`/`uv sync`; CONTRIBUTING.md:8,10,11,21,63 — то же.
- **Что смутило:** бейджи обещают только "Python 3.12+". Новичок без установленного
  `uv` падает на **первой же команде** установки (~2-я минута чтения). Альтернатива
  `pip install -e spec` в README не упомянута вовсе (она есть только в
  QUICKSTART-CONSUMER, до которого self-hosting-читатель не дойдёт — README ведёт
  туда только "если ставите в свой отдельный репозиторий").
- **Отвал:** минута ~2, шаг 1 Quick start.
- **Фикс:** либо перевести README/CONTRIBUTING на `pip install -e .` +
  `python -m pytest -q` / `python tools/what_now.py` (deps проекта пустые,
  dev-стек ставится одной строкой), либо хотя бы добавить строку-prerequisite
  "requires [uv](https://docs.astral.sh/uv/)" + pip-эквивалент рядом. Замечание
  усиливается тем, что стьюард ранее просил убрать зависимость от uv.
- **Смежное:** `spec/pyproject.toml:45` — build-backend `uv_build`; даже чистый
  `pip install -e spec` тянет uv_build с PyPI. Работает, но семейство uv остаётся
  в потребительском пути. Рассмотреть hatchling/setuptools, если курс "без uv".

### F2. [СРЕДНЕЕ] "Regenerate the crystal" — AI-жаргон внутри секции "no AI agent needed"

- **Где:** README.md:124 (`# 7. regenerate the crystal and read your pulse.`)
  внутри секции "Required for any team (no AI agent needed)" (README.md:80).
- **Что смутило:** README.md:21-24 только что пообещал: "operators, crystals,
  context budgets, spawn logs — internal machinery … skip it". Через 100 строк
  обязательный шаг 7 человеческого happy path говорит "regenerate the crystal".
  Читатель, честно пропустивший AI-раздел, не знает слова.
- **Отвал:** нет (команда скопируется и так), но доверие к границе
  "human-only vs AI-only" подрывается.
- **Фикс:** переформулировать комментарий: `# 7. regenerate docs and read your
  pulse.` — `gen_spec.py` и так регенерирует docs/gen/; слово crystal тут не нужно.

### F3. [СРЕДНЕЕ] PROPOSAL-REFERENCE: enum-ы названы, но не расшифрованы — JSON "с нуля" пишется, но вслепую

Сверка с кодом: все 11 kind-строк совпадают с диспетчером
`spec/tools/apply_proposal.py:264-291`; required/optional поля Stakeholder /
Axis / Requirement / Conflict совпадают с `_validate_*` (в т.ч. `why` optional —
подтверждено apply_proposal.py:377). Т.е. **фактических ошибок нет**, по
референсу реально написать валидный JSON. Но три словаря значений отсутствуют:

- **PROPOSAL-REFERENCE.md:54** — `status` у Requirement: допустимые значения
  нигде не перечислены. Код (`spec/src/hotam_spec/proposal.py:35`) говорит
  `DRAFT | SETTLED | OPEN(question)`; REJECTED — только через kind Rejection.
  Новичок этого знать не может.
- **PROPOSAL-REFERENCE.md:57** — `enforcement` `PROSE | STRUCTURAL | ENFORCED`:
  значения перечислены, семантика — нет (что значит STRUCTURAL? когда выбирать?).
  Одна строка глоссы на каждое сняла бы вопрос.
- **PROPOSAL-REFERENCE.md:58-59** — `enforceability` "(default `"ENFORCEABLE"`)":
  второе допустимое значение (`INHERENTLY_PROSE`,
  `spec/src/hotam_spec/requirement.py:67`) не названо вообще.
- **PROPOSAL-REFERENCE.md:57** — `relations` "list of `[kind, target]` pairs":
  словарь допустимых kind-ов связей не приведён и нет ссылки, где его искать.

**Фикс:** добавить в референс мини-таблицу enum-ов (status, enforcement,
enforceability, relation kinds) с одной строкой семантики на значение.

### F4. [НИЗКОЕ] Первый R-анкер появляется без легенды нотации

- **Где:** README.md:71-72 — "(`R-no-hand-edit-graph`, enforced by a committed
  PreToolUse guard)"; далее README.md:138 `R-ai-presents-not-decides`.
- **Что смутило:** читатель ещё не знает, что `R-…` — это id требования в
  графе самого фреймворка. Догадаться можно, но это первый момент
  "тут внутренний жаргон".
- **Фикс:** при первом употреблении: "…(rule `R-no-hand-edit-graph` — rules are
  themselves Requirements in the self-hosting graph, hence the `R-` ids)".

### F5. [НИЗКОЕ] QUICKSTART-CONSUMER: мелкие шероховатости (команды верны)

Сверено с кодом — всё главное честно:
- "26 `hotam-*` console scripts" (QUICKSTART-CONSUMER.md:25) — ровно 26 в
  `spec/pyproject.toml:17-42`. Совпадает.
- Флаги `hotam-create-domain` (:40-44) совпадают с
  `spec/tools/create_domain.py:308-343` (все три флага действительно required —
  проверка на :337-343). `hotam-create-axis` (:79) совпадает с
  `spec/tools/create_axis.py:192-203`.
- Секция "How project root is found (R1-R6)" (:100-143) дословно совпадает с
  `spec/src/hotam_spec/project_paths.py` (R1/R2 env, R3 два яруса маркеров с
  порогом 2 для secondary, R4 `.hotam-spec-project`, R5 pyproject, R6 fallback).
  Починка шага 2 (`touch .hotam-spec-project` вместо `git init`) корректна:
  git-каталог маркером не является.

Шероховатости:
- **:32 vs :35** — заголовок говорит "run inside **your own project
  directory**", а первая команда — `mkdir my-project && cd my-project`
  (создание нового). Читатель с существующим проектом на секунду спотыкается:
  mkdir мне нужен или нет? Фикс: пометить mkdir как "(or cd into your existing
  project)".
- **:147** — "`docs/PROPOSAL-REFERENCE.md`" в What's next дан plain-текстом и с
  путём от корня, хотя файл — сосед; на :98 та же ссылка оформлена правильно.
  Фикс: сделать markdown-ссылкой `[PROPOSAL-REFERENCE.md](PROPOSAL-REFERENCE.md)`.
- **:103-126** — у R3/R4 в коде лимит подъёма 5 уровней
  (`project_paths.py: MAX_MARKER_SEARCH_DEPTH = 5`), в доке — "searched upward"
  без лимита. В глубоком monorepo команда "не найдёт" root, а читатель будет
  уверен, что должна. Фикс: упомянуть "(up to 5 levels up)".
- **:82-83** — `"enforcement":"PROSE"` появляется в JSON без объяснения
  (та же дыра, что F3).

### F6. [ИНФО] Что уже хорошо (фиксировать, чтобы не сломали)

- Тэглайны согласованы во всех трёх точках: README.md:6, CLAUDE.md:3,
  spec/pyproject.toml:4 — везде "executable memory and discipline for a
  human + LLM-agent fleet", противоречивые требования поданы как
  "one of its properties" (README.md:10). Рассинхрона нет.
- Битых внутренних ссылок между README / QUICKSTART-CONSUMER /
  PROPOSAL-REFERENCE / CONTRIBUTING / LICENSE* не найдено (все цели существуют).
- Happy path за ~5 минут без AI-концепций существует и явно размечен:
  QUICKSTART-CONSUMER целиком CLI-only (заявлено на :10-11), README разделяет
  "Required for any team" / "Optional: AI operator". Единственная утечка
  AI-жаргона в human-путь — F2.
- Терминологический барьер до первого действия: 6 терминов глоссария — все
  реально нужны для шагов 4a-4e. Это близко к минимуму; сверх него до первого
  действия протекают только enforcement-enum (F3) и "crystal" (F2).

## Сводка отвалов

| Момент | Кто отвалится |
|---|---|
| README Quick start, `uv sync` (~2-я минута) | новичок без uv — жёсткий стоп (F1) |
| Шаг 5 README / шаг 4c QUICKSTART, `"enforcement":"PROSE"` | никто не отвалится, но поле заполняется наугад (F3/F5) |
| PROPOSAL-REFERENCE, `status` без словаря значений | тот, кто пишет Requirement-JSON не по образцу из quickstart (F3) |

Приоритет исправлений: F1 → F3 → F2 → F5 → F4.
