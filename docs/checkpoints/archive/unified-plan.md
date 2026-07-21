# Checkpoint — 2026-07-02 19:45 [unified-plan]

## Session summary

Финал марафонской сессии (полная дуга дня — в соседнем чекпоинте 2026-07-02-1931.md; этот — про план). Два fxx-исследования завершены: (А) «реальность — не дерево» — варианты A/B/C, рекомендация B+C (скоуп-как-проекция + HELD-состояние конфликта с вариантами поведения); (Б) «опровергнуть потолок 149/178» — потолок опровергнут в 4 точках, достижимо ~161–164/178 без внешнего референта, 8 ранжированных ходов. По просьбе стьюарда оба доклада сшиты в ЕДИНЫЙ ПЛАН из шести волн (ниже) и заведены в TaskList цепочкой #25→#30. Ключевая сшивка: ход «конфликт freeze-vs-burn-down» из (Б) — первый боевой кейс для HELD+вариантов из (А): механизм и его первый пациент приезжают одной волной. Волны 1–2 подписей не требуют; 3–4 ждут одобрения направления B+C; 6 ждёт GO + контента от инициатора. Репо чисто (f5073b5 запушен), состояние графа: 149/178 ENFORCED, 16 closeable, 12 DRAFT, 2 OPEN (R-partition-vs-border, R-unresolvable-conflict-carries-variants — оба закрываются Волнами 4 и 3 соответственно). Контекст-шифр UNMEASURED — ждёт запуска пользователем setup_context_hook.py --patch-global --apply. Babysit-крон снят; волны вести вручную через фоновых sh-агентов либо переармировать /babysit.

## ЕДИНЫЙ ПЛАН — шесть волн (строго последовательных: все мутируют graph/gen_spec)

### Волна 1 — механическая честность [S×6] · #25 · БЕЗ ПОДПИСЕЙ
1. R-frozen-aspects-snapshot-guarded: hash-baseline замороженных файлов; изменение baseline = явный акт разморозки стьюардом.
2. check_transition_guard_assumption_resolves: dangling-ref семейство + строка RULES_AS_DATA + drift-fallout тест (поле Transition.guard_assumption уже существует).
3. Entity-МЕХАНИКА через demo-фикстуру (прецедент R-entity-state-conflict-surfaced): второй EntityType в seed.py ⇒ ENFORCED для R-entity-checks-by-iteration + R-entity-reuses-lifecycle. Политику (R-entity-derived-requirement) НЕ трогать — уходит в Волну 3 конфликтом.
4. Reflection-тест delegate-пути (R-context-bounded-delegation): синтетический over-budget ⇒ P0 называет crystallize→delegate.
5. Срез R-no-observation-type: в hotam_spec нет классов Observation/Evidence, Assumption — единственный belief-носитель.
6. Срез backend-нейтральности: src импортирует только stdlib+hotam_spec.
Эффект: 149 → ~156-157 ENFORCED.

### Волна 2 — чистка DRAFT + ратчеты [S+M] · #26 · тексты в отчёте
1. REJECT×2 с REPLACES: R-operator-backend-protocol (убит вердиктом M37 «каждый умный агент под себя допишет»), R-claude-md-budget-phi-cap (вытеснен CRYSTAL_CHARS).
2. PROMOTE×3: R-agent-imports-framework (сначала сплит, COMPOUND по AUDIT; энфорсер — тест направления импортов), R-task-spawn-is-ephemeral, R-private-tools-in-agent-folder.
3. Перепривязать триггер R-claude-md-tree-of-crystals с мёртвого φ-cap на CRYSTAL_CHARS warn.
4. Ратчеты атомарности ×2: baseline от audit_atomicity (COMPOUND claims + COMPOUND check_*), новый компаунд = красный, старые заморожены честно.
5. Requalify R-initiator-supplies-domain-content → INHERENTLY_PROSE (+опц. срез R-domain-boot-brief-recorded).
Эффект: DRAFT 12→7, +~4 ENFORCED.

### Волна 3 — онтология C: HELD + варианты + первый боевой кейс [M] · #27 · ЖДЁТ «одобряю B+C»
1. Лайфсайкл Conflict + HELD(reason), вход только по подписи человека (клон signoff-lock DECIDED).
2. Payload variants: tuple[Variant,...] (id V-…, behavior, implies, costs) — НЕ новый тип узла (анти-RDF аргумент); check_held_has_min_two_variants, check_held_has_decided_by.
3. what_now: HELD = отдельная P4-строка «choose a variant»; TENSIONS.md печатает варианты.
4. Атомы: R-conflict-held-state, R-held-carries-variants, R-variant-choice-is-decision (derived только из выбранного), R-unresolvable-classified-by-human.
5. ПЕРВЫЙ КЕЙС: ProposedConflict «freeze-vs-burn-down» (R-speculative-aspects-frozen ↔ энфорсируемость R-entity-derived-requirement) с двумя вариантами: (а) разморозить проекцию EntityType→R-entity-<slug>; (б) держать заморозку, энфорс отложен. Стьюард классифицирует/выбирает.
Закрывает OPEN R-unresolvable-conflict-carries-variants.

### Волна 4 — проекция скоупов B [S-M] · #28 · ЖДЁТ «одобряю B+C»
1. scope.py: кортеж префиксов → Scope(prefixes, ids, axes, assumptions) — view над единым графом, узлы не копируются.
2. graph.scope_overlap(op1, op2); gen_spec печатает OVERLAP-блок в кристалл каждого затронутого агента.
3. check_scoped_node_has_single_presenter: у каждого узла пересечения ровно один детерминированный предъявитель.
4. Атомы: R-scope-is-projection, R-scope-overlap-generated, R-overlap-single-presenter.
Закрывает OPEN R-partition-vs-border. ФЛАГ: соседство с замороженной суб-агентной семьёй (минимальное касание — расширение существующего scope.py).

### Волна 5 — измеримые срезы дисциплины [M] · #29 · без подписей
1. R-spawn-log-carries-isolation: поля isolation+mutating в spawn-log → срез R-parallel-mutating-agents-use-worktree.
2. R-boot-cite-measured: Stop-хук лексически проверяет первое предложение на анкер, пишет .runtime/boot-cite-log.jsonl + ридер; «форма-не-суть» декларируется в claim честно.

### Волна 6 — мини-референт domains/hotam-dev/ [L] · #30 · ЖДЁТ GO + контента инициатора
Второй домен, моделирующий разработку САМОГО репозитория: волны/коммиты/спавны/CI как Process+Entity; нагрузка уже существует в git-истории и .runtime-логах (land-log, spawn-log). Разблокирует: dependency-drives-*, Delegation-семью (+DRAFT R-domain-delegation-as-node → затем R-trust-anchor-delegation-explicit-only), runtime-факты, первое живое пересечение скоупов. Content-free не нарушен (контент в домене; поставляет инициатор — по его же вердикту C-8600b1b8).
Итоговое ожидание после всех волн: ~164–168 ENFORCED из ~187. Честный остаток: Z3-глубина (R-constituting-requirements-converge), R-delegation-conclusions-only (срез был бы ложной точностью).

## Active goal

none (babysit-крон снят; /goal не устанавливался)

## TaskList

### pending
- #25 Волна 1 — механическая честность [S×6] (не заблокирована — готова к старту)
- #26 Волна 2 — чистка DRAFT + ратчеты (blockedBy: #25)
- #27 Волна 3 — HELD + варианты + боевой кейс (blockedBy: #26; + подпись B+C)
- #28 Волна 4 — проекция скоупов (blockedBy: #27; + подпись B+C)
- #29 Волна 5 — измеримые срезы (blockedBy: #28)
- #30 Волна 6 — мини-референт hotam-dev (blockedBy: #29; + GO и контент стьюарда)

### recently completed
Задачи #1–#24 предыдущих заходов завершены/удалены (см. чекпоинт 2026-07-02-1931.md).

## Decisions

- Сшивка исследований: конфликт freeze-vs-burn-down (из «потолка») = первый боевой кейс HELD+вариантов (из «не-дерева») — одной волной.
- Порядок волн: дешёвая честность → чистка → онтология C → проекция B → срезы → референт; строго последовательно (общие файлы).
- Вариант A (Border-узлы) отклонён исследованием: контроль вместо видимости, церемония при одном стьюарде.
- Ратчет-baseline выбран вместо «только advisory» для атомарности — монотонный запрет нового долга без ложной точности.
- Мини-референт hotam-dev выбран как замена полной Фазе 5: нагрузка уже существует в git/логах.

## Open questions

- «Запускай 1–2» — старт волн без подписей (вести вручную или переармировать /babysit).
- «Одобряю B+C» — открывает Волны 3–4 (доклады в диалоге и в этом файле).
- GO + контент по hotam-dev (Волна 6).
- Пользователю: запустить setup_context_hook.py --patch-global --apply для живого контекст-шифра.
- Сплит A-most-knowledge-crystallizable (P5-кластер из 7) — опционально, кластеризация сняла остроту.

## Repo state

```
?? docs/checkpoints/2026-07-02-1931.md
```

```
f5073b5 feat: signature-wave -- 6 resolver verdicts landed + pending-proposal machinery + global-patch tool
70ee197 feat: seed coherence + LAND-tier trace + explicit doc-reader bindings
ccac757 feat: DECIDE C-8600b1b8 + M22 rules-as-data classification + context hook installer
6807a9e feat: tiered LAND gates + 3x regen speedup + doc readers + honesty wave
3ea9bdc feat: close the loop -- conflict pipeline + P5 clustering + P0 burn-down 28->11 + reflection extraction
a6459b3 feat: crystal budget honesty + tiered seed + plumbing relocation + aspect freeze
```
