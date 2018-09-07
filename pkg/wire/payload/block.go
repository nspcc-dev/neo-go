package payload

import (
	"bufio"
	"bytes"
	"io"

	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

type Block struct {
	BlockBase
	Txs []transaction.Transactioner
}

func (b *Block) Decode(r io.Reader) error {
	br := &util.BinReader{R: r}
	b.DecodePayload(br)
	return br.Err
}
func (b *Block) Encode(w io.Writer) error {
	bw := &util.BinWriter{W: w}
	b.EncodePayload(bw)
	return bw.Err
}
func (b *Block) EncodePayload(bw *util.BinWriter) {
	b.BlockBase.EncodePayload(bw)
	bw.VarUint(uint64(len(b.Txs)))
	for _, tx := range b.Txs {
		tx.Encode(bw.W)
	}
}

func (b *Block) DecodePayload(br *util.BinReader) error {

	b.BlockBase.DecodePayload(br)
	lenTXs := br.VarUint()

	b.Txs = make([]transaction.Transactioner, lenTXs)

	reader := bufio.NewReader(br.R)
	for i := 0; i < int(lenTXs); i++ {

		tx, err := transaction.FromBytes(reader)
		if err != nil {
			return err
		}
		b.Txs[i] = tx

	}
	return nil
}

func (b *Block) Bytes() ([]byte, error) {
	buf := new(bytes.Buffer)
	err := b.Encode(buf)
	return buf.Bytes(), err
}
