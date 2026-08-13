package stream

import (
	"encoding/json"

	"github.com/vanelm/tzsp-radius-collector/internal/config"
)

const (
	FeedTypeRadius = "radius"
	FeedVersion1   = "1"
)

// Hello is the first WebSocket message sent to every new client.
type Hello struct {
	Type         string   `json:"type"`
	FeedType     string   `json:"feed_type"`
	FeedVersion  string   `json:"feed_version"`
	SchemaID     string   `json:"schema_id"`
	CollectorID  string   `json:"collector_id"`
	Capabilities []string `json:"capabilities"`
}

var radiusCapabilities = []string{
	"filter.codes",
	"filter.macs",
	"filter.user_names",
	"filter.nas_ips",
	"filter.vendors",
	"filter.status_types",
	"filter.sources",
}

func BuildHello(cfg config.Config) Hello {
	return Hello{
		Type:         "hello",
		FeedType:     cfg.FeedType,
		FeedVersion:  cfg.FeedVersion,
		SchemaID:     cfg.SchemaID,
		CollectorID:  cfg.CollectorID,
		Capabilities: radiusCapabilities,
	}
}

func MarshalHello(cfg config.Config) ([]byte, error) {
	return json.Marshal(BuildHello(cfg))
}
