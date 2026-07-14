# Линза 5: Инженерное здоровье — долг и риски роста

fxx-agent, read-only, 2026-07-05. gen_spec 4852 стр, invariants 4151 (85 check_*), apply_proposal 3093, доменный graph.py 4097 (~19 стр/атом); тестов 102 файла / 799 def test_ / 1004 collected.

---

## КРИТИЧНО

### K1. Тройная реализация резолва активного домена

graph.py::_active_domain_graph_file, gen_spec::_select_active_domain_dir, apply_proposal::_resolve_content_graph — три копии env→pin→alphabetical. Синхронизация руками. Расхождение = разные тулы пишут в разные домены.

Вектор: один hotam_spec.domain_resolution модуль.

### K2. hotam_spec (библиотека) ходит по ФС репозитория

invariants.py глобит/парсит ВСЕ tests/*.py (enforced_by), importlib-загружает каждый доменный graph.py (entities_md), читает сгенерированный ENTITIES.md (цикл check↔gen). all_violations = O(репозиторий), на каждый промпт в холодном интерпретаторе.

Вектор: mtime-keyed персистентный индекс enforcer→node-id в .runtime/; вынос fs-чеков из горячего all_violations.

### K3. Единый graph.py как AST-хирургический монолит

4097 строк; apply_proposal: ~1500 строк построчно-колоночной правки (_replace_or_insert_field, _byte_col_to_char_col…). Магнит merge-конфликтов при параллельных агентах. При ×3 → 12k строк.

Вектор: шардирование на graph_parts/*.py + компилированный снапшот (hash-keyed).

### K4. Каждый LAND платит O(всех доменов)

gen_spec перегенерирует ВСЕ домены. T2 принудителен для каждого нового узла (T1 «fails closed»). Рост через new-node → T1 не помогает.

Вектор: dirty-tracking (--only-domain); семейный гейт для new-node.

## ВАЖНО

### V1. gen_spec 4852 строк / 9 ответственностей + мёртвый код

Границы: paths/cache/14 builders/template/tooldocs/agents/multi-domain/CLI + **8 мёртвых апдейтеров** (0 call-sites после шаблона, несколько сотен строк).

Вектор: пакет genspec/; мёртвые удалить сразу.

### V2. rules-as-data — аннотация, не движок

38 TABLE_DRIVEN vs 37 BESPOKE, но все 38 «табличных» остаются рукописными клонами (8× typed-anchors, 6× dangling-refs…). Блокер: Jaccard ≥ 0.05 — мета-линт нулевой силы.

Вектор: движок по таблице для 4 семейств; Jaccard только для BESPOKE.

### V3. tools/ — не пакет, а библиотека по совместительству

21 тул с sys.path-прологом; приватные кросс-импорты; цикл gen_spec→what_now→apply_proposal→(subprocess gen_spec) на lazy-импортах; сентинели CLAUDE.md продублированы литералами.

Вектор: tools как пакет + hotam_spec.claude_md.

### V4. Самоссылочный LIVE-STATE (fixpoint)

Резидентный размер аппроксимируется от соседних блоков; test_claude_md_template побайтно восстанавливает.

Вектор: считать бюджет по «CLAUDE.md без LIVE-STATE-блока».

### V5. Session-фикстуры на ~10%

30 файлов / 76 call-sites зовут load_content_graph() (exec 4097 строк), 15 sys.executable-спавнов.

Вектор: механическая миграция на session-фикстуры.

## ИНТЕРЕСНО

- I1. proposal.py (394 стр dataclass) vs apply_proposal._validate_* (~480 стр) — схема в двух местах.
- I2. Делегаторы (check_no_dangling_ids etc) — навигационный шум, но enforced_by ссылается.
- I3. Здоровое (не лечить): enforcer_resolution шарится; негативные fires образцовые; conftest-ярусы; _capture_tool_help in-process.

## Числа по тест-корпусу

- Тавтологии: ~2-3%
- Линт-как-тест: ~20-22% (194 collected из одного test_public_object_has_section_label)
- Дубли: ~8%
- Хрупкие к markdown: ~5% реально прозо-хрупких + ~2% byte-equality by design
- **Итого балласт: ~25-30% по числу, <10% по времени** (балласт дёшев)
- Качество ядра: ~40/60 negative fires в test_invariants, Hypothesis-свипы CRITICAL_CORE

## ТОП-3 СУПЕРВЕКТОРА

1. **Единая инфраструктурная плоскость** — domain_resolution/repo_paths/claude_md в библиотеку; tools как пакет. Убирает 3 копии резолва, 21 sys.path, кросс-импорты. Предусловие всего дальнейшего.

2. **Распил монолитов** — genspec/ пакет + удаление мёртвого + invariants/ с табличным исполнителем. −1500-2000 строк; «добавить чек = строка таблицы».

3. **Отвязка стоимости от масштаба** — персистентный enforcer-индекс, компилированный граф-снапшот, dirty-домены, семейный гейт. Единственный вектор, удерживающий per-prompt хуки и per-land плоскими при 600 атомах / 5 доменах.
