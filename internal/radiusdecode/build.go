package radiusdecode

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
)

type AccessRequestParams struct {
	UserName       string
	NASIP          string
	CallingStation string
	NASIdentifier  string
	Identifier     uint8
}

type AccountingRequestParams struct {
	UserName       string
	NASIP          string
	CallingStation string
	NASIdentifier  string
	SessionID      string
	StatusType     uint32 // 1 Start, 2 Stop, 3 Interim
	SessionTime    uint32
	InputOctets    uint32
	OutputOctets   uint32
	TerminateCause uint32
	Identifier     uint8
}

func randomAuthenticator() ([16]byte, error) {
	var auth [16]byte
	_, err := rand.Read(auth[:])
	return auth, err
}

func encodeAVP(attrType uint8, value []byte) []byte {
	b := make([]byte, 2+len(value))
	b[0] = attrType
	b[1] = uint8(len(b))
	copy(b[2:], value)
	return b
}

func encodeStringAVP(attrType uint8, s string) []byte {
	return encodeAVP(attrType, []byte(s))
}

func encodeIPAVP(attrType uint8, ip string) ([]byte, error) {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return nil, fmt.Errorf("invalid ip %q", ip)
	}
	v4 := parsed.To4()
	if v4 == nil {
		return nil, fmt.Errorf("ipv4 required for %q", ip)
	}
	return encodeAVP(attrType, v4), nil
}

func encodeUint32AVP(attrType uint8, v uint32) []byte {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, v)
	return encodeAVP(attrType, buf)
}

func buildPacket(code uint8, identifier uint8, attrs [][]byte) ([]byte, error) {
	auth, err := randomAuthenticator()
	if err != nil {
		return nil, err
	}

	bodyLen := 20
	for _, a := range attrs {
		bodyLen += len(a)
	}
	if bodyLen > 65535 {
		return nil, fmt.Errorf("packet too large")
	}

	out := make([]byte, bodyLen)
	out[0] = code
	out[1] = identifier
	binary.BigEndian.PutUint16(out[2:4], uint16(bodyLen))
	copy(out[4:20], auth[:])
	offset := 20
	for _, a := range attrs {
		copy(out[offset:], a)
		offset += len(a)
	}
	return out, nil
}

func BuildAccessRequest(p AccessRequestParams) ([]byte, error) {
	if p.Identifier == 0 {
		var id [1]byte
		_, _ = rand.Read(id[:])
		p.Identifier = id[0]
	}
	var attrs [][]byte
	attrs = append(attrs, encodeStringAVP(1, p.UserName))
	if ipAVP, err := encodeIPAVP(4, p.NASIP); err == nil {
		attrs = append(attrs, ipAVP)
	} else {
		return nil, err
	}
	if p.CallingStation != "" {
		attrs = append(attrs, encodeStringAVP(31, p.CallingStation))
	}
	if p.NASIdentifier != "" {
		attrs = append(attrs, encodeStringAVP(32, p.NASIdentifier))
	}
	return buildPacket(1, p.Identifier, attrs)
}

func BuildAccountingRequest(p AccountingRequestParams) ([]byte, error) {
	if p.Identifier == 0 {
		var id [1]byte
		_, _ = rand.Read(id[:])
		p.Identifier = id[0]
	}
	if p.SessionID == "" {
		return nil, fmt.Errorf("session id required")
	}
	var attrs [][]byte
	attrs = append(attrs, encodeStringAVP(1, p.UserName))
	if ipAVP, err := encodeIPAVP(4, p.NASIP); err == nil {
		attrs = append(attrs, ipAVP)
	} else {
		return nil, err
	}
	if p.CallingStation != "" {
		attrs = append(attrs, encodeStringAVP(31, p.CallingStation))
	}
	if p.NASIdentifier != "" {
		attrs = append(attrs, encodeStringAVP(32, p.NASIdentifier))
	}
	attrs = append(attrs, encodeStringAVP(44, p.SessionID))
	attrs = append(attrs, encodeUint32AVP(40, p.StatusType))
	if p.SessionTime > 0 {
		attrs = append(attrs, encodeUint32AVP(46, p.SessionTime))
	}
	if p.InputOctets > 0 {
		attrs = append(attrs, encodeUint32AVP(42, p.InputOctets))
	}
	if p.OutputOctets > 0 {
		attrs = append(attrs, encodeUint32AVP(43, p.OutputOctets))
	}
	if p.TerminateCause > 0 {
		attrs = append(attrs, encodeUint32AVP(49, p.TerminateCause))
	}
	return buildPacket(4, p.Identifier, attrs)
}
