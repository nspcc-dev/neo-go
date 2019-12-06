package transaction

import (
	"encoding/hex"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/stretchr/testify/assert"
)

func TestEncodeDecodeContract(t *testing.T) {

	// mainnet transaction: bdf6cc3b9af12a7565bda80933a75ee8cef1bc771d0d58effc08e4c8b436da79
	rawtx := "80000001888da99f8f497fd65c4325786a09511159c279af4e7eb532e9edd628c87cc1ee0000019b7cffdaa674beae0f930ebe6085af9093e5fe56b34a5c220ccdcf6efc336fc50082167010000000a8666b4830229d6a1a9b80f6088059191c122d2b0141409e79e132290c82916a88f1a3db5cf9f3248b780cfece938ab0f0812d0e188f3a489c7d1a23def86bd69d863ae67de753b2c2392e9497eadc8eb9fc43aa52c645232103e2f6a334e05002624cf616f01a62cff2844c34a3b08ca16048c259097e315078ac"
	tx := decodeTransaction(rawtx, t)

	assert.Equal(t, ContractType, tx.Type)
	assert.IsType(t, tx.Data, &ContractTX{})
	assert.Equal(t, 0, int(tx.Version))
	assert.Equal(t, 1, len(tx.Inputs))

	input := tx.Inputs[0]

	assert.Equal(t, "eec17cc828d6ede932b57e4eaf79c2591151096a7825435cd67f498f9fa98d88", input.PrevHash.StringLE())
	assert.Equal(t, 0, int(input.PrevIndex))
	assert.Equal(t, int64(706), tx.Outputs[0].Amount.Int64Value())
	assert.Equal(t, "c56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b", tx.Outputs[0].AssetID.StringLE())
	assert.Equal(t, "a8666b4830229d6a1a9b80f6088059191c122d2b", tx.Outputs[0].ScriptHash.String())
	assert.Equal(t, "bdf6cc3b9af12a7565bda80933a75ee8cef1bc771d0d58effc08e4c8b436da79", tx.Hash().StringLE())

	// Encode
	buf := io.NewBufBinWriter()

	tx.EncodeBinary(buf.BinWriter)
	assert.Equal(t, nil, buf.Err)
	assert.Equal(t, rawtx, hex.EncodeToString(buf.Bytes()))
}
