package core

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestEncodeDecodeSpentCoinState(t *testing.T) {
	spent := &SpentCoinState{
		txHash:   randomUint256(),
		txHeight: 1001,
		items: map[uint16]uint32{
			1: 3,
			2: 8,
			4: 100,
		},
	}

	buf := io.NewBufBinWriter()
	spent.EncodeBinary(buf.BinWriter)
	assert.Nil(t, buf.Err)
	spentDecode := new(SpentCoinState)
	r := io.NewBinReaderFromBuf(buf.Bytes())
	spentDecode.DecodeBinary(r)
	assert.Nil(t, r.Err)
	assert.Equal(t, spent, spentDecode)
}

func TestCommitSpentCoins(t *testing.T) {
	var (
		store      = storage.NewMemoryStore()
		spentCoins = make(SpentCoins)
	)

	txx := []util.Uint256{
		randomUint256(),
		randomUint256(),
		randomUint256(),
	}

	for i := 0; i < len(txx); i++ {
		spentCoins[txx[i]] = &SpentCoinState{
			txHash:   txx[i],
			txHeight: 1,
		}
	}
	assert.Nil(t, spentCoins.commit(store))
}
