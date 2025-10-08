package manifest

import (
	"encoding/json"
	"gopkg.in/yaml.v3"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
)

type (
	ExtendedType struct {
		Base       smartcontract.ParamType `json:"type" yaml:"type"`
		Name       string                  `json:"namedtype,omitempty" yaml:"namedtype,omitempty"`
		Length     uint32                  `json:"length,omitempty" yaml:"length,omitempty"`
		ForbidNull bool                    `json:"forbidnull,omitempty" yaml:"forbidnull,omitempty"`
		Interface  string                  `json:"interface,omitempty" yaml:"interface,omitempty"`
		Key        smartcontract.ParamType `json:"key,omitempty" yaml:"key,omitempty"`
		Value      *ExtendedType           `json:"value,omitempty" yaml:"value,omitempty"`
		Fields     []Parameter             `json:"fields,omitempty" yaml:"fields,omitempty"`
	}
)

// Equals compares two extended types field-by-field and returns true if they are
// equal.
func (e *ExtendedType) Equals(other *ExtendedType) bool {
	if e == other {
		return true
	}
	if e == nil || other == nil {
		return false
	}
	if e.Base != other.Base && (e.Base != smartcontract.ByteArrayType && e.Base != smartcontract.StringType ||
		other.Base != smartcontract.ByteArrayType && other.Base != smartcontract.StringType) {
		return false
	}
	if e.Name != other.Name || e.Interface != other.Interface ||
		e.Key != other.Key || e.Length != other.Length || e.ForbidNull != other.ForbidNull {
		return false
	}
	if len(e.Fields) != len(other.Fields) {
		return false
	}
	for i := range e.Fields {
		pa := &e.Fields[i]
		pb := &other.Fields[i]
		if pa.Name != pb.Name {
			return false
		}
		if pa.Type != pb.Type && (pa.Type != smartcontract.ByteArrayType && pa.Type != smartcontract.StringType ||
			pb.Type != smartcontract.ByteArrayType && pb.Type != smartcontract.StringType) {
			return false
		}
		if !pa.ExtendedType.Equals(pb.ExtendedType) {
			return false
		}
	}
	return (e.Value == nil && other.Value == nil) || (e.Value != nil && other.Value != nil && e.Value.Equals(other.Value))
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (e *ExtendedType) UnmarshalYAML(node *yaml.Node) error {
	var raw map[string]interface{}
	if err := node.Decode(&raw); err != nil {
		return err
	}
	if v, ok := raw["base"]; ok {
		if _, ok := raw["type"]; !ok {
			raw["type"] = v
		}
		delete(raw, "base")
	}
	if v, ok := raw["name"]; ok {
		if _, ok := raw["namedtype"]; !ok {
			raw["namedtype"] = v
		}
		delete(raw, "name")
	}
	b, err := yaml.Marshal(raw)
	if err != nil {
		return err
	}
	type ext ExtendedType
	var tmp ext
	if err = yaml.Unmarshal(b, &tmp); err != nil {
		return err
	}
	*e = ExtendedType(tmp)
	return nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (e *ExtendedType) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if v, ok := raw["base"]; ok {
		if _, ok := raw["type"]; !ok {
			raw["type"] = v
		}
		delete(raw, "base")
	}
	if v, ok := raw["name"]; ok {
		if _, ok := raw["namedtype"]; !ok {
			raw["namedtype"] = v
		}
		delete(raw, "name")
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	type ext ExtendedType
	var tmp ext
	if err := json.Unmarshal(b, &tmp); err != nil {
		return err
	}
	*e = ExtendedType(tmp)
	return nil
}
