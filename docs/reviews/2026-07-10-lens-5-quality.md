<!-- LEGACY (Python-era) audit/review — describes a point-in-time snapshot of the Python prototype (pytest/spec/tools references below are historical, not current instructions); see README.md for the current Go CLI. -->

# Аудит 2026-07-10 — линза 5: качество кода и технический долг

Независимый аудит D:\dev\HotamSpec. Линза: реальный техдолг (не архитектура, не позиционирование).
Прогон: `spec/.venv/Scripts/python.exe -m pytest -q` → **1237 passed, 2 skipped, 0 warnings, 86.17s**.
Побочное подтверждение: полный T2 оставил рабочее дерево чистым (tracked-файлы) — фиксы #99/#102 держат;
ранее «M» REPO-MAP.md обоих доменов вернулись к закоммиченному состоянию после идемпотентного regen.

Легенда серьёзности: **RISK** — реальный риск поведения/поддержки · **DEBT** — накопленный долг, не горит · **COSM** — косметика.

---

## 1. Статус тестов

| Находка | Где | Серьёзность | Усилие |
|---|---|---|---|
| 1237 passed, 2 skipped, ноль pytest-warnings (нет deprecation-шума) | полный прогон из `spec/` | зелёно — фиксировать нечего | — |
| 2 skipped — slow e2e (`test_portability_w4_smoke_e2e.py`, opt-in через env var) и/или guard-скипы; skip-by-default осознанный (`slow`-маркер задокументирован в conftest) | `spec/tests/conftest.py:96-100` | COSM | — |

## 2. Флаки-потенциал

| Находка | Где | Серьёзность | Усилие |
|---|---|---|---|
| `os.environ["HOTAM_SPEC_ACTIVE_DOMAIN"] = "hotam-dev"` восстанавливается через безусловный `pop()`, а НЕ через возврат прежнего значения. Если переменная была выставлена снаружи (shell разработчика, CI, соседний фикстур), она молча удаляется → порядкозависимое поведение последующих тестов. Все остальные файлы сюиты (conftest.py:187-198, test_invariants.py:1025-1032, test_dependency_traversal.py:141-148, test_claude_md_template.py:41-50, test_gen_spec_idempotency.py:45-56) корректно сохраняют/восстанавливают prev. Правильный инструмент — `monkeypatch.setenv` (уже используется в test_portability_*). | `spec/tests/test_root_crystal_follows_pin.py:158-162` | RISK (мягкий: изоляция от внешней среды) | 5 мин |
| `time.sleep(0.05)` для гарантии разницы mtime двух файлов. На NTFS (100 нс) и ext4 надёжно; на файловых системах с грубым mtime (FAT, некоторые CI-контейнеры, 1-2 с) 50 мс может не хватить → флак. Надёжнее выставить mtime явно через `os.utime`. | `spec/tests/test_pending_proposal_archive.py:88` | DEBT (низкая вероятность) | 10 мин |
| `date.today()` + относительные дельты (−30/−15/−5 дней) в тестах decay-логики: детерминированы относительно дня запуска; теоретический флак только при пересечении полуночи между генерацией фикстуры и assert'ом — пренебрежимо. | `spec/tests/test_timestamps_and_implements_decay.py:111-365`, `test_reflection.py:474` | COSM | — |
| Module-level кэши в src: `_CONTENT_GRAPH_CACHE` (ключ — resolved-путь graph.py, есть явный тест-хук `_load_content_graph_cache_clear()`) и `lru_cache` в enforcer_resolution (ключ — tests_dir, кросс-процессная инвалидация по mtime-fingerprint). Дизайн осознанный и задокументированный. Единственная щель: `_test_scan_cached` считает fingerprint ОДИН раз на процесс на директорию — тест, меняющий файлы в той же tests-директории внутри процесса, увидит устаревший скан. Сегодня тесты используют tmp-директории (другой ключ), так что латентно. | `spec/src/hotam_spec/graph.py:331-383`; `spec/src/hotam_spec/enforcer_resolution.py:296-333` | DEBT (латентный, задокументирован) | наблюдать; фикс не требуется |
| Session-фикстура `gen_spec_snapshot` делает `os.chdir` + мутацию env с корректным try/finally-восстановлением — чисто, но несовместимо с pytest-xdist (не используется — ок). | `spec/tests/conftest.py:172-215` | COSM | — |

## 3. Шесть closeable-debt правил (UNENFORCED.md) — реальная цена закрытия

| Правило | Оценка | Скрытая сложность | Усилие |
|---|---|---|---|
| `R-crystal-carries-short-form` (STRUCTURAL) | **Enforcer уже написан**: `spec/tests/test_short_form.py` покрывает ровно claim (no-mid-word-cutoff, summary-приоритет, первое предложение). Закрытие = proposal: enforcement→ENFORCED, enforced_by=test_short_form.py. | нет | ~15 мин |
| `R-delegation-is-a-file` (STRUCTURAL) | **Enforcer уже написан**: `spec/tests/test_tool_delegate.py`, докстринг прямо говорит «Enforcer/lift for R-delegation-is-a-file». Более того, R-trust-anchor-delegation-explicit-only уже ССЫЛАЕТСЯ на этот файл как на shared enforcer (graph.py:3442) — рассинхрон: правило-донор enforcer'а само числится STRUCTURAL. | нет | ~15 мин |
| `R-project-root-not-hardcoded` (STRUCTURAL) | `project_paths.project_root()` существует (`spec/src/hotam_spec/project_paths.py:237`), `test_project_paths.py` — 32 теста. Для честного ENFORCED стоит добавить синтаксический скан «нет `parents[N]` вне repo_paths/project_paths» (в духе R-machine-check-syntactic) — сейчас `grep parents[` по src чист вне этих модулей. | малая | ~1 ч |
| `R-framework-suite-domain-independent` (PROSE) | Инфраструктура готова (маркеры framework/domain, реестр DOMAIN_COUPLED в conftest.py:58-83), но честный enforcer = subprocess-прогон `pytest -m framework` под чужим пином/без пина = +1 прогон сюиты (~90 с) → конфликт с R-run-speed-guarded. Реалистично: slow-маркер (skip-by-default) или CI-job. | **да** (стоимость прогона) | 0.5-1 день |
| `R-work-within-launch-dir` (PROSE) | Кодом проверяем только синтаксически: скан tools/src на записи в `Path.home()`/`~/.claude`. Семантика («оператор не мутирует вне launch-dir») — поведение агента, не кода; риск form-metric-театра, от которого проект сам отказался (прецедент R-boot-cite-measured REJECTED). Кандидат либо на частичный syntactic-check, либо на реклассификацию INHERENTLY_PROSE. | **да** (граница честности) | решение стьюарда + ~2 ч |
| `R-conflict-resolved-in-members-or-mediator` (PROSE) | «Резолюция только in-graph» — семантика решения, машинно проверяется лишь суррогат (у DECIDED есть derived/amendment — частично уже покрыто check_decided_has_rationale_or_derived). Честного check_* без театра нет; кандидат на INHERENTLY_PROSE. | **да** | решение стьюарда |

**Итог по п.3: два из шести закрываются почти бесплатно (тесты уже существуют — это не долг, а незакрытая бухгалтерия), третье — дёшево; три оставшихся требуют решения стьюарда о границе честности, а не кода.**

## 4. Зависимости (spec/pyproject.toml)

| Находка | Где | Серьёзность | Усилие |
|---|---|---|---|
| Runtime `dependencies = []` — stdlib-only. Образцово (соответствует R-core-imports-stdlib-or-hotam-spec-only). | pyproject.toml:11 | зелёно | — |
| `cosmic-ray>=8.4.6` и `z3-solver>=4.16.0.0` объявлены и **не используются** (сам комментарий это признаёт: «listed now but UNUSED — DEFERRED formal layers»). z3-solver — тяжёлое колесо (десятки МБ) в каждом dev-окружении + лишняя supply-chain-поверхность. Дешевле убрать и вернуть при включении слоёв 4-8. | pyproject.toml:48-59 | DEBT | 15 мин + решение стьюарда (заявлены сознательно) |
| Версии dev-стека (pytest>=9.0.3, ruff>=0.15.16, hypothesis>=6.155.2) — актуальной эпохи, нижние границы, не устаревшие. | pyproject.toml:53-58 | зелёно | — |
| build-backend `uv_build` — единственная СТРУКТУРНАЯ привязка к uv. `pip install` всё же работает (uv_build — PEP 517-бэкенд, ставится с PyPI), но сборка колеса завязана на экосистему uv. Если линия «uv — опция, не требование» принципиальна, hatchling/flit снимут привязку. | pyproject.toml:44-46 | COSM→DEBT | ~30 мин при желании |

## 5. Консистентность uv vs python

| Находка | Где | Серьёзность | Усилие |
|---|---|---|---|
| **Программных вызовов uv нет нигде** (tools/, src/, тесты — чисто); хуки .claude/settings.json корректно предпочитают `.venv` и падают на uv только как fallback. Требования uv в рантайме НЕТ. | .claude/settings.json:8-57 | зелёно | — |
| Но документация подаёт uv как ЕДИНСТВЕННУЮ форму команды: README.md (~20 мест: 54-200), CONTRIBUTING.md:10-63, .github/PULL_REQUEST_TEMPLATE.md:12-14, spec/README.md:16-19, domains/hotam-dev/CLAUDE.md:12, domains/hotam-dev/agents/director/README.md:16 — везде `uv run ...` без альтернативы `python ...`. Корневой CLAUDE.md при этом использует чистый `python tools/...` — рассинхрон двух канонов. | перечисленные файлы | DEBT (противоречит линии стьюарда) | ~1-2 ч |
| Источник части рассинхрона — докстринги 17 инструментов (`tools/apply_proposal.py`, `attention.py`, `attention_hook.py`, `audit_atomicity.py`, `audit_tensions.py`, `confront.py`, `create_axis.py`, `gate.py`, `gate_status.py`, `gen_spec.py`, `invoke_agent.py`, `land.py`, `mark_revisit_evaluated.py`, `setup_hooks.py`, `spawn_agent.py`, `spawn_log_isolation_status.py`, `what_now.py`) содержат `uv run python tools/...` в Usage-примерах, откуда генерятся spec/docs/tools/*.md. Правка = sweep докстрингов + `gen_spec.py` regen. | spec/tools/*.py (17 файлов) | DEBT | входит в ~1-2 ч выше |

## 6. .gitignore / закоммиченный runtime

| Находка | Где | Серьёзность | Усилие |
|---|---|---|---|
| `git ls-files` не находит НИ ОДНОГО файла из `.runtime/`, `.pytest_cache/`, `.hypothesis/`, `__pycache__/`, `.venv/`. Двухуровневый .gitignore (корень + spec/) покрывает всё, включая `.hotam-spec/` и `.claude/*.bak-*`. | .gitignore, spec/.gitignore | зелёно | — |

## 7. Code smells: длинные функции, монолиты, magic numbers

| Находка | Где | Серьёзность | Усилие |
|---|---|---|---|
| **Монолитные модули**: `tools/gen_spec.py` — 5003 строки, `src/hotam_spec/invariants.py` — 4482, `tools/apply_proposal.py` — 3565. Это главный структурный долг кодовой базы: три файла держат ~14k строк. Смягчается 1237 тестами и atomicity-ratchet, но каждый merge/refactor в них дорог. | указанные файлы | DEBT (основной) | распил — дни; не срочно |
| Функции >150 строк: `apply_proposal.apply` — **333 строки** (tools/apply_proposal.py:3102), `gen_spec.build_constitution` — 249 (tools/gen_spec.py:1536), `_apply_requirement_to_source` — 158 (tools/apply_proposal.py:1443), `attention.diagnose_signals` — 156 (src/hotam_spec/attention.py:152), `spawn_agent.main` — 152 (tools/spawn_agent.py:212), `gen_spec.main` — 144 (tools/gen_spec.py:4856). `apply()` на 333 строки — худший кандидат: это транзакционное ядро LAND-шага (write→regen→gate→verify→archive), ошибки в нём дороги. | см. файл:строка | DEBT | apply(): ~0.5-1 день на выделение фаз |
| Magic numbers в основном задокументированы на месте: `_FULL_SUITE_THRESHOLD = 100` (conftest.py:247, с комментарием), baseline `×1.2` (conftest.py:304, «+20%» из R-run-speed-guarded), `max_chars` в short_form. Существенных необъяснённых констант не найдено. | — | COSM | — |

---

## Приоритет починки (по соотношению риск/усилие)

1. **5 мин**: test_root_crystal_follows_pin.py:158 — перейти на `monkeypatch.setenv` (изоляция от внешнего env).
2. **~30 мин**: закрыть R-crystal-carries-short-form и R-delegation-is-a-file (enforcers уже написаны — чистая бухгалтерия, −2 к closeable debt, гасит часть P0-пульса).
3. **~1 ч**: R-project-root-not-hardcoded — syntactic-check + флип (−3-й пункт долга).
4. **~1-2 ч**: uv-sweep — докстринги 17 tools + README/CONTRIBUTING/PR-template: везде primary `python`, uv — как опция.
5. **15 мин + решение стьюарда**: выкинуть cosmic-ray/z3-solver из dev-группы до реальной надобности.
6. **10 мин**: os.utime вместо time.sleep(0.05) в test_pending_proposal_archive.py.
7. **Решение стьюарда, не код**: R-work-within-launch-dir и R-conflict-resolved-in-members-or-mediator — честнее реклассифицировать (или частичный syntactic-check), чем изображать enforcement; R-framework-suite-domain-independent — slow-тест/CI-job.
8. **Фон, не срочно**: план распила apply() (333 строки) и трёх модулей-монолитов.
