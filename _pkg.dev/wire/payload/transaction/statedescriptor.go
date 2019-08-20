package transaction

import (
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

// DescStateType represents the type of StateDescriptor.
type DescStateType uint8

// Valid DescStateType constants.
const (
	Account   DescStateType = 0x40
	Validator DescStateType = 0x48
)

// StateDescriptor represents a state descriptor on the neo network
// used in a state transaction
type StateDescriptor struct {
	Type  DescStateType
	Key   []byte
	Value []byte
	Field string
}

// Decode decodes a binary reader into a state descriptor
func (s *StateDescriptor) Decode(br *util.BinReader) {
	br.Read(&s.Type)

	keyLen := br.VarUint()
	s.Key = make([]byte, keyLen)
	br.Read(s.Key)

	valLen := br.VarUint()
	s.Value = make([]byte, valLen)
	br.Read(s.Value)

	fieldLen := br.VarUint()
	field := make([]byte, fieldLen)
	br.Read(field)

	s.Field = string(field)
}

//Encode encodes a state descriptor into a binary writer
func (s *StateDescriptor) Encode(bw *util.BinWriter) {
	bw.Write(s.Type)

	bw.VarUint(uint64(len(s.Key)))
	bw.Write(s.Key)

	bw.VarUint(uint64(len(s.Value)))
	bw.Write(s.Value)

	bw.VarString(s.Field)
}
