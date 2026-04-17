package pipeline

import (
	"encoding/hex"
	"log"

	"github.com/elin/tzsp-radius-collector/internal/radiusdecode"
)

type Processor struct {
	dict   *radiusdecode.AttributeDictionary
	logger *log.Logger
}

func NewProcessor(dict *radiusdecode.AttributeDictionary, logger *log.Logger) *Processor {
	return &Processor{dict: dict, logger: logger}
}

func (p *Processor) Transform(event Event) (StreamMessage, error) {
	msg := StreamMessage{
		Timestamp:        event.ReceivedAt.UTC(),
		Source:           event.Source,
		RemoteAddr:       event.RemoteAddr,
		CaptureInterface: event.CaptureIface,
		DecodedAttributes: make([]map[string]any, 0),
	}

	packet, err := radiusdecode.ParsePacket(event.PacketBytes)
	if err != nil {
		msg.Errors = append(msg.Errors, err.Error())
		return msg, err
	}

	msg.Radius = RadiusPacket{
		Code:         packet.Code,
		CodeName:     radiusdecode.CodeName(packet.Code),
		Identifier:   packet.Identifier,
		Length:       packet.Length,
		Authenticator: hex.EncodeToString(packet.Authenticator[:]),
	}

	decoded := radiusdecode.DecodePacketAttributes(packet, p.dict)
	for _, attr := range decoded {
		msg.DecodedAttributes = append(msg.DecodedAttributes, map[string]any{
			"name":        attr.Name,
			"vendor_id":   attr.VendorID,
			"vendor_name": attr.VendorName,
			"attr_id":     attr.AttrID,
			"is_vsa":      attr.IsVSA,
			"value":       attr.ValueText,
			"raw":         attr.Raw,
			"enum_name":   attr.EnumName,
		})
	}

	if snapshot, ok := radiusdecode.BuildAccountingSnapshot(packet, p.dict, event.ReceivedAt.UTC()); ok {
		msg.Accounting = snapshot
	}

	return msg, nil
}
