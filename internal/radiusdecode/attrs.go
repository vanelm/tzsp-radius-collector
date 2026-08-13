package radiusdecode

import "net"

const (
	AttrUserName             uint8 = 1
	AttrNASIPAddress         uint8 = 4
	AttrNASPort              uint8 = 5
	AttrFramedIPAddress      uint8 = 8
	AttrCallingStationID     uint8 = 31
	AttrNASIdentifier        uint8 = 32
	AttrProxyState           uint8 = 33
	AttrAcctStatusType       uint8 = 40
	AttrAcctSessionID        uint8 = 44
	AttrMessageAuthenticator uint8 = 80
	AttrNASIPv6Address       uint8 = 95
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
