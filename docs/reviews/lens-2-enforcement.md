# Линза 2: Enforcement-механика — дыры между обещанием и проверкой

fxx-agent, read-only, 2026-07-05.

---

## КРИТИЧНО

### 1. Enforcer можно выпотрошить незаметно (`return []` проходит весь мета-слой)

`check_method_matches_docstring` проверяет Jaccard(RULE-строка, Violation-тексты) ≥ 5%, но при 0 Violation `continue` → молча пропускает. Биекция довольна именем. Fires-тестов 51 на ~90 чеков.

Эксплойт: заменяю тело check_* на `return []` → атом остаётся [E], pytest зелёный.
Фикс: (а) RULE без Violation-сайтов = Violation; (б) биекция check ↔ fires-тест.

### 2. Подпись стьюарда — свободная строка + вечная DEL-1

decided_by проверяется ТОЛЬКО: непустой и ∈ stakeholder_ids. Агент под DEL-1 пишет любую подпись. Авторство коммита не проверяет ничто.

Фикс: привязать decided_by к git-authorship; surface делегации и возраст в what_now; --triggering-kind обязателен.

### 3. Enforcement-машинерия не защищена PreToolUse-гардом

Гард денаит ТОЛЬКО domains/*/graph.py. invariants.py, gate.py, тесты, baselines, .active-domain — редактируются свободно.

Эксплойт: красный инвариант «чинится» ослаблением invariants.py.
Фикс: расширить гард на enforcement-периметр (или хэш-пин).

## ВАЖНО

### 4. enforced_by резолвится в тавтологию

Grep по сырому тексту → имя в комментарии «резолвится». Тест с телом pass = «ENFORCED».
Фикс: AST-резолв; линт на пустое тело.

### 5. Closure опциональна (--triggering-kind)

check_closure вызывается только с флагом; без флага exit 0 на зелёных тестах. В данных: closure_exit в 9/171 лендов (5%).

Фикс: --triggering-kind обязателен или выводится из what_now.

### 6. gate_status fail-open на пустом логе

Пустой land-log → `satisfied=True`. На свежем клоне гарантия испаряется.
Фикс: пустой = NOT satisfied (fail-closed).

### 7. Ratchet-протечки

Компаундность лексическая; baseline дописываем; тест редактируем (см. №3).

## ИНТЕРЕСНО

- 8. boot-cite [E] меряет ритуал, не истину (FORM, NOT SUBSTANCE).
- 9. machine_check только компилируется, не исполняется.
- 10. PROSE/STRUCTURAL, врущие: R-tiered-gate-not-a-commit-gate (INHERENTLY_PROSE при механическом gate_status); R-ticket-mutation-via-tools-only (enforceable гардом); 2 closeable debt ENFORCEABLE; расхождение счётчиков LIVE-STATE vs UNENFORCED.

## ТОП-3 СУПЕРВЕКТОРА

**A. Замкнуть периметр самомодификации** — гард/хэш-пин на invariants.py+gate.py+тесты+baselines. Без этого ВСЕ гарантии в одном Edit от обнуления.

**B. «ENFORCED = доказуемо срабатывает»** — (1) RULE без Violation = Violation; (2) биекция check↔fires-тест; (3) AST-резолв enforced_by.

**C. Trust-anchor к реальному человеческому сигналу** — decided_by→git-authorship; surface делегаций; closure обязательна.
