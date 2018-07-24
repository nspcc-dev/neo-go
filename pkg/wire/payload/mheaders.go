package payload

import (
	"bytes"
	"errors"
	"io"

	"github.com/CityOfZion/neo-go/pkg/wire/command"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

type HeadersMessage struct {
	w       *bytes.Buffer
	headers []*BlockHeader
}

// Users can at most request 2k header
const (
	maxHeadersAllowed = 2000
)

var (
	ErrMaxHeaders = errors.New("Maximum amount of headers allowed is 2000")
)

func NewHeadersMessage(heads []*BlockHeader) (*HeadersMessage, error) {

	headers := &HeadersMessage{nil, heads}
	if err := headers.EncodePayload(headers.w); err != nil {
		return nil, err
	}
	return headers, nil
}

func (h *HeadersMessage) AddHeader(head *BlockHeader) error {
	if len(h.headers)+1 > maxHeadersAllowed {
		return ErrMaxHeaders
	}
	h.headers = append(h.headers, head)
	return nil
}

// Implements Messager interface
func (v *HeadersMessage) DecodePayload(r io.Reader) error {

	br := &util.BinReader{R: r}

	lenHeaders := br.VarUint()
	v.headers = make([]*BlockHeader, lenHeaders)

	for i := 0; i < int(lenHeaders); i++ {
		header := &BlockHeader{}
		header.DecodePayload(br)
		v.headers[i] = header
	}

	return br.Err
}

// Implements messager interface
func (v *HeadersMessage) EncodePayload(w io.Writer) error {
	bw := &util.BinWriter{W: w}
	bw.VarUint(uint64(len(v.headers)))
	for _, header := range v.headers {
		header.EncodePayload(bw)
	}
	return bw.Err
}

// Implements messager interface
func (v *HeadersMessage) PayloadLength() uint32 {
	return util.CalculatePayloadLength(v.w)
}

// Implements messager interface
func (v *HeadersMessage) Checksum() uint32 {
	return util.CalculateCheckSum(v.w)
}

// Implements messager interface
func (v *HeadersMessage) Command() command.Type {
	return command.Headers
}
