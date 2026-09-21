# logcollector

Триажный сборщик следов с Windows-машины для DFIR: один статический бинарник на Go,
который за один запуск от администратора собирает всё нужное для расследования
инцидента и упаковывает в zip с манифестом.

```
Windows-машина ──► EVTX-экспорт + следы удалёнки + логи АВ + Run-ключи ──► <хост>.zip
```

Инструмент только **читает**: ничего не изменяет на машине — не чистит журналы,
не правит реестр, не удаляет файлы.

## Что собирает

**Журналы событий (EVTX)** — через `wevtutil epl`, 10 журналов:

Security, System, Application, RDP-журналы (RemoteConnectionManager,
LocalSessionManager), PowerShell/Operational, WinRM, TaskScheduler, WMI-Activity,
Sysmon (если установлен).

**Следы удалённого доступа** — 11 инструментов:

AnyDesk, Ammyy, RuDesktop (по официальным путям документации: сервисные логи,
пользовательские логи, конфиги, peers), Radmin, TeamViewer, MeshAgent, Supremo,
AeroAdmin, UltraViewer, DWService, LogMeIn.

**Логи антивирусов** — 6 вендоров:

Kaspersky, DrWeb, AVG, Avast, ESET, Windows Defender (MPLog + история сканов —
детекты и отключения защиты).

**Run-ключи автозапуска** — персистентность вредоноса:

HKLM/HKCU `Run` и `RunOnce` + Wow6432Node — экспорт через `reg export`
и сводный `run_keys.json` (hive/path/name/data).

## Установка

Требуется Go 1.27+.

```bash
go install github.com/81beastie/logcollector/cmd/logcollector@latest
```

Бинарник кладётся в `$(go env GOPATH)/bin` (обычно `~/go/bin`).
Если не находится в shell — добавь каталог в `PATH`:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

### Сборка под Windows (кросс-сборка с любой машины)

```bash
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o logcollector.exe ./cmd/logcollector
```

### Обновление и проверка версии

```bash
go install github.com/81beastie/logcollector/cmd/logcollector@latest
logcollector -version
```

Версия берётся из build info (тега релиза), локальная сборка показывает `dev`.

## Использование

На исследуемой машине, **от администратора** (нужно для `wevtutil epl Security`
и HKLM-веток реестра):

```cmd
logcollector.exe D:\triage
```

Итог: `D:\triage\<имя-хоста>.zip` — EVTX, следы удалёнки, логи АВ, `.reg`-файлы,
`run_keys.json` и `manifest.json` (хост, время запуска, список артефактов с размерами).

Отчёт в консоли: собранные артефакты (`+`) и ошибки (`!`) — отсутствие инструмента
на машине не ошибка, а тишина; ошибка экспорта не останавливает сбор остального.

### Флаги

| Флаг | По умолчанию | Описание |
|---|---|---|
| `-out` | `.` | каталог для итогового zip |
| позиционный аргумент | — | то же, что `-out` |
| `-no-eventlog` | — | не экспортировать журналы событий |
| `-no-remote` | — | не собирать следы удалёнки и антивирусов |
| `-no-registry` | — | не собирать Run-ключи автозапуска |
| `-version` | — | версия (из тега релиза, локальная сборка — `dev`) |
| `-h`, `-help` | — | справка |

### Замечания для DFIR

- Инструмент **не минимизирует** следы своего присутствия: запуск фиксируется
  в журналах — учитывай при планировании сбора;
- бинарник не подписан цифровой подписью — на машинах с агрессивным АВ
  заранее внеси его в исключения/доверенную зону;
- вывод консольных утилит Windows (`wevtutil`, `reg`) приходит в OEM-кодировке
  (CP866 на русской локали) — декодируется в UTF-8 автоматически;
- для передачи заказчику зафиксируй SHA256 бандла в акте.

## Архитектура

Clean Architecture, единственная внешняя зависимость — `golang.org/x/text`
(декодирование CP866):

```
cmd/logcollector/        CLI: флаги → оркестрация → zip
internal/domain/         модели: Artifact, Bundle, Report; реестры источников
internal/collector/      интерфейс Collector + Runner (ошибка источника не валит сбор)
internal/eventlog/       экспорт EVTX через wevtutil epl
internal/registry/       Run-ключи: reg export + парсер .reg + run_keys.json
internal/remotefiles/    раскрытие %Var% + glob-резолв путей
internal/remotesrc/      копирование следов удалёнки и АВ по реестру
internal/archive/        zip с манифестом
internal/wincp/          декодирование OEM/CP866 → UTF-8
```

## Тесты

```bash
go test ./... -cover
```

| Пакет | Покрытие |
|---|---|
| `internal/domain` | 100% |
| `internal/eventlog` | 95% |
| `internal/remotefiles` | 90% |
| `internal/registry` | 81% |
| `internal/archive` | 82% |
| `internal/collector` | 79% |
| `internal/remotesrc` | 76% |
| `internal/wincp` | 83% |
| `cmd/logcollector` | 86% |

## Релизы

Каждый merge в `main` автоматически публикует patch-релиз (CI-воркфлоу
`.github/workflows/release.yml`: gofmt → vet → тесты → гейт покрытия 70% →
кросс-сборка windows/darwin → следующий semver-тег). Поэтому
`go install @latest` всегда соответствует актуальному `main`. Minor/major теги
(`v0.2.0`, `v1.0.0`) ставятся вручную при смене API или флагов.

Чтобы пропустить публикацию релиза для конкретного merge, добавь
`[skip release]` в сообщение коммита.

## Лицензия

[MIT](LICENSE) — [81beastie](https://github.com/81beastie)

## AI-assistance

Этот проект разрабатывался в паре с AI-ассистентом [Koda](https://kodacode.ru)
(команда NLP-Core-Team).

- значительная часть кода написана в диалоге с Koda
- все фичи прошли цикл TDD: сначала падающие тесты, затем реализация
- синтаксис `wevtutil epl` и пути логов RuDesktop выверены по официальной
  документации в ходе полевых испытаний на реальной машине

Автор проекта ревьюил и принимает ответственность за весь код.
