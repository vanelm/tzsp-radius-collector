package catalog

import (
	"log/slog"
	"strings"

	"github.com/vanelm/tzsp-radius-collector/internal/pipeline"
	"github.com/vanelm/tzsp-radius-collector/internal/radiusdecode"
	"github.com/vanelm/tzsp-radius-collector/internal/store"
)

// Autofill discovers NAS and client entries from decoded RADIUS traffic.
type Autofill struct {
	logger *slog.Logger
	store  *store.Store
}

func NewAutofill(logger *slog.Logger, st *store.Store) *Autofill {
	return &Autofill{logger: logger, store: st}
}

func (a *Autofill) Observe(msg pipeline.StreamMessage) {
	if msg.Accounting == nil {
		return
	}

	nasIP := mapString(msg.Accounting, "nas")
	nasIdentifier := mapString(msg.Accounting, "nas_identifier")
	nasVendor := mapString(msg.Accounting, "nas_vendor")
	mac := mapString(msg.Accounting, "mac")
	userName := mapString(msg.Accounting, "user_name")

	// NAS entries are learned from client requests only; responses mirrored from
	// the RADIUS server carry the server IP as UDP source and must not be cataloged.
	if nasIP != "" && radiusdecode.IsRequestCode(msg.Radius.Code) {
		if err := a.store.EnsureNAS(nasIP, nasIdentifier, nasVendor); err != nil {
			a.logger.Debug("catalog nas autofill failed", "error", err, "ip", nasIP)
		}
	}
	if mac != "" {
		if err := a.store.EnsureClient(mac, userName); err != nil {
			a.logger.Debug("catalog client autofill failed", "error", err, "mac", mac)
		}
	}
}

func mapString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}
