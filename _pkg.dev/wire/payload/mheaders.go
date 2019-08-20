package payload

import (
	"errors"
	"io"

	"github.com/CityOfZion/neo-go/pkg/wire/command"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

// HeadersMessage represents a Header(s) Message on the neo-network
type HeadersMessage struct {
	Headers []*BlockBase

	// Padding that is fixed to 0
	_ uint8
}

// Users can at most request 2k header
const (
	maxHeadersAllowed = 2000
)

var (
	errMaxHeaders = errors.New("Maximum amount of headers allowed is 2000")
)

//NewHeadersMessage returns a HeadersMessage Object
func NewHeadersMessage() (*HeadersMessage, error) {

	headers := &HeadersMessage{nil, 0}
	return headers, nil
}

// AddHeader adds a header into the list of Headers.
// Since a header is just blockbase with padding, we use BlockBase
func (h *HeadersMessage) AddHeader(head *BlockBase) error {
	if len(h.Headers)+1 > maxHeadersAllowed {
		return errMaxHeaders
	}
	h.Headers = append(h.Headers, head)

	return nil
}

// DecodePayload Implements Messager interface
func (h *HeadersMessage) DecodePayload(r io.Reader) error {

	br := &util.BinReader{R: r}

	lenHeaders := br.VarUint()
	h.Headers = make([]*BlockBase, lenHeaders)

	for i := 0; i < int(lenHeaders); i++ {
		header := &BlockBase{}
		header.DecodePayload(br)
		var padding uint8
		br.Read(&padding)
		if padding != 0 {
			return errPadding
		}
		h.Headers[i] = header
	}

	return br.Err
}

// EncodePayload Implements messager interface
func (h *HeadersMessage) EncodePayload(w io.Writer) error {
	bw := &util.BinWriter{W: w}
	bw.VarUint(uint64(len(h.Headers)))
	for _, header := range h.Headers {
		header.EncodePayload(bw)
		bw.Write(uint8(0))
	}
	return bw.Err
}

// Command Implements messager interface
func (h *HeadersMessage) Command() command.Type {
	return command.Headers
}
