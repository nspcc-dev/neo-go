package payload

import (
	"errors"
	"io"

	"github.com/CityOfZion/neo-go/pkg/wire/command"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

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
	ErrMaxHeaders = errors.New("Maximum amount of headers allowed is 2000")
)

func NewHeadersMessage() (*HeadersMessage, error) {

	headers := &HeadersMessage{nil, 0}
	return headers, nil
}

func (h *HeadersMessage) AddHeader(head *BlockBase) error {
	if len(h.Headers)+1 > maxHeadersAllowed {
		return ErrMaxHeaders
	}
	h.Headers = append(h.Headers, head)

	return nil
}

// Implements Messager interface
func (v *HeadersMessage) DecodePayload(r io.Reader) error {

	br := &util.BinReader{R: r}

	lenHeaders := br.VarUint()
	v.Headers = make([]*BlockBase, lenHeaders)

	for i := 0; i < int(lenHeaders); i++ {
		header := &BlockBase{}
		header.DecodePayload(br)
		var padding uint8
		br.Read(&padding)
		if padding != 0 {
			return ErrPadding
		}
		v.Headers[i] = header
	}

	return br.Err
}

// Implements messager interface
func (v *HeadersMessage) EncodePayload(w io.Writer) error {
	bw := &util.BinWriter{W: w}
	bw.VarUint(uint64(len(v.Headers)))
	for _, header := range v.Headers {
		header.EncodePayload(bw)
		bw.Write(uint8(0))
	}
	return bw.Err
}

// Implements messager interface
func (v *HeadersMessage) Command() command.Type {
	return command.Headers
}
