package wire

import (
	"bytes"
	"io"

	"github.com/CityOfZion/neo-go/pkg/core/wire/util"
)

type GetHeadersMessage struct {
	w         *bytes.Buffer
	hashStart []Uint256
	hashStop  Uint256
}

func NewGetHeadersMessage(start []Uint256, stop Uint256) (*GetHeadersMessage, error) {
	getHeaders := &GetHeadersMessage{new(bytes.Buffer), start, stop}
	if err := getHeaders.EncodePayload(getHeaders.w); err != nil {
		return nil, err
	}
	return getHeaders, nil

}

// Implements Messager interface
func (v *GetHeadersMessage) DecodePayload(r io.Reader) error {
	br := util.BinReader{R: r}
	lenStart := br.VarUint()
	v.hashStart = make([]Uint256, lenStart)
	br.Read(&v.hashStart)
	br.Read(&v.hashStop)
	return br.Err
}

// Implements messager interface
func (v *GetHeadersMessage) EncodePayload(w io.Writer) error {
	bw := &util.BinWriter{W: w}
	bw.VarUint(uint64(len(v.hashStart)))
	bw.Write(v.hashStart)
	bw.Write(v.hashStop)
	return bw.Err
}

// Implements messager interface
func (v *GetHeadersMessage) PayloadLength() uint32 {
	return calculatePayloadLength(v.w)
}

// Implements messager interface
func (v *GetHeadersMessage) Checksum() uint32 {
	return calculateCheckSum(v.w)
}

// Implements messager interface
func (v *GetHeadersMessage) Command() CommandType {
	return CMDGetHeaders
}
