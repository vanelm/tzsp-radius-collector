package capture

import (
	"context"
	"log"
	"net"
	"time"

	"github.com/elin/tzsp-radius-collector/internal/config"
	"github.com/elin/tzsp-radius-collector/internal/pipeline"
	"github.com/elin/tzsp-radius-collector/internal/tzsp"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

type TZSPUDPListener struct {
	cfg    config.Config
	logger *log.Logger
}

func NewTZSPUDPListener(cfg config.Config, logger *log.Logger) *TZSPUDPListener {
	return &TZSPUDPListener{cfg: cfg, logger: logger}
}

func (l *TZSPUDPListener) Run(ctx context.Context, out chan<- pipeline.Event) {
	conn, err := net.ListenPacket("udp", l.cfg.TZSPListen)
	if err != nil {
		l.logger.Printf("tzsp udp listen failed: %v", err)
		return
	}
	defer conn.Close()

	l.logger.Printf("tzsp udp capture enabled on %s", l.cfg.TZSPListen)
	buffer := make([]byte, 64*1024)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, addr, readErr := conn.ReadFrom(buffer)
		if readErr != nil {
			if ne, ok := readErr.(net.Error); ok && ne.Timeout() {
				select {
				case <-ctx.Done():
					return
				default:
					continue
				}
			}
			continue
		}
		pkt, err := tzsp.Parse(buffer[:n])
		if err != nil {
			continue
		}
		radiusPayload := extractUDPPayload(pkt.Payload)
		if len(radiusPayload) == 0 {
			continue
		}
		select {
		case out <- pipeline.Event{Source: "tzsp_udp", ReceivedAt: time.Now().UTC(), PacketBytes: radiusPayload, RemoteAddr: addr.String()}:
		case <-ctx.Done():
			return
		}
	}
}

func extractUDPPayload(payload []byte) []byte {
	if len(payload) == 0 {
		return nil
	}
	if looksLikeRadius(payload) {
		return append([]byte(nil), payload...)
	}
	packet := gopacket.NewPacket(payload, layers.LayerTypeEthernet, gopacket.Default)
	if udp := packet.Layer(layers.LayerTypeUDP); udp != nil {
		udpLayer := udp.(*layers.UDP)
		if udpLayer.SrcPort == 1812 || udpLayer.SrcPort == 1813 || udpLayer.DstPort == 1812 || udpLayer.DstPort == 1813 {
			return append([]byte(nil), udpLayer.Payload...)
		}
	}
	return nil
}

func looksLikeRadius(payload []byte) bool {
	if len(payload) < 20 {
		return false
	}
	length := int(payload[2])<<8 | int(payload[3])
	if length < 20 || length > len(payload) {
		return false
	}
	return true
}
