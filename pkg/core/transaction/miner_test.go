package transaction

import (
	"encoding/hex"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/stretchr/testify/assert"
)

func TestEncodeDecodeMiner(t *testing.T) {
	// transaction from mainnet a1f219dc6be4c35eca172e65e02d4591045220221b1543f1a4b67b9e9442c264
	rawtx := "0000fcd30e22000001e72d286979ee6cb1b7e65dfddfb2e384100b8d148e7758de42e4168b71792c60c8000000000000001f72e68b4e39602912106d53b229378a082784b200"
	tx := decodeTransaction(rawtx, t)
	assert.Equal(t, MinerType, tx.Type)
	assert.IsType(t, tx.Data, &MinerTX{})
	assert.Equal(t, 0, int(tx.Version))
	m := tx.Data.(*MinerTX)
	assert.Equal(t, uint32(571397116), m.Nonce)

	assert.Equal(t, "a1f219dc6be4c35eca172e65e02d4591045220221b1543f1a4b67b9e9442c264", tx.Hash().StringLE())

	// Encode
	buf := io.NewBufBinWriter()

	tx.EncodeBinary(buf.BinWriter)
	assert.Equal(t, nil, buf.Err)

	assert.Equal(t, rawtx, hex.EncodeToString(buf.Bytes()))
}
