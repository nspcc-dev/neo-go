package transaction

import (
	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction/types"
	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction/version"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

type Enrollment struct {
	*Base
	Key PublicKey
}

func NewEnrollment(ver version.TX) *Enrollment {
	basicTrans := createBaseTransaction(types.Enrollment, ver)

	Enrollment := &Enrollment{}
	Enrollment.Base = basicTrans
	Enrollment.encodeExclusive = Enrollment.encodeExcl
	Enrollment.decodeExclusive = Enrollment.decodeExcl
	return Enrollment
}

func (e *Enrollment) encodeExcl(bw *util.BinWriter) {
	e.Key.Encode(bw)

}

func (e *Enrollment) decodeExcl(br *util.BinReader) {
	e.Key.Decode(br)
}
