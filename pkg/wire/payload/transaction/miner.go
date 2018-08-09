package transaction

import (
	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction/types"
	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction/version"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

type Miner struct {
	*Base
	Nonce uint32
}

func NewMiner(ver version.TX) *Miner {
	basicTrans := createBaseTransaction(types.Miner, ver)

	Miner := &Miner{}
	Miner.Base = basicTrans
	Miner.encodeExclusive = Miner.encodeExcl
	Miner.decodeExclusive = Miner.decodeExcl
	return Miner
}

func (c *Miner) encodeExcl(bw *util.BinWriter) {

	bw.Write(c.Nonce)
	return
}

func (c *Miner) decodeExcl(br *util.BinReader) {

	br.Read(&c.Nonce)

}
