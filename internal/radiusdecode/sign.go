package radiusdecode

import (
	"crypto/hmac"
	"crypto/md5"
)

// SignPacket recalculates fields that depend on the shared secret.
// Accounting-Request Authenticator is MD5(packet with zero authenticator + secret).
// Message-Authenticator (if present) is HMAC-MD5 over the packet with that AVP zeroed.
func SignPacket(raw []byte, secret []byte) []byte {
	if len(raw) < 20 || len(secret) == 0 {
		return raw
	}
	out := append([]byte(nil), raw...)
	zeroMessageAuthenticator(out)
	if out[0] == 4 {
		for i := 4; i < 20; i++ {
			out[i] = 0
		}
		h := md5.New()
		h.Write(out)
		h.Write(secret)
		copy(out[4:20], h.Sum(nil))
	}
	signMessageAuthenticator(out, secret)
	return out
}

func NeedsSecretToModify(p *Packet) bool {
	if p == nil {
		return false
	}
	return p.Code == 4 || HasAttribute(p, AttrMessageAuthenticator)
}

func zeroMessageAuthenticator(raw []byte) {
	offset := 20
	for offset+2 <= len(raw) {
		typ := raw[offset]
		length := int(raw[offset+1])
		if length < 2 || offset+length > len(raw) {
			return
		}
		if typ == AttrMessageAuthenticator && length == 18 {
			for i := offset + 2; i < offset+length; i++ {
				raw[i] = 0
			}
			return
		}
		offset += length
	}
}

func signMessageAuthenticator(raw []byte, secret []byte) {
	offset := 20
	maOff := -1
	for offset+2 <= len(raw) {
		typ := raw[offset]
		length := int(raw[offset+1])
		if length < 2 || offset+length > len(raw) {
			return
		}
		if typ == AttrMessageAuthenticator && length == 18 {
			maOff = offset + 2
			for i := 0; i < 16; i++ {
				raw[maOff+i] = 0
			}
			break
		}
		offset += length
	}
	if maOff < 0 {
		return
	}
	mac := hmac.New(md5.New, secret)
	mac.Write(raw)
	copy(raw[maOff:maOff+16], mac.Sum(nil))
}
