package transaction

import "github.com/CityOfZion/neo-go/pkg/wire/util"

type TXVersion uint8

const (
	DefaultTxVersion TXVersion = 0
)

func (v *TXVersion) EncodePayload(bw *util.BinWriter) {
	bw.Write(v)
}

func (v *TXVersion) DecodePayload(br *util.BinReader) {
	br.Read(&v)
}
