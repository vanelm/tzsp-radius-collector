package synth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/vanelm/tzsp-radius-collector/internal/forwarder"
	"github.com/vanelm/tzsp-radius-collector/internal/pipeline"
	"github.com/vanelm/tzsp-radius-collector/internal/radiusdecode"
	"github.com/vanelm/tzsp-radius-collector/internal/store"
)

type RunConfig struct {
	NASID              string   `json:"nas_id"`
	ClientIDs          []string `json:"client_ids"`
	Pattern            string   `json:"pattern"`
	AuthRateRPS        float64  `json:"auth_rate_rps"`
	Sessions           int      `json:"sessions"`
	InterimIntervalSec int      `json:"interim_interval_sec"`
	SessionDurationSec int      `json:"session_duration_sec"`
	MirrorToStream     bool     `json:"mirror_to_stream"`
}

type Status struct {
	Active bool `json:"active"`
	Sent   int  `json:"sent"`
}

type Engine struct {
	logger    *slog.Logger
	store     *store.Store
	forwarder *forwarder.Forwarder
	emit      func(pipeline.Event)

	mu     sync.Mutex
	cancel context.CancelFunc
	status Status
}

func New(logger *slog.Logger, st *store.Store, fwd *forwarder.Forwarder, emit func(pipeline.Event)) *Engine {
	return &Engine{
		logger:    logger,
		store:     st,
		forwarder: fwd,
		emit:      emit,
	}
}

func (e *Engine) Status() Status {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.status
}

func (e *Engine) Stop() {
	e.mu.Lock()
	cancel := e.cancel
	e.cancel = nil
	e.status = Status{}
	e.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (e *Engine) Run(cfg RunConfig) error {
	nas, err := e.store.GetNAS(cfg.NASID)
	if err != nil {
		return err
	}
	if len(cfg.ClientIDs) == 0 {
		return errNoClients
	}
	clients := make([]store.Client, 0, len(cfg.ClientIDs))
	for _, id := range cfg.ClientIDs {
		c, err := e.store.GetClient(id)
		if err != nil {
			return err
		}
		clients = append(clients, c)
	}

	if cfg.AuthRateRPS <= 0 {
		cfg.AuthRateRPS = 2
	}
	if cfg.Sessions <= 0 {
		cfg.Sessions = len(clients)
	}
	if cfg.InterimIntervalSec <= 0 {
		cfg.InterimIntervalSec = 60
	}
	if cfg.SessionDurationSec <= 0 {
		cfg.SessionDurationSec = 180
	}

	e.Stop()
	ctx, cancel := context.WithCancel(context.Background())
	e.mu.Lock()
	e.cancel = cancel
	e.status = Status{Active: true}
	e.mu.Unlock()

	go e.run(ctx, nas, clients, cfg)
	return nil
}

func (e *Engine) run(ctx context.Context, nas store.NAS, clients []store.Client, cfg RunConfig) {
	defer e.Stop()

	interval := time.Duration(float64(time.Second) / cfg.AuthRateRPS)
	sent := 0

	emit := func(raw []byte, src, dst string) {
		e.forwarder.ForwardBytes(raw)
		if cfg.MirrorToStream && e.emit != nil {
			e.emit(pipeline.Event{
				Source:      "synthetic",
				ReceivedAt:  time.Now().UTC(),
				PacketBytes: raw,
				SrcAddr:     src,
				DstAddr:     dst,
			})
		}
		e.mu.Lock()
		sent++
		e.status.Sent = sent
		e.mu.Unlock()
	}

	nasSrc := nas.IP + ":0"
	authDst := "127.0.0.1:1812"
	acctDst := "127.0.0.1:1813"

	for i := 0; i < cfg.Sessions && i < len(clients); i++ {
		select {
		case <-ctx.Done():
			return
		default:
		}

		client := clients[i%len(clients)]
		user := client.Username
		if user == "" {
			user = client.MAC
		}
		sessionID := randomSessionID()

		switch cfg.Pattern {
		case "auth_only":
			raw, err := radiusdecode.BuildAccessRequest(radiusdecode.AccessRequestParams{
				UserName:       user,
				NASIP:          nas.IP,
				CallingStation: client.MAC,
				NASIdentifier:  nas.Identifier,
			})
			if err != nil {
				e.logger.Warn("build access request failed", "error", err)
				continue
			}
			emit(raw, nasSrc, authDst)

		case "acct_only", "full_session":
			if cfg.Pattern == "full_session" {
				raw, err := radiusdecode.BuildAccessRequest(radiusdecode.AccessRequestParams{
					UserName:       user,
					NASIP:          nas.IP,
					CallingStation: client.MAC,
					NASIdentifier:  nas.Identifier,
				})
				if err == nil {
					emit(raw, nasSrc, authDst)
				}
				select {
				case <-ctx.Done():
					return
				case <-time.After(interval):
				}
			}

			start, _ := radiusdecode.BuildAccountingRequest(radiusdecode.AccountingRequestParams{
				UserName:       user,
				NASIP:          nas.IP,
				CallingStation: client.MAC,
				NASIdentifier:  nas.Identifier,
				SessionID:      sessionID,
				StatusType:     1,
			})
			emit(start, nasSrc, acctDst)

			elapsed := 0
			for elapsed < cfg.SessionDurationSec {
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Duration(cfg.InterimIntervalSec) * time.Second):
				}
				elapsed += cfg.InterimIntervalSec
				if elapsed >= cfg.SessionDurationSec {
					break
				}
				interim, _ := radiusdecode.BuildAccountingRequest(radiusdecode.AccountingRequestParams{
					UserName:       user,
					NASIP:          nas.IP,
					CallingStation: client.MAC,
					NASIdentifier:  nas.Identifier,
					SessionID:      sessionID,
					StatusType:     3,
					SessionTime:    uint32(elapsed),
					InputOctets:    uint32(elapsed * 1000),
					OutputOctets:   uint32(elapsed * 500),
				})
				emit(interim, nasSrc, acctDst)
			}

			stop, _ := radiusdecode.BuildAccountingRequest(radiusdecode.AccountingRequestParams{
				UserName:       user,
				NASIP:          nas.IP,
				CallingStation: client.MAC,
				NASIdentifier:  nas.Identifier,
				SessionID:      sessionID,
				StatusType:     2,
				SessionTime:    uint32(cfg.SessionDurationSec),
				InputOctets:    uint32(cfg.SessionDurationSec * 1000),
				OutputOctets:   uint32(cfg.SessionDurationSec * 500),
				TerminateCause: 1,
			})
			emit(stop, nasSrc, acctDst)

		default:
			e.logger.Warn("unknown pattern", "pattern", cfg.Pattern)
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}

	e.mu.Lock()
	e.status.Active = false
	e.mu.Unlock()
}

func randomSessionID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func (cfg RunConfig) JSON() string {
	raw, _ := json.Marshal(cfg)
	return string(raw)
}

var errNoClients = errString("no clients selected")

type errString string

func (e errString) Error() string { return string(e) }
