# PLAN: атомарная исполняемая conformance-спецификация

Дата: 2026-10-04
Статус: план; реализация не начата.
Основание: запрос пользователя о доработках HotamSpec для удобной реализации спецификации Ktav; последующее указание записать план и завести задачи.
Связанный план: [атомарная многоязычность](PLAN-atomic-multilingual-spec.md).

## 1. Цель и границы

Сохранить принцип HotamSpec: максимально короткий смысловой атом отражается в конкретном методе, а проверка показывает, соответствует ли реальное поведение этой норме. Сделать удобной спецификацию языка с множеством входов, ordered rules, байтовой диагностикой и несколькими implementation profiles.

Нужно разделить:

- **атом нормы** — стабильный смысл, ID и метод-субъект;
- **case** — вход, независимое нормативное ожидание, профиль и проверяемые атомы;
- **наблюдение** — точный результат реального исполнения;
- **finding** — выявленное расхождение или недоступность проверки, а не автоматически назначенная вина.

Не превращать каждую corpus fixture в самостоятельное требование. Не превращать одинаковый метод на разных входах в разные языковые/профильные копии нормы. Не менять ожидание по результату SDK, не добавлять parser shims для конкретных строк, не выдавать подтверждённым непроверенный смысл.

Область реализации — движок HotamSpec, его контракты, генераторы, self-hosting и документация. `apps/ktav` и исходный `D:/dev/ktav-lang` остаются на паузе: не менять, не запускать, не мигрировать. Приёмка — собственные временные consumer-домены. Нет commits/push/version bumps/dependency upgrades без отдельного указания. Агентов не запускать без явного разрешения пользователя.

## 2. Подтверждённые основания

### Текущее ограничение атомов

`internal/gate/atom_source.go` строит Claim из doc-фразы и исполненного значения. `internal/selfspec/atom_derive.go` объединяет одинаковые IDs только при одинаковых Claim/subjects, иначе возвращает ошибку conflicting executed values.

В собственном временном consumer были два вызова одного метода `Result.Number`: actual=1/want=1 и actual=2/want=2. Реальный `sync-domain` отказал с:

```text
atom R-result-number produced conflicting subjects or executed values; use an explicit ID override
```

Это ограничение текущего value-fact контракта, не подтверждённый дефект Ktav. Временный consumer удалён. Существующий `Observed` позволяет хранить детали под общим summary, но не заменяет явной модели случаев и не должен требовать boilerplate для каждого corpus input.

### Материал Ktav

- `versions/0.8/tests/manifest.json`: 319 fixtures в valid, invalid, unrepresentable, parseable-unrepresentable, strict-lossy; отдельная raw-bytes fixture InvalidUtf8.
- §5.2 задаёт ordered scalar inference, включая профильные пределы Integer/Float и fallback.
- §6.15 задаёт приоритет InvalidUtf8 до line/grammar processing.
- В сохранённом `apps/ktav/FINDINGS-spec-implementation.md`: корпус ранее прошёл 782 проверки, но дополнительные методы нашли расхождения за его пределами; исправление adapter не доказывает корректность pristine SDK; есть unsupported recommendation и profile-qualified unreachable branch.

Это прочитанные источники и исторически записанные результаты, не новый запуск Ktav и не свежая приёмка backend.

### Текущие точки расширения HotamSpec

- `internal/recorder/canon/hotamspec.go`: Fact/Holds/Observed/Observe, Subject, input/context, реальный test failure.
- `internal/gate/atom_source.go`, `spec_build.go`: source index, derivation, execution rows и SPEC.
- `internal/selfspec/atom_derive.go`: discovery и Go → graph; `cmd/hotam/sync_domain.go` и `sync_self.go` — preview/confirm/gates.
- `internal/ontology`, `internal/ontology/canon`, `internal/loader`: переносимые типы, manifest, serialization.
- `internal/evidence/evidence.go`: общие requirement/test/artifact/finding rows; public Observation сейчас содержит строковые input/actual/expected.
- `internal/source`: bytes/version/hash/anchor verification объявленных источников и links.
- `internal/invariants`, `internal/generator`, `cmd/hotam`: структурные проверки, projections, evidence/findings CLI и review.

Перед экспортируемыми изменениями найти references и реальные definitions. Снять карту всех затронутых parser/clone/merge/diff/history/vendor/render callers; не открывать угаданные файлы и не вводить второй pipeline.

## 3. Атом нормы и атом конкретного значения

### Не ломать простой случай

Обычный `Fact(t, method, want)` остаётся простым value-fact. Например, «год рождения — 1987» конкретного объекта сохраняет текущую семантику doc-фраза + реально исполненное значение. Разные значения такого одного fact ID не становятся молча одним и тем же фактом.

Doc-комментарий одного языка не требует разметки, дополнительного schema file или языка в каждом тесте. Существующие домены не получают новых обязанностей от старых opt-ins.

### Добавить явный rule/case контракт

Для параметризованной нормы вводится явно объявленная разновидность атома, а не эвристика по числу tests или по тому, что метод вернул bool. Один rule atom связывает doc-норму и метод с несколькими case executions; actual/expected находятся в cases/observations, не подменяют текст нормы разными witness values.

- Стабильный ID атома выводится из нормализованного метода либо явного override, как сегодня.
- Case ID стабилен в рамках домена, не зависит от языка doc-текста и фактического ответа реализации.
- Исполненный sample остаётся адресуемым через atom + case + test/subtest + профиль + implementation identity.
- Разные actual при разных входах и совпавших независимых ожиданиях допустимы; разные описания одного case или несовместимые повторные наблюдения не теряются.
- Несколько независимо проверяемых условий — отдельные атомы. Case может быть общим свидетельством нескольких атомов, но каждый comparison указывает собственную норму.
- Нельзя заменить реальные сравнения постоянным `true` или подавить nested failure ради стабильного Claim.
- Case expectation авторится из нормы/corpus, не извлекается из того же actual или второго вызова тестируемой реализации.
- Явный rule/case режим имеет собственный trigger. Не переносить новые ограничения на все старые `self_executing_atoms` домены.

Точные публичные имена kind/metadata/recorder options и JSON fields фиксируются в К0 после inventory. Предпочесть малый typed контракт рядом с существующими Fact/Holds/Observed, а не новый assertion DSL. Никаких aliases разных spellings и универсальных plugin engines.

## 4. Инвентарь исходных норм и структурное покрытие

Текущие SourceLinks проверяют лишь объявленные ссылки. Необъявленный исходный пункт невозможно найти таким обходом. Поэтому нужна явная source-owned inventory нормы:

```text
source clause → один или несколько атомов → методы → cases → наблюдения/findings
```

Каждая исходная нормативная единица имеет стабильный clause key, source ID/version/hash и настоящий anchor. Ключ пункта не выводится из текста перевода, числа строк или имени метода. Заголовок/§-номер — адрес/представление, а не обязательная identity после редакторского переноса.

- Инвентарь авторится явно либо импортируется из существующего machine-readable source inventory по документированному контракту.
- Не считать regex/LLM-выделение MUST/SHOULD/MAY доказательством полного списка норм.
- Один исходный clause может требовать нескольких атомов; atom может иметь несколько source clauses. Дублированные keys, неизвестные links и source drift диагностируются.
- Нормативный inventory не умножается на число языков. Языковые anchors могут быть разными при общем logical clause key; каждый объявленный source fingerprint проверяется отдельно.
- Аудит выводит: clause без декомпозиции, атом без method/cases, неисполненные cases, расхождение, profile qualification, invalid source link.
- Обязательные стороны/условия clause явно адресуются, чтобы наличие одного happy-path case не представлялось покрытием всех условий.
- Статус «структурно покрыт относительно объявленного инвентаря» не равен семантической полноте всей естественно-языковой спеки и не равен exhaustive input coverage.
- Для домена без inventory отдельной полноты не обещать и не требовать; ordinary authored links продолжают работать.

## 5. Corpus и cases

Один case содержит identity, input reference/value, независимое expected/oracle reference, проверяемые atom IDs, выбранный профиль и provenance исходных fixtures. Fixture identity хранит category/stem и source content hashes; связанные input/expected/canonical/error файлы различаются по роли.

- Поддержать данные рядом с обычными Go tests и `t.Run`. Corpus loader не является вторым runner со своим test language.
- Нужен небольшой documented mapping, позволяющий одну fixture связать с несколькими атомами и один атом — с несколькими fixtures. Не копировать сам corpus в registry литералы и не писать по требованию на файл.
- Raw-byte input читается как bytes до UTF-8 decoding. Не терять flags/source metadata при загрузке.
- Валидация fixture/schema version не угадывает неизвестный формат. Ktav-specific categories/manifest adapter живёт в consumer, не в ядре HotamSpec; engine контракт общий.
- По возможности одна реальная операция реализации питает несколько проверок результата без повторного parse. Общее execution identity не означает, что все связанные атомы прошли.
- Каждый comparison адресует конкретное свойство результата: type/value, selected error category, line/span, canonical bytes, round-trip property и т.д.
- Результат corpus и состояние нормы показываются отдельно. «Все fixtures прошли» не превращается в «все нормы реализованы».
- Дополнительные boundary/negative/overlap cases имеют ту же identity/provenance модель и явно отличимы от upstream corpus.
- Метод/атом имеет устойчивую identity при добавлении case. Изменение input/expectation/hash не наследует старую finding review как будто данные не изменились.

## 6. Точные observations и сравнения

Строковый readable output оставить проекцией, не единственным носителем всех данных.

Минимальные typed представления:

- text и raw bytes; для bytes однозначные encoding + exact payload;
- scalar kind и точное значение, включая Integer/String различие;
- error code/class/reason, one-based line, half-open byte-span как отдельные поля;
- float domain, exact bits/representation для binary64, signed zero и точные decimal/canonical bytes где свойство это требует;
- структурированный Value/result и адрес конкретного сравниваемого свойства.

Правила:

1. Unknown/absent field отличать от zero/empty/null; не выдумывать Line/Span/Reason, которых producer не предоставил.
2. Invalid UTF-8 не проводить через JSON string с replacement rune; LF/CRLF/CR и байтовые offsets остаются точными.
3. Не включать неявную numeric tolerance или normalization в общий Eq. Comparison semantics объявляется явно; exact bytes, structural value и float bits — разные свойства.
4. Не сравнивать map iteration order с source-ordered canonical output и не объявлять потерянный через transport порядок сохранённым.
5. Raw SDK data и adapter-adjusted data — разные observations с собственным producer identity. Не переписывать raw result corrected result-ом.
6. При large payload отчёт может ссылаться на owned content-addressed artifact, но bytes/hash и provenance остаются доступными. Не строить отдельную абстракцию storage без реального требования размера.
7. Сохранить настоящий failure при любом вложенном discrepancy и passing siblings при package failure. Никаких fake FAIL/PASS summary вместо consumer-visible comparison.
8. Locale selection не меняет machine data, comparison, coverage, finding identity или review. В служебном тексте не переводить API/error identifiers и SDK messages как будто это actual.

Расширение JSON/schema и old consumers: clean cutover всех engine callers/canon/vendor, явная version contract; не повышать версии проекта/библиотек. Исторические reviews и snapshots не уничтожать ради миграции.

## 7. Приоритеты и взаимодействия атомов

Атомарная норма не означает отсутствие взаимодействия норм. Ввести typed отношение выбора/приоритета между атомами там, где спецификация явно задаёт ordered selection. Точный relation kind и scope фиксируются в К0.

- Связь включает scope/applicability, потому что «A раньше B» не обязана быть глобальной для всех operations/profiles.
- Проверять resolving references, дубли и невозможные cycles в объявленной strict precedence области.
- Не выводить приоритет из порядка методов в файле, doc-пунктуации, `depends_on` или совпавших слов.
- Саму норму приоритета отражать отдельным методом/атомом взаимодействия; отношение в графе не является поведенческим доказательством.
- Cases конкуренции должны действительно удовлетворять основаниям нескольких правил, иначе они не проверяют выбор.
- Для InvalidUtf8-before-grammar проверять raw-byte witness с конкурирующим grammar defect и фактическую выбранную категорию.
- Для scalar inference/fallback проверять boundary/overlap witnesses. Нельзя замаскировать изменение порядка тем, что отдельные happy paths проходят.
- Graph-проекция и документация показывают порядок и scope; counterexample сохраняет matched conditions/selected branch, когда producer/модель их действительно наблюдали, без выдуманной backend trace.
- Для обычного домена без precedence declarations никакой новой обязанности.

## 8. Обязательность, применимость и профили

Отдельные оси, не одна enum-строка «pass/not applicable»:

- normative strength: MUST / SHOULD / MAY, авторская декларация со source link;
- applicability condition: operation, features и выбранный профиль;
- capabilities/profile: например, Integer range, Float domain, rounding semantics, order preservation, exposed configuration;
- исполнение: pass/fail/skip/unavailable;
- existing coverage qualification: unsupported recommendation, justified unreachable, unverified и discrepancy.

Applicability не вычислять из failed result. False condition не делает атом verified: это явно неприменимая обязанность для указанного selection, отличная от доказанного unreachable branch и от ещё не запущенного case.

- Profile имеет стабильный ID и typed декларацию значимых возможностей/параметров; raw string недостаточна для механического сравнения capability matrix.
- Нельзя считать MAY-расширение обязательным для всех backend или MUST условной нормы исчезнувшим из-за недостающей feature.
- Отсутствующая рекомендация SHOULD остаётся явно указанной unsupported recommendation с rationale; обязательное нарушение остаётся discrepancy.
- Unreachable требует rationale/source/profile; math rationale не превращать в наблюдённый тест и не распространять на другой профиль.
- Инвентарь/coverage matrix различает обязательные, рекомендуемые, optional и условные пункты в каждом profile; определения относятся к одной норме, а не создают дубликаты atoms.
- Новые qualifications/selection semantics проходят через validation/report/render/proposals и не подавляют уже наблюдённый failure.
- Без явных profile/capability declarations сохранить текущий evidence context и не заявлять capability-complete audit.

## 9. Состав проверяемой реализации и ответственность

Вместо одного неразличимого «implementation version» дать возможность объявить composition:

```text
model → adapter → language bindings → native core
```

Каждый реально участвующий компонент имеет ID, role, declared/measured version и при необходимости artifact/build hash; provenance сообщает, что измерено, а что заявлено автором. Case selection фиксирует проверяемую composition/profile.

- Native version, bindings version и adapter revision не подменяют друг друга.
- Прямой SDK run и составное приложение адресуются разными targets; общий input не делает их одним наблюдением.
- Наблюдение указывает producer/operation и фактически доступные данные; engine не угадывает виновный слой.
- Model issue / implementation issue / specification issue / needs_review остаётся отдельным human review. Не записывать automatic blame из текста ошибки.
- Model/oracle не должен реализовывать ещё один неполный parser вместо реального backend. Нормативные expected values и отдельные reference calculations допускаются лишь как независимые явные oracle; не делить с SUT ту же ошибочную реализацию сравниваемого правила.
- Один компонент может использоваться несколькими profiles/cases без повторного копирования metadata. Invocation-local normalized snapshot без нового persistent verdict cache.

## 10. Отчёты, source → graph и политики публикации

- Одна норма/граф и общий snapshot наблюдений; последующие языковые views берут их из многоязычного плана, не создают три case sets.
- Coverage/source inventory всегда показывают отсутствующие и непроверенные обязанности, а не только уже существующие links.
- Summary distinguish: declared clauses, decomposed atoms, selected cases, executed comparisons, discrepancies и profile qualifications. Один счётчик «проверок» не заменяет эти уровни.
- Не менять этим планом legacy публикацию value-fact SPEC. Для new rule mode источник текста нормы — doc метода, а статус исполнения/cases представлен отдельно; текст нормы не сочиняется из единственного actual sample. Отображение authored нормы не обозначается успешным proof без реально соответствующих результатов.
- Наличие cases/links и их passing output не доказывает universal quantification по всем входам. Не писать «полностью соответствует Ktav» по corpus pass.
- Изменение case/source/profile/composition участвует в соответствующем diff/fingerprint; stale or changed findings не наследуют human review.
- Sync dry-run/confirm handshake остаётся; новая graph структура проходит canonical/vendor/merge/history checks и named enforcers с реальным self-hosting carrier.
- Публичные machine-readable CLI контракты обновляются целиком. Нет source-text/golden/count-only тестов вместо consumer-visible behavior.

## 11. Этапы реализации и отдельные задачи сессии

Каждый К-пункт — отдельная задача с тем же названием. Текущий запрос — план и регистрация, не старт реализации.

### К0. Зафиксировать общий контракт atom/rule/case и карту затронутых callers.

Reference inventory, typed identities/schema/own triggers, preservation legacy Fact, связи с М0–М4 multilingual-плана. Приёмка: нет implicit переключения kinds, exact equality и language-neutral identity; все boundary/API decisions названы.

### К1. Реализовать typed rule atoms и стабильные case identities без изменения legacy value-facts.

Ontology/canon/vendor, source/recorder metadata, validation и collision handling. Приёмка: два корректных разных результата одного rule atom допустимы; разные значения одного legacy value-fact не объединяются молча; один method/ID и независимые case expectations.

### К2. Провести cases через discovery, общий execution snapshot и Go → graph sync.

Atom source/derivation, Test/subtest links, merge/diff/history/confirm hashes, strict proposals; no per-case graph copies. Приёмка: case-only edits адресуются и видны; true failures fail; language/case views не умножают методовые вызовы.

### К3. Добавить явный source clause inventory и resolving связи clause → atom.

Source-owned keys/version/hash/anchors, many-to-many links, optional inventory trigger. Приёмка: пропущенный clause обнаружим относительно инвентаря; stale/unknown links диагностируются; no inferred semantic completeness.

### К4. Реализовать структурный аудит покрытия норм, условий и cases.

Missing decomposition/method/cases, explicit sides/conditions, discrepancies и profile qualifications. Приёмка: happy path и corpus pass не скрывают непривязанные обязанности; legacy no-inventory domain не получает новых требований.

### К5. Добавить общий corpus provenance и удобную fixture → cases → atoms привязку.

Typed references/hashes/roles/flags и обычные Go table/subtests; domain-specific loader contracts. Приёмка: одна fixture проверяет несколько атомов без duplication нормы/parse; дополнительные cases отличимы от upstream; нет Ktav-specific category engine code.

### К6. Реализовать точные typed observations и явные comparison semantics.

Bytes/text/kind/error fields/spans/float bits, absence semantics, canon/JSON/report migration. Приёмка: invalid bytes/CRLF/signed zero/Integer vs String различимы; exact comparisons не заменены normalization/tolerance; raw/adjusted producer data разделены.

### К7. Добавить scoped precedence declarations и атомы взаимодействия правил.

Relations/validation/render, cycles и witness contracts. Приёмка: конкурирующие основания и branch priority реально проверяются; ordered inference не приписывается depends_on; no fake backend trace.

### К8. Реализовать normative strength, applicability и typed profile/capability matrix.

Conditional obligations, SHOULD/MAY, non-applicable vs unreachable/unverified, source rationale. Приёмка: profile selection не скрывает failures; qualifications остаются честными и адресуемыми; legacy context сохранён.

### К9. Добавить composition provenance модели, adapter, bindings и core.

Declared/measured versions/hashes/roles, case target/producer, fingerprints и review separation. Приёмка: прямой SDK и composed app различаются; adapter correction не доказывает pristine backend; no automatic blame.

### К10. Обновить evidence/findings/coverage/source reports и их CLI machine contracts.

Общие snapshots, separate atom/case/comparison levels, source inventory debt, typed raw observations, review identity и multilingual handoff. Приёмка: failed case и passing siblings видимы; no corpus-equals-compliance; locale не меняет execution или review.

### К11. Обновить self-hosting anchors, engine документацию и все affected callers.

AUTHORED-SPEC-CONTRACT/QUICKSTART/README/PROPOSAL-REFERENCE/CHANGELOG и generated workflows, canonical carriers и sync-self. Приёмка: complete cutover, no stale aliases/proofs; обычный Fact и простой doc остаются удобными; Ktav/apps untouched.

### К12. Проверить реальные consumer CLI сценарии атомов, corpus и source inventory.

Собственные fixtures без Ktav run: varied results, unexpected mismatch, boundary/negative/overlap, source clause omission, raw bytes/error offsets, profile conditions, direct/composed producer, fresh/changed review. Приёмка: реальные exit/state/output, deterministic isolated regressions и удалённые throwaway artifacts.

### К13. Проверить совместную работу conformance и multilingual контрактов.

Один atom/case/execution set, несколько doc languages; case/source/profile edits и language-only edits; общий raw JSON/review. Приёмка: никакого умножения requirements/executions и никакого переноса ошибочного статуса между views; прежний plain single-language путь сохранён.

### К14. Пройти полный интеграционный прогон HotamSpec и зафиксировать приёмку.

После source freeze: build, go vet ./..., полный go test -timeout 30m ./..., live CLI smoke и self-hosting checks. Приёмка: все §13; каждое падение/флейк исправлено в том же ходе; Ktav не возобновлялся; no commits/push/version changes.

## 12. Порядок и связь с многоязычностью

- К0 совместно с М0: один text/identity/execution контракт, не две несовместимые модели.
- К0 → К1 → К2. К3 после К0; К4 после К2/К3.
- К5 и К6 после К1/К2. К7 после К2/К3. К8 после К0/К3/К4. К9 после К0/К2/К6/К8.
- К10 после К4/К5/К6/К7/К8/К9; К11 после contract freeze; К12 после К10/К11.
- М1/М2 могут не зависеть от полной реализации conformance; М3/М4 должны использовать уже зафиксированные К0 и К1/К2 контракты.
- М6–М10 не строить поверх нового per-witness Claim confounding. К10 и М7/М8 используют один report/data contract.
- К13 после К12 и готовности М6–М10/М12; финальные К14/М13 проводят один согласованный full-suite freeze, не два конкурирующих прогона.
- М0–М13 остаются заведёнными задачами; этот план не заменяет и не закрывает их. Порядок обоих планов определяется реальными зависимостями, а не механически «сначала весь один, потом весь другой».
- Возможные независимые slices не являются разрешением запуска подагентов; только явный запрос пользователя.

## 13. Полная приёмка

1. Legacy одноязычный doc и Fact не требуют новых kinds/config/tags, не меняют identity/layout/semantics.
2. Rule atom проходит several independent cases с разными корректными actual и не получает конфликт sample-dependent Claim.
3. Case mismatch действительно делает test failed и finding discrepancy; нельзя получить green постоянным summary или скрытием nested comparison.
4. Источники expectations независимы от actual; одним SDK result не вырабатывается собственный oracle.
5. Atom/case/comparison identities различимы; добавление case не меняет норму и её ID, language selection не меняет runtime data.
6. Source inventory audit видит явно объявленный clause без atom/case и не обещает coverage вне своего инвентаря.
7. Corpus references сохраняют hashes/roles/raw-byte flags; many-to-many fixture/atom links не создают duplicate requirements.
8. Corpus pass не обозначается полной conformance; boundary/overlap cases равноправно сообщают нарушения.
9. Raw invalid UTF-8, line endings, spans, scalar kinds, signed zero/float bits не теряются в JSON/Markdown форматировании; missing producer fields не выдумываются.
10. Scoped precedence имеет structural validation и реальный competing-case behavioral witness, без автоматической семантической догадки.
11. MUST/SHOULD/MAY, applicability, unsupported, unreachable и unverified различаются; наблюдённый failure не подавляется profile декларацией.
12. Profile и composition достаточно точны, чтобы отличить direct SDK от adapter-adjusted result; attribution остаётся human review.
13. Changed source/case/profile/composition obey fingerprint/review rules; locale-only rendering не меняет finding identity; отсутствует persistent verdict cache.
14. Все новые obligations имеют собственные явные triggers; старый домен не получает новые duties из уже spent opt-in.
15. Multilingual views содержат те же атомы/cases/links/statuses, один общий JSON/review и один execution snapshot.
16. Ontology/canon/vendor/loader/parser/sync/history/report/docs/self-hosting мигрированы полностью, без obsolete aliases/shims/stubs.
17. Реальные CLI smoke, полный vet/test и self-hosting checks пройдены. Кtav/backend/apps не запускались и не изменялись.

## 14. Риски и правила проверки

Главные риски: превращение runtime values в норму; implicit change legacy Fact; cases размножают parse/test runs; shared oracle с SUT; missing clauses исчезают из audit; byte payload превращается в replacement text; predicate-only precedence; profile masks failure; composed-app proof объявлен pristine SDK proof; changed finding наследует чужой review; язык меняет identity.

Permanent regression checks должны ловить consumer-visible boundaries/transitions/errors/precedence/identity и изменения файлов/графа. Не проверять registry counts, copies, rendered wording или source text и не перепинивать golden hashes. Исторические observations не перезапускать ради подтверждения пользовательских ошибок; выполнять только необходимые проверки изменённого пути в разрешённой области.

Не расширять план в собственный parser, test language, универсальный plugin runtime, автопереводчик, persistent cache, backend patch или незапрошенную миграцию приложений. Если реализация требует materially different API/семантического выбора, показать tradeoff, не выдавать фиктивное закрытие задачи.

## 15. Реализация и проверка — 2026-10-05

К0–К14 выполнены и проверены. Delta из 140 файлов перенесён в основной checkout
без перезаписи исходных пользовательских изменений.

- Rule с корректными `false`/`true` cases сохраняет одну норму; два разных
  legacy value-fact результата дают реальный discovery conflict.
- Case-only edit меняет только `Cases` и sync hash. Реальный mismatch даёт
  exit 1 и discrepancy; passing siblings остаются в raw report.
- Typed consumer: 4 norms, 7 уникальных cases, 11 comparisons, 0 discrepancies.
  Invalid UTF-8, LF/CRLF/CR, `int64` max и signed-zero bits сохранены. Integer
  против String с одинаковым читаемым `1` остаются разными raw значениями.
- Valid pinned source с тремя clauses обнаруживает unmodeled clause и
  отсутствующую side; изменение source bytes даёт drift и source-unverified.
- Реальный `corpus.Read` для input/expected/canonical/error/extra возвращает
  точные `ff00`; неправильный SHA и выход за root отвергаются.
- Реальный competing selection подтверждает priority witness; ошибочная ветка
  даёт discrepancy, scoped cycle отвергается loader. Shared witness считается
  один раз, explicit empty `matched: []` читается обратно из report JSON.
- MUST unsupported без исполнения — violation; SHOULD qualification и false
  applicability не скрывают наблюдённые failures.
- Direct и adapter-adjusted observations сохраняют original/adjusted bytes и
  declared/measured components. Drift adapter pin не меняет direct finding IDs;
  attribution остаётся review, не автоматическим обвинением.
- Locale default не меняет raw cases/IDs/review store. Case/profile changes
  инвалидируют соответствующий review, включая unexecuted case, без fabricated
  recorded context.
- `internal/conformance`, `internal/evidence`, `internal/gate` suites пройдены.
  Frozen build/vet и `go test -p 1 -timeout 30m ./...` пройдены: 26 пакетов
  `ok`, 2 без тестов. Main binary повторно собран; canonical checks возвращают
  `[]`, raw consumer — 4 norms / 7 cases / 11 comparisons / 0 discrepancies.

Ktav/backend/apps не запускались и не изменялись; commits/push/version bumps нет.
Собственные восемь worktrees/branches и throwaway fixtures удалены; receipts
остаются в этих планах и CHANGELOG, не в production test scaffolds.
