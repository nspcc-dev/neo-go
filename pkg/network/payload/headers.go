package payload

import (
	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/io"
	log "github.com/sirupsen/logrus"
)

// Headers payload.
type Headers struct {
	Hdrs []*core.Header
}

// Users can at most request 2k header.
const (
	maxHeadersAllowed = 2000
)

// DecodeBinary implements Serializable interface.
func (p *Headers) DecodeBinary(br *io.BinReader) {
	lenHeaders := br.ReadVarUint()

	// C# node does it silently
	if lenHeaders > maxHeadersAllowed {
		log.Warnf("received %d headers, capping to %d", lenHeaders, maxHeadersAllowed)
		lenHeaders = maxHeadersAllowed
	}

	p.Hdrs = make([]*core.Header, lenHeaders)

	for i := 0; i < int(lenHeaders); i++ {
		header := &core.Header{}
		header.DecodeBinary(br)
		p.Hdrs[i] = header
	}
}

// EncodeBinary implements Serializable interface.
func (p *Headers) EncodeBinary(bw *io.BinWriter) {
	bw.WriteArray(p.Hdrs)
}
