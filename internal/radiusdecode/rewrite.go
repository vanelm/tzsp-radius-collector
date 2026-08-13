package radiusdecode

import "net"

type ForwardPrep struct {
	Packet     []byte
	Rewritten  bool
	Identifier uint8
}

// PrepareForward rewrites the request so a RADIUS server replies to this host:
// NAS-IP-Address (or NAS-IPv6-Address) is set to localIP, and Identifier to ident.
// Accounting-Request and Message-Authenticator require secret; without it the
// original bytes are returned unchanged (UDP source still attracts the reply).
func PrepareForward(raw []byte, localIP net.IP, ident uint8, secret []byte) (ForwardPrep, error) {
	pkt, err := ParsePacket(raw)
	if err != nil {
		return ForwardPrep{}, err
	}

	// Access-Request without Message-Authenticator can be rewritten without a secret.
	// Accounting-Request and Message-Authenticator cover the packet with the secret.
	canRewrite := pkt.Code == 1 && !HasAttribute(pkt, AttrMessageAuthenticator)
	if !canRewrite && len(secret) == 0 {
		return ForwardPrep{Packet: append([]byte(nil), raw...), Identifier: pkt.Identifier}, nil
	}

	pkt.Identifier = ident
	if ip4 := localIP.To4(); ip4 != nil && !ip4.IsUnspecified() {
		SetAttribute(pkt, AttrNASIPAddress, []byte(ip4))
	} else if len(localIP) == net.IPv6len && !localIP.IsUnspecified() {
		SetAttribute(pkt, AttrNASIPv6Address, []byte(localIP.To16()))
	}

	encoded := EncodePacket(pkt)
	if len(secret) > 0 && (pkt.Code == 4 || HasAttribute(pkt, AttrMessageAuthenticator)) {
		encoded = SignPacket(encoded, secret)
	}
	return ForwardPrep{Packet: encoded, Rewritten: true, Identifier: ident}, nil
}

func LocalIPFromAddr(addr net.Addr) net.IP {
	if addr == nil {
		return nil
	}
	switch a := addr.(type) {
	case *net.UDPAddr:
		return a.IP
	case *net.TCPAddr:
		return a.IP
	case *net.IPAddr:
		return a.IP
	default:
		host, _, err := net.SplitHostPort(addr.String())
		if err != nil {
			return net.ParseIP(addr.String())
		}
		return net.ParseIP(host)
	}
}

func HostPart(addr string) string {
	if addr == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}
