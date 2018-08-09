package payload

import (
	"bytes"
	"errors"
	"io"

	"github.com/CityOfZion/neo-go/pkg/wire/command"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
	checksum "github.com/CityOfZion/neo-go/pkg/wire/util/Checksum"
)

type HeadersMessage struct {
	w       *bytes.Buffer
	headers []*BlockBase

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

	headers := &HeadersMessage{new(bytes.Buffer), nil, 0}
	if err := headers.EncodePayload(headers.w); err != nil {
		return nil, err
	}
	return headers, nil
}

func (h *HeadersMessage) AddHeader(head *BlockBase) error {
	if len(h.headers)+1 > maxHeadersAllowed {
		return ErrMaxHeaders
	}
	h.headers = append(h.headers, head)
	if err := h.EncodePayload(h.w); err != nil {
		return err
	}
	return nil
}

// Implements Messager interface
func (v *HeadersMessage) DecodePayload(r io.Reader) error {

	buf, err := util.ReaderToBuffer(r)
	if err != nil {
		return err
	}

	v.w = buf

	r = bytes.NewReader(buf.Bytes())

	br := &util.BinReader{R: r}

	lenHeaders := br.VarUint()
	v.headers = make([]*BlockBase, lenHeaders)

	for i := 0; i < int(lenHeaders); i++ {
		header := &BlockBase{}
		header.DecodePayload(br)
		var padding uint8
		br.Read(&padding)
		if padding != 0 {
			return ErrPadding
		}
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
		bw.Write(uint8(0))
	}
	return bw.Err
}

// Implements messager interface
func (v *HeadersMessage) PayloadLength() uint32 {
	return util.CalculatePayloadLength(v.w)
}

// Implements messager interface
func (v *HeadersMessage) Checksum() uint32 {
	return checksum.FromBuf(v.w)
}

// Implements messager interface
func (v *HeadersMessage) Command() command.Type {
	return command.Headers
}
