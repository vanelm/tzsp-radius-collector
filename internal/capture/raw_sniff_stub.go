//go:build !cgo || !linux

package capture

import (
	"context"

	"github.com/vanelm/tzsp-radius-collector/internal/pipeline"
)

func (s *RawSniffer) Run(ctx context.Context, out chan<- pipeline.Event) {
	s.logger.Warn("raw sniff unavailable in this build (needs linux+cgo for gopacket/afpacket)", "interface", s.cfg.SniffInterface)
	<-ctx.Done()
}
