package radiusdecode

import "time"

func BuildAccountingSnapshot(packet *Packet, dict *AttributeDictionary, now time.Time) (map[string]any, bool) {
	if packet == nil || packet.Code != 4 {
		return nil, false
	}
	decoded := DecodePacketAttributes(packet, dict)
	if len(decoded) == 0 {
		return nil, false
	}

	attrs := map[string][]string{}
	for _, attr := range decoded {
		attrs[attr.Name] = append(attrs[attr.Name], attr.ValueText)
	}

	status := first(attrs, "Acct-Status-Type")
	if status == "" {
		status = first(attrs, "Attr-40")
	}

	mac := NormalizeMAC(first(attrs, "Calling-Station-Id"))
	if mac == "" {
		mac = NormalizeMAC(first(attrs, "Attr-31"))
	}
	if mac == "" {
		return nil, false
	}

	snapshot := map[string]any{
		"mac":                   mac,
		"status_type":           status,
		"acct_session_id":       first(attrs, "Acct-Session-Id"),
		"acct_multi_session_id": first(attrs, "Acct-Multi-Session-Id"),
		"user_name":             first(attrs, "User-Name"),
		"calling_station_id":    first(attrs, "Calling-Station-Id"),
		"called_station_id":     first(attrs, "Called-Station-Id"),
		"nas":                   first(attrs, "NAS-IP-Address"),
		"nas_identifier":        first(attrs, "NAS-Identifier"),
		"session_time_sec":      parseUintFromText(first(attrs, "Acct-Session-Time")),
		"input_octets":          parseUintFromText(first(attrs, "Acct-Input-Octets")),
		"output_octets":         parseUintFromText(first(attrs, "Acct-Output-Octets")),
		"input_packets":         parseUintFromText(first(attrs, "Acct-Input-Packets")),
		"output_packets":        parseUintFromText(first(attrs, "Acct-Output-Packets")),
		"terminate_cause":       first(attrs, "Acct-Terminate-Cause"),
		"last_event_at":         now.UTC().Format(time.RFC3339),
	}

	vendor := detectNASVendor(decoded)
	if vendor != "" {
		snapshot["nas_vendor"] = vendor
	}
	if framedIP := first(attrs, "Framed-IP-Address"); framedIP != "" {
		snapshot["framed_ip"] = framedIP
	}

	switch status {
	case "Start", "1":
		snapshot["online"] = true
		snapshot["started_at"] = now.UTC().Format(time.RFC3339)
	case "Interim-Update", "3":
		snapshot["online"] = true
		snapshot["last_interim_at"] = now.UTC().Format(time.RFC3339)
	case "Stop", "2":
		snapshot["online"] = false
		snapshot["stopped_at"] = now.UTC().Format(time.RFC3339)
	}

	return snapshot, true
}

func first(attrs map[string][]string, key string) string {
	values := attrs[key]
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func detectNASVendor(attrs []DecodedAttribute) string {
	for _, attr := range attrs {
		if attr.VendorName != "" {
			return attr.VendorName
		}
	}
	return ""
}
