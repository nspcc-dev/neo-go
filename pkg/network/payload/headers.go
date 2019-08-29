package payload

import (
	"io"

	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// Headers payload
type Headers struct {
	Hdrs []*core.Header
}

// DecodeBinary implements the Payload interface.
func (p *Headers) DecodeBinary(r io.Reader) error {
	br := util.BinReader{R: r}
	lenHeaders := br.ReadVarUint()
	if br.Err != nil {
		return br.Err
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
	bw := util.BinWriter{W: w}
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
