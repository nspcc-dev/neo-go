package transaction

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction/types"

	"github.com/stretchr/testify/assert"
)

func TestEncodeDecodeContract(t *testing.T) {

	// mainnet transaction: bdf6cc3b9af12a7565bda80933a75ee8cef1bc771d0d58effc08e4c8b436da79
	rawtx := "80000001888da99f8f497fd65c4325786a09511159c279af4e7eb532e9edd628c87cc1ee0000019b7cffdaa674beae0f930ebe6085af9093e5fe56b34a5c220ccdcf6efc336fc50082167010000000a8666b4830229d6a1a9b80f6088059191c122d2b0141409e79e132290c82916a88f1a3db5cf9f3248b780cfece938ab0f0812d0e188f3a489c7d1a23def86bd69d863ae67de753b2c2392e9497eadc8eb9fc43aa52c645232103e2f6a334e05002624cf616f01a62cff2844c34a3b08ca16048c259097e315078ac"
	rawtxBytes, _ := hex.DecodeString(rawtx)
	c := NewContract(30)

	r := bytes.NewReader(rawtxBytes)
	err := c.Decode(r)
	assert.Equal(t, nil, err)

	assert.Equal(t, types.Contract, c.Type)
	assert.Equal(t, 0, int(c.Version))
	assert.Equal(t, 1, int(len(c.Inputs)))

	input := c.Inputs[0]

	assert.Equal(t, "eec17cc828d6ede932b57e4eaf79c2591151096a7825435cd67f498f9fa98d88", input.PrevHash.String())
	assert.Equal(t, 0, int(input.PrevIndex))
	assert.Equal(t, int64(70600000000), c.Outputs[0].Amount)
	assert.Equal(t, "c56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b", c.Outputs[0].AssetID.String())
	assert.Equal(t, "2b2d121c19598008f6809b1a6a9d2230486b66a8", c.Outputs[0].ScriptHash.ReverseString())
	assert.Equal(t, "bdf6cc3b9af12a7565bda80933a75ee8cef1bc771d0d58effc08e4c8b436da79", c.Hash.String())

	// Encode
	buf := new(bytes.Buffer)

	err = c.Encode(buf)
	assert.Equal(t, nil, err)

	assert.Equal(t, rawtxBytes, buf.Bytes())

}

func TestEncodeDecodeContract2(t *testing.T) {
	// https://github.com/CityOfZion/neo-python/blob/master/neo/Core/TX/test_transactions.py#L122

	rawtx := "800001f012e99481e4bb93e59088e7baa6e6b58be8af9502f8e0bc69b6af579e69a56d3d3d559759cdb848cb55b54531afc6e3322c85badf08002c82c09c5b49d10cd776c8679789ba98d0b0236f0db4dc67695a1eb920a646b9000001cd5e195b9235a31b7423af5e6937a660f7e7e62524710110b847bab41721090c0061c2540cd1220067f97110a66136d38badc7b9f88eab013027ce490241400bd2e921cee90c8de1a192e61e33eb8980a3dc00c388ee9aac0712178cc8fceed8bb59788f7caf3c4dc082abcdaaa49772fda86db4ceea243bda31bcde9b8a0b3c21034b44ed9c8a88fb2497b6b57206cc08edd42c5614bd1fee790e5b795dee0f4e1104182f145967cc4ee2f1c9f4e0782756dabf246d0a4fe60a035441402fe3e20c303e26c3817fed6fc7db8edde4ac62b16eee796c01c2b59e382b7ddfc82f0b36c7f7520821c7b72b9aff50ae27a016961f1ef1dade9cafa85655380f2321034b44ed9c8a88fb2497b6b57206cc08edd42c5614bd1fee790e5b795dee0f4e11ac"
	rawtxBytes, _ := hex.DecodeString(rawtx)
	c := NewContract(30)

	r := bytes.NewReader(rawtxBytes)
	err := c.Decode(r)
	assert.Equal(t, nil, err)

	assert.Equal(t, types.Contract, c.Type)

	//@todo: add the following assert once we have the Size calculation in place
	//assert.Equal(t, i.Size(), 157)

	assert.Equal(t, 0, int(c.Version))
	assert.Equal(t, 2, len(c.Inputs))

	assert.Equal(t, "dfba852c32e3c6af3145b555cb48b8cd5997553d3d6da5699e57afb669bce0f8", c.Inputs[0].PrevHash.String())
	assert.Equal(t, "b946a620b91e5a6967dcb40d6f23b0d098ba899767c876d70cd1495b9cc0822c", c.Inputs[1].PrevHash.String())

	assert.Equal(t, 8, int(c.Inputs[0].PrevIndex))
	assert.Equal(t, 0, int(c.Inputs[1].PrevIndex))
	assert.Equal(t, int64(9800000100000000), c.Outputs[0].Amount)
	assert.Equal(t, "0c092117b4ba47b81001712425e6e7f760a637695eaf23741ba335925b195ecd", c.Outputs[0].AssetID.String())
	assert.Equal(t, "49ce273001ab8ef8b9c7ad8bd33661a61071f967", c.Outputs[0].ScriptHash.ReverseString())
	assert.Equal(t, "e4d2ea5df2adf77df91049beccbb16f98863b93a16439c60381eac1f23bff178", c.Hash.String())

	// Encode
	buf := new(bytes.Buffer)

	err = c.Encode(buf)
	assert.Equal(t, nil, err)

	assert.Equal(t, rawtxBytes, buf.Bytes())

}
