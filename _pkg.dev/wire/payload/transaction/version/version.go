package version

import (
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

// TX represents a tx version
type TX uint8

// List of latest tx version
const (
	Contract   TX = 0
	Invocation TX = 1
)

// Encode encodes the tx version into the binary writer
func (v *TX) Encode(bw *util.BinWriter) {
	bw.Write(v)
}

// Decode decodes the binary reader into a tx type
func (v *TX) Decode(br *util.BinReader) {
	br.Read(v)
}
