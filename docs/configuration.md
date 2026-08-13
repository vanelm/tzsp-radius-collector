# Конфигурация

Все переменные с префиксом `URSA_`. Загрузка: `internal/config/config.go`. Пример: [`.env.example`](../.env.example).

## HTTP / идентичность / очереди

| Переменная | Default | Описание |
|------------|---------|----------|
| `URSA_APP_ENV` | `development` | влияет на уровень логов (development → debug) |
| `URSA_LOG_LEVEL` | _(пусто)_ | override: `debug` / `info` / `warn` / `error` |
| `URSA_TZSP_HTTP_LISTEN` | `:8098` | HTTP + WebSocket + UI |
| `URSA_TZSP_WS_PATH` | `/ws` | путь WebSocket |
| `URSA_COLLECTOR_ID` | `tzsp-radius-collector` | id в hello; fallback на `URSA_TZSP_MDNS_NAME` |
| `URSA_COLLECTOR_FEED_TYPE` | `radius` | поле `feed_type` в hello |
| `URSA_COLLECTOR_FEED_VERSION` | `1` | версия ленты |
| `URSA_COLLECTOR_SCHEMA_ID` | URL схемы на GitHub Pages | `schema_id` в hello |
| `URSA_TZSP_EVENT_QUEUE` | `1024` | буфер канала capture → orchestrator |
| `URSA_TZSP_CLIENT_QUEUE` | `128` | буфер на WebSocket-клиента; overflow → disconnect |

## Capture

| Переменная | Default | Описание |
|------------|---------|----------|
| `URSA_TZSP_ENABLE_UDP` | `true` | TZSP UDP ingest |
| `URSA_TZSP_UDP_LISTEN` | `:37008` | адрес listen |
| `URSA_TZSP_ENABLE_RAW_SNIFF` | `false` | AF_PACKET sniff |
| `URSA_TZSP_SNIFF_IFACE` | `eth0` | интерфейс |
| `URSA_TZSP_SNIFF_FILTER` | `udp and (port 1812 or port 1813)` | **не используется** кодом (см. ограничения) |
| `URSA_TZSP_SNIFF_PROMISCUOUS` | `false` | **не включается** на afpacket-бэкенде |

## Словари

| Переменная | Default |
|------------|---------|
| `URSA_TZSP_DICTIONARY_GLOB` | `./config/dictionary/dictionary.*` |

В Docker по умолчанию: `/app/config/dictionary/dictionary.*`.

## mDNS

| Переменная | Default |
|------------|---------|
| `URSA_TZSP_ENABLE_MDNS` | `true` |
| `URSA_TZSP_MDNS_NAME` | `tzsp-radius-collector` |
| `URSA_TZSP_MDNS_PORT` | `8098` |

Сервис: `_tzsp_collector._tcp`. В TXT: `ws_path`, `feed_type`, `collector_id`, `schema_id`, `capture_modes`.

## Test harness / SQLite

| Переменная | Default | Описание |
|------------|---------|----------|
| `URSA_TZSP_DB_PATH` | `./data/collector.db` | SQLite |
| `URSA_TZSP_FORWARD_AUTH_TARGET` | `127.0.0.1:1812` | default auth target форвардера |
| `URSA_TZSP_FORWARD_ACCT_TARGET` | `127.0.0.1:1813` | default acct target |

Сами targets — стартовые defaults в памяти; runtime-конфиг (включая `enabled`) хранится в таблице `settings` и переживает рестарт. **Enabled по умолчанию false**, пока не включат через UI/API.

## Булевы значения

`1` / `true` / `yes` / `on` и `0` / `false` / `no` / `off` (регистр не важен). Пустое → default.
