package wire

import (
	"bytes"
	"io"

	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

type GetBlocksMessage struct {
	w         *bytes.Buffer
	hashStart []Uint256
	hashStop  Uint256
}

func NewGetBlocksMessage(start []Uint256, stop Uint256) (*GetBlocksMessage, error) {
	getBlocks := &GetBlocksMessage{new(bytes.Buffer), start, stop}
	if err := getBlocks.EncodePayload(getBlocks.w); err != nil {
		return nil, err
	}
	return getBlocks, nil

}

// Implements Messager interface
func (v *GetBlocksMessage) DecodePayload(r io.Reader) error {
	br := util.BinReader{R: r}
	lenStart := br.VarUint()
	v.hashStart = make([]Uint256, lenStart)
	br.Read(&v.hashStart)
	br.Read(&v.hashStop)
	return br.Err
}

// Implements messager interface
func (v *GetBlocksMessage) EncodePayload(w io.Writer) error {
	bw := &util.BinWriter{W: w}
	bw.VarUint(uint64(len(v.hashStart)))
	bw.Write(v.hashStart)
	bw.Write(v.hashStop)
	return bw.Err
}

// Implements messager interface
func (v *GetBlocksMessage) PayloadLength() uint32 {
	return calculatePayloadLength(v.w)
}

// Implements messager interface
func (v *GetBlocksMessage) Checksum() uint32 {
	return calculateCheckSum(v.w)
}

// Implements messager interface
func (v *GetBlocksMessage) Command() CommandType {
	return CMDGetBlocks
}
