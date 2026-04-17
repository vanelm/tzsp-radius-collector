package tzsp

import "testing"

func TestParseTZSPWithEndTag(t *testing.T) {
	raw := []byte{
		1, 0, 0, 1,
		1,
		0x01, 0x02, 0x03,
	}
	pkt, err := Parse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkt.Version != 1 || pkt.Type != 0 || pkt.Encap != 1 {
		t.Fatalf("unexpected header: %+v", pkt)
	}
	if len(pkt.Payload) != 3 {
		t.Fatalf("unexpected payload len: %d", len(pkt.Payload))
	}
}
