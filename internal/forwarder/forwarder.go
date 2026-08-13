package forwarder

import (
	"encoding/json"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/vanelm/tzsp-radius-collector/internal/pipeline"
	"github.com/vanelm/tzsp-radius-collector/internal/radiusdecode"
	"github.com/vanelm/tzsp-radius-collector/internal/store"
)

const (
	maxConversations = 200
	responseTimeout  = 5 * time.Second
)

type Config struct {
	Enabled    bool   `json:"enabled"`
	AuthTarget string `json:"auth_target"`
	AcctTarget string `json:"acct_target"`
}

type Stats struct {
	Forwarded uint64 `json:"forwarded_total"`
	Errors    uint64 `json:"forward_errors"`
	Responses uint64 `json:"responses_total"`
	TimedOut  uint64 `json:"timed_out"`
	Pending   int    `json:"pending"`
	AuthBind  string `json:"auth_bind,omitempty"`
	AcctBind  string `json:"acct_bind,omitempty"`
}

type udpPath struct {
	target string
	conn   *net.UDPConn
	local  string
}

type pendingKey struct {
	channel    string
	identifier uint8
}

type Forwarder struct {
	logger    *slog.Logger
	store     *store.Store
	processor *pipeline.Processor

	mu     sync.RWMutex
	config Config
	emit   func(pipeline.Event)

	sockMu sync.Mutex
	auth   *udpPath
	acct   *udpPath

	convMu  sync.Mutex
	pending map[pendingKey]*Conversation
	convs   []*Conversation

	closed    chan struct{}
	closeOnce sync.Once

	forwardedTotal atomic.Uint64
	forwardErrors  atomic.Uint64
	responsesTotal atomic.Uint64
	timedOut       atomic.Uint64
}

func New(logger *slog.Logger, st *store.Store, proc *pipeline.Processor, defaults Config) *Forwarder {
	f := &Forwarder{
		logger:    logger,
		store:     st,
		processor: proc,
		config:    defaults,
		pending:   make(map[pendingKey]*Conversation),
		closed:    make(chan struct{}),
	}
	f.loadFromStore()
	go f.sweepLoop()
	f.syncSockets()
	return f
}

func (f *Forwarder) SetProcessor(p *pipeline.Processor) {
	f.mu.Lock()
	f.processor = p
	f.mu.Unlock()
}

func (f *Forwarder) SetEmit(fn func(pipeline.Event)) {
	f.mu.Lock()
	f.emit = fn
	f.mu.Unlock()
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
	f.syncSockets()
}

func (f *Forwarder) Stats() (forwarded, errors uint64) {
	return f.forwardedTotal.Load(), f.forwardErrors.Load()
}

func (f *Forwarder) Snapshot() Stats {
	s := Stats{
		Forwarded: f.forwardedTotal.Load(),
		Errors:    f.forwardErrors.Load(),
		Responses: f.responsesTotal.Load(),
		TimedOut:  f.timedOut.Load(),
	}
	f.convMu.Lock()
	s.Pending = len(f.pending)
	f.convMu.Unlock()
	f.sockMu.Lock()
	if f.auth != nil {
		s.AuthBind = f.auth.local
	}
	if f.acct != nil {
		s.AcctBind = f.acct.local
	}
	f.sockMu.Unlock()
	return s
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

	channel := "auth"
	target := cfg.AuthTarget
	if event.PacketBytes[0] == 4 {
		channel = "acct"
		target = cfg.AcctTarget
	}
	if target == "" {
		return
	}

	path, err := f.ensurePath(channel, target)
	if err != nil {
		f.forwardErrors.Add(1)
		f.logger.Warn("forward socket failed", "target", target, "error", err)
		return
	}

	pkt, err := radiusdecode.ParsePacket(event.PacketBytes)
	if err != nil {
		f.forwardErrors.Add(1)
		f.logger.Warn("forward parse failed", "error", err)
		return
	}

	ident := f.allocateIdent(channel, pkt.Identifier)
	secret := f.lookupSecret(pkt, event.SrcAddr)
	localIP := radiusdecode.LocalIPFromAddr(path.conn.LocalAddr())
	prep, err := radiusdecode.PrepareForward(event.PacketBytes, localIP, ident, secret)
	if err != nil {
		f.forwardErrors.Add(1)
		f.logger.Warn("forward prepare failed", "error", err)
		return
	}

	conv := newConversation(channel, target, path.local, event, pkt, prep)
	conv.Request = f.decode(prep.Packet, "forward", path.local, target)

	f.trackPending(conv)

	if _, err := path.conn.Write(prep.Packet); err != nil {
		f.forwardErrors.Add(1)
		f.finishError(conv, err.Error())
		f.logger.Warn("forward write failed", "target", target, "error", err)
		return
	}
	f.forwardedTotal.Add(1)
}

func (f *Forwarder) ForwardBytes(packet []byte) {
	if len(packet) < 1 {
		return
	}
	f.MaybeForward(pipeline.Event{PacketBytes: packet, Source: "forward", ReceivedAt: time.Now().UTC()})
}

func (f *Forwarder) lookupSecret(pkt *radiusdecode.Packet, srcAddr string) []byte {
	if f.store == nil {
		return nil
	}
	ip := radiusdecode.NASIPString(pkt)
	if ip == "" {
		ip = radiusdecode.HostPart(srcAddr)
	}
	if ip == "" {
		return nil
	}
	nas, err := f.store.GetNASByIP(ip)
	if err != nil || nas.Secret == "" {
		return nil
	}
	return []byte(nas.Secret)
}

func (f *Forwarder) decode(raw []byte, source, src, dst string) *pipeline.StreamMessage {
	f.mu.RLock()
	proc := f.processor
	f.mu.RUnlock()
	if proc == nil {
		return headerMessage(raw, source, src, dst)
	}
	msg, err := proc.Transform(pipeline.Event{
		Source:      source,
		ReceivedAt:  time.Now().UTC(),
		PacketBytes: raw,
		SrcAddr:     src,
		DstAddr:     dst,
	})
	if err != nil {
		fallback := headerMessage(raw, source, src, dst)
		fallback.Errors = append(fallback.Errors, err.Error())
		return fallback
	}
	return &msg
}

func headerMessage(raw []byte, source, src, dst string) *pipeline.StreamMessage {
	msg := &pipeline.StreamMessage{
		Timestamp: time.Now().UTC(),
		Source:    source,
		SrcAddr:   src,
		DstAddr:   dst,
	}
	if len(raw) >= 20 {
		msg.Radius = pipeline.RadiusPacket{
			Code:       raw[0],
			CodeName:   radiusdecode.CodeName(raw[0]),
			Identifier: raw[1],
			Length:     uint16(len(raw)),
		}
	}
	return msg
}

func (f *Forwarder) syncSockets() {
	cfg := f.GetConfig()
	if !cfg.Enabled {
		f.closePath("auth")
		f.closePath("acct")
		return
	}
	if _, err := f.ensurePath("auth", cfg.AuthTarget); err != nil {
		f.logger.Warn("auth socket failed", "target", cfg.AuthTarget, "error", err)
	}
	if _, err := f.ensurePath("acct", cfg.AcctTarget); err != nil {
		f.logger.Warn("acct socket failed", "target", cfg.AcctTarget, "error", err)
	}
}

func (f *Forwarder) ensurePath(channel, target string) (*udpPath, error) {
	f.sockMu.Lock()
	defer f.sockMu.Unlock()

	cur := f.pathLocked(channel)
	if cur != nil && cur.target == target && cur.conn != nil {
		return cur, nil
	}
	if cur != nil && cur.conn != nil {
		_ = cur.conn.Close()
	}

	addr, err := net.ResolveUDPAddr("udp", target)
	if err != nil {
		f.setPathLocked(channel, nil)
		return nil, err
	}
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		f.setPathLocked(channel, nil)
		return nil, err
	}
	path := &udpPath{
		target: target,
		conn:   conn,
		local:  conn.LocalAddr().String(),
	}
	f.setPathLocked(channel, path)
	go f.readLoop(channel, conn)
	return path, nil
}

func (f *Forwarder) closePath(channel string) {
	f.sockMu.Lock()
	defer f.sockMu.Unlock()
	cur := f.pathLocked(channel)
	if cur != nil && cur.conn != nil {
		_ = cur.conn.Close()
	}
	f.setPathLocked(channel, nil)
}

func (f *Forwarder) pathLocked(channel string) *udpPath {
	if channel == "acct" {
		return f.acct
	}
	return f.auth
}

func (f *Forwarder) setPathLocked(channel string, path *udpPath) {
	if channel == "acct" {
		f.acct = path
		return
	}
	f.auth = path
}

func (f *Forwarder) readLoop(channel string, conn *net.UDPConn) {
	buf := make([]byte, 4096)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return
		}
		raw := append([]byte(nil), buf[:n]...)
		f.handleResponse(channel, raw, conn)
	}
}

func (f *Forwarder) handleResponse(channel string, raw []byte, conn *net.UDPConn) {
	if !radiusdecode.IsResponse(raw) {
		f.logger.Debug("forward ignore non-response", "channel", channel, "code", raw[0])
		return
	}
	ident := raw[1]
	key := pendingKey{channel: channel, identifier: ident}

	f.convMu.Lock()
	conv := f.pending[key]
	if conv != nil {
		delete(f.pending, key)
	}
	f.convMu.Unlock()
	if conv == nil {
		f.logger.Debug("forward unmatched response", "channel", channel, "identifier", ident)
		return
	}

	now := time.Now().UTC()
	src, dst := "", ""
	if conn != nil {
		if ra := conn.RemoteAddr(); ra != nil {
			src = ra.String()
		}
		dst = conv.LocalAddr
	}
	resp := f.decode(raw, "forward-response", src, dst)

	f.convMu.Lock()
	conv.Status = "complete"
	conv.CompletedAt = &now
	conv.ResponseCode = raw[0]
	conv.ResponseName = radiusdecode.CodeName(raw[0])
	conv.Response = resp
	rtt := now.Sub(conv.StartedAt).Seconds() * 1000
	conv.RTTMs = &rtt
	f.convMu.Unlock()

	f.responsesTotal.Add(1)

	f.mu.RLock()
	emit := f.emit
	f.mu.RUnlock()
	if emit != nil {
		emit(pipeline.Event{
			Source:      "forward-response",
			ReceivedAt:  now,
			PacketBytes: raw,
			SrcAddr:     src,
			DstAddr:     dst,
		})
	}
}

func (f *Forwarder) Close() {
	f.closeOnce.Do(func() { close(f.closed) })
	f.closePath("auth")
	f.closePath("acct")
}
