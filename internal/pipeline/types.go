package pipeline

import "time"

type Event struct {
	Source       string
	ReceivedAt   time.Time
	PacketBytes  []byte
	RemoteAddr   string
	CaptureIface string
}

type StreamMessage struct {
	Timestamp         time.Time                      `json:"timestamp"`
	Source            string                         `json:"source"`
	RemoteAddr        string                         `json:"remote_addr,omitempty"`
	CaptureInterface  string                         `json:"capture_interface,omitempty"`
	Radius            RadiusPacket                   `json:"radius"`
	DecodedAttributes []map[string]any               `json:"decoded_attributes"`
	Accounting        map[string]any                 `json:"accounting,omitempty"`
	Errors            []string                       `json:"errors,omitempty"`
}

type RadiusPacket struct {
	Code         uint8  `json:"code"`
	CodeName     string `json:"code_name"`
	Identifier   uint8  `json:"identifier"`
	Length       uint16 `json:"length"`
	Authenticator string `json:"authenticator"`
}
