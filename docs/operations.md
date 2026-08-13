# Operations

## Docker

Сборка **без CGO** по умолчанию (pure Go SQLite; TZSP UDP не требует CGO).  
`gopacket/afpacket` для raw sniff **требует CGO** (`import "C"`) — в Docker-образе raw sniff отключён stub’ом. Для sniff собирайте локально с `CGO_ENABLED=1` (gcc достаточно, **libpcap не нужен**).

В Dockerfile: `CGO_ENABLED=0`, `GOMAXPROCS=1`, `go build -p 1` — чтобы не съесть RAM на маленьких хостах (раньше `CGO_ENABLED=1` + gcc + libpcap давали OOM и роняли SSH).

```bash
docker compose up -d --build
```

Порты: `8098` (HTTP/WS/UI), `37008/udp` (TZSP).  
Данные: volume `./data` → `/app/data` (SQLite).

Если снова мало памяти перед сборкой:

```bash
docker builder prune
```

и не гонять параллельно тяжёлые контейнеры.

### Dozzle (опционально)

```bash
cp docker-compose.override.devel.yml docker-compose.override.yml
docker compose up -d
```

Логи: http://localhost:9999. `docker-compose.override.yml` в `.gitignore`.

## Логи

`slog` JSON на stdout. Поля `snake_case`, часто есть `component` (`collector`, `pipeline`, `forwarder`, …).  
Уровень: `URSA_APP_ENV=development` → debug; иначе info; override `URSA_LOG_LEVEL`.

Подходит для Vector/Loki (см. `config/vector.local.toml` как локальный пример).

## mDNS

При `URSA_TZSP_ENABLE_MDNS=true`:

```bash
avahi-browse -art | grep tzsp_collector
```

В Docker/bridge mDNS часто не виден с хоста — ожидаемо. Отключить: `URSA_TZSP_ENABLE_MDNS=false`.

## Raw sniff в контейнере

Стандартный Docker-образ собран с `CGO_ENABLED=0` → raw sniff **не работает** (stub). Нужен бинарь с `linux+cgo`, права на iface (`NET_RAW` / `NET_ADMIN` или `network_mode: host`) и `URSA_TZSP_SNIFF_IFACE`. По умолчанию raw sniff выключен.

## Ограничения (важно)

1. **`URSA_TZSP_SNIFF_FILTER` не применяется** — BPF не ставится; весь UDP с iface читается, отбор эвристикой RADIUS.
2. **Promiscuous mode не работает** на текущем afpacket-бэкенде.
3. **Forwarder off by default** — replay/synth без включения forward не шлют UDP наружу.
4. **Verbatim authenticator** — нет пересчёта Response/Request Authenticator при forward.
5. Schema vs runtime: в событиях есть `src_addr`/`dst_addr` (отражено в `docs/collector-feed.schema.json`).
6. Медленные WS-клиенты отключаются при переполнении клиентской очереди.
7. **Docker/CGO=0**: AF_PACKET raw sniff недоступен; используйте TZSP UDP.

## Локальная сборка без Docker

Только TZSP (как в Docker):

```bash
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o tzsp-radius-collector ./main.go
```

С raw sniff (нужен gcc):

```bash
CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -p 1 -o tzsp-radius-collector ./main.go
```
