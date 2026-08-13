package radiusdecode

import (
	"encoding/binary"
	"testing"
)

func TestBuildIdentitySnapshotFromAccessRequest(t *testing.T) {
	raw, err := BuildAccessRequest(AccessRequestParams{
		UserName:       "alice",
		NASIP:          "10.0.0.5",
		CallingStation: "AA-BB-CC-DD-EE-FF",
		NASIdentifier:  "ap-1",
		Identifier:     7,
	})
	if err != nil {
		t.Fatal(err)
	}
	pkt, err := ParsePacket(raw)
	if err != nil {
		t.Fatal(err)
	}
	dict := NewAttributeDictionary()
	snap, ok := BuildIdentitySnapshot(pkt, dict, "")
	if !ok {
		t.Fatal("expected identity snapshot")
	}
	if snap["user_name"] != "alice" {
		t.Fatalf("user_name: got %v", snap["user_name"])
	}
	if snap["mac"] != "AA:BB:CC:DD:EE:FF" {
		t.Fatalf("mac: got %v", snap["mac"])
	}
	if snap["nas"] != "10.0.0.5" {
		t.Fatalf("nas: got %v", snap["nas"])
	}
}

func TestBuildIdentitySnapshotUsesNASFallback(t *testing.T) {
	raw, err := buildPacket(1, 1, [][]byte{
		encodeStringAVP(1, "bob"),
		encodeStringAVP(31, "AA-BB-CC-DD-EE-FF"),
	})
	if err != nil {
		t.Fatal(err)
	}
	pkt, err := ParsePacket(raw)
	if err != nil {
		t.Fatal(err)
	}
	dict := NewAttributeDictionary()
	snap, ok := BuildIdentitySnapshot(pkt, dict, "192.168.1.1")
	if !ok {
		t.Fatal("expected identity snapshot")
	}
	if snap["nas"] != "192.168.1.1" {
		t.Fatalf("nas fallback: got %v", snap["nas"])
	}
}

func TestBuildIdentitySnapshotProxyAndRFDomain(t *testing.T) {
	raw, err := BuildAccessRequest(AccessRequestParams{
		UserName:      "alice",
		NASIP:         "10.0.33.15",
		NASIdentifier: "IZB-DMD-069-AP310-1-C83C6A",
		Identifier:    1,
	})
	if err != nil {
		t.Fatal(err)
	}
	pkt, err := ParsePacket(raw)
	if err != nil {
		t.Fatal(err)
	}
	SetAttribute(pkt, AttrProxyState, []byte{0x01})
	pkt.Attributes = append(pkt.Attributes, AVP{
		Type:  AttrVendorSpecific,
		Value: encodeSymbolRFDomain("IZB-DMD-WH"),
	})
	pkt, err = ParsePacket(EncodePacket(pkt))
	if err != nil {
		t.Fatal(err)
	}
	snap, ok := BuildIdentitySnapshot(pkt, NewAttributeDictionary(), "")
	if !ok {
		t.Fatal("expected snapshot")
	}
	if snap["has_proxy_state"] != true {
		t.Fatalf("has_proxy_state: %v", snap["has_proxy_state"])
	}
	if snap["rf_domain"] != "IZB-DMD-WH" {
		t.Fatalf("rf_domain: %v", snap["rf_domain"])
	}
}

func encodeSymbolRFDomain(rfd string) []byte {
	inner := 2 + len(rfd)
	out := make([]byte, 4+inner)
	binary.BigEndian.PutUint32(out[0:4], VendorSymbol)
	out[4] = SymbolAttrDeviceRFDomain
	out[5] = byte(inner)
	copy(out[6:], rfd)
	return out
}
