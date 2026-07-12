<!-- LEGACY (Python-era) audit/review — describes a point-in-time snapshot of the Python prototype (pytest/spec/tools references below are historical, not current instructions); see README.md for the current Go CLI. -->

# Линза 1 — Избыточность и дублирование

Метрики базы: `spec/tools/` — 40 файлов, 15 523 строк; `spec/src/hotam_spec/` — ~11 100 строк (из них `invariants.py` — 4 482, 88 `check_*`); `spec/tests/` — 116 файлов, 22 926 строк; `spec/src/hotam_spec/cli/` — 31 файл, 625 строк; docs: `spec/docs/thinking/` 265K + `spec/docs/tools/` 156K + `domains/*/docs/gen/` 433K.

---

## 1. Две параллельные системы делегаций — ВЫСОКАЯ

- `spec/tools/record_delegation.py` (281 строка) пишет в `domains/hotam-spec-self/delegations.jsonl` (файл существует).
- `spec/tools/delegate.py` (138 строк) + `spec/tools/_delegation_store.py` (178 строк) пишут `delegations/DG-*.md` (4 файла существуют).

Вторая система (файловая, R-delegation-is-a-file, commit ed1dbab) явно ПРИНЯТА стьюардом взамен «делегация как нода», но jsonl-реестр (`R-trust-anchor-delegation-explicit-only`) не выведен из эксплуатации — оба живы, оба с CLI-обёртками (`cli/delegation.py`, `cli/record_delegation.py`) и тестами (`test_tool_record_delegation.py`, `test_delegation_marker_honesty.py`). Это буквально пример из RECENTLY-REJECTED, где отвергнутый механизм не убран, а сосуществует. **Рекомендация:** мигрировать trust-anchor-записи в frontmatter DG-файлов, удалить record_delegation.py + jsonl + ~460 строк кода и тестов.

## 2. Три baseline-механизма с тремя копиями логики — СРЕДНЯЯ (частично уже консолидировано)

`spec/tests/enforcement_perimeter_baseline.json`, `frozen_aspects_baseline.json`, `atomicity_compound_baseline.json` — два первых семантически идентичны: «sha256-пины набора файлов, менять только через update_baseline.py». Разница только в списке файлов и в правиле-обосновании (R-enforcement-perimeter-visible vs R-speculative-aspects-frozen). `spec/tools/update_baseline.py:47-72` (`_update_frozen_aspects`) и его enforcement-perimeter-близнец — дублированный код рехеша; плюс два отдельных теста (`test_enforcement_perimeter_pinned.py`, `test_frozen_aspects_snapshot.py`) с одинаковой механикой сверки. **Рекомендация:** один `protected_baselines.json` с секциями `{name: {rule, files{path:hash}}}` и одна параметризованная функция рехеша/проверки; atomicity-baseline (не хеши, а множества имён) можно оставить отдельным, но он уже пуст (`invariants: [check_method_matches_docstring]`, `requirements: []`) — ratchet почти выродился в «список из одного исключения».

Отдельный запах: `_comment` в `frozen_aspects_baseline.json` — ~2 500 символов истории разморозок в JSON-строке. Это changelog, живущий в поле комментария; ему место в git-истории или HISTORY.md.

## 3. Замороженные модули — мёртвый груз с живой стоимостью — СРЕДНЯЯ

- `spec/src/hotam_spec/entity.py` — 12 checks + 6 тестов + `create_entity_type.py` (286 строк) + `test_entity*.py` (3 файла), при этом `domains/hotam-spec-self/docs/gen/ENTITIES.md` — **8 строк** (пусто): в самохостящемся домене нет ни одной сущности. Вся Entity-подсистема (~1 000+ строк кода и тестов) работает вхолостую, но каждый T2-прогон гоняет её тесты (и run-speed guard её оплачивает).
- Federation/spawn recursion: `create_agent.py` (232), `invoke_agent.py` (176), `spawn_agent.py` (367), `create_domain.py` (353) заморожены хешами, но при этом frozen_aspects_baseline трижды «частично размораживался» за неделю (Wave 5, 10, 13, W2, W4) — заморозка не защищает, а лишь добавляет церемонию `update_baseline.py` к каждому реальному изменению. Agent Map: «no sub-operators yet» — рекурсия агентов не использована ни разу.

**Рекомендация:** признать frozen-модули «attic»: вынести entity/federation-тесты в отдельный pytest-маркер, исключить из T1 и из run-speed baseline; либо честно удалить Entity до появления первого потребителя (git всё хранит).

## 4. tools/ vs cli/ — двойная бухгалтерия, но дешёвая — НИЗКАЯ

31 обёртка в `spec/src/hotam_spec/cli/` (625 строк) — чистые ре-экспорты `main()` ради pip entry-points, логика не дублируется. Реальная цена — обязательство создавать обёртку на каждый новый тул (и сейчас уже есть рассинхрон имён: `cli/mark_revisit.py` vs `tools/mark_revisit_evaluated.py`, `cli/ticket.py` vs пять `tools/ticket_*.py`). **Рекомендация:** генерировать entry-points из таблицы в pyproject + один шаблонный модуль-диспетчер вместо 31 файла.

## 5. Пульс-конвейер с обратным парсингом собственного вывода — СРЕДНЯЯ

Цепочка: граф → `gen_spec.py` рендерит LIVE-STATE в CLAUDE.md → `emit_cipher.py:1` **регулярками парсит обратно** свой же сгенерированный markdown, чтобы извлечь три цифры для хука. Параллельно `what_now.py` (635 строк) считает те же приоритеты напрямую из графа, `tick.py` (179 строк) — обёртка, вызывающая `what_now.diagnose` (tick.py:34,76) и добавляющая только «render report», а `attention.py`-core объявлен супернадмножеством diagnose. Четыре входа в один и тот же ответ «что сейчас важно»: what_now (CLI), tick (advisory-обёртка), attention/attention_hook, emit_cipher (regex по markdown). **Рекомендация:** emit_cipher должен звать `attention.collect`/`diagnose` напрямую (или gen_spec должен писать machine-readable `live-state.json` рядом с markdown); tick.py — кандидат на слияние в `what_now --report`.

## 6. Тесты: 92 файла с одинаковым sys.path-бойлерплейтом и неслитые паттерны — СРЕДНЯЯ

- 92 из 116 тест-файлов повторяют один и тот же блок `Path(__file__).resolve().parents[1] / "src" ... sys.path.insert` (пример: `test_tool_ticket_create.py:13-18`), хотя `conftest.py` уже делает то же самое (conftest.py:26-30). Это ~550 строк чистого дубля; удаляется в ноль.
- `parametrize` используется всего 7 раз на 22 926 строк тестов. Явные семейства-клоны: `test_tool_ticket_{create,comment,edit,move}.py` (общая механика redirect+assert History), `test_tool_create_{agent,axis,domain,entity_type}.py`, `test_tool_apply_proposal_{assumption,stakeholder}.py`. Соотношение тесты/код = 22.9K / 26.6K строк — почти 1:1 на фреймворк без внешних потребителей.

## 7. Документация: три слоя с пересечением — НИЗКАЯ/СРЕДНЯЯ

- `spec/docs/tools/*.md` (156K, 33 файла) — по признанию tool-reference, «full text» тех же docstrings, что лежат в `spec/tools/*.py`; третье представление (docstring → docs/tools/*.md → строка в CLAUDE.md).
- `domains/hotam-spec-self/docs/gen/` — 12 файлов, где REQUIREMENTS.md (861 строк) ⊃ CONSTITUTION.md (179) ⊃ Constitution index в CLAUDE.md; FRAMEWORK-INVARIANTS.md и UNENFORCED.md — ещё две проекции того же роестра. Пять представлений одного графа. REPO-MAP.md обоих доменов «дрожит» в git status при каждом regen — признак того, что автоген трогает файлы без содержательных изменений.
- Плюс сам источник `domains/hotam-spec-self/graph.py` — 4 216 строк рукоподобного Python как база данных.

**Рекомендация:** сократить docs/gen до REQUIREMENTS + TENSIONS + HISTORY; CONSTITUTION/FRAMEWORK-INVARIANTS/UNENFORCED сделать секциями одного файла или вьюхами on-demand.

## 8. Монолиты gen_spec.py и apply_proposal.py — СРЕДНЯЯ

`spec/tools/gen_spec.py` — 4 786 строк, `spec/tools/apply_proposal.py` — 3 555 строк: вдвоём 54% всего tools/. Оба — «бог-тулы» (все рендереры / все писатели видов proposal в одном файле), при том что фреймворк проповедует атомарность (R-requirement-claim-is-atomic) для требований, но не для собственных инструментов. `audit_atomicity.py` меряет compound-условия check_* — но не compound-тулы. **Рекомендация:** разнести рендереры gen_spec по модулям на docs/gen-файл, писатели apply_proposal — по типу proposal.

## 9. gate.py / gate_status.py / closure.py — НИЗКАЯ

Три тула вокруг одного land-log.jsonl (селектор T1, вопрос commit-boundary, verify-closure). Ответственности разделены осмысленно, но это три CLI + три doc-страницы + три тест-файла для одного жизненного цикла «посадка изменения»; объединение в один `land.py` с сабкомандами убрало бы 2 файла и 2 обёртки в cli/.

---

## Итоговая сводка

| # | Находка | Серьёзность | Экономия |
|---|---|---|---|
| 1 | delegate.py + record_delegation.py — две системы делегаций | высокая | ~460 строк + 2 теста |
| 2 | 3 baseline-механизма / 2 идентичных | средняя | ~150 строк + унификация |
| 3 | Замороженные entity/federation — балласт в T2 | средняя | ~1000+ строк из горячего пути |
| 5 | 4 входа в «что сейчас важно», emit_cipher парсит свой markdown | средняя | tick.py целиком + regex-слой |
| 6 | sys.path-бойлерплейт ×92, parametrize ×7 | средняя | ~550+ строк |
| 8 | gen_spec 4.8K / apply_proposal 3.6K монолиты | средняя | читаемость |
| 7 | 5 проекций графа в docs/gen + docs/tools дубль docstrings | низкая/средняя | 3 файла docs/gen |
| 4 | cli/ 31 обёртка | низкая | 31→1 файл |
| 9 | gate/gate_status/closure — 3 CLI на один land-log | низкая | 2 файла |

Общий паттерн: фреймворк жёстко следит за атомарностью и не-дублированием *требований* (ratchet, confront, anti-relitigation), но не применяет ту же линзу к *собственным инструментам* — отвергнутые механизмы (jsonl-делегации) и замороженные ветки (Entity) остаются исполняемыми и тестируемыми вместо архивирования.
