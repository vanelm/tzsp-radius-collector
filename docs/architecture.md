# Архитектура

## Поток данных

```
TZSP UDP (:37008) ──┐
                    ├──► pipeline.Event ──► Orchestrator ──┬──► Recorder (requests)
AF_PACKET sniff ────┘         │                            ├──► Forwarder (if enabled)
                              │                            ├──► Processor → StreamMessage
replay / synth ───────────────┘                            ├──► Catalog Autofill
                                                           └──► Hub → WebSocket clients
```

Точка входа: `main.go`.

1. Загрузка словарей FreeRADIUS (`radiusdecode.LoadAttributeDictionaryByGlob`).
2. SQLite (`store.Open`) — каталог, записи, настройки форвардера.
3. Capture → канал `events` → `runtime.Orchestrator`.
4. HTTP: WebSocket (`stream`), REST harness (`api`), UI (`webui` embed).

## Пакеты `internal/`

| Пакет | Роль |
|-------|------|
| `config` | env `URSA_*` |
| `capture` | TZSP UDP + raw sniff (gopacket/afpacket) |
| `tzsp` | разбор заголовка TZSP |
| `pipeline` | `Event`, `StreamMessage`, decode → JSON |
| `radiusdecode` | словарь, пакет, accounting/identity snapshot |
| `runtime` | оркестрация: record/forward/decode/publish |
| `stream` | Hub, фильтры, hello, `/ws`, `/healthz` |
| `forwarder` | verbatim UDP forward запросов |
| `recorder` | запись request-пакетов в SQLite |
| `replay` | воспроизведение записей |
| `synth` | синтез auth/acct сессий из каталога |
| `catalog` | автозаполнение NAS/клиентов из ленты |
| `store` | modernc.org/sqlite, миграции |
| `api` | REST `/api/v1` |
| `discovery` | mDNS `_tzsp_collector._tcp` |
| `webui` | embed `dist/` (HTML/CSS/JS) |

## Capture

### TZSP UDP (`capture/tzsp_udp.go`)

- Слушает `URSA_TZSP_UDP_LISTEN`.
- `tzsp.Parse` → payload; если это Ethernet/IP/UDP на 1812/1813 — извлекает RADIUS и `src_addr`/`dst_addr`.
- Если payload уже похож на RADIUS — отдаёт как есть (без L3 адресов).

### Raw sniff (`capture/raw_sniff*.go`)

- `afpacket.NewTPacket` на `URSA_TZSP_SNIFF_IFACE` — только в сборке `linux && cgo` (`raw_sniff_afpacket.go`).
- Без CGO (`raw_sniff_stub.go`) — warning и выход; Docker-образ по умолчанию такой.
- **Не** применяет `URSA_TZSP_SNIFF_FILTER` (переменная только в конфиге).
- Promiscuous: при `true` — warning, режим не включается.
- Фильтрация: UDP + `looksLikeRadius` (длина ≥ 20 и поле Length согласовано).

## Decode

`pipeline.Processor.Transform`:

- парсит RADIUS header → `radius` (code, identifier, length, authenticator hex);
- декодирует атрибуты по словарю (в т.ч. VSA);
- строит `accounting` snapshot (или identity fallback для non-acct).

## Orchestrator

Для **request**-пакетов (`IsRequest`): сначала `recorder.MaybeRecord`, затем `forwarder.MaybeForward`.  
Далее transform → catalog observe → `hub.Publish`. Ошибки парсинга — событие в ленту не попадает (только debug-лог).

## Зависимости без CGO

- SQLite: `modernc.org/sqlite` (pure Go).
- TZSP UDP + decode/stream/harness: без CGO.
- Raw sniff: `gopacket/afpacket` **требует CGO** (не libpcap). В `CGO_ENABLED=0` — stub.
- RADIUS helper: `layeh.com/radius` (сборка пакетов в synth).
