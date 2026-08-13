package radiusdecode

import "strings"

func collectAttributeValues(packet *Packet, dict *AttributeDictionary) map[string][]string {
	if packet == nil {
		return nil
	}
	decoded := DecodePacketAttributes(packet, dict)
	if len(decoded) == 0 {
		return nil
	}
	attrs := make(map[string][]string, len(decoded))
	for _, attr := range decoded {
		attrs[attr.Name] = append(attrs[attr.Name], attr.ValueText)
	}
	return attrs
}

// BuildIdentitySnapshot extracts user, MAC, and NAS fields from any RADIUS packet.
// nasFallback is used when NAS-IP-Address is absent (typically the UDP source IP).
func BuildIdentitySnapshot(packet *Packet, dict *AttributeDictionary, nasFallback string) (map[string]any, bool) {
	attrs := collectAttributeValues(packet, dict)
	if len(attrs) == 0 {
		return nil, false
	}

	userName := first(attrs, "User-Name")
	if userName == "" {
		userName = first(attrs, "Attr-1")
	}

	mac := NormalizeMAC(first(attrs, "Calling-Station-Id"))
	if mac == "" {
		mac = NormalizeMAC(first(attrs, "Attr-31"))
	}

	nas := NASIPString(packet)
	if nas == "" {
		nas = first(attrs, "NAS-IP-Address")
	}
	if nas == "" {
		nas = first(attrs, "Attr-4")
	}
	if nas == "" {
		nas = strings.TrimSpace(nasFallback)
	}

	nasIdentifier := first(attrs, "NAS-Identifier")
	if nasIdentifier == "" {
		nasIdentifier = first(attrs, "Attr-32")
	}

	if userName == "" && mac == "" && nas == "" {
		return nil, false
	}

	snapshot := map[string]any{
		"user_name":          userName,
		"mac":                mac,
		"nas":                nas,
		"calling_station_id": first(attrs, "Calling-Station-Id"),
		"called_station_id":  first(attrs, "Called-Station-Id"),
		"nas_identifier":     nasIdentifier,
	}

	if HasProxyState(packet) || first(attrs, "Proxy-State") != "" || first(attrs, "Attr-33") != "" {
		snapshot["has_proxy_state"] = true
	}
	rfDomain := first(attrs, "Symbol-Device-RF-Domain")
	if rfDomain == "" {
		rfDomain = RFDomainString(packet)
	}
	if rfDomain != "" {
		snapshot["rf_domain"] = rfDomain
	}

	decoded := DecodePacketAttributes(packet, dict)
	if vendor := detectNASVendor(decoded); vendor != "" {
		snapshot["nas_vendor"] = vendor
	}

	return snapshot, true
}
