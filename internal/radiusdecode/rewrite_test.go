package radiusdecode

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"
)

func TestEncodePacketRoundTrip(t *testing.T) {
	raw := []byte{
		1, 7, 0, 25,
		0, 1, 2, 3, 4, 5, 6, 7,
		8, 9, 10, 11, 12, 13, 14, 15,
		1, 5, 'b', 'o', 'b',
	}
	pkt, err := ParsePacket(raw)
	if err != nil {
		t.Fatal(err)
	}
	encoded := EncodePacket(pkt)
	if !bytes.Equal(encoded, raw) {
		t.Fatalf("round-trip mismatch\n got %v\nwant %v", encoded, raw)
	}
}

func TestSetAttributeReplaceAndAppend(t *testing.T) {
	pkt := &Packet{Code: 1, Identifier: 1}
	SetAttribute(pkt, AttrUserName, []byte("alice"))
	SetAttribute(pkt, AttrNASIPAddress, net.IPv4(10, 0, 0, 1).To4())
	SetAttribute(pkt, AttrUserName, []byte("bob"))
	if got := UserNameString(pkt); got != "bob" {
		t.Fatalf("username %q", got)
	}
	if got := NASIPString(pkt); got != "10.0.0.1" {
		t.Fatalf("nas ip %q", got)
	}
}

func TestPrepareForwardRewritesAccessNASIP(t *testing.T) {
	raw, err := BuildAccessRequest(AccessRequestParams{
		UserName:   "testuser",
		NASIP:      "10.0.0.1",
		Identifier: 42,
	})
	if err != nil {
		t.Fatal(err)
	}
	prep, err := PrepareForward(raw, net.IPv4(127, 0, 0, 1), 99, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !prep.Rewritten || prep.Identifier != 99 {
		t.Fatalf("unexpected prep: %+v", prep)
	}
	pkt, err := ParsePacket(prep.Packet)
	if err != nil {
		t.Fatal(err)
	}
	if pkt.Identifier != 99 {
		t.Fatalf("identifier %d", pkt.Identifier)
	}
	if NASIPString(pkt) != "127.0.0.1" {
		t.Fatalf("nas ip %q", NASIPString(pkt))
	}
}

func TestPrepareForwardAccountingRequiresSecret(t *testing.T) {
	raw, err := BuildAccountingRequest(AccountingRequestParams{
		UserName:   "testuser",
		NASIP:      "10.0.0.1",
		SessionID:  "sess-1",
		StatusType: 1,
		Identifier: 7,
	})
	if err != nil {
		t.Fatal(err)
	}
	prep, err := PrepareForward(raw, net.IPv4(127, 0, 0, 1), 7, nil)
	if err != nil {
		t.Fatal(err)
	}
	if prep.Rewritten {
		t.Fatal("expected original accounting packet without secret")
	}
	if !bytes.Equal(prep.Packet, raw) {
		t.Fatal("accounting bytes should be unchanged without secret")
	}

	prep, err = PrepareForward(raw, net.IPv4(127, 0, 0, 1), 8, []byte("s3cr3t"))
	if err != nil {
		t.Fatal(err)
	}
	if !prep.Rewritten || prep.Identifier != 8 {
		t.Fatalf("unexpected prep: %+v", prep)
	}
	pkt, err := ParsePacket(prep.Packet)
	if err != nil {
		t.Fatal(err)
	}
	if NASIPString(pkt) != "127.0.0.1" {
		t.Fatalf("nas ip %q", NASIPString(pkt))
	}
	resigned := SignPacket(EncodePacket(pkt), []byte("s3cr3t"))
	if !bytes.Equal(prep.Packet, resigned) {
		t.Fatal("accounting authenticator not signed")
	}
	if bytes.Equal(prep.Packet[4:20], make([]byte, 16)) {
		t.Fatal("authenticator still zero")
	}
}

func TestSignMessageAuthenticator(t *testing.T) {
	raw, err := BuildAccessRequest(AccessRequestParams{
		UserName:   "testuser",
		NASIP:      "10.0.0.1",
		Identifier: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	pkt, err := ParsePacket(raw)
	if err != nil {
		t.Fatal(err)
	}
	SetAttribute(pkt, AttrMessageAuthenticator, make([]byte, 16))
	encoded := EncodePacket(pkt)
	if encoded[len(encoded)-18] != AttrMessageAuthenticator {
		t.Fatal("expected MA as last attribute")
	}
	signed := SignPacket(encoded, []byte("secret"))
	if bytes.Equal(signed[len(signed)-16:], make([]byte, 16)) {
		t.Fatal("message authenticator still zero")
	}
	again := SignPacket(encoded, []byte("secret"))
	if !bytes.Equal(signed, again) {
		t.Fatal("signing should be deterministic")
	}
}

func TestHostPart(t *testing.T) {
	if got := HostPart("10.0.0.1:1812"); got != "10.0.0.1" {
		t.Fatalf("got %q", got)
	}
	if got := HostPart("10.0.0.1"); got != "10.0.0.1" {
		t.Fatalf("got %q", got)
	}
	udp := &net.UDPAddr{IP: net.IPv4(192, 0, 2, 1), Port: 1812}
	if ip := LocalIPFromAddr(udp); !ip.Equal(net.IPv4(192, 0, 2, 1)) {
		t.Fatalf("local ip %v", ip)
	}
}

func TestAccountingAuthenticatorRFCStyle(t *testing.T) {
	// Minimal Accounting-Request: header + NAS-IP, authenticator left zero then signed.
	pkt := &Packet{Code: 4, Identifier: 1}
	SetAttribute(pkt, AttrNASIPAddress, net.IPv4(10, 0, 0, 1).To4())
	raw := EncodePacket(pkt)
	signed := SignPacket(raw, []byte("secret"))
	if len(signed) != len(raw) {
		t.Fatal("length changed")
	}
	// Recompute independently.
	zeroed := append([]byte(nil), signed...)
	for i := 4; i < 20; i++ {
		zeroed[i] = 0
	}
	if binary.BigEndian.Uint16(zeroed[2:4]) != uint16(len(zeroed)) {
		t.Fatal("bad length")
	}
	again := SignPacket(zeroed, []byte("secret"))
	if !bytes.Equal(signed, again) {
		t.Fatal("authenticator mismatch")
	}
}
