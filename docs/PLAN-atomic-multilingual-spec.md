# PLAN: атомарная многоязычная спецификация

Дата: 2026-10-04
Статус: план согласованного направления; реализация не начата.
Основание: обсуждение с пользователем после доработок evidence/findings и изучение polydoc в спецификации Ktav.

## 1. Цель и принятые решения

Единица авторинга — смысловой атом, максимально короткий и предметно отражаемый в методе. Не большой переводимый раздел документа. Один атом имеет один ID, один метод-субъект, общий набор тестов и наблюдений, а также короткую формулировку того же смысла на каждом языке домена. Документы — упорядоченные языковые проекции этих же атомов.

Обязательные решения пользователя:

1. Максимальная смысловая атомарность сохраняется. Два независимо проверяемых условия не объединяются ради удобства перевода.
2. Все переводы атома авторятся рядом с его методом в doc-комментарии.
3. Переводы проще проверять по короткой паре «формулировка ↔ метод ↔ примеры выполнения», а не сопоставлять независимые большие документы.
4. Одноязычный авторинг работает без языковой разметки и обязательной конфигурации.
5. Многоязычные документы собираются из одного набора атомов; языки не создают отдельные методы, графы или прогоны тестов.
6. Из polydoc заимствуется принцип совместного авторинга и структурного паритета, не зависимость от Node.js и не его отдельные content units.

Реализация Ktav остаётся на паузе. Этот план относится к движку HotamSpec. Не менять `apps/ktav`, исходный `D:/dev/ktav-lang` или другие пользовательские приложения, не возобновлять их тесты и миграции. Для приёмки использовать собственные временные consumer-домены. Нет коммитов, push, обновления версий или зависимостей без отдельного указания.

Связанный контракт: [атомарная conformance-спецификация](PLAN-atomic-conformance-spec.md).
Его К0 фиксируется совместно с М0; М3/М4 используют контракты rule/case после
К1/К2, а М7/М8 и К10 — общую модель отчётов. Совместная приёмка К13 предшествует
одному согласованному финальному прогону К14/М13. Это не запуск реализации.

Для явно объявленного rule atom текст нормы устойчив и берётся из doc метода,
а разные case results остаются в наблюдениях; языковая проекция не вклеивает
один witness value в универсальную норму. Обычные value-facts сохраняют текущую
семантику «фраза + исполненное значение». Обязательства инвентаря норм, cases
и profiles не активируются одним включением languages.

## 2. Исследованная отправная точка

### Polydoc/Ktav

- `D:/dev/ktav-lang/spec/scripts/internal/builder/build_spec/shared.mjs` объявляет en/ru/zh и имена `spec.md`, `spec.ru.md`, `spec.zh.md`.
- `versions/0.8/content/manifest.js` задаёт общий порядок units.
- `named/named-abstract/meta.js` хранит заголовки всех языков, `body-1.md` — языковые блоки за `>>>>> lang=<code>`.
- Проверяются наличие переводов и структурный паритет, а не автоматическая эквивалентность смысла.
- Read-only сборка `buildBuffers` выполнена: 107 units, три языковых buffer; файлы Ktav не записывались.

### HotamSpec

- `internal/gate/atom_source.go`: `AtomSource.Phrase` сейчас берётся из первой строки `fn.Doc.Text()`; `DeriveClaim` соединяет её с реальным значением прошедшего atom artifact.
- `internal/recorder/canon/hotamspec.go`: `Fact`/`Holds`, нормализованный Subject и ID по receiver/method; doc-текст не задаёт identity.
- `internal/selfspec/atom_derive.go`: discovery, связки method/test, lifecycle overrides, Go → graph.
- `internal/gate/spec_build.go`: сбор исполнения и SPEC; `BuildSpecDocumentsFromRows` выдаёт `SPEC.md` и пакетные `spec/<pkg>.md`.
- `internal/loader/manifest.go`, `internal/ontology`, `internal/ontology/canon`: конфигурация и переносимые контракты.
- `internal/evidence`, `internal/source`, `cmd/hotam/evidence.go`, `findings.go`: наблюдения, источники и отдельный human review.
- `internal/generator`, `cmd/hotam/gen_spec.go`: остальные проекции, ссылки, кристаллы и владение generated-файлами.

Имена остальных затронутых функций и файлов уточнять средствами code intelligence при реализации, не открывать угаданные пути. До изменения экспортируемого контракта найти все references. Не создавать второй параллельный pipeline.

## 3. Авторинг атома

### Один язык — нулевая обвязка

```go
// год рождения не раньше первого календарного года
func (p Person) BirthYearHasValidLowerBound() bool {
    return p.Year >= 1
}
```

Работает без `languages`, `default_language`, `lang=...`, дополнительных файлов или ручных заголовков в `Fact`. Выход остаётся `SPEC.md` и обычными пакетными страницами. Движок не угадывает язык комментария. Существующее правило короткой первой doc-фразы сохраняется; комментарии с обычными пояснениями не превращаются автоматически в дополнительные нормы.

Одноязычный домен может явно указать `languages: ["ru"]`, если нужны русские служебные подписи. Это не требует размечать каждый метод и не меняет одноязычные имена файлов. Без настройки служебные подписи сохраняют действующее поведение, а авторская фраза выводится на языке автора.

### Несколько языков — один комментарий атома

```go
// >>>>> lang=ru
// год рождения не раньше первого календарного года
//
// >>>>> lang=en
// birth year is not earlier than the first calendar year
func (p Person) BirthYearHasValidLowerBound() bool {
    return p.Year >= 1
}
```

Каждый блок несёт одну короткую фразу одного и того же атома. Верхняя граница года — отдельный метод/атом. Не вводить формулы, Markdown-заголовки, IDs, таблицы или шаблонные переменные как обязательную обвязку обычного doc-комментария. Имена методов, bodies и вызовы `Fact`/`Holds` не дублировать по языкам. Runtime-метод не конструирует словарь переводов.

Синтаксис маркера один: `>>>>> lang=<code>` в Go doc-комментарии. Не поддерживать набор альтернативных тегов и legacy aliases. Парсер использует AST и исходные позиции; неизвестный код, дубль, пустой блок, посторонний текст вне языковых блоков и отсутствие обязательного языка диагностируются с file/method/line/language. Обычная фраза допустима только в одноязычном режиме; в многоязычном режиме она не считается молчаливым переводом всех языков.

## 4. Конфигурация языков

Новые поля manifest:

```json
{
  "languages": ["ru", "en"],
  "default_language": "ru"
}
```

- Отсутствующий `languages` — действующий одноязычный режим, без новых обязанностей.
- Явный список содержит хотя бы один уникальный код. `default_language` для нескольких языков обязателен и входит в список; для одного может отсутствовать или совпадать с единственным кодом.
- Не выбирать основной язык по порядку map, языку ОС, терминала или эвристике doc-текста.
- Коды безопасны для относительных output paths: запретить разделители путей, `..` и неоднозначные case-варианты. Нормализация и порядок определяются один раз, до сборки.
- Первый комплект служебных переводов — en/ru/zh, как проверяемый пример Ktav. Поддержка дополнительных языков идёт через тот же каталог, не через специальные ветки генератора. Язык без полного каталога служебных текстов не выдавать за поддержанный; явная ошибка вместо English fallback.
- Требования полного комплекта переводов запускает собственный явный multilingual trigger (`languages` с несколькими элементами), не старые `self_executing_atoms`, `discipline` или provenance opt-ins.
- Manifest read/write сохраняет поля; Graph переносит разрешённую конфигурацию. Состояние invocation-local, не package-global.

## 5. Модель данных и идентичность

1. Source index хранит набор фраз по language вместе с точными source positions. В одноязычном режиме обычная фраза не требует пользовательского language key.
2. Один Requirement содержит один общий ID, lifecycle, method/test/source links, qualifications и историю. Не создавать `R-foo-en`/`R-foo-ru` и не писать отдельный graph.json на язык.
3. Выбранная языковая проекция не меняет атомный ID, relation edges, verdict, coverage или finding ID.
4. Действующий `Claim` остаётся явно определённой основной формулировкой для существующего графового mediation/history контракта. Полный набор локализованных формулировок — отдельное typed поле Requirement; в multilingual режиме Claim механически выводится из текста default_language, не авторится второй раз руками. Это основной текст, не deprecated alias и не fallback отсутствующего перевода.
5. Все callers, clone/merge/diff, sync hashes, histories, strict proposal decode, canonical JSON и vendored ontology mirror получают новый контракт. В одноязычных payload не появляются пустые multilingual поля.
6. Изменение перевода отражается в source → graph diff и confirm hash даже при неизменном основном Claim. Не терять изменение неосновного языка при sync.
7. Чистый выбор render language не меняет finding identity или перенос review. Изменение исходного нормативного материала/его version/hash подчиняется текущей content-addressed finding identity; не обещать, что редактирование перевода source никогда не изменит finding ID.
8. Свободные SourceRefs и проверенные SourceLinks остаются различными сущностями. При разных языковых source-файлах version/hash/anchor проверяются для каждого явно объявленного источника; не приписывать переводу подтверждение другого файла.

Точные Go type/field names зафиксировать в М0 после reference inventory. Один общий localized-text контракт, без независимых `ClaimRu`, `PhraseEn` и ad hoc maps в каждом генераторе.

## 6. Исполнение и derivation

- Сбор исполнения один на invocation, не по языку. Все renderers используют один набор реальных artifacts/rows; родной Go test cache остаётся единственным verdict cache.
- `Fact`/`Holds` продолжают исполнять методы один раз на соответствующий вызов. Переводы не добавляют вызовы метода и не меняют `want`.
- Input/actual/expected, bytes, values, API/error identifiers, профили, IDs и адреса method/test/source — точные общие данные. Не переводить SDK output и не менять данные сравнения ради языка.
- Локализуются doc-фразы, служебная связка фразы со значением, подписи и представление статусов. Не применять English capitalization/punctuation ко всем языкам без явного правила; одноязычный legacy вывод не менять.
- Ручной заголовок не становится источником текста атома и не подменяет doc-фразу. Сохранить действующую проверку конфликта explicit title с derived claim; язык проверки должен быть явным.
- Непрошедший тест, nested discrepancy и passing sibling остаются одинаковыми во всех языковых проекциях.
- Не менять этим проектом нормотворческую границу SPEC: действующее требование успешного исполнения для публикации executed normative claim сохраняется. Из failed/unexecuted artifact не выводить «подтверждённую норму». Все языки соблюдают одну и ту же политику видимости и честные статусы evidence/findings. Отдельная смена правила публикации непроверенных authored норм не входит в этот план.

## 7. Сборка файлов, навигация и служебные тексты

Одноязычный комплект сохраняет существующие имена и layout.

Многоязычный SPEC:

```text
docs/gen/SPEC.ru.md
docs/gen/SPEC.en.md
docs/gen/spec/ru/<pkg>.md
docs/gen/spec/en/<pkg>.md
```

- Общие package order, source order и anchors; каждое языковое оглавление ведёт к shards своего языка.
- Для остальных человекочитаемых domain projections использовать тот же language suffix: REQUIREMENTS, TRACEABILITY, COVERAGE, MODELS, EVIDENCE, FINDINGS, AGENT-CONTEXT и прочие фактически выпускаемые документы.
- Операционный `CLAUDE.md` остаётся единственной стандартной boot-точкой и строится на default_language. Дополнительные языковые кристаллы имеют `CLAUDE.<lang>.md`; не выпускать дублирующую default-language копию только ради симметрии. Аналогично стандартные project-root точки входа сохраняются в одном основном языке, с дополнительными локализованными представлениями где эти документы генерирует HotamSpec.
- Общие машиночитаемые данные: один graph.json, один evidence.json, один review store, не три состояния разбора.
- Перечень outputs выводится одним общим механизмом; не поддерживать отдельно ручные списки в генераторе, cleanup, freshness и link validation.
- Служебные heading/status/table/navigation/help-workflow тексты должны быть полноценными на каждом поддержанном языке. Не выдавать документ с русскими атомами и случайной смесью английских служебных разделов за полностью локализованный.
- Языковые версии представляют один semantic document; не увеличивать счётчики obligations на число languages и не умножать бюджет кристалла на суммарный размер всех переводов. Бюджет применяется к одной view.
- URI, code identifiers, команды и технические ключи остаются точными. Авторские review rationale, resolver quotes и исторические decision records не переводятся автоматически; это цитируемые исходные данные, а не незаполненные переводы атома.
- Locale-context передаётся явно в рендеринг; не менять глобальный язык процесса. English fallback внутри multilingual представления запрещён.
- Markdown templates/framework catalog локализуются тем же контрактом текста; методы пользовательских атомов не должны хранить тексты заголовков движка.

## 8. Паритет и честность переводов

Механически проверить:

- каждый участвующий атом имеет ровно один непустой текст для каждого объявленного языка;
- во всех языковых SPEC одинаковы IDs, package membership, порядок, relation/source/method/test links и proof/coverage state;
- дубли и missing translations не теряются при source indexing или graph projection;
- ссылки разрешаются внутри правильного языкового комплекта; machine-data links указывают на общий файл;
- plain-комментарии одноязычного домена не требуют language markers;
- отрицание/границы не проверяются фиктивной «семантической» эвристикой.

Человек проверяет каждый короткий набор формулировок относительно метода и реальных примеров. Смысловой дрейф перевода — отдельная finding/review с указанием atom/language/source; не вводить automated «translation verified» по совпавшим токенам, длинам, числу строк или только наличию текста. Не требовать одинаковую длину и Markdown-пунктуацию разных языков.

## 9. Freshness, запись и cutover

- Freshness сравнивает весь ожидаемый языковой комплект и shards, а не только legacy `SPEC.md`. Multilingual obligation не должна исчезать, когда старого имени больше нет.
- Сначала проверить конфигурацию, собрать общий source/execution snapshot и отрендерить весь комплект в памяти. Missing translation/ошибка render не должна оставить половину нового комплекта на диске.
- Применять существующие механизмы подготовки, атомарной записи файлов и recovery HotamSpec. Честно указать фактические межфайловые гарантии; не обещать filesystem snapshot атомарность, которой нет.
- При переходе одного языка → нескольким удалить только obsolete outputs, принадлежащие генератору. Не удалять чужие authored файлы. Обратный переход и удаление одного языка также очищают только obsolete owned outputs.
- При смене default_language обновить Claim/view, boot-кристалл, навигацию и sync diff без смены identity атомов и verdict.
- Нет постоянного собственного кэша execution/переводов и нет новых npm dependencies.
- Полная миграция engine callers/tests/docs и self-hosting anchors. Не оставлять obsolete API, парсеры альтернативных тегов и compatibility re-exports.

## 10. Работы и порядок

Каждый М-пункт ниже — отдельная задача сессии с тем же названием. Все задачи реализации пока ожидают отдельного запуска; текущий запрос — записать план и завести задачи.

### М0. Зафиксировать контракты языков, текстов атома и языковых outputs.

Снять reference inventory; определить localized-text тип/JSON fields, manifest semantics, locale-safe naming, набор doc kinds и default-language boot routing. Обновить AUTHORED-SPEC-CONTRACT. Приёмка: один непротиворечивый контракт и список мигрируемых callers, без обязательной разметки одного языка.

### М1. Реализовать manifest language configuration и перенос в Graph.

Loader/serialization/config validation, отсутствие новых обязательств для существующих доменов, invocation-local state. Приёмка: legacy, explicit single language и multilingual; reject duplicates/empty/unsafe codes/invalid default/unsupported catalog.

### М2. Реализовать AST-разбор doc-фраз всех языков одного атома.

Заменить single Phrase extraction общим source-text механизмом; plain docs остаются простыми. Приёмка: точные file/method/line diagnostics и полный набор переводов без эвристического угадывания языка.

### М3. Провести локализованные формулировки через ontology и Go → graph sync.

Основной Claim, typed localized texts, canon/vendor, cloning, overrides, merge/diff/history/confirm hashes и strict proposal fields. Приёмка: изменение только неосновного перевода видно в dry-run, меняет confirm hash и сохраняется; method/ID/links общие.

### М4. Разделить общий execution snapshot и языковой claim derivation.

Переиспользовать один source/artifact/rows snapshot, сохранить Fact/Holds/title contracts, реальные значения и failing siblings. Приёмка: добавление языка не добавляет исполнения; locale selection не меняет verdict/coverage/finding identity.

### М5. Реализовать общий каталог служебных переводов en/ru/zh и явный locale-context.

Все фактически генерируемые headings, статусы, таблицы, навигация и workflow text; без глобального locale/fallback. Приёмка: supported locale не выдаёт незаполненные English шаблоны; raw data/цитаты не подменяются переводом.

### М6. Собирать отдельные языковые SPEC indexes и package shards.

Общие атомы и anchors, выбранные фразы, языкозависимая sentence formatting и локальные ссылки. Приёмка: одноязычный layout не меняется; все multilingual indexes/shards полны и согласованы.

### М7. Локализовать остальные domain/project projections и default-language кристалл.

REQUIREMENTS/TRACEABILITY/COVERAGE/MODELS, остальные emitted doc kinds и общие framework projections. Приёмка: один boot CLAUDE на default_language, корректные дополнительные views, одинаковая структура и языково-корректные ссылки/бюджеты.

### М8. Добавить языковые EVIDENCE/FINDINGS views поверх общих JSON и review store.

Сохранить raw observations, версии/профили, precedence discrepancy, identity и immutability review. Приёмка: одна находка имеет несколько представлений, не несколько независимых разборов; locale switching не переносит review на изменившийся source fingerprint.

### М9. Внедрить structural translation parity и полную freshness проверку.

Собственный language trigger, одинаковые atom sets/links/statuses, source positions, typed source verification каждого объявленного source. Приёмка: missing/stale translation или shard даёт реальное нарушение; старый opt-in не запускает новую обязанность; нет автоматической семантической полноты.

### М10. Обеспечить согласованную запись и owned-output cleanup при смене языков.

Один output inventory для write/check/cleanup; render all before publish; existing recovery; single ↔ multilingual и default-language switch. Приёмка: translation error не публикует частичный комплект; удаляются только obsolete generated files; гарантии crash recovery названы точно.

### М11. Обновить self-hosting anchors, документацию и затронутые engine контракты.

README, QUICKSTART-CONSUMER, AUTHORED-SPEC-CONTRACT, PROPOSAL-REFERENCE, CHANGELOG и генерируемые инструкции. Новые enforcers имеют реальный canonical carrier; projection через sync-self с confirm hash. Приёмка: no stale callers/proofs/aliases; примеры одного языка без разметки. Ktav и другие apps не мигрируются.

### М12. Проверить реальные одноязычные и многоязычные consumer сценарии.

Собственные throwaway domains: plain русские docs без config, explicit single ru, en/ru/zh, multiple packages, Fact/Holds/nested failure/passing sibling, первый отчёт до sync. Проверить language-only edits, default switch, missing translations, source drift, links, reviews и unchanged outputs при отказе. Приёмка: реальные CLI outputs/state/exit codes; удалённые временные fixtures.

### М13. Пройти финальный интеграционный прогон HotamSpec и зафиксировать доказательства.

После freeze исходников: build, go vet ./..., полный go test -timeout 30m ./..., реальная self-hosting freshness/violations проверка и финальный live CLI smoke. Любое падение/флейк чинить в этом же ходе. Приёмка: все критерии §11 и exercised receipts; без Ktav run, commits/push/version changes.

Порядок зависимостей:

- М0 → М1 → М2 → М3 → М4.
- М5 после М0/М1; М6 после М3/М4/М5.
- М7 и М8 после общего text/render контракта и М4/М5/М6.
- М9 после М2/М3/М6/М7/М8; М10 после полного output inventory М6/М7/М8 и проверок М9.
- М11 после контрактного freeze; М12 после М9/М10/М11; М13 после М12.
- Делегирование только по явному указанию пользователя. Возможная параллельность не означает разрешение запускать агентов.

## 11. Полная приёмка

1. Обычный один doc-комментарий работает без language tags/config, существующие одноязычные домены не получают новых обязанностей или иной layout.
2. Явный single ru локализует служебные подписи без разметки каждого метода.
3. Один method/atom содержит en/ru/zh; один набор executions собирает три согласованные языковые спеки.
4. Добавление языка не меняет число реальных вызовов метода; tests не дублируются по языкам.
5. Все locale views содержат одинаковые atom IDs/relations/packages/anchors/links и proof/coverage status.
6. Missing/empty/duplicate/unknown language диагностируется с точным источником и не приводит к частичному publish или fallback.
7. Правка только ru-фразы меняет ru output и localized source projection/sync hash; не переписывает en/zh атомный текст, ID или runtime result.
8. Изменение реального значения обновляет value-dependent текст всех языков; nested failure остаётся failure везде, passing sibling не пропадает, unverified не становится verified.
9. Все language indexes, shards, other projections и boot routing свежие; ссылка не ведёт молча в чужой язык.
10. Одна raw evidence JSON модель и один review store. Locale selection не меняет raw input/actual/expected, finding ID и review; реальные source/version/hash edits obey existing identity rules.
11. Single → multilingual, удаление языка, обратный переход и смена default не оставляют obsolete owned outputs и не трогают authored files.
12. Все новые ограничения принадлежат собственному language trigger; checks остаются структурными и не объявляют перевод семантически доказанным.
13. Engine canon/vendor/callers/self-hosting/docs мигрированы; no deprecated aliases, fake fallbacks, stubs или deferred implementation.
14. Реальный CLI smoke, full vet/test и self-hosting checks пройдены; receipts записаны в итог выполнения. Ktav остаётся остановленным.

## 12. Правила проверки и риски

- Нужны behavioral regression checks для полноты переводов, locale selection, identity, partial-publish refusal, source drift и переходов файлового комплекта. Не закреплять весь Markdown golden-хэшем, счётчиком количества registry entries или копией текста реализации.
- Паритет структуры не доказывает семантику. Любая формулировка «перевод проверен» должна различать structural completeness и human semantic review.
- Главные риски: single-language API regression, silently stale secondary-language Claim, повторный Go test на каждый locale, зависящие от языка IDs, потеря failing evidence, ложное inheritance review, неправильные cross-language links, legacy SPEC.md-only freshness и удаление authored outputs.
- Не распространять короткую doc-фразу в длинный narrative block ради документа. Проблема атомарности решается декомпозицией методов; структура документа получается из пакетов/связей и общего порядка этих атомов.
- Никаких автопереводов на сборке, вызовов модели/сети, отдельного JS-сборщика, постоянного кэша вердиктов, новых бизнес-норм и миграций остановленного Ktav.

## 13. Реализация и проверка — 2026-10-05

М0–М13 выполнены и проверены. Проверенный delta из 140 файлов перенесён в
основной checkout поверх сохранённого пользовательского baseline.

- Реальные plain, explicit single `ru` и `en/ru/zh` consumers: `all-violations`
  возвращает `0 violations`.
- Три cases в одном method/norm: публикация трёх языков делает ровно три
  вызова метода. Secondary-language-only, case-only и default-language edits
  подтверждены отдельными sync hashes; norm ID сохраняется.
- Реальный multi-package consumer публикует `spec/model`, `spec/model/access`
  и `spec/model/limits`. Fact/Holds и nested comparison проверены: `1 != 2`
  остаётся failure при проходящем summary `false == false`, siblings сохранены.
- Legacy value `17`→`18` обновляет все три ClaimTexts без смены ID/kind;
  китайская фраза сохраняет собственную пунктуацию `配置限值 — 18。`.
- Unknown language даёт file/method/line diagnostic; SHA всего существующего
  комплекта не меняется при отказе. Authored output collision также не пишет
  ни одного другого файла.
- Non-default Chinese shard drift обнаруживается без legacy `SPEC.md`.
  Удаление языка, multi→single и single→multi удаляют только owned outputs;
  authored `NOTES.zh.md` сохраняется.
- Default `ru`→`zh` меняет primary Claim и boot CLAUDE; один raw report и review
  store сохраняют finding IDs. Изменение case/profile/composition metadata
  инвалидирует соответствующий review, не добавляя вымышленных measurements.
- Canonical `sync-self` с confirm hashes добавил три новых carrier и обновил
  актуальные proof/shape links; оба landed прогона завершились с `0 violations`.
- Frozen `go build`, `go vet -p 1 ./...` и
  `go test -p 1 -timeout 30m ./...` пройдены: 26 пакетов `ok`, 2 без тестов.
  Main-checkout binary собран отдельно; canonical `all-violations --json`
  возвращает `[]`, typed/multilingual consumer evidence — exit 0.

Ktav/apps не запускались и не изменялись; commits, push и поднятия версий нет.
Собственные восемь worktrees/branches и все throwaway consumer fixtures удалены.
