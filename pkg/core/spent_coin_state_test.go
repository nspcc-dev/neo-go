package core

import (
	"bytes"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestEncodeDecodeSpentCoinState(t *testing.T) {
	spent := &SpentCoinState{
		txHash:   util.RandomUint256(),
		txHeight: 1001,
		items: map[uint16]uint32{
			1: 3,
			2: 8,
			4: 100,
		},
	}

	buf := new(bytes.Buffer)
	assert.Nil(t, spent.EncodeBinary(buf))
	spentDecode := new(SpentCoinState)
	assert.Nil(t, spentDecode.DecodeBinary(buf))
	assert.Equal(t, spent, spentDecode)
}

func TestCommitSpentCoins(t *testing.T) {
	var (
		store      = storage.NewMemoryStore()
		batch      = store.Batch()
		spentCoins = make(SpentCoins)
	)

	txx := []util.Uint256{
		util.RandomUint256(),
		util.RandomUint256(),
		util.RandomUint256(),
	}

	for i := 0; i < len(txx); i++ {
		spentCoins[txx[i]] = &SpentCoinState{
			txHash:   txx[i],
			txHeight: 1,
		}
	}
	assert.Nil(t, spentCoins.commit(batch))
	assert.Nil(t, store.PutBatch(batch))
}
