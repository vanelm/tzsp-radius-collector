# Test harness

Harness живёт в том же процессе, что и коллектор. Состояние — SQLite (`URSA_TZSP_DB_PATH`). Управление — UI `/` или REST `/api/v1`.

## Forward

`internal/forwarder`: verbatim UDP send **только request**-пакетов (Access-Request code 1 → auth target, Accounting-Request code 4 → acct target).

- По умолчанию **`enabled: false`**.
- Defaults targets из env; runtime-конфиг в `settings`.
- Authenticator **не** пересчитывается — байты как есть.
- Ответы RADIUS не форвардятся и не ожидаются.

Replay и synth вызывают `ForwardBytes` → тот же путь `MaybeForward`. Если форвардер выключен, UDP **не** уходит; при `mirror_to_stream` пакеты всё равно появляются в WebSocket.

## Record

`internal/recorder`: при активной записи сохраняет request-пакеты (`recording_packets`: blob, code, src/dst, delta_ms).

- Одна активная запись за раз.
- Stop фиксирует `stopped_at` / `packet_count`.

## Replay

`internal/replay`:

- `rate_rps` — равномерная отправка;
- `preserve_timing` + `speed` — по `delta_ms` записи;
- `loop` — цикл;
- `mirror_to_stream` — `emit` с `source: "replay"` в orchestrator.

Снова: UDP только при включённом forwarder; authenticator verbatim.

## Catalog

Таблицы `nas`, `clients`. Autofill (`catalog.Autofill`) подсматривает decoded ленту и может дополнять каталог. Export/import JSON через API.

## Synthesize

`internal/synth`: по NAS + clients строит RADIUS-пакеты (`layeh.com/radius` + словари).

Patterns:

- `auth_only` — Access-Request;
- `acct_only` — accounting без auth;
- `full_session` — auth + Start / Interim / Stop.

Параметры: rate, число сессий, interim interval, session duration, `mirror_to_stream`.

Секреты NAS используются при сборке пакетов там, где библиотека их требует; при forward наружу пакеты всё равно уходят как собранные байты без повторного «подписания» под чужой секрет на стороне forwarder.

## SQLite схема

Миграция: `internal/store/migrations/001_init.sql`.

| Таблица | Назначение |
|--------|------------|
| `nas` | каталог NAS |
| `clients` | MAC / username |
| `recordings` | метазаписи |
| `recording_packets` | бинарные пакеты + тайминг |
| `scenarios` | сохранённые JSON-конфиги |
| `settings` | key/value (forwarder_config) |

Драйвер: `modernc.org/sqlite` — **CGO не нужен**.
