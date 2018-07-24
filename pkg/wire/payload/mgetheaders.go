package payload

import (
	"bytes"
	"io"

	"github.com/CityOfZion/neo-go/pkg/wire/command"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

type GetHeadersMessage struct {
	w         *bytes.Buffer
	hashStart []util.Uint256
	hashStop  util.Uint256
}

func NewGetHeadersMessage(start []util.Uint256, stop util.Uint256) (*GetHeadersMessage, error) {
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
	v.hashStart = make([]util.Uint256, lenStart)
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
	return util.CalculatePayloadLength(v.w)
}

// Implements messager interface
func (v *GetHeadersMessage) Checksum() uint32 {
	return util.CalculateCheckSum(v.w)
}

// Implements messager interface
func (v *GetHeadersMessage) Command() command.Type {
	return command.GetHeaders
}
