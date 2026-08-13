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
	hasProxy := mapBool(msg.Accounting, "has_proxy_state")
	rfDomain := mapString(msg.Accounting, "rf_domain")

	// NAS entries are learned from client requests only; responses mirrored from
	// the RADIUS server carry the server IP as UDP source and must not be cataloged.
	if radiusdecode.IsRequestCode(msg.Radius.Code) {
		if hasProxy {
			if src := radiusdecode.HostPart(msg.SrcAddr); src != "" {
				nasIP = src
			}
		}
		if nasIP != "" {
			incoming := CatalogNASName(hasProxy, rfDomain, nasIdentifier)
			name := incoming
			if existing, err := a.store.GetNASByIP(nasIP); err == nil {
				name = MergeAutofillName(existing.Name, incoming, nasIP)
			}
			if err := a.store.EnsureNAS(nasIP, name, nasIdentifier, nasVendor); err != nil {
				a.logger.Debug("catalog nas autofill failed", "error", err, "ip", nasIP)
			}
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

func mapBool(m map[string]any, key string) bool {
	if m == nil {
		return false
	}
	v, ok := m[key]
	if !ok || v == nil {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}
