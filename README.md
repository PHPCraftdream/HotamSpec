# HotamSpecGo

Go-порт методологии Hotam-Spec — подхода к работе с конфликтующими бизнес-требованиями, моделируемыми как граф напряжений (tension graph). Противоречивые требования — не баг, а свойство модели: они держатся открытыми как узлы `Conflict`, а не тихо отбрасываются.

> **TODO(module-path):** module path в `go.mod` — `github.com/PHPCraftdream/HotamSpecGo` — унаследован от отдельного репозитория `HotamSpecGo`, слитого сюда; фактический git remote этого репозитория — `https://github.com/PHPCraftdream/HotamSpec.git` (без `Go` в имени) — расхождение не переименовано самовольно, требует явного решения стюарда.

## Install

```bash
go install github.com/PHPCraftdream/HotamSpecGo/cmd/hotam@latest
```

Кладёт бинарник `hotam` в `$GOBIN` (или `$(go env GOPATH)/bin`). Требует Go 1.25+ (см. `go.mod`). Из-за расхождения module path/remote выше, `@latest` сработает только когда module path реально резолвится на публичный Go-модуль-прокси под этим путём — до решения TODO проверяйте актуальность командой `go install <path>@<конкретный тег>` вместо `@latest`.

Сборка из исходников (для разработки или пока публикация module path не подтверждена) — см. раздел «Сборка» ниже.

### Версия

```bash
hotam version
# или
hotam --version
```

Печатает `hotam dev` для локальной сборки без флагов. Релизная сборка проставляет версию через `ldflags`:

```bash
go build -ldflags "-X main.version=v0.1.0" -o bin/hotam ./cmd/hotam
```

### Тегирование релиза (процесс, не выполнено в этой волне)

Когда стюард решает опубликовать версию: подтвердить/выправить расхождение module path ↔ remote (TODO выше) → прогнать `go test ./...` на чистом дереве → проставить git-тег вида `v0.x.y` на коммит → (опционально) собрать релизные бинарники с `-ldflags "-X main.version=v0.x.y"` для целевых платформ. Тег не создаётся автоматически этим документом — это ручной шаг стюарда.

## Сборка

```bash
go build -o bin/hotam ./cmd/hotam
```

или без сборки бинарника:

```bash
go run ./cmd/hotam <command> [flags] [args]
```

## Команды CLI

Бинарник `hotam` (см. `cmd/hotam/main.go`) поддерживает пять команд:

```bash
hotam gen-spec [--domain <path>]
        Сгенерировать docs/gen/*.md + graph.json для графа домена.

hotam what-now [--domain <path>] [--limit N]
        Показать top-N приоритизированных сигналов (по умолчанию 20).

hotam apply-proposal <proposal.json> --domain <path> --today YYYY-MM-DD
        Применить proposal к графу домена.

hotam land <proposal.json> --domain <path> --today YYYY-MM-DD
        Применить proposal, перегенерировать docs/gen и перепроверить инварианты за один шаг.

hotam gate <target-anchor> [--domain <path>]
        Выбрать Tier-1 подмножество тестов для целевого узла.

hotam all-violations [--domain <path>]
        Показать все нарушения инвариантов; exit 1, если есть хоть одно.

hotam version | hotam --version
        Показать версию бинарника (см. раздел Install → Версия выше).
```

`--domain` по умолчанию — `domains/hotam-spec-self`, путь резолвится относительно корня проекта.

## Тесты

```bash
go test ./...
go vet ./...
go test -race ./...
```

## Структура репозитория

```
cmd/hotam/            CLI-точка входа и подкоманды
internal/
  ontology/            типы узлов графа (Requirement, Conflict, Assumption, ...)
  loader/               чтение graph.json
  proposal/             валидация и применение Proposed*-структур
  invariants/           check_*-инварианты графа
  diagnose/              вычисление сигналов (what-now)
  gate/                    выбор Tier-1 тестов по якорю
  generator/               генерация docs/gen/*.md из графа
  methodology/             справочные данные методологии
  registry/                 реестр инструментов
  paths/                    резолюция корня проекта
domains/
  hotam-spec-self/       граф самоописания методологии
  hotam-dev/               граф разработки самого репозитория
docs/                      справочная документация (в т.ч. PROPOSAL-REFERENCE.md)
```

## Рабочий цикл агента

1. `hotam what-now` — узнать приоритетное следующее действие.
2. Составить JSON-предложение (proposal) — формат см. `docs/PROPOSAL-REFERENCE.md`.
3. `hotam apply-proposal <file.json> --domain <path> --today YYYY-MM-DD` — применить.
4. `hotam gen-spec --domain <path>` — перегенерировать документацию из графа.
5. `hotam all-violations --domain <path>` — убедиться, что граф остаётся структурно корректным.

Ручное редактирование `graph.json` не допускается — все изменения идут через `apply-proposal` (см. `CONTRIBUTING.md`).

## Лицензия

Dual-licensed под MIT OR Apache-2.0 — см. `LICENSE-MIT` и `LICENSE-APACHE`.
