package transaction

import (
	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction/types"
	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction/version"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

type StateTX struct {
	*Base
	Descriptors []*StateDescriptor
}

func NewStateTX(ver version.TX) *StateTX {
	basicTrans := createBaseTransaction(types.State, ver)

	StateTX := &StateTX{}
	StateTX.Base = basicTrans
	StateTX.encodeExclusive = StateTX.encodeExcl
	StateTX.decodeExclusive = StateTX.decodeExcl
	return StateTX
}

func (s *StateTX) encodeExcl(bw *util.BinWriter) {

	bw.VarUint(uint64(len(s.Descriptors)))
	for _, desc := range s.Descriptors {
		desc.Encode(bw)
	}
}

func (s *StateTX) decodeExcl(br *util.BinReader) {
	lenDesc := br.VarUint()

	s.Descriptors = make([]*StateDescriptor, lenDesc)
	for i := 0; i < int(lenDesc); i++ {
		s.Descriptors[i] = &StateDescriptor{}
		s.Descriptors[i].Decode(br)
	}
}
