package radiusdecode

import (
	"encoding/binary"
	"net"
	"strings"
)

const (
	AttrUserName             uint8  = 1
	AttrNASIPAddress         uint8  = 4
	AttrNASPort              uint8  = 5
	AttrFramedIPAddress      uint8  = 8
	AttrVendorSpecific       uint8  = 26
	AttrCallingStationID     uint8  = 31
	AttrNASIdentifier        uint8  = 32
	AttrProxyState           uint8  = 33
	AttrAcctStatusType       uint8  = 40
	AttrAcctSessionID        uint8  = 44
	AttrMessageAuthenticator uint8  = 80
	AttrNASIPv6Address       uint8  = 95
	VendorSymbol             uint32 = 388
	SymbolAttrDeviceRFDomain uint8  = 32
)

func NASIPString(p *Packet) string {
	if v, ok := AttributeValue(p, AttrNASIPAddress); ok && len(v) == 4 {
		return net.IP(v).String()
	}
	if v, ok := AttributeValue(p, AttrNASIPv6Address); ok && len(v) == 16 {
		return net.IP(v).String()
	}
	return ""
}

func UserNameString(p *Packet) string {
	if v, ok := AttributeValue(p, AttrUserName); ok {
		return string(v)
	}
	return ""
}

func CallingStationString(p *Packet) string {
	if v, ok := AttributeValue(p, AttrCallingStationID); ok {
		return NormalizeMAC(string(v))
	}
	return ""
}

func HasProxyState(p *Packet) bool {
	return HasAttribute(p, AttrProxyState)
}

// RFDomainString returns Symbol-Device-RF-Domain from a Symbol VSA (vendor 388, attr 32).
func RFDomainString(p *Packet) string {
	if p == nil {
		return ""
	}
	for _, a := range p.Attributes {
		if a.Type != AttrVendorSpecific || len(a.Value) < 6 {
			continue
		}
		if binary.BigEndian.Uint32(a.Value[:4]) != VendorSymbol {
			continue
		}
		offset := 4
		for offset+2 <= len(a.Value) {
			typ := a.Value[offset]
			length := int(a.Value[offset+1])
			if length < 2 || offset+length > len(a.Value) {
				break
			}
			if typ == SymbolAttrDeviceRFDomain {
				return strings.TrimSpace(string(a.Value[offset+2 : offset+length]))
			}
			offset += length
		}
	}
	return ""
}
