package capture

import (
	"context"
	"log"
	"time"

	"github.com/elin/tzsp-radius-collector/internal/config"
	"github.com/elin/tzsp-radius-collector/internal/pipeline"
	"github.com/google/gopacket"
	"github.com/google/gopacket/afpacket"
	"github.com/google/gopacket/layers"
)

type RawSniffer struct {
	cfg    config.Config
	logger *log.Logger
}

func NewRawSniffer(cfg config.Config, logger *log.Logger) *RawSniffer {
	return &RawSniffer{cfg: cfg, logger: logger}
}

func (s *RawSniffer) Run(ctx context.Context, out chan<- pipeline.Event) {
	handle, err := afpacket.NewTPacket(
		afpacket.OptInterface(s.cfg.SniffInterface),
		afpacket.OptFrameSize(65536),
		afpacket.OptPollTimeout(500*time.Millisecond),
	)
	if err != nil {
		s.logger.Printf("raw sniff disabled: %v", err)
		return
	}
	defer handle.Close()
	s.logger.Printf("raw sniff enabled on iface=%s", s.cfg.SniffInterface)

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
