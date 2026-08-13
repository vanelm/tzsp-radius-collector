package forwarder

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/vanelm/tzsp-radius-collector/internal/pipeline"
	"github.com/vanelm/tzsp-radius-collector/internal/radiusdecode"
)

type Conversation struct {
	ID           string                  `json:"id"`
	StartedAt    time.Time               `json:"started_at"`
	CompletedAt  *time.Time              `json:"completed_at,omitempty"`
	Channel      string                  `json:"channel"`
	Target       string                  `json:"target"`
	LocalAddr    string                  `json:"local_addr,omitempty"`
	Identifier   uint8                   `json:"identifier"`
	Status       string                  `json:"status"`
	Rewritten    bool                    `json:"rewritten"`
	Source       string                  `json:"source,omitempty"`
	UserName     string                  `json:"user_name,omitempty"`
	MAC          string                  `json:"mac,omitempty"`
	NAS          string                  `json:"nas,omitempty"`
	OriginalNAS  string                  `json:"original_nas,omitempty"`
	RequestCode  uint8                   `json:"request_code"`
	RequestName  string                  `json:"request_name"`
	ResponseCode uint8                   `json:"response_code,omitempty"`
	ResponseName string                  `json:"response_name,omitempty"`
	RTTMs        *float64                `json:"rtt_ms,omitempty"`
	Error        string                  `json:"error,omitempty"`
	Request      *pipeline.StreamMessage `json:"request,omitempty"`
	Response     *pipeline.StreamMessage `json:"response,omitempty"`
}

func newConversation(channel, target, local string, event pipeline.Event, orig *radiusdecode.Packet, prep radiusdecode.ForwardPrep) *Conversation {
	now := event.ReceivedAt.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	originalNAS := radiusdecode.NASIPString(orig)
	if originalNAS == "" {
		originalNAS = radiusdecode.HostPart(event.SrcAddr)
	}
	return &Conversation{
		ID:          newConvID(),
		StartedAt:   now,
		Channel:     channel,
		Target:      target,
		LocalAddr:   local,
		Identifier:  prep.Identifier,
		Status:      "pending",
		Rewritten:   prep.Rewritten,
		Source:      event.Source,
		UserName:    radiusdecode.UserNameString(orig),
		MAC:         radiusdecode.CallingStationString(orig),
		NAS:         originalNAS,
		OriginalNAS: originalNAS,
		RequestCode: orig.Code,
		RequestName: radiusdecode.CodeName(orig.Code),
	}
}

func newConvID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func (f *Forwarder) ListConversations() []Conversation {
	f.expirePending()
	f.convMu.Lock()
	defer f.convMu.Unlock()
	out := make([]Conversation, len(f.convs))
	for i, c := range f.convs {
		out[i] = *c
	}
	return out
}

func (f *Forwarder) ClearConversations() {
	f.convMu.Lock()
	defer f.convMu.Unlock()
	f.pending = make(map[pendingKey]*Conversation)
	f.convs = nil
}

func (f *Forwarder) allocateIdent(channel string, preferred uint8) uint8 {
	f.convMu.Lock()
	defer f.convMu.Unlock()
	if _, taken := f.pending[pendingKey{channel: channel, identifier: preferred}]; !taken {
		return preferred
	}
	for i := 0; i < 256; i++ {
		id := uint8((int(preferred) + i + 1) % 256)
		if _, taken := f.pending[pendingKey{channel: channel, identifier: id}]; !taken {
			return id
		}
	}
	return preferred
}

func (f *Forwarder) trackPending(conv *Conversation) {
	key := pendingKey{channel: conv.Channel, identifier: conv.Identifier}
	f.convMu.Lock()
	defer f.convMu.Unlock()
	if old := f.pending[key]; old != nil && old.Status == "pending" {
		old.Status = "timeout"
		f.timedOut.Add(1)
	}
	f.pending[key] = conv
	f.convs = append([]*Conversation{conv}, f.convs...)
	if len(f.convs) > maxConversations {
		f.dropOldestLocked()
	}
}

func (f *Forwarder) finishError(conv *Conversation, msg string) {
	now := time.Now().UTC()
	f.convMu.Lock()
	defer f.convMu.Unlock()
	conv.Status = "error"
	conv.Error = msg
	conv.CompletedAt = &now
	delete(f.pending, pendingKey{channel: conv.Channel, identifier: conv.Identifier})
}

func (f *Forwarder) expirePending() {
	now := time.Now().UTC()
	f.convMu.Lock()
	defer f.convMu.Unlock()
	for key, conv := range f.pending {
		if conv.Status != "pending" {
			delete(f.pending, key)
			continue
		}
		if now.Sub(conv.StartedAt) < responseTimeout {
			continue
		}
		conv.Status = "timeout"
		conv.CompletedAt = &now
		delete(f.pending, key)
		f.timedOut.Add(1)
	}
}

func (f *Forwarder) dropOldestLocked() {
	for len(f.convs) > maxConversations {
		old := f.convs[len(f.convs)-1]
		f.convs = f.convs[:len(f.convs)-1]
		if old.Status == "pending" {
			delete(f.pending, pendingKey{channel: old.Channel, identifier: old.Identifier})
		}
	}
}

func (f *Forwarder) sweepLoop() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-f.closed:
			return
		case <-ticker.C:
			f.expirePending()
		}
	}
}
