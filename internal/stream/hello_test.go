package stream

import (
	"encoding/json"
	"testing"

	"github.com/vanelm/tzsp-radius-collector/internal/config"
)

func TestBuildHello(t *testing.T) {
	cfg := config.Config{
		CollectorID: "lab-collector",
		FeedType:    config.DefaultFeedType,
		FeedVersion: config.DefaultFeedVersion,
		SchemaID:    config.DefaultSchemaID,
	}

	hello := BuildHello(cfg)
	if hello.Type != "hello" {
		t.Fatalf("type = %q, want hello", hello.Type)
	}
	if hello.FeedType != "radius" {
		t.Fatalf("feed_type = %q, want radius", hello.FeedType)
	}
	if hello.CollectorID != "lab-collector" {
		t.Fatalf("collector_id = %q", hello.CollectorID)
	}
	if len(hello.Capabilities) == 0 {
		t.Fatal("expected capabilities")
	}
	foundSources := false
	for _, cap := range hello.Capabilities {
		if cap == "filter.sources" {
			foundSources = true
			break
		}
	}
	if !foundSources {
		t.Fatal("expected filter.sources capability")
	}

	raw, err := MarshalHello(cfg)
	if err != nil {
		t.Fatal(err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["type"] != "hello" {
		t.Fatalf("json type = %v", decoded["type"])
	}
}
