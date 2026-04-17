package radiusdecode

import "testing"

func TestParseRadiusPacket(t *testing.T) {
	// Access-Request with one User-Name AVP containing "bob"
	raw := []byte{
		1, 7, 0, 25,
		0, 1, 2, 3, 4, 5, 6, 7,
		8, 9, 10, 11, 12, 13, 14, 15,
		1, 5, 'b', 'o', 'b',
	}
	pkt, err := ParsePacket(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkt.Code != 1 || pkt.Identifier != 7 || pkt.Length != 25 {
		t.Fatalf("unexpected packet header: %+v", pkt)
	}
	if len(pkt.Attributes) != 1 {
		t.Fatalf("expected one attribute, got %d", len(pkt.Attributes))
	}
	if pkt.Attributes[0].Type != 1 {
		t.Fatalf("unexpected attr type %d", pkt.Attributes[0].Type)
	}
}
