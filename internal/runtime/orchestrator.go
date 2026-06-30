package runtime

import (
	"context"
	"log/slog"

	"github.com/vanelm/tzsp-radius-collector/internal/catalog"
	"github.com/vanelm/tzsp-radius-collector/internal/forwarder"
	"github.com/vanelm/tzsp-radius-collector/internal/pipeline"
	"github.com/vanelm/tzsp-radius-collector/internal/radiusdecode"
	"github.com/vanelm/tzsp-radius-collector/internal/recorder"
	"github.com/vanelm/tzsp-radius-collector/internal/stream"
)

type Orchestrator struct {
	logger    *slog.Logger
	hub       *stream.Hub
	processor *pipeline.Processor
	forwarder *forwarder.Forwarder
	recorder  *recorder.Recorder
	catalog   *catalog.Autofill
}

func NewOrchestrator(
	logger *slog.Logger,
	hub *stream.Hub,
	processor *pipeline.Processor,
	fwd *forwarder.Forwarder,
	rec *recorder.Recorder,
	cat *catalog.Autofill,
) *Orchestrator {
	return &Orchestrator{
		logger:    logger,
		hub:       hub,
		processor: processor,
		forwarder: fwd,
		recorder:  rec,
		catalog:   cat,
	}
}

func (o *Orchestrator) Emit(event pipeline.Event) {
	o.process(event)
}

func (o *Orchestrator) Run(ctx context.Context, in <-chan pipeline.Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-in:
			if !ok {
				return
			}
			o.process(event)
		}
	}
}

func (o *Orchestrator) process(event pipeline.Event) {
	if radiusdecode.IsRequest(event.PacketBytes) {
		o.recorder.MaybeRecord(event)
		o.forwarder.MaybeForward(event)
	}

	msg, err := o.processor.Transform(event)
	if err != nil {
		o.logger.Debug("event transform skipped", "error", err, "source", event.Source)
		return
	}
	o.logger.Debug("packet decoded", "code", msg.Radius.CodeName, "source", event.Source, "attrs", len(msg.DecodedAttributes))
	if o.catalog != nil {
		o.catalog.Observe(msg)
	}
	o.hub.Publish(msg)
}
