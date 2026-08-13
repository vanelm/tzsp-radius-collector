package forwarder

import (
	"encoding/binary"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/vanelm/tzsp-radius-collector/internal/pipeline"
	"github.com/vanelm/tzsp-radius-collector/internal/radiusdecode"
	"github.com/vanelm/tzsp-radius-collector/internal/store"
)

func TestForwarderCapturesAccessConversation(t *testing.T) {
	ln, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go mockRADIUSReply(t, ln, 2, func(req []byte) {
		pkt, err := radiusdecode.ParsePacket(req)
		if err != nil {
			t.Errorf("parse forwarded request: %v", err)
			return
		}
		if radiusdecode.NASIPString(pkt) != "127.0.0.1" {
			t.Errorf("expected rewritten NAS-IP 127.0.0.1, got %q", radiusdecode.NASIPString(pkt))
		}
	})

	fwd := New(slog.Default(), nil, pipeline.NewProcessor(radiusdecode.NewAttributeDictionary(), slog.Default()), Config{})
	defer fwd.Close()
	fwd.SetConfig(Config{
		Enabled:    true,
		AuthTarget: ln.LocalAddr().String(),
		AcctTarget: ln.LocalAddr().String(),
	})

	raw, err := radiusdecode.BuildAccessRequest(radiusdecode.AccessRequestParams{
		UserName:       "alice",
		NASIP:          "10.0.0.9",
		CallingStation: "aa-bb-cc-dd-ee-ff",
		Identifier:     17,
	})
	if err != nil {
		t.Fatal(err)
	}
	fwd.MaybeForward(pipeline.Event{
		Source:      "test",
		ReceivedAt:  time.Now().UTC(),
		PacketBytes: raw,
		SrcAddr:     "10.0.0.9:1812",
	})

	conv := waitConversation(t, fwd, 2*time.Second)
	if conv.Status != "complete" {
		t.Fatalf("status %s", conv.Status)
	}
	if conv.RequestName != "Access-Request" || conv.ResponseName != "Access-Accept" {
		t.Fatalf("codes %s -> %s", conv.RequestName, conv.ResponseName)
	}
	if conv.UserName != "alice" {
		t.Fatalf("user %q", conv.UserName)
	}
	if conv.Request != nil || conv.Response != nil {
		t.Fatal("list endpoint should omit packet payloads")
	}
	full, ok := fwd.GetConversation(conv.ID)
	if !ok || full.Response == nil || full.Request == nil {
		t.Fatal("expected request and response payloads")
	}
	if !conv.Rewritten {
		t.Fatal("expected NAS-IP rewrite")
	}
}

func TestForwarderAccountingWithSecret(t *testing.T) {
	st := openTestStore(t)
	defer st.Close()
	if _, err := st.CreateNAS(store.NAS{Name: "ap", IP: "10.0.0.9", Secret: "s3cr3t"}); err != nil {
		t.Fatal(err)
	}

	ln, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go mockRADIUSReply(t, ln, 5, func(req []byte) {
		pkt, err := radiusdecode.ParsePacket(req)
		if err != nil {
			t.Errorf("parse: %v", err)
			return
		}
		if radiusdecode.NASIPString(pkt) != "127.0.0.1" {
			t.Errorf("expected rewritten NAS-IP, got %q", radiusdecode.NASIPString(pkt))
		}
	})

	fwd := New(slog.Default(), st, pipeline.NewProcessor(radiusdecode.NewAttributeDictionary(), slog.Default()), Config{})
	defer fwd.Close()
	fwd.SetConfig(Config{
		Enabled:    true,
		AuthTarget: ln.LocalAddr().String(),
		AcctTarget: ln.LocalAddr().String(),
	})

	raw, err := radiusdecode.BuildAccountingRequest(radiusdecode.AccountingRequestParams{
		UserName:   "bob",
		NASIP:      "10.0.0.9",
		SessionID:  "sess-1",
		StatusType: 1,
		Identifier: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	fwd.MaybeForward(pipeline.Event{
		Source:      "test",
		ReceivedAt:  time.Now().UTC(),
		PacketBytes: raw,
		SrcAddr:     "10.0.0.9:1813",
	})

	conv := waitConversation(t, fwd, 2*time.Second)
	if conv.Status != "complete" || conv.ResponseName != "Accounting-Response" {
		t.Fatalf("unexpected conversation: %+v", conv)
	}
	if !conv.Rewritten {
		t.Fatal("expected accounting rewrite with catalog secret")
	}
}

func mockRADIUSReply(t *testing.T, ln *net.UDPConn, code uint8, inspect func([]byte)) {
	t.Helper()
	buf := make([]byte, 4096)
	_ = ln.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, addr, err := ln.ReadFromUDP(buf)
	if err != nil {
		t.Errorf("mock read: %v", err)
		return
	}
	req := buf[:n]
	if inspect != nil {
		inspect(req)
	}
	resp := make([]byte, 20)
	resp[0] = code
	resp[1] = req[1]
	binary.BigEndian.PutUint16(resp[2:4], 20)
	copy(resp[4:20], req[4:20])
	if _, err := ln.WriteToUDP(resp, addr); err != nil {
		t.Errorf("mock write: %v", err)
	}
}

func waitConversation(t *testing.T, fwd *Forwarder, d time.Duration) Conversation {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		list := fwd.ListConversations()
		if len(list) > 0 && list[0].Status == "complete" {
			return list[0]
		}
		time.Sleep(20 * time.Millisecond)
	}
	list := fwd.ListConversations()
	if len(list) == 0 {
		t.Fatal("no conversations")
	}
	t.Fatalf("conversation not complete: %+v", list[0])
	return Conversation{}
}

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	return st
}
