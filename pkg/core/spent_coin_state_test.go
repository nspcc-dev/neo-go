package core

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/core/testutil"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/stretchr/testify/assert"
)

func TestEncodeDecodeSpentCoinState(t *testing.T) {
	spent := &SpentCoinState{
		txHash:   testutil.RandomUint256(),
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
