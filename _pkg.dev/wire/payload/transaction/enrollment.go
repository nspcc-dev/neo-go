package transaction

import (
	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction/types"
	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction/version"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

//Enrollment represents an Enrollment transaction on the neo network
type Enrollment struct {
	*Base
	Key PublicKey
}

//NewEnrollment returns an Enrollment transaction
func NewEnrollment(ver version.TX) *Enrollment {
	basicTrans := createBaseTransaction(types.Enrollment, ver)

	enrollment := &Enrollment{
		Base: basicTrans,
	}
	enrollment.encodeExclusive = enrollment.encodeExcl
	enrollment.decodeExclusive = enrollment.decodeExcl
	return enrollment
}

func (e *Enrollment) encodeExcl(bw *util.BinWriter) {
	e.Key.Encode(bw)
}

func (e *Enrollment) decodeExcl(br *util.BinReader) {
	e.Key.Decode(br)
}
