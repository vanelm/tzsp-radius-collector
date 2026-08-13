package capture

import (
	"log/slog"

	"github.com/vanelm/tzsp-radius-collector/internal/config"
)

type RawSniffer struct {
	cfg    config.Config
	logger *slog.Logger
}

func NewRawSniffer(cfg config.Config, logger *slog.Logger) *RawSniffer {
	return &RawSniffer{cfg: cfg, logger: logger}
}
