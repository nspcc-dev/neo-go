package payload

import (
	"io"

	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/util"
	log "github.com/sirupsen/logrus"
)

// Headers payload
type Headers struct {
	Hdrs []*core.Header
}

// Users can at most request 2k header
const (
	maxHeadersAllowed = 2000
)

// DecodeBinary implements the Payload interface.
func (p *Headers) DecodeBinary(r io.Reader) error {
	br := util.NewBinReaderFromIO(r)
	lenHeaders := br.ReadVarUint()
	if br.Err != nil {
		return br.Err
	}
	// C# node does it silently
	if lenHeaders > maxHeadersAllowed {
		log.Warnf("received %d headers, capping to %d", lenHeaders, maxHeadersAllowed)
		lenHeaders = maxHeadersAllowed
	}

	p.Hdrs = make([]*core.Header, lenHeaders)

	for i := 0; i < int(lenHeaders); i++ {
		header := &core.Header{}
		if err := header.DecodeBinary(r); err != nil {
			return err
		}
		p.Hdrs[i] = header
	}

	return nil
}

// EncodeBinary implements the Payload interface.
func (p *Headers) EncodeBinary(w io.Writer) error {
	bw := util.NewBinWriterFromIO(w)
	bw.WriteVarUint(uint64(len(p.Hdrs)))
	if bw.Err != nil {
		return bw.Err
	}

	for _, header := range p.Hdrs {
		if err := header.EncodeBinary(w); err != nil {
			return err
		}
	}
	return nil
}
