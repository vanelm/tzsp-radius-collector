package recorder

import (
	"log/slog"
	"sync"
	"time"

	"github.com/vanelm/tzsp-radius-collector/internal/pipeline"
	"github.com/vanelm/tzsp-radius-collector/internal/radiusdecode"
	"github.com/vanelm/tzsp-radius-collector/internal/store"
)

type Recorder struct {
	logger *slog.Logger
	store  *store.Store

	mu            sync.Mutex
	activeID      string
	seq           int
	lastTimestamp time.Time
}

func New(logger *slog.Logger, st *store.Store) *Recorder {
	return &Recorder{logger: logger, store: st}
}

func (r *Recorder) Active() (bool, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.activeID != "", r.activeID
}

func (r *Recorder) Start(name string) (store.Recording, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.activeID != "" {
		return store.Recording{}, errRecordingActive
	}
	rec, err := r.store.CreateRecording(name)
	if err != nil {
		return store.Recording{}, err
	}
	r.activeID = rec.ID
	r.seq = 0
	r.lastTimestamp = time.Time{}
	return rec, nil
}

func (r *Recorder) Stop() error {
	r.mu.Lock()
	id := r.activeID
	r.activeID = ""
	r.seq = 0
	r.lastTimestamp = time.Time{}
	r.mu.Unlock()

	if id == "" {
		return errNoActiveRecording
	}
	return r.store.StopRecording(id)
}

func (r *Recorder) MaybeRecord(event pipeline.Event) {
	if len(event.PacketBytes) < 1 || !radiusdecode.IsRequest(event.PacketBytes) {
		return
	}

	r.mu.Lock()
	id := r.activeID
	if id == "" {
		r.mu.Unlock()
		return
	}

	now := event.ReceivedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}

	var deltaMS int64
	if !r.lastTimestamp.IsZero() {
		deltaMS = now.Sub(r.lastTimestamp).Milliseconds()
	}
	r.lastTimestamp = now
	seq := r.seq
	r.seq++
	r.mu.Unlock()

	pkt := store.RecordingPacket{
		RecordingID: id,
		Seq:         seq,
		RecordedAt:  now,
		Code:        event.PacketBytes[0],
		PacketBlob:  append([]byte(nil), event.PacketBytes...),
		SrcAddr:     event.SrcAddr,
		DstAddr:     event.DstAddr,
		DeltaMS:     deltaMS,
	}
	if err := r.store.AppendRecordingPacket(pkt); err != nil {
		r.logger.Warn("recording append failed", "error", err, "recording_id", id)
	}
}

var (
	errRecordingActive    = errString("recording already active")
	errNoActiveRecording  = errString("no active recording")
)

type errString string

func (e errString) Error() string { return string(e) }
