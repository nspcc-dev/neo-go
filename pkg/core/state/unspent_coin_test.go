package state

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestDecodeEncodeUnspentCoin(t *testing.T) {
	unspent := &UnspentCoin{
		Height: 100500,
		States: []OutputState{
			{
				Output: transaction.Output{
					AssetID:    random.Uint256(),
					Amount:     util.Fixed8(42),
					ScriptHash: random.Uint160(),
				},
				SpendHeight: 201000,
				State:       CoinSpent,
			},
			{
				Output: transaction.Output{
					AssetID:    random.Uint256(),
					Amount:     util.Fixed8(420),
					ScriptHash: random.Uint160(),
				},
				SpendHeight: 0,
				State:       CoinConfirmed,
			},
			{
				Output: transaction.Output{
					AssetID:    random.Uint256(),
					Amount:     util.Fixed8(4200),
					ScriptHash: random.Uint160(),
				},
				SpendHeight: 111000,
				State:       CoinSpent & CoinClaimed,
			},
		},
	}

	buf := io.NewBufBinWriter()
	unspent.EncodeBinary(buf.BinWriter)
	assert.Nil(t, buf.Err)
	unspentDecode := &UnspentCoin{}
	r := io.NewBinReaderFromBuf(buf.Bytes())
	unspentDecode.DecodeBinary(r)
	assert.Nil(t, r.Err)
}
