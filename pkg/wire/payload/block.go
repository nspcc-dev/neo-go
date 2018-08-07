package payload

import (
	"bufio"
	"io"

	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction"
	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction/types"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

type Block struct {
	BlockHeader
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
	b.BlockHeader.EncodePayload(bw)
	bw.VarUint(uint64(len(b.Txs)))
	for _, tx := range b.Txs {
		tx.Encode(bw.W)
	}
}

func (b *Block) DecodePayload(br *util.BinReader) error {

	b.BlockHeader.DecodePayload(br)
	lenTXs := br.VarUint()

	b.Txs = make([]transaction.Transactioner, lenTXs)

	reader := bufio.NewReader(br.R)
	for i := 0; i < int(lenTXs); i++ {

		tx, err := BytesToTransaction(reader)
		if err != nil {
			return err
		}
		b.Txs[i] = tx

	}
	return nil

}

func BytesToTransaction(reader *bufio.Reader) (transaction.Transactioner, error) {

	t, err := reader.Peek(1)

	typ := types.TX(t[0])
	var trans transaction.Transactioner

	switch typ {
	case types.Miner:
		miner := transaction.NewMiner(0)
		err = miner.Decode(reader)
		// fmt.Println(miner.Type, miner.Hash.String())
		trans = miner
	case types.Contract:
		contract := transaction.NewContract(0)
		err = contract.Decode(reader)
		// fmt.Println(contract.Type, contract.Hash.String())
		trans = contract
	case types.Invocation:
		invoc := transaction.NewInvocation(0)
		err = invoc.Decode(reader)
		// fmt.Println(invoc.Type, invoc.Hash.String())
		trans = invoc
	case types.Claim:
		claim := transaction.NewClaim(0)
		err = claim.Decode(reader)
		// fmt.Println(claim.Type, claim.Hash.String())
		trans = claim
	}
	return trans, err
}
