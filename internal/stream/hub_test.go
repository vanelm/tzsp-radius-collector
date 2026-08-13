package stream

import (
	"log/slog"
	"testing"

	"github.com/vanelm/tzsp-radius-collector/internal/pipeline"
)

func TestPublishSkipsMarshalWithoutSubscribers(t *testing.T) {
	h := NewHub(slog.Default(), 4)
	// Must not panic or block with zero clients.
	h.Publish(pipeline.StreamMessage{Source: "tzsp_udp", Radius: pipeline.RadiusPacket{CodeName: "Access-Request"}})
}

func TestMatchesFilterSources(t *testing.T) {
	msg := pipeline.StreamMessage{Source: "forward-response", Radius: pipeline.RadiusPacket{CodeName: "Access-Accept"}}
	if !matchesFilter(msg, Filter{}) {
		t.Fatal("empty filter should match")
	}
	if matchesFilter(msg, Filter{Sources: []string{"tzsp_udp"}}) {
		t.Fatal("source filter should exclude forward-response")
	}
	if !matchesFilter(msg, Filter{Sources: []string{"forward-response", "replay"}}) {
		t.Fatal("source filter should include forward-response")
	}
}

func TestPublishRespectsSourceFilter(t *testing.T) {
	h := NewHub(slog.Default(), 4)
	c := h.Add("c1")
	h.UpdateFilter("c1", Filter{Sources: []string{"tzsp_udp"}})
	h.Publish(pipeline.StreamMessage{Source: "forward-response", Radius: pipeline.RadiusPacket{CodeName: "Access-Accept"}})
	select {
	case <-c.send:
		t.Fatal("filtered source should not be queued")
	default:
	}
	h.Publish(pipeline.StreamMessage{Source: "tzsp_udp", Radius: pipeline.RadiusPacket{CodeName: "Access-Request"}})
	select {
	case <-c.send:
	default:
		t.Fatal("matching source should be queued")
	}
}
