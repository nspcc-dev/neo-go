package transaction

import (
	"encoding/binary"
	"io"

	"github.com/CityOfZion/neo-go/pkg/util"
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
func (s *StateDescriptor) DecodeBinary(r io.Reader) error {
	if err := binary.Read(r, binary.LittleEndian, &s.Type); err != nil {
		return err
	}

	keyLen := util.ReadVarUint(r)
	s.Key = make([]byte, keyLen)
	if err := binary.Read(r, binary.LittleEndian, s.Key); err != nil {
		return err
	}

	valLen := util.ReadVarUint(r)
	s.Value = make([]byte, valLen)
	if err := binary.Read(r, binary.LittleEndian, s.Value); err != nil {
		return err
	}

	fieldLen := util.ReadVarUint(r)
	field := make([]byte, fieldLen)
	if err := binary.Read(r, binary.LittleEndian, field); err != nil {
		return err
	}
	s.Field = string(field)

	return nil
}

// EncodeBinary implements the Payload interface.
func (s *StateDescriptor) EncodeBinary(w io.Writer) error {
	return nil
}
