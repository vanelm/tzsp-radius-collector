package replay

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/vanelm/tzsp-radius-collector/internal/forwarder"
	"github.com/vanelm/tzsp-radius-collector/internal/pipeline"
	"github.com/vanelm/tzsp-radius-collector/internal/store"
)

type Options struct {
	RateRPS         float64 `json:"rate_rps"`
	PreserveTiming  bool    `json:"preserve_timing"`
	Speed           float64 `json:"speed"`
	Loop            bool    `json:"loop"`
	MirrorToStream  bool    `json:"mirror_to_stream"`
}

type Status struct {
	Active   bool    `json:"active"`
	Sent     int     `json:"sent"`
	Total    int     `json:"total"`
	Progress float64 `json:"progress"`
}

type Engine struct {
	logger    *slog.Logger
	store     *store.Store
	forwarder *forwarder.Forwarder
	emit      func(pipeline.Event)

	mu      sync.Mutex
	cancel  context.CancelFunc
	status  Status
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

func (e *Engine) Start(recordingID string, opts Options) error {
	packets, err := e.store.ListRecordingPackets(recordingID)
	if err != nil {
		return err
	}
	if len(packets) == 0 {
		return errNoPackets
	}

	e.Stop()

	if opts.RateRPS <= 0 {
		opts.RateRPS = 10
	}
	if opts.Speed <= 0 {
		opts.Speed = 1.0
	}

	ctx, cancel := context.WithCancel(context.Background())
	e.mu.Lock()
	e.cancel = cancel
	e.status = Status{Active: true, Total: len(packets)}
	e.mu.Unlock()

	go e.run(ctx, packets, opts)
	return nil
}

func (e *Engine) run(ctx context.Context, packets []store.RecordingPacket, opts Options) {
	defer func() {
		e.mu.Lock()
		e.cancel = nil
		e.status.Active = false
		e.mu.Unlock()
	}()

	for {
		for i, pkt := range packets {
			select {
			case <-ctx.Done():
				return
			default:
			}

			if i > 0 {
				var wait time.Duration
				if opts.PreserveTiming {
					wait = time.Duration(float64(packets[i].DeltaMS)/opts.Speed) * time.Millisecond
				} else {
					wait = time.Duration(float64(time.Second) / opts.RateRPS)
				}
				if wait > 0 {
					timer := time.NewTimer(wait)
					select {
					case <-ctx.Done():
						timer.Stop()
						return
					case <-timer.C:
					}
				}
			}

			raw := append([]byte(nil), pkt.PacketBlob...)
			e.forwarder.ForwardBytes(raw)

			if opts.MirrorToStream && e.emit != nil {
				e.emit(pipeline.Event{
					Source:      "replay",
					ReceivedAt:  time.Now().UTC(),
					PacketBytes: raw,
					SrcAddr:     pkt.SrcAddr,
					DstAddr:     pkt.DstAddr,
				})
			}

			e.mu.Lock()
			e.status.Sent = i + 1
			if e.status.Total > 0 {
				e.status.Progress = float64(e.status.Sent) / float64(e.status.Total)
			}
			e.mu.Unlock()
		}

		if !opts.Loop {
			return
		}
		e.mu.Lock()
		e.status.Sent = 0
		e.mu.Unlock()
	}
}

var errNoPackets = errString("recording has no packets")

type errString string

func (e errString) Error() string { return string(e) }
