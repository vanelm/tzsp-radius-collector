CREATE TABLE IF NOT EXISTS nas (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    ip TEXT NOT NULL,
    secret TEXT NOT NULL DEFAULT '',
    vendor TEXT NOT NULL DEFAULT '',
    identifier TEXT NOT NULL DEFAULT '',
    auth_port INTEGER NOT NULL DEFAULT 1812,
    acct_port INTEGER NOT NULL DEFAULT 1813,
    notes TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS clients (
    id TEXT PRIMARY KEY,
    mac TEXT NOT NULL UNIQUE,
    username TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS recordings (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    started_at TEXT NOT NULL,
    stopped_at TEXT,
    packet_count INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS recording_packets (
    recording_id TEXT NOT NULL,
    seq INTEGER NOT NULL,
    recorded_at TEXT NOT NULL,
    code INTEGER NOT NULL,
    packet_blob BLOB NOT NULL,
    src_addr TEXT NOT NULL DEFAULT '',
    dst_addr TEXT NOT NULL DEFAULT '',
    delta_ms INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (recording_id, seq),
    FOREIGN KEY (recording_id) REFERENCES recordings(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS scenarios (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    config_json TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_clients_mac ON clients(mac);
CREATE INDEX IF NOT EXISTS idx_recording_packets_recording ON recording_packets(recording_id);
