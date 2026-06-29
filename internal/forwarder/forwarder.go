package forwarder

import (
	"encoding/json"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"

	"github.com/vanelm/tzsp-radius-collector/internal/pipeline"
	"github.com/vanelm/tzsp-radius-collector/internal/radiusdecode"
	"github.com/vanelm/tzsp-radius-collector/internal/store"
)

type Config struct {
	Enabled    bool   `json:"enabled"`
	AuthTarget string `json:"auth_target"`
	AcctTarget string `json:"acct_target"`
}

type Forwarder struct {
	logger *slog.Logger
	store  *store.Store

	mu     sync.RWMutex
	config Config

	forwardedTotal atomic.Uint64
	forwardErrors  atomic.Uint64
}

func New(logger *slog.Logger, st *store.Store, defaults Config) *Forwarder {
	f := &Forwarder{
		logger: logger,
		store:  st,
		config: defaults,
	}
	f.loadFromStore()
	return f
}

func (f *Forwarder) loadFromStore() {
	if f.store == nil {
		return
	}
	if raw, ok, err := f.store.GetSetting("forwarder_config"); err == nil && ok && raw != "" {
		var cfg Config
		if json.Unmarshal([]byte(raw), &cfg) == nil {
			f.config = cfg
		}
	}
}

func (f *Forwarder) persist() {
	if f.store == nil {
		return
	}
	f.mu.RLock()
	raw, _ := json.Marshal(f.config)
	f.mu.RUnlock()
	_ = f.store.SetSetting("forwarder_config", string(raw))
}

func (f *Forwarder) GetConfig() Config {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.config
}

func (f *Forwarder) SetConfig(cfg Config) {
	f.mu.Lock()
	f.config = cfg
	f.mu.Unlock()
	f.persist()
}

func (f *Forwarder) Stats() (forwarded, errors uint64) {
	return f.forwardedTotal.Load(), f.forwardErrors.Load()
}

func (f *Forwarder) MaybeForward(event pipeline.Event) {
	f.mu.RLock()
	cfg := f.config
	f.mu.RUnlock()

	if !cfg.Enabled || len(event.PacketBytes) < 1 {
		return
	}
	if !radiusdecode.IsRequest(event.PacketBytes) {
		return
	}

	target := cfg.AuthTarget
	if event.PacketBytes[0] == 4 {
		target = cfg.AcctTarget
	}
	if target == "" {
		return
	}

	addr, err := net.ResolveUDPAddr("udp", target)
	if err != nil {
		f.forwardErrors.Add(1)
		f.logger.Warn("forward resolve failed", "target", target, "error", err)
		return
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		f.forwardErrors.Add(1)
		f.logger.Warn("forward dial failed", "target", target, "error", err)
		return
	}
	defer conn.Close()

	_, err = conn.Write(event.PacketBytes)
	if err != nil {
		f.forwardErrors.Add(1)
		f.logger.Warn("forward write failed", "target", target, "error", err)
		return
	}
	f.forwardedTotal.Add(1)
}

func (f *Forwarder) ForwardBytes(packet []byte) {
	if len(packet) < 1 {
		return
	}
	f.MaybeForward(pipeline.Event{PacketBytes: packet, Source: "forward"})
}
