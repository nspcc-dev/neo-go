package transaction

import (
	"github.com/CityOfZion/neo-go/pkg/io"
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

// DecodeBinary implements the Payload interface.
func (s *StateDescriptor) DecodeBinary(r *io.BinReader) error {
	r.ReadLE(&s.Type)

	s.Key = r.ReadBytes()
	s.Value = r.ReadBytes()
	s.Field = r.ReadString()

	return r.Err
}

// EncodeBinary implements the Payload interface.
func (s *StateDescriptor) EncodeBinary(w *io.BinWriter) error {
	w.WriteLE(s.Type)
	w.WriteBytes(s.Key)
	w.WriteBytes(s.Value)
	w.WriteString(s.Field)
	return w.Err
}

// Size returns serialized binary size for state descriptor.
func (s *StateDescriptor) Size() int {
	return 1 + io.GetVarSize(s.Key) + io.GetVarSize(s.Value) + io.GetVarSize(s.Field)
}
