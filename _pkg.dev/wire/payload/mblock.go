package payload

import (
	"io"

	"github.com/CityOfZion/neo-go/pkg/wire/util"

	"github.com/CityOfZion/neo-go/pkg/wire/command"
)

// BlockMessage represents a block message on the neo-network
type BlockMessage struct {
	Block
}

// NewBlockMessage will return a block message object
func NewBlockMessage() (*BlockMessage, error) {
	return &BlockMessage{}, nil
}

// DecodePayload Implements Messager interface
func (b *BlockMessage) DecodePayload(r io.Reader) error {
	br := &util.BinReader{R: r}
	b.Block.DecodePayload(br)
	return br.Err
}

// EncodePayload Implements messager interface
func (b *BlockMessage) EncodePayload(w io.Writer) error {
	bw := &util.BinWriter{W: w}
	b.Block.EncodePayload(bw)
	return bw.Err
}

// Command Implements messager interface
func (b *BlockMessage) Command() command.Type {
	return command.Block
}
