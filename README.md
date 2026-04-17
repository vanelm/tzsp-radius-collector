# TZSP RADIUS Collector

Small collector that ingests TZSP and raw-sniffed RADIUS traffic, decodes attributes using dictionary files, and streams JSON over WebSocket subscriptions.

## Features

- TZSP UDP ingest (`URSA_TZSP_ENABLE_UDP=true`)
- Optional raw interface sniff (`URSA_TZSP_ENABLE_RAW_SNIFF=true`)
- Dictionary-based attribute decode (standard + VSA)
- Accounting snapshot extraction
- WebSocket stream endpoint with subscription filters
- mDNS advertisement on `_tzsp_collector._tcp`

## Configuration

All service variables are URSA-prefixed:

- `URSA_TZSP_HTTP_LISTEN` default `:8098`
- `URSA_TZSP_WS_PATH` default `/ws`
- `URSA_TZSP_ENABLE_UDP` default `true`
- `URSA_TZSP_UDP_LISTEN` default `:37008`
- `URSA_TZSP_ENABLE_RAW_SNIFF` default `false`
- `URSA_TZSP_SNIFF_IFACE` default `eth0`
- `URSA_TZSP_SNIFF_FILTER` default `udp and (port 1812 or port 1813)`
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

## Health

`GET /healthz`

## mDNS check

```bash
avahi-browse -art | grep tzsp_collector
```
