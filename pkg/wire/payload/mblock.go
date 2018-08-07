package payload

import (
	"bytes"
	"io"

	"github.com/CityOfZion/neo-go/pkg/wire/util"

	"github.com/CityOfZion/neo-go/pkg/wire/command"
)

type BlockMessage struct {
	Block
}

func NewBlockMessage() (*BlockMessage, error) {
	return &BlockMessage{}, nil
}

// Implements Messager interface
func (b *BlockMessage) DecodePayload(r io.Reader) error {
	br := &util.BinReader{R: r}
	b.Block.DecodePayload(br)
	return br.Err
}

// Implements messager interface
func (b *BlockMessage) EncodePayload(w io.Writer) error {
	bw := &util.BinWriter{W: w}
	b.Block.EncodePayload(bw)
	return bw.Err
}

// Implements messager interface
func (v *BlockMessage) PayloadLength() uint32 {
	return 0
}

// Implements messager interface
func (v *BlockMessage) Checksum() uint32 {
	return util.CalculateCheckSum(new(bytes.Buffer))
}

// Implements messager interface
func (v *BlockMessage) Command() command.Type {
	return command.Block
}
