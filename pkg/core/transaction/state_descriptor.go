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

// DecodeBinary implements Serializable interface.
func (s *StateDescriptor) DecodeBinary(r *io.BinReader) {
	r.ReadLE(&s.Type)

	s.Key = r.ReadBytes()
	s.Value = r.ReadBytes()
	s.Field = r.ReadString()
}

// EncodeBinary implements Serializable interface.
func (s *StateDescriptor) EncodeBinary(w *io.BinWriter) {
	w.WriteLE(s.Type)
	w.WriteVarBytes(s.Key)
	w.WriteVarBytes(s.Value)
	w.WriteString(s.Field)
}
