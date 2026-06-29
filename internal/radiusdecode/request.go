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

func RequestPort(code uint8) int {
	if code == 4 {
		return PortAcct
	}
	return PortAuth
}
