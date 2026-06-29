package radiusdecode

import "testing"

func TestIsRequest(t *testing.T) {
	if !IsRequestCode(1) || !IsRequestCode(4) {
		t.Fatal("expected codes 1 and 4 to be requests")
	}
	if IsRequestCode(2) || IsRequestCode(5) {
		t.Fatal("expected codes 2 and 5 not to be requests")
	}
	raw := []byte{1, 0, 0, 20}
	if !IsRequest(raw) {
		t.Fatal("expected access-request bytes to be request")
	}
	if RequestPort(4) != PortAcct || RequestPort(1) != PortAuth {
		t.Fatal("unexpected request ports")
	}
}

func TestBuildAccessRequest(t *testing.T) {
	raw, err := BuildAccessRequest(AccessRequestParams{
		UserName:       "testuser",
		NASIP:          "10.0.0.1",
		CallingStation: "AA-BB-CC-DD-EE-FF",
		NASIdentifier:  "ap-1",
		Identifier:     42,
	})
	if err != nil {
		t.Fatal(err)
	}
	pkt, err := ParsePacket(raw)
	if err != nil {
		t.Fatal(err)
	}
	if pkt.Code != 1 || pkt.Identifier != 42 {
		t.Fatalf("unexpected packet header: %+v", pkt)
	}
}
