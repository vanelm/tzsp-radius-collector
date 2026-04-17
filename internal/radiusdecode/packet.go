package radiusdecode

import (
	"encoding/binary"
	"fmt"
)

type AVP struct {
	Type  uint8
	Value []byte
}

type Packet struct {
	Code          uint8
	Identifier    uint8
	Length        uint16
	Authenticator [16]byte
	Attributes    []AVP
}

func ParsePacket(raw []byte) (*Packet, error) {
	if len(raw) < 20 {
		return nil, fmt.Errorf("radius packet too short")
	}
	length := binary.BigEndian.Uint16(raw[2:4])
	if int(length) > len(raw) || length < 20 {
		return nil, fmt.Errorf("radius invalid length %d", length)
	}
	body := raw[:length]
	packet := &Packet{
		Code:       body[0],
		Identifier: body[1],
		Length:     length,
	}
	copy(packet.Authenticator[:], body[4:20])

	offset := 20
	for offset < len(body) {
		if offset+2 > len(body) {
			return nil, fmt.Errorf("radius avp header truncated")
		}
		avpType := body[offset]
		avpLen := int(body[offset+1])
		if avpLen < 2 || offset+avpLen > len(body) {
			return nil, fmt.Errorf("radius avp invalid length")
		}
		packet.Attributes = append(packet.Attributes, AVP{
			Type:  avpType,
			Value: append([]byte(nil), body[offset+2:offset+avpLen]...),
		})
		offset += avpLen
	}
	return packet, nil
}

func CodeName(code uint8) string {
	switch code {
	case 1:
		return "Access-Request"
	case 2:
		return "Access-Accept"
	case 3:
		return "Access-Reject"
	case 4:
		return "Accounting-Request"
	case 5:
		return "Accounting-Response"
	case 11:
		return "Access-Challenge"
	default:
		return fmt.Sprintf("Code-%d", code)
	}
}
