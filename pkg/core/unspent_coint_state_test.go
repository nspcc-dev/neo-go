package core

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/stretchr/testify/assert"
)

func TestDecodeEncodeUnspentCoinState(t *testing.T) {
	unspent := &UnspentCoinState{
		states: []CoinState{
			CoinStateConfirmed,
			CoinStateSpent,
			CoinStateSpent,
			CoinStateSpent,
			CoinStateConfirmed,
		},
	}

	buf := io.NewBufBinWriter()
	unspent.EncodeBinary(buf.BinWriter)
	assert.Nil(t, buf.Err)
	unspentDecode := &UnspentCoinState{}
	r := io.NewBinReaderFromBuf(buf.Bytes())
	unspentDecode.DecodeBinary(r)
	assert.Nil(t, r.Err)
}

func TestCommitUnspentCoins(t *testing.T) {
	var (
		store        = storage.NewMemoryStore()
		unspentCoins = make(UnspentCoins)
	)

	txA := randomUint256()
	txB := randomUint256()
	txC := randomUint256()

	unspentCoins[txA] = &UnspentCoinState{
		states: []CoinState{CoinStateConfirmed},
	}
	unspentCoins[txB] = &UnspentCoinState{
		states: []CoinState{
			CoinStateConfirmed,
			CoinStateConfirmed,
		},
	}
	unspentCoins[txC] = &UnspentCoinState{
		states: []CoinState{
			CoinStateConfirmed,
			CoinStateConfirmed,
			CoinStateConfirmed,
		},
	}

	assert.Nil(t, unspentCoins.commit(store))
}
