package tzsp

import "fmt"

const (
	tagEnd = 1
)

type Packet struct {
	Version uint8
	Type    uint8
	Encap   uint16
	Payload []byte
}

func Parse(datagram []byte) (Packet, error) {
	if len(datagram) < 4 {
		return Packet{}, fmt.Errorf("tzsp packet too short")
	}
	pkt := Packet{
		Version: datagram[0],
		Type:    datagram[1],
		Encap:   uint16(datagram[2])<<8 | uint16(datagram[3]),
	}
	offset := 4
	for offset < len(datagram) {
		tagType := datagram[offset]
		offset++
		if tagType == tagEnd {
			break
		}
		if offset >= len(datagram) {
			return Packet{}, fmt.Errorf("tzsp malformed tag")
		}
		tagLen := int(datagram[offset])
		offset++
		offset += tagLen
		if offset > len(datagram) {
			return Packet{}, fmt.Errorf("tzsp tag length overflow")
		}
	}
	if offset > len(datagram) {
		return Packet{}, fmt.Errorf("tzsp payload overflow")
	}
	pkt.Payload = datagram[offset:]
	return pkt, nil
}
