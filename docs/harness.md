# Test harness

Harness живёт в том же процессе, что и коллектор. Состояние — SQLite (`URSA_TZSP_DB_PATH`). Управление — UI `/` или REST `/api/v1`.

## Forward

`internal/forwarder`: UDP proxy для **request**-пакетов (Access-Request code 1 → auth target, Accounting-Request code 4 → acct target).

- По умолчанию **`enabled: false`**.
- Defaults targets из env; runtime-конфиг в `settings`.
- Держит постоянные UDP-сокеты и **ждёт ответы**. Request+response склеиваются в conversation (по identifier, таймаут 5s).
- Исходящий запрос уходит с IP/порта харнесса; **NAS-IP-Address** подменяется на локальный адрес сокета, чтобы сервер отвечал сюда (Access-Request без Message-Authenticator — всегда; Accounting / Message-Authenticator — если в каталоге NAS есть secret, пакет переподписывается).
- Ответы появляются в UI Forward и в Live-ленте (`source: forward-response`).

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

Таблицы `nas`, `clients`. Autofill (`catalog.Autofill`) подсматривает decoded ленту и может дополнять каталог.

Имя NAS:

- нет `Proxy-State` — `NAS-Identifier`;
- есть `Proxy-State` — `PROXY-<Symbol-Device-RF-Domain>` (запись ведётся по UDP source, т.е. RADIUS client / proxy);
- у того же client IP несколько RF-Domain — имя становится `PROXY-VX`.

## Synthesize

`internal/synth`: по NAS + clients строит RADIUS-пакеты (`layeh.com/radius` + словари).

Patterns:

- `auth_only` — Access-Request;
- `acct_only` — accounting без auth;
- `full_session` — auth + Start / Interim / Stop.

Параметры: rate, число сессий, interim interval, session duration, `mirror_to_stream`.

Секреты NAS из каталога используются при сборке synth-пакетов и при forward: Accounting-Request / Message-Authenticator переподписываются, если secret найден по NAS-IP.

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
