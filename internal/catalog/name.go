package catalog

import "strings"

const (
	ProxyNamePrefix = "PROXY-"
	ProxyMultiName  = "PROXY-VX"
)

// CatalogNASName is the autofill display name for a RADIUS client.
// Proxy-State present → PROXY-<RF-Domain>; no RFD yet → empty (do not stamp AP name).
// No Proxy-State → NAS-Identifier.
func CatalogNASName(hasProxy bool, rfDomain, nasIdentifier string) string {
	if hasProxy {
		return proxyNameForRFD(rfDomain)
	}
	return strings.TrimSpace(nasIdentifier)
}

func proxyNameForRFD(rfDomain string) string {
	rfd := strings.TrimSpace(rfDomain)
	rfd = strings.ReplaceAll(rfd, " ", "")
	if rfd == "" {
		return ""
	}
	if strings.EqualFold(rfd, "VX") {
		return ProxyMultiName
	}
	return ProxyNamePrefix + rfd
}

func isProxyAutofillName(name string) bool {
	return strings.HasPrefix(name, ProxyNamePrefix)
}

// MergeAutofillName combines a stored NAS name with a newly observed candidate.
// PROXY-VX is sticky. Two distinct PROXY-<RFD> names collapse to PROXY-VX.
func MergeAutofillName(existing, incoming, ip string) string {
	existing = strings.TrimSpace(existing)
	incoming = strings.TrimSpace(incoming)
	ip = strings.TrimSpace(ip)

	if existing == ProxyMultiName {
		return existing
	}
	if incoming == ProxyMultiName {
		return incoming
	}
	if incoming == "" {
		if existing == "" {
			return ip
		}
		return existing
	}
	if isProxyAutofillName(existing) && isProxyAutofillName(incoming) && existing != incoming {
		return ProxyMultiName
	}
	if isProxyAutofillName(incoming) {
		return incoming
	}
	if existing == "" || existing == ip {
		return incoming
	}
	return existing
}
