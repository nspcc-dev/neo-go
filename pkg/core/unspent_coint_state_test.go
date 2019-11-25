package core

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/core/entities"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/stretchr/testify/assert"
)

func TestDecodeEncodeUnspentCoinState(t *testing.T) {
	unspent := &UnspentCoinState{
		states: []entities.CoinState{
			entities.CoinStateConfirmed,
			entities.CoinStateSpent,
			entities.CoinStateSpent,
			entities.CoinStateSpent,
			entities.CoinStateConfirmed,
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
