package discovery

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/elin/tzsp-radius-collector/internal/config"
	"github.com/grandcat/zeroconf"
)

type Handle struct {
	server *zeroconf.Server
}

func (h *Handle) Shutdown() {
	if h == nil || h.server == nil {
		return
	}
	h.server.Shutdown()
}

func StartMDNS(cfg config.Config, logger *log.Logger) (*Handle, error) {
	host, _ := os.Hostname()
	if host == "" {
		host = "localhost"
	}
	instance := strings.TrimSpace(cfg.MDNSName)
	if instance == "" {
		instance = fmt.Sprintf("tzsp-radius-collector-%s", host)
	}

	meta := []string{
		"version=v1",
		"ws_path=" + cfg.WSPath,
		"capture_modes=" + captureModes(cfg),
		"service=tzsp-radius-collector",
	}

	server, err := zeroconf.Register(instance, "_tzsp_collector._tcp", "local.", cfg.MDNSPort, meta, nil)
	if err != nil {
		return nil, err
	}
	logger.Printf("mdns published: instance=%s type=_tzsp_collector._tcp port=%d", instance, cfg.MDNSPort)
	return &Handle{server: server}, nil
}

func captureModes(cfg config.Config) string {
	modes := make([]string, 0, 2)
	if cfg.EnableTZSPUDP {
		modes = append(modes, "tzsp_udp")
	}
	if cfg.EnableRawSniff {
		modes = append(modes, "raw_sniff")
	}
	if len(modes) == 0 {
		return "none"
	}
	return strings.Join(modes, ",")
}
