package radiusdecode

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"layeh.com/radius/dictionary"
)

type AttributeDefinition struct {
	VendorID       uint32
	AttrID         uint32
	Name           string
	VendorName     string
	Description    string
	Type           dictionary.AttributeType
	TypeName       string
	HasTag         bool
	Encrypt        int
	ValuesByNumber map[uint64]string
	ValuesByName   map[string]uint64
}

type VendorDefinition struct {
	VendorID     uint32
	VendorName   string
	TypeOctets   int
	LengthOctets int
	ByIDDense    []*AttributeDefinition
	ByIDSparse   map[uint32]*AttributeDefinition
	ByName       map[string]*AttributeDefinition
}

type AttributeDictionary struct {
	standard       []*AttributeDefinition
	standardByName map[string]*AttributeDefinition
	vendors        map[uint32]*VendorDefinition
	vendorLabels   map[uint32]string
	vendorByName   map[string]uint32
}

type DecodedAttribute struct {
	Name       string
	VendorID   uint32
	VendorName string
	AttrID     uint32
	IsVSA      bool
	Value      any
	ValueText  string
	Raw        string
	EnumName   string
	Definition *AttributeDefinition
}

func NewAttributeDictionary() *AttributeDictionary {
	return &AttributeDictionary{
		standard:       make([]*AttributeDefinition, 256),
		standardByName: map[string]*AttributeDefinition{},
		vendors:        map[uint32]*VendorDefinition{},
		vendorLabels:   map[uint32]string{},
		vendorByName:   map[string]uint32{},
	}
}

func LoadAttributeDictionaryByGlob(pattern string) (*AttributeDictionary, []string, error) {
	if strings.TrimSpace(pattern) == "" {
		return NewAttributeDictionary(), nil, nil
	}
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, nil, fmt.Errorf("glob dictionaries %s: %w", pattern, err)
	}
	sort.Strings(files)
	dict := NewAttributeDictionary()
	for _, file := range files {
		if err := dict.AddFile(file); err != nil {
			return nil, nil, err
		}
	}
	return dict, files, nil
}

func parseInlineComments(path string) map[string]string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	result := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "ATTRIBUTE") {
			continue
		}
		parts := strings.SplitN(line, "#", 2)
		if len(parts) != 2 {
			continue
		}
		comment := strings.TrimSpace(parts[1])
		if comment == "" {
			continue
		}
		fields := strings.Fields(parts[0])
		if len(fields) < 2 {
			continue
		}
		result[fields[1]] = comment
	}
	return result
}

func (d *AttributeDictionary) AddFile(path string) error {
	comments := parseInlineComments(path)
	parser := dictionary.Parser{Opener: &dictionary.FileSystemOpener{Root: filepath.Dir(path)}}
	parsed, err := parser.ParseFile(filepath.Base(path))
	if err != nil {
		return fmt.Errorf("parse dictionary %s: %w", path, err)
	}

	for _, attribute := range parsed.Attributes {
		if len(attribute.OID) == 1 {
			def := d.ensureStandardDefinition(uint32(attribute.OID[0]))
			d.applyAttribute(def, attribute, 0, "")
			if c := comments[attribute.Name]; c != "" {
				def.Description = c
			}
		}
	}
	for _, value := range parsed.Values {
		def := d.LookupStandardByName(value.Attribute)
		if def != nil {
			d.applyValue(def, value)
		}
	}
	for _, vendor := range parsed.Vendors {
		vendorID := uint32(vendor.Number)
		d.vendorLabels[vendorID] = vendor.Name
		d.vendorByName[canonicalAttributeName(vendor.Name)] = vendorID
		vendorDef := d.ensureVendorDefinition(vendorID)
		vendorDef.VendorName = vendor.Name
		vendorDef.TypeOctets = vendor.GetTypeOctets()
		vendorDef.LengthOctets = vendor.GetLengthOctets()
		for _, attribute := range vendor.Attributes {
			if len(attribute.OID) == 0 {
				continue
			}
			attributeID := uint32(attribute.OID[len(attribute.OID)-1])
			def := vendorDef.ensureAttribute(attributeID)
			d.applyAttribute(def, attribute, vendorID, vendor.Name)
			if c := comments[attribute.Name]; c != "" {
				def.Description = c
			}
		}
		for _, value := range vendor.Values {
			def := vendorDef.ByName[canonicalAttributeName(value.Attribute)]
			if def != nil {
				d.applyValue(def, value)
			}
		}
	}
	return nil
}

func (d *AttributeDictionary) LookupStandard(attrType int) *AttributeDefinition {
	if d == nil || attrType < 0 || attrType >= len(d.standard) {
		return nil
	}
	return d.standard[attrType]
}

func (d *AttributeDictionary) LookupStandardByName(name string) *AttributeDefinition {
	if d == nil {
		return nil
	}
	return d.standardByName[canonicalAttributeName(name)]
}

func (d *AttributeDictionary) LookupVendorAttribute(vendorID uint32, attrType uint32) *AttributeDefinition {
	if d == nil {
		return nil
	}
	vendor := d.vendors[vendorID]
	if vendor == nil {
		return nil
	}
	if int(attrType) < len(vendor.ByIDDense) {
		if def := vendor.ByIDDense[attrType]; def != nil {
			return def
		}
	}
	return vendor.ByIDSparse[attrType]
}

func (d *AttributeDictionary) VendorName(vendorID uint32) string {
	if d == nil {
		return ""
	}
	if vendor := d.vendors[vendorID]; vendor != nil && vendor.VendorName != "" {
		return vendor.VendorName
	}
	return d.vendorLabels[vendorID]
}

func (d *AttributeDictionary) ensureStandardDefinition(attrID uint32) *AttributeDefinition {
	if int(attrID) >= len(d.standard) {
		next := make([]*AttributeDefinition, attrID+1)
		copy(next, d.standard)
		d.standard = next
	}
	if d.standard[attrID] == nil {
		d.standard[attrID] = &AttributeDefinition{AttrID: attrID}
	}
	return d.standard[attrID]
}

func (d *AttributeDictionary) ensureVendorDefinition(vendorID uint32) *VendorDefinition {
	if vendor, ok := d.vendors[vendorID]; ok {
		return vendor
	}
	vendor := &VendorDefinition{
		VendorID:   vendorID,
		ByIDDense:  make([]*AttributeDefinition, 256),
		ByIDSparse: map[uint32]*AttributeDefinition{},
		ByName:     map[string]*AttributeDefinition{},
	}
	d.vendors[vendorID] = vendor
	return vendor
}

func (v *VendorDefinition) ensureAttribute(attrID uint32) *AttributeDefinition {
	if int(attrID) < len(v.ByIDDense) {
		if v.ByIDDense[attrID] == nil {
			v.ByIDDense[attrID] = &AttributeDefinition{VendorID: v.VendorID, AttrID: attrID}
		}
		return v.ByIDDense[attrID]
	}
	if def, ok := v.ByIDSparse[attrID]; ok {
		return def
	}
	def := &AttributeDefinition{VendorID: v.VendorID, AttrID: attrID}
	v.ByIDSparse[attrID] = def
	return def
}

func (d *AttributeDictionary) applyAttribute(def *AttributeDefinition, attribute *dictionary.Attribute, vendorID uint32, vendorName string) {
	if def == nil || attribute == nil {
		return
	}
	def.VendorID = vendorID
	def.Name = attribute.Name
	def.VendorName = vendorName
	def.Type = attribute.Type
	def.TypeName = attribute.Type.String()
	def.HasTag = attribute.HasTag()
	if attribute.FlagEncrypt.Valid {
		def.Encrypt = attribute.FlagEncrypt.Int
	}
	if def.ValuesByNumber == nil {
		def.ValuesByNumber = map[uint64]string{}
	}
	if def.ValuesByName == nil {
		def.ValuesByName = map[string]uint64{}
	}
	if vendorID == 0 {
		d.standardByName[canonicalAttributeName(attribute.Name)] = def
		return
	}
	vendor := d.ensureVendorDefinition(vendorID)
	vendor.ByName[canonicalAttributeName(attribute.Name)] = def
}

func (d *AttributeDictionary) applyValue(def *AttributeDefinition, value *dictionary.Value) {
	if def == nil || value == nil {
		return
	}
	if def.ValuesByNumber == nil {
		def.ValuesByNumber = map[uint64]string{}
	}
	if def.ValuesByName == nil {
		def.ValuesByName = map[string]uint64{}
	}
	def.ValuesByNumber[value.Number] = value.Name
	def.ValuesByName[canonicalAttributeName(value.Name)] = value.Number
}

func DecodePacketAttributes(packet *Packet, dict *AttributeDictionary) []DecodedAttribute {
	if packet == nil {
		return nil
	}
	decoded := make([]DecodedAttribute, 0, len(packet.Attributes))
	for _, avp := range packet.Attributes {
		if avp.Type != 26 {
			def := dict.LookupStandard(int(avp.Type))
			decoded = append(decoded, newDecodedAttribute(def, 0, uint32(avp.Type), false, avp.Value))
			continue
		}
		if len(avp.Value) < 4 {
			decoded = append(decoded, DecodedAttribute{Name: "Vendor-Specific", AttrID: uint32(avp.Type), IsVSA: true, ValueText: decodeRawAttribute(avp.Value), Raw: decodeRawAttribute(avp.Value)})
			continue
		}
		vendorID := binary.BigEndian.Uint32(avp.Value[:4])
		vendorRaw := avp.Value[4:]
		parsed := decodeVendorAttributes(dict, vendorID, vendorRaw)
		if len(parsed) == 0 {
			decoded = append(decoded, DecodedAttribute{
				Name:       fmt.Sprintf("Vendor-%d", vendorID),
				VendorID:   vendorID,
				VendorName: dict.VendorName(vendorID),
				AttrID:     uint32(avp.Type),
				IsVSA:      true,
				ValueText:  decodeRawAttribute(vendorRaw),
				Raw:        decodeRawAttribute(vendorRaw),
			})
			continue
		}
		decoded = append(decoded, parsed...)
	}
	return decoded
}

func decodeVendorAttributes(dict *AttributeDictionary, vendorID uint32, raw []byte) []DecodedAttribute {
	vendor := (*VendorDefinition)(nil)
	if dict != nil {
		vendor = dict.vendors[vendorID]
	}
	typeOctets, lengthOctets := 1, 1
	vendorName := ""
	if vendor != nil {
		typeOctets = vendor.TypeOctets
		lengthOctets = vendor.LengthOctets
		vendorName = vendor.VendorName
	}
	if typeOctets <= 0 {
		typeOctets = 1
	}
	if lengthOctets <= 0 {
		lengthOctets = 1
	}
	headerLen := typeOctets + lengthOctets
	if len(raw) < headerLen {
		return nil
	}

	decoded := make([]DecodedAttribute, 0)
	for offset := 0; offset+headerLen <= len(raw); {
		attrID := parseBigEndianUint(raw[offset : offset+typeOctets])
		length := parseBigEndianUint(raw[offset+typeOctets : offset+headerLen])
		if int(length) < headerLen || offset+int(length) > len(raw) {
			return decoded
		}
		value := raw[offset+headerLen : offset+int(length)]
		def := (*AttributeDefinition)(nil)
		if vendor != nil {
			def = vendor.ensureAttribute(attrID)
			if def.Name == "" {
				def = nil
			}
		}
		attr := newDecodedAttribute(def, vendorID, attrID, true, value)
		if attr.VendorName == "" {
			attr.VendorName = vendorName
		}
		decoded = append(decoded, attr)
		offset += int(length)
	}
	return decoded
}

func newDecodedAttribute(def *AttributeDefinition, vendorID, attrID uint32, isVSA bool, value []byte) DecodedAttribute {
	decodedValue, valueText, enumName := decodeAttributeByDefinition(def, value)
	name := ""
	vendorName := ""
	if def != nil {
		name = def.Name
		vendorName = def.VendorName
	}
	if name == "" {
		if isVSA {
			name = fmt.Sprintf("Vendor-%d-Attr-%d", vendorID, attrID)
		} else {
			name = fmt.Sprintf("Attr-%d", attrID)
		}
	}
	return DecodedAttribute{Name: name, VendorID: vendorID, VendorName: vendorName, AttrID: attrID, IsVSA: isVSA, Value: decodedValue, ValueText: valueText, Raw: decodeRawAttribute(value), EnumName: enumName, Definition: def}
}

func decodeAttributeByDefinition(def *AttributeDefinition, value []byte) (any, string, string) {
	if def == nil {
		text := decodeRawAttribute(value)
		return append([]byte(nil), value...), text, ""
	}
	var decoded any
	var text string
	var enumName string
	var err error

	switch def.Type {
	case dictionary.AttributeString:
		decoded = string(value)
		text = decoded.(string)
	case dictionary.AttributeOctets, dictionary.AttributeABinary, dictionary.AttributeTLV, dictionary.AttributeVSA:
		decoded = append([]byte(nil), value...)
		text = decodeRawAttribute(value)
	case dictionary.AttributeIPAddr:
		if len(value) != 4 {
			err = fmt.Errorf("invalid ipv4 length")
		} else {
			decoded = net.IP(value)
		}
	case dictionary.AttributeInteger:
		if len(value) != 4 {
			err = fmt.Errorf("invalid integer length")
		} else {
			decoded = uint64(binary.BigEndian.Uint32(value))
		}
	case dictionary.AttributeByte:
		if len(value) != 1 {
			err = fmt.Errorf("invalid byte length")
		} else {
			decoded = uint64(value[0])
		}
	case dictionary.AttributeShort:
		if len(value) != 2 {
			err = fmt.Errorf("invalid short length")
		} else {
			decoded = uint64(binary.BigEndian.Uint16(value))
		}
	case dictionary.AttributeDate:
		if len(value) != 4 {
			err = fmt.Errorf("invalid date length")
		} else {
			decoded = time.Unix(int64(binary.BigEndian.Uint32(value)), 0).UTC()
		}
	case dictionary.AttributeEther:
		decoded = decodeHardwareAddress(value)
	default:
		decoded = append([]byte(nil), value...)
		text = decodeRawAttribute(value)
	}

	if err != nil {
		fallback := decodeRawAttribute(value)
		return append([]byte(nil), value...), fallback, ""
	}
	if text == "" {
		text = stringifyDecodedValue(decoded)
	}
	if numeric, ok := decodedAsUint64(decoded); ok && len(def.ValuesByNumber) > 0 {
		if enum, exists := def.ValuesByNumber[numeric]; exists {
			return decoded, enum, enum
		}
	}
	return decoded, text, enumName
}

func parseBigEndianUint(raw []byte) uint32 {
	var value uint32
	for _, part := range raw {
		value = (value << 8) | uint32(part)
	}
	return value
}

func decodeRawAttribute(value []byte) string {
	if len(value) == 0 {
		return ""
	}
	printable := true
	for _, b := range value {
		if b >= 32 && b <= 126 {
			continue
		}
		if b == '\n' || b == '\r' || b == '\t' {
			continue
		}
		printable = false
		break
	}
	if printable {
		return strings.TrimSpace(string(value))
	}
	return fmt.Sprintf("0x%x", value)
}

func decodeHardwareAddress(value []byte) string {
	if len(value) == 6 || len(value) == 8 {
		addr := net.HardwareAddr(value)
		return strings.ToUpper(addr.String())
	}
	return decodeRawAttribute(value)
}

func stringifyDecodedValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case []byte:
		return decodeRawAttribute(typed)
	case net.IP:
		return typed.String()
	case time.Time:
		return typed.UTC().Format(time.RFC3339)
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func decodedAsUint64(value any) (uint64, bool) {
	switch typed := value.(type) {
	case uint64:
		return typed, true
	case uint32:
		return uint64(typed), true
	case uint16:
		return uint64(typed), true
	case uint8:
		return uint64(typed), true
	case int64:
		return uint64(typed), true
	case int32:
		return uint64(typed), true
	case int:
		return uint64(typed), true
	default:
		return 0, false
	}
}

func canonicalAttributeName(name string) string {
	trimmed := strings.TrimSpace(strings.ToLower(name))
	if trimmed == "" {
		return ""
	}
	return strings.NewReplacer("_", "-", " ", "-", ".", "-").Replace(trimmed)
}

func NormalizeMAC(value string) string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return ""
	}
	trimmed = strings.ReplaceAll(trimmed, "-", ":")
	trimmed = strings.ReplaceAll(trimmed, ".", "")
	if len(trimmed) == 12 {
		parts := make([]string, 0, 6)
		for i := 0; i < 12; i += 2 {
			parts = append(parts, trimmed[i:i+2])
		}
		trimmed = strings.Join(parts, ":")
	}
	return strings.ToUpper(trimmed)
}

func parseUintFromText(v string) uint64 {
	n, _ := strconv.ParseUint(strings.TrimSpace(v), 10, 64)
	return n
}
