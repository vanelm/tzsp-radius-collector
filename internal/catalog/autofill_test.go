package catalog

import (
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/vanelm/tzsp-radius-collector/internal/pipeline"
	"github.com/vanelm/tzsp-radius-collector/internal/store"
)

func TestAutofillProxyNamesByRFDomain(t *testing.T) {
	st := openTestStore(t)
	defer st.Close()
	a := NewAutofill(slog.Default(), st)

	a.Observe(proxyMsg("10.0.1.10", "10.0.33.15", "IZB-DMD-069-AP310-1-C83C6A", "IZB-DMD-WH"))
	n, err := st.GetNASByIP("10.0.1.10")
	if err != nil {
		t.Fatal(err)
	}
	if n.Name != "PROXY-IZB-DMD-WH" {
		t.Fatalf("name %q", n.Name)
	}
	if n.Identifier != "IZB-DMD-069-AP310-1-C83C6A" {
		t.Fatalf("identifier %q", n.Identifier)
	}

	a.Observe(proxyMsg("10.0.1.10", "10.8.226.150", "IZB-VSK-B1A5-030-AP8533-26DB0B", "IZB-VSK"))
	n, err = st.GetNASByIP("10.0.1.10")
	if err != nil {
		t.Fatal(err)
	}
	if n.Name != "PROXY-VX" {
		t.Fatalf("expected PROXY-VX after second RFD, got %q", n.Name)
	}

	list, err := st.ListNAS()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected one RADIUS client, got %d", len(list))
	}
}

func TestAutofillWithoutProxyUsesNASIdentifier(t *testing.T) {
	st := openTestStore(t)
	defer st.Close()
	a := NewAutofill(slog.Default(), st)

	a.Observe(pipeline.StreamMessage{
		SrcAddr: "10.0.33.15",
		Radius:  pipeline.RadiusPacket{Code: 1},
		Accounting: map[string]any{
			"nas":            "10.0.33.15",
			"nas_identifier": "IZB-DMD-069-AP310-1-C83C6A",
			"nas_vendor":     "Symbol",
		},
	})
	n, err := st.GetNASByIP("10.0.33.15")
	if err != nil {
		t.Fatal(err)
	}
	if n.Name != "IZB-DMD-069-AP310-1-C83C6A" {
		t.Fatalf("name %q", n.Name)
	}
}

func proxyMsg(src, nasIP, ident, rfd string) pipeline.StreamMessage {
	return pipeline.StreamMessage{
		SrcAddr: src,
		Radius:  pipeline.RadiusPacket{Code: 4},
		Accounting: map[string]any{
			"nas":             nasIP,
			"nas_identifier":  ident,
			"nas_vendor":      "Symbol",
			"has_proxy_state": true,
			"rf_domain":       rfd,
		},
	}
}

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	return st
}
