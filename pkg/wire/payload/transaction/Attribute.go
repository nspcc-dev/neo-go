package transaction

import (
	"errors"

	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

// Attribute represents a Transaction attribute.
type Attribute struct {
	Usage AttrUsage
	Data  []byte
}

var errMaxData = errors.New("max Size of Attribute reached")

const maxAttrSize = 65535

// Encode encodes the given Attribute into the binary writer
func (a *Attribute) Encode(bw *util.BinWriter) {
	if len(a.Data) > maxAttrSize {
		bw.Err = errMaxData
		return
	}
	bw.Write(uint8(a.Usage))

	if a.Usage == DescriptionURL || a.Usage == Vote || (a.Usage >= Hash1 && a.Usage <= Hash15) {
		bw.Write(a.Data[:32])

	} else if a.Usage == Script {
		bw.Write(a.Data[:20])
	} else if a.Usage == ECDH02 || a.Usage == ECDH03 {
		bw.Write(a.Data[1:33])
	} else if a.Usage == CertURL || a.Usage == DescriptionURL || a.Usage == Description || a.Usage >= Remark {
		bw.VarUint(uint64(len(a.Data)))
		bw.Write(a.Data)
	} else {
		bw.Write(a.Data)
	}

}

// Decode decodes the binary reader into an Attribute object
func (a *Attribute) Decode(br *util.BinReader) {
	br.Read(&a.Usage)
	if a.Usage == DescriptionURL || a.Usage == Vote || a.Usage >= Hash1 && a.Usage <= Hash15 {
		a.Data = make([]byte, 32)
		br.Read(&a.Data)

	} else if a.Usage == Script {
		a.Data = make([]byte, 20)
		br.Read(&a.Data)
	} else if a.Usage == ECDH02 || a.Usage == ECDH03 {
		a.Data = make([]byte, 32)
		br.Read(&a.Data)
	} else if a.Usage == CertURL || a.Usage == DescriptionURL || a.Usage == Description || a.Usage >= Remark {
		lenData := br.VarUint()
		a.Data = make([]byte, lenData)
		br.Read(&a.Data)
	} else {
		br.Read(&a.Data)
	}
}
