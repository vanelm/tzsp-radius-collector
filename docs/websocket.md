# WebSocket

Путь по умолчанию: `URSA_TZSP_WS_PATH` → `/ws`.  
Upgrade: gorilla/websocket, `CheckOrigin` всегда true.

## Handshake `hello`

Первое сообщение сервера — всегда `hello` (не событие ленты):

```json
{
  "type": "hello",
  "feed_type": "radius",
  "feed_version": "1",
  "schema_id": "https://vanelm.github.io/tzsp-radius-collector/schemas/collector-feed.schema.json",
  "collector_id": "tzsp-radius-collector",
  "capabilities": [
    "filter.codes",
    "filter.macs",
    "filter.user_names",
    "filter.nas_ips",
    "filter.vendors",
    "filter.status_types",
    "filter.sources"
  ]
}
```

## Подписка

Клиент отправляет:

```json
{
  "action": "subscribe",
  "filter": {
    "codes": ["Accounting-Request"],
    "macs": ["AA:BB:CC:DD:EE:FF"],
    "user_names": [],
    "nas_ips": [],
    "vendors": [],
    "status_types": [],
    "sources": []
  }
}
```

Пустой `filter` / пустые массивы — без ограничений по этому полю. Повторный `subscribe` обновляет фильтр. Keepalive: server ping ~20s; read deadline 60s (продлевается на pong).

Медленный клиент (очередь `URSA_TZSP_CLIENT_QUEUE` полна) отключается.

## Формат события ленты

Схема: [collector-feed.schema.json](collector-feed.schema.json).

Ключевые поля:

| Поле | Описание |
|------|----------|
| `timestamp` | UTC RFC3339 |
| `source` | `tzsp_udp` \| `raw_sniff` \| `synthetic` \| `replay` \| `forward` \| `forward-response` |
| `remote_addr` | отправитель TZSP (UDP) |
| `capture_interface` | iface raw sniff |
| `src_addr` / `dst_addr` | L3 из захвата (если удалось извлечь) |
| `radius` | code, code_name, identifier, length, authenticator (hex) |
| `decoded_attributes` | массив атрибутов (standard / VSA) |
| `accounting` | snapshot / identity (open object) |
| `errors` | ошибки decode (обычно событие с hard-fail не публикуется) |

Пример урезанного события:

```json
{
  "timestamp": "2026-08-13T18:00:00Z",
  "source": "tzsp_udp",
  "src_addr": "10.0.0.1",
  "dst_addr": "10.0.0.2",
  "radius": {
    "code": 4,
    "code_name": "Accounting-Request",
    "identifier": 42,
    "length": 120,
    "authenticator": "0123456789abcdef0123456789abcdef"
  },
  "decoded_attributes": [
    {
      "name": "User-Name",
      "attr_id": 1,
      "is_vsa": false,
      "value": "alice",
      "raw": "alice",
      "vendor_id": 0,
      "vendor_name": ""
    }
  ],
  "accounting": {
    "user_name": "alice",
    "mac": "AA:BB:CC:DD:EE:FF",
    "nas": "10.0.0.1"
  }
}
```

UI Live Feed подключается к `/ws` только пока открыта вкладка Live и не включён Pause. Пустой фильтр — все события. Сравнение глазами: таблица Forward (poll summaries) + инспектор (полный пакет по клику). Отдельный analytics-канал не нужен.

При нагрузке не держите Live открытым: каждый WS-клиент заставляет коллектор сериализовать decoded-ленту. Ursa на том же `/ws` — та же стоимость. `filter.sources` отсекает доставку; `json.Marshal` делается один раз, только если хотя бы один клиент прошёл фильтр. Если подписчиков нет — marshal не делается. Отдельный analytics WebSocket не нужен.
