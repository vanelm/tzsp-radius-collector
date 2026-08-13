# tzsp-radius-collector — документация

Go-сервис **tzsp-radius-collector**: принимает RADIUS через TZSP UDP (`:37008`) или AF_PACKET, декодирует атрибуты по словарям FreeRADIUS и стримит JSON в WebSocket. Плюс test harness — форвардинг запросов, запись/replay, каталог NAS/клиентов, синтез сессий. UI вшит в бинарник на `/`.

## Оглавление

| Файл | О чём |
|------|--------|
| [architecture.md](architecture.md) | пакеты и поток данных |
| [configuration.md](configuration.md) | env-переменные |
| [api.md](api.md) | REST `/api/v1` |
| [websocket.md](websocket.md) | hello, фильтры, формат ленты |
| [harness.md](harness.md) | forward / record / replay / synth / SQLite |
| [operations.md](operations.md) | Docker, логи, mDNS, ограничения |
| [collector-feed.schema.json](collector-feed.schema.json) | JSON Schema сообщений ленты |

Краткий обзор для пользователей — корневой [README.md](../README.md).

## Быстрый старт

```bash
docker compose up -d --build
```

- UI / API / WS: `http://host:8098/`
- TZSP UDP: `:37008`
- Health: `GET /healthz`

## Известные ограничения (по коду)

- `URSA_TZSP_SNIFF_FILTER` читается в конфиг, но **нигде не передаётся** — BPF на AF_PACKET не ставится; отбор — `looksLikeRadius` по UDP payload.
- `URSA_TZSP_SNIFF_PROMISCUOUS=true` только пишет warning: promiscuous mode **не поддерживается** текущим afpacket-бэкендом.
- Docker-сборка с `CGO_ENABLED=0`: raw sniff недоступен (`gopacket/afpacket` требует CGO; libpcap не нужен).
- Форвардер по умолчанию **выключен** (`enabled: false`). Replay/synth вызывают `ForwardBytes`, но UDP уходит только если форвардер включён.
- Authenticator при forward/replay/synth **не пересчитывается** — пакеты уходят verbatim (как записаны / собраны).
- JSON Schema ленты включает `src_addr` / `dst_addr` (есть и в реальном JSON).
