# REST API `/api/v1`

Базовый путь: `/api/v1/`. Ответы JSON. Ошибки: `{"error":"..."}`.

UI и внешние клиенты используют те же эндпоинты. Реализация: `internal/api/`.

## Status

### `GET /api/v1/status`

```json
{
  "forwarder": {
    "config": { "enabled": false, "auth_target": "...", "acct_target": "..." },
    "forwarded_total": 0,
    "forward_errors": 0,
    "responses_total": 0,
    "timed_out": 0,
    "pending": 0,
    "auth_bind": "",
    "acct_bind": ""
  },
  "recorder": { "active": false, "recording_id": "" },
  "replay": { "active": false, "sent": 0, "total": 0, "progress": 0 },
  "synth": { "active": false, "sent": 0 }
}
```

## Forwarder

| Method | Path | Описание |
|--------|------|----------|
| GET | `/forwarder` | текущий конфиг |
| PUT | `/forwarder` | тело: `{ "enabled", "auth_target", "acct_target" }` |
| GET | `/forwarder/conversations` | последние пары без тел пакетов (до 200) |
| GET | `/forwarder/conversations/{id}` | полные request/response |
| DELETE | `/forwarder/conversations` | очистить список |

Конфиг персистится в SQLite (`settings.forwarder_config`).

Список — summary (`id`, `status`, codes, identity, `rtt_ms`, `rewritten`) без `decoded_attributes`. Полные пакеты — только `{id}` (инспектор UI).

## Catalog — NAS

| Method | Path |
|--------|------|
| GET | `/nas` |
| POST | `/nas` |
| GET | `/nas/{id}` |
| PUT | `/nas/{id}` |
| DELETE | `/nas/{id}` |

Поля NAS: `name`, `ip`, `secret`, `vendor`, `identifier`, `auth_port`, `acct_port`, `notes`.

## Catalog — clients

| Method | Path |
|--------|------|
| GET | `/clients` |
| POST | `/clients` |
| GET | `/clients/{id}` |
| PUT | `/clients/{id}` |
| DELETE | `/clients/{id}` |

Поля: `mac` (unique), `username`, `notes`.

## Catalog import/export

| Method | Path | Описание |
|--------|------|----------|
| GET | `/catalog/export` | JSON attachment: NAS + clients |
| POST | `/catalog/import` | `mode`: `merge` \| `replace`; массивы `nas`, `clients` |

Можно слать тело экспорта как есть; `mode` берётся из query или UI (default merge).

## Recordings

| Method | Path | Описание |
|--------|------|----------|
| GET | `/recordings` | список |
| POST | `/recordings/start` | `{ "name": "..." }` → 201 |
| POST | `/recordings/stop` | остановить |
| GET | `/recordings/{id}` | метаданные |
| DELETE | `/recordings/{id}` | удалить (+ пакеты CASCADE) |
| POST | `/recordings/{id}/replay` | старт replay |

Тело replay:

```json
{
  "rate_rps": 10,
  "preserve_timing": false,
  "speed": 1.0,
  "loop": false,
  "mirror_to_stream": true
}
```

Пустое тело → `rate_rps=10`, `mirror_to_stream=true`.

| Method | Path |
|--------|------|
| POST | `/replay/stop` |

## Scenarios / synth

| Method | Path | Описание |
|--------|------|----------|
| GET | `/scenarios` | сохранённые сценарии |
| POST | `/scenarios` | `{ "name", "config_json" }` |
| DELETE | `/scenarios/{id}` | |
| POST | `/scenarios/run` | запуск синтеза |
| POST | `/synth/stop` | |

Тело `/scenarios/run`:

```json
{
  "nas_id": "...",
  "client_ids": ["..."],
  "pattern": "full_session",
  "auth_rate_rps": 2,
  "sessions": 3,
  "interim_interval_sec": 60,
  "session_duration_sec": 180,
  "mirror_to_stream": true
}
```

`pattern`: `auth_only` | `acct_only` | `full_session`.

## Прочее

`GET /healthz` → `{"ok":true}` (вне `/api/v1`, пакет `stream`).
