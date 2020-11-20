package payload

import (
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/io"
)

// Headers payload.
type Headers struct {
	Hdrs    []*block.Header
	Network netmode.Magic
	// StateRootInHeader specifies whether header contains state root.
	StateRootInHeader bool
}

// Users can at most request 2k header.
const (
	MaxHeadersAllowed = 2000
)

// ErrTooManyHeaders is an error returned when too many headers were received.
var ErrTooManyHeaders = fmt.Errorf("too many headers were received (max: %d)", MaxHeadersAllowed)

// DecodeBinary implements Serializable interface.
func (p *Headers) DecodeBinary(br *io.BinReader) {
	lenHeaders := br.ReadVarUint()

	var limitExceeded bool

	// C# node does it silently
	if limitExceeded = lenHeaders > MaxHeadersAllowed; limitExceeded {
		lenHeaders = MaxHeadersAllowed
	}

	p.Hdrs = make([]*block.Header, lenHeaders)

	for i := 0; i < int(lenHeaders); i++ {
		header := &block.Header{}
		header.Network = p.Network
		header.StateRootEnabled = p.StateRootInHeader
		header.DecodeBinary(br)
		p.Hdrs[i] = header
	}

	if br.Err == nil && limitExceeded {
		br.Err = ErrTooManyHeaders
	}
}

// EncodeBinary implements Serializable interface.
func (p *Headers) EncodeBinary(bw *io.BinWriter) {
	bw.WriteArray(p.Hdrs)
}
