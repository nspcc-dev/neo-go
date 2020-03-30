package transaction

import (
	"encoding/hex"
	"encoding/json"

	"github.com/nspcc-dev/neo-go/pkg/io"
)

// DescStateType represents the type of StateDescriptor.
type DescStateType uint8

// Valid DescStateType constants.
const (
	Account   DescStateType = 0x40
	Validator DescStateType = 0x48
)

// StateDescriptor ..
type StateDescriptor struct {
	Type  DescStateType
	Key   []byte
	Value []byte
	Field string
}

// DecodeBinary implements Serializable interface.
func (s *StateDescriptor) DecodeBinary(r *io.BinReader) {
	s.Type = DescStateType(r.ReadB())

	s.Key = r.ReadVarBytes()
	s.Field = r.ReadString()
	s.Value = r.ReadVarBytes()
}

// EncodeBinary implements Serializable interface.
func (s *StateDescriptor) EncodeBinary(w *io.BinWriter) {
	w.WriteB(byte(s.Type))
	w.WriteVarBytes(s.Key)
	w.WriteString(s.Field)
	w.WriteVarBytes(s.Value)
}

// stateDescriptor is a wrapper for StateDescriptor
type stateDescriptor struct {
	Type  DescStateType `json:"type"`
	Key   string        `json:"key"`
	Value string        `json:"value"`
	Field string        `json:"field"`
}

// MarshalJSON implements json.Marshaler interface.
func (s *StateDescriptor) MarshalJSON() ([]byte, error) {
	return json.Marshal(&stateDescriptor{
		Type:  s.Type,
		Key:   hex.EncodeToString(s.Key),
		Value: hex.EncodeToString(s.Value),
		Field: s.Field,
	})
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (s *StateDescriptor) UnmarshalJSON(data []byte) error {
	t := new(stateDescriptor)
	if err := json.Unmarshal(data, t); err != nil {
		return err
	}
	key, err := hex.DecodeString(t.Key)
	if err != nil {
		return err
	}
	value, err := hex.DecodeString(t.Value)
	if err != nil {
		return err
	}
	s.Key = key
	s.Value = value
	s.Field = t.Field
	s.Type = t.Type
	return nil
}
