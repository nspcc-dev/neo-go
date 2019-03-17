package payload

import (
	"bufio"
	"bytes"
	"io"

	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

// Block representa a Block in the neo-network
type Block struct {
	BlockBase
	Txs []transaction.Transactioner
}

// Decode decodes an io.Reader into a Block
func (b *Block) Decode(r io.Reader) error {
	br := &util.BinReader{R: r}
	b.DecodePayload(br)
	return br.Err
}

// Encode writes a block into a io.Writer
func (b *Block) Encode(w io.Writer) error {
	bw := &util.BinWriter{W: w}
	b.EncodePayload(bw)
	return bw.Err
}

//EncodePayload implements Messager interface
func (b *Block) EncodePayload(bw *util.BinWriter) {
	b.BlockBase.EncodePayload(bw)
	bw.VarUint(uint64(len(b.Txs)))
	for _, tx := range b.Txs {
		tx.Encode(bw.W)
	}
}

// DecodePayload implements Messager interface
func (b *Block) DecodePayload(br *util.BinReader) error {

	b.BlockBase.DecodePayload(br)
	lenTXs := br.VarUint()

	b.Txs = make([]transaction.Transactioner, lenTXs)

	reader := bufio.NewReader(br.R)
	for i := 0; i < int(lenTXs); i++ {

		tx, err := transaction.FromReader(reader)
		if err != nil {
			return err
		}
		b.Txs[i] = tx

	}
	return nil
}

// Bytes returns the Byte representation of Block
func (b *Block) Bytes() ([]byte, error) {
	buf := new(bytes.Buffer)
	err := b.Encode(buf)
	return buf.Bytes(), err
}
