package capture

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/afpacket"
	"github.com/google/gopacket/layers"
	"github.com/vanelm/tzsp-radius-collector/internal/config"
	"github.com/vanelm/tzsp-radius-collector/internal/pipeline"
)

type RawSniffer struct {
	cfg    config.Config
	logger *slog.Logger
}

func NewRawSniffer(cfg config.Config, logger *slog.Logger) *RawSniffer {
	return &RawSniffer{cfg: cfg, logger: logger}
}

func (s *RawSniffer) Run(ctx context.Context, out chan<- pipeline.Event) {
	handle, err := afpacket.NewTPacket(
		afpacket.OptInterface(s.cfg.SniffInterface),
		afpacket.OptFrameSize(65536),
		afpacket.OptPollTimeout(500*time.Millisecond),
	)
	if err != nil {
		s.logger.Warn("raw sniff disabled", "error", err, "interface", s.cfg.SniffInterface)
		return
	}
	defer handle.Close()
	if s.cfg.SniffPromisc {
		s.logger.Warn("promiscuous mode requested but unsupported by current backend", "promiscuous", true)
	}
	s.logger.Info("raw sniff enabled", "interface", s.cfg.SniffInterface)

	source := gopacket.NewPacketSource(handle, layers.LayerTypeEthernet)
	packets := source.Packets()
	for {
		select {
		case <-ctx.Done():
			return
		case packet, ok := <-packets:
			if !ok || packet == nil {
				continue
			}
			udpLayer := packet.Layer(layers.LayerTypeUDP)
			if udpLayer == nil {
				continue
			}
			udp := udpLayer.(*layers.UDP)
			if !looksLikeRadius(udp.Payload) {
				continue
			}
			event := pipeline.Event{Source: "raw_sniff", ReceivedAt: time.Now().UTC(), PacketBytes: append([]byte(nil), udp.Payload...), CaptureIface: s.cfg.SniffInterface}
			select {
			case out <- event:
			case <-ctx.Done():
				return
			}
		}
	}
}
