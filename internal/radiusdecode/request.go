package radiusdecode

const (
	PortAuth = 1812
	PortAcct = 1813
)

func IsRequestCode(code uint8) bool {
	return code == 1 || code == 4
}

func IsRequest(raw []byte) bool {
	if len(raw) < 1 {
		return false
	}
	return IsRequestCode(raw[0])
}

func IsResponseCode(code uint8) bool {
	switch code {
	case 2, 3, 5, 11:
		return true
	default:
		return false
	}
}

func IsResponse(raw []byte) bool {
	if len(raw) < 1 {
		return false
	}
	return IsResponseCode(raw[0])
}

func RequestPort(code uint8) int {
	if code == 4 {
		return PortAcct
	}
	return PortAuth
}

// NASFallbackAddr returns the UDP endpoint most likely to be the NAS when
// NAS-IP-Address is missing from the RADIUS attributes.
// Requests are sent by the NAS (src); responses are sent to the NAS (dst).
func NASFallbackAddr(code uint8, srcAddr, dstAddr string) string {
	if IsRequestCode(code) {
		return srcAddr
	}
	return dstAddr
}
