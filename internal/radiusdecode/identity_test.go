package radiusdecode

import "testing"

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
