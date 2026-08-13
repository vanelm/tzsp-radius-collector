package radiusdecode

import "time"

func BuildAccountingSnapshot(packet *Packet, dict *AttributeDictionary, now time.Time, nasFallback string) (map[string]any, bool) {
	if packet == nil || packet.Code != 4 {
		return nil, false
	}
	attrs := collectAttributeValues(packet, dict)
	if len(attrs) == 0 {
		return nil, false
	}

	status := first(attrs, "Acct-Status-Type")
	if status == "" {
		status = first(attrs, "Attr-40")
	}

	identity, ok := BuildIdentitySnapshot(packet, dict, nasFallback)
	if !ok {
		return nil, false
	}

	snapshot := map[string]any{
		"mac":                   identity["mac"],
		"status_type":           status,
		"acct_session_id":       first(attrs, "Acct-Session-Id"),
		"acct_multi_session_id": first(attrs, "Acct-Multi-Session-Id"),
		"user_name":             identity["user_name"],
		"calling_station_id":    identity["calling_station_id"],
		"called_station_id":     identity["called_station_id"],
		"nas":                   identity["nas"],
		"nas_identifier":        identity["nas_identifier"],
		"session_time_sec":      parseUintFromText(first(attrs, "Acct-Session-Time")),
		"input_octets":          parseUintFromText(first(attrs, "Acct-Input-Octets")),
		"output_octets":         parseUintFromText(first(attrs, "Acct-Output-Octets")),
		"input_packets":         parseUintFromText(first(attrs, "Acct-Input-Packets")),
		"output_packets":        parseUintFromText(first(attrs, "Acct-Output-Packets")),
		"terminate_cause":       first(attrs, "Acct-Terminate-Cause"),
		"last_event_at":         now.UTC().Format(time.RFC3339),
	}

	if vendor, _ := identity["nas_vendor"].(string); vendor != "" {
		snapshot["nas_vendor"] = vendor
	}
	if v, ok := identity["has_proxy_state"]; ok {
		snapshot["has_proxy_state"] = v
	}
	if rfd, _ := identity["rf_domain"].(string); rfd != "" {
		snapshot["rf_domain"] = rfd
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
