# TZSP RADIUS Collector

Small collector that ingests TZSP and raw-sniffed RADIUS traffic, decodes attributes using dictionary files, and streams JSON over WebSocket subscriptions.

## Features

- TZSP UDP ingest (`URSA_TZSP_ENABLE_UDP=true`)
- Optional raw interface sniff (`URSA_TZSP_ENABLE_RAW_SNIFF=true`)
- Dictionary-based attribute decode (standard + VSA)
- Accounting snapshot extraction
- WebSocket stream endpoint with subscription filters
- mDNS advertisement on `_tzsp_collector._tcp`
- Structured JSON logging via `slog` (Vector-friendly)

## Configuration

All service variables are URSA-prefixed:

- `URSA_TZSP_HTTP_LISTEN` default `:8098`
- `URSA_TZSP_WS_PATH` default `/ws`
- `URSA_APP_ENV` default `development`
- `URSA_LOG_LEVEL` optional override (`debug`, `info`, `warn`, `error`)
- `URSA_TZSP_ENABLE_UDP` default `true`
- `URSA_TZSP_UDP_LISTEN` default `:37008`
- `URSA_TZSP_ENABLE_RAW_SNIFF` default `false`
- `URSA_TZSP_SNIFF_IFACE` default `eth0`
- `URSA_TZSP_SNIFF_FILTER` default `udp and (port 1812 or port 1813)`
- `URSA_TZSP_SNIFF_PROMISCUOUS` default `false`
- `URSA_TZSP_DICTIONARY_GLOB` default `./config/dictionary/dictionary.*`
- `URSA_TZSP_ENABLE_MDNS` default `true`
- `URSA_TZSP_MDNS_NAME` default `tzsp-radius-collector`
- `URSA_TZSP_MDNS_PORT` default `8098`

## WebSocket

Connect to `ws://host:8098/ws` and send:

```json
{"action":"subscribe","filter":{"codes":["Accounting-Request"],"macs":["AA:BB:CC:DD:EE:FF"]}}
```

Filter fields:

- `codes`
- `user_names`
- `nas_ips`
- `vendors`
- `macs`
- `status_types`

Feed JSON Schema:

- `docs/collector-feed.schema.json`
- Models standard vs VSA decoded attributes without enumerating vendor dictionary value lists.

## Health

`GET /healthz`

## mDNS check

```bash
avahi-browse -art | grep tzsp_collector
```

## Observability Notes

- Logs are emitted as JSON to `stdout` using `slog`.
- Field names follow `snake_case` and include component labels for easier routing.

## Local Docker

Start the collector:

```bash
docker compose up -d --build
```

Exposed ports: `8098` (HTTP/WS), `37008/udp` (TZSP ingest).

### Dozzle (optional log viewer)

Copy the override file so `docker compose` picks it up automatically:

```bash
cp docker-compose.override.devel.yml docker-compose.override.yml
docker compose up -d
```

Then open http://localhost:9999 for a web-based container log viewer.

`docker-compose.override.yml` is git-ignored — the override is opt-in per developer.
