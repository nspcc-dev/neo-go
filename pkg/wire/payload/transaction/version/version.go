package version

import (
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

type TX uint8

const (
	Contract TX = 0
)

func (v *TX) Encode(bw *util.BinWriter) {
	bw.Write(v)
}

func (v *TX) Decode(br *util.BinReader) {
	br.Read(v)
}
