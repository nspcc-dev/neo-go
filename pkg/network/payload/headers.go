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
	lenHeaders := util.ReadVarUint(r)

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
	if err := util.WriteVarUint(w, uint64(len(p.Hdrs))); err != nil {
		return err
	}
	for _, header := range p.Hdrs {
		if err := header.EncodeBinary(w); err != nil {
			return err
		}
	}
	return nil
}
