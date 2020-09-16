package block

import (
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func trim0x(value interface{}) string {
	s := value.(string)
	return strings.TrimPrefix(s, "0x")
}

// Test blocks are blocks from testnet with their corresponding index.
func TestDecodeBlock1(t *testing.T) {
	data, err := getBlockData(1)
	require.NoError(t, err)

	b, err := hex.DecodeString(data["raw"].(string))
	require.NoError(t, err)

	block := New(netmode.TestNet)
	assert.NoError(t, testserdes.DecodeBinary(b, block))

	assert.Equal(t, uint32(data["index"].(float64)), block.Index)
	assert.Equal(t, uint32(data["version"].(float64)), block.Version)
	assert.Equal(t, trim0x(data["hash"]), block.Hash().StringLE())
	assert.Equal(t, trim0x(data["previousblockhash"]), block.PrevHash.StringLE())
	assert.Equal(t, trim0x(data["merkleroot"]), block.MerkleRoot.StringLE())
	assert.Equal(t, trim0x(data["nextconsensus"]), address.Uint160ToString(block.NextConsensus))

	scripts := data["witnesses"].([]interface{})
	script := scripts[0].(map[string]interface{})
	assert.Equal(t, script["invocation"].(string), base64.StdEncoding.EncodeToString(block.Script.InvocationScript))
	assert.Equal(t, script["verification"].(string), base64.StdEncoding.EncodeToString(block.Script.VerificationScript))

	tx := data["tx"].([]interface{})
	tx0 := tx[0].(map[string]interface{})
	assert.Equal(t, len(tx), len(block.Transactions))
	assert.Equal(t, len(tx0["attributes"].([]interface{})), len(block.Transactions[0].Attributes))
}

func TestTrimmedBlock(t *testing.T) {
	block := getDecodedBlock(t, 1)

	b, err := block.Trim()
	require.NoError(t, err)

	trimmedBlock, err := NewBlockFromTrimmedBytes(netmode.TestNet, b)
	require.NoError(t, err)

	assert.True(t, trimmedBlock.Trimmed)
	assert.Equal(t, block.Version, trimmedBlock.Version)
	assert.Equal(t, block.PrevHash, trimmedBlock.PrevHash)
	assert.Equal(t, block.MerkleRoot, trimmedBlock.MerkleRoot)
	assert.Equal(t, block.Timestamp, trimmedBlock.Timestamp)
	assert.Equal(t, block.Index, trimmedBlock.Index)
	require.NoError(t, trimmedBlock.ConsensusData.createHash())
	assert.Equal(t, block.ConsensusData, trimmedBlock.ConsensusData)
	assert.Equal(t, block.NextConsensus, trimmedBlock.NextConsensus)

	assert.Equal(t, block.Script, trimmedBlock.Script)
	assert.Equal(t, len(block.Transactions), len(trimmedBlock.Transactions))
	for i := 0; i < len(block.Transactions); i++ {
		assert.Equal(t, block.Transactions[i].Hash(), trimmedBlock.Transactions[i].Hash())
		assert.True(t, trimmedBlock.Transactions[i].Trimmed)
	}
}

func newDumbBlock() *Block {
	return &Block{
		Base: Base{
			Version:       0,
			PrevHash:      hash.Sha256([]byte("a")),
			MerkleRoot:    hash.Sha256([]byte("b")),
			Timestamp:     100500,
			Index:         1,
			NextConsensus: hash.Hash160([]byte("a")),
			Script: transaction.Witness{
				VerificationScript: []byte{0x51}, // PUSH1
				InvocationScript:   []byte{0x61}, // NOP
			},
		},
		ConsensusData: ConsensusData{
			PrimaryIndex: 0,
			Nonce:        1111,
		},
		Transactions: []*transaction.Transaction{
			transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0),
		},
	}
}

func TestHashBlockEqualsHashHeader(t *testing.T) {
	block := newDumbBlock()

	assert.Equal(t, block.Hash(), block.Header().Hash())
}

func TestBinBlockDecodeEncode(t *testing.T) {
	// transaction taken from mainnet: 2000000
	rawtx := "0000000005440c786a66aaebf472aacb1d1db19d5b494c6a9226ea91bf5cf0e63a6605138cde5064efb81bc6539620b9e6d6d7c74f97d415b922c4fb4bb1833ce6a97a9d61f962fb7301000065f000005d12ac6c589d59f92e82d8bf60659cb716ffc1f101fd4a010c4011ff5d2138cf546d112ef712ee8a15277f7b6f1d5d2564b97497ac155782e6089cd3005dc9de81a8b22bb2f1c3a2edbac55e01581cb27980fdedf3a8bc57fa470c40657253c374a48da773fc653591f282a63a60695f29ab6c86300020ed505a019e5563e1be493efa71bdde37b16b4ec3f5f6dc2d2a2550151b020176b4dbe7afe40c403efdc559cb6bff135fd79138267db897c6fded01e3a0f15c0fb1c337359935d65e7ac49239f020951a74a96e11e73d225c9789953ffec40d5f7c9a84707b1d9a0c402804f24ab8034fa41223977ba48883eb94951184e31e5739872daf4f65461de3196ebf333f6d7dc4aff0b7b2143793179415f50a715484aba4e33b97dc636e150c40ed6b2ffeaef97eef746815ad16f5b8aed743892e93f7216bb744eb5c2f4cad91ae291919b61cd9a8d50fe85630d5e010c49a01ed687727c3ae5a7e17d4da213afdfd00150c2103009b7540e10f2562e5fd8fac9eaec25166a58b26e412348ff5a86927bfac22a20c21030205e9cefaea5a1dfc580af20c8d5aa2468bb0148f1a5e4605fc622c80e604ba0c210214baf0ceea3a66f17e7e1e839ea25fd8bed6cd82e6bb6e68250189065f44ff010c2103408dcd416396f64783ac587ea1e1593c57d9fea880c8a6a1920e92a2594778060c2102a7834be9b32e2981d157cb5bbd3acb42cfd11ea5c3b10224d7a44e98c5910f1b0c2102ba2c70f5996f357a43198705859fae2cfea13e1172962800772b3d588a9d4abd0c2102f889ecd43c5126ff1932d75fa87dea34fc95325fb724db93c8f79fe32cc3f180170b41138defaf0202c1353ed4e94d0cbc00be80024f7673890000000000261c130000000000e404210001f813c2cc8e18bbe4b3b87f8ef9105b50bb93918e01005d0300743ba40b0000000c14aa07cc3f2193a973904a09a6e60b87f1f96273970c14f813c2cc8e18bbe4b3b87f8ef9105b50bb93918e13c00c087472616e736665720c14bcaf41d684c7d4ad6ee0d99da9707b9d1f0c8e6641627d5b523801420c402360bbf64b9644c25f066dbd406454b07ab9f56e8e25d92d90c96c598f6c29d97eabdcf226f3575481662cfcdd064ee410978e5fae3f09a2f83129ba9cd82641290c2103caf763f91d3691cba5b5df3eb13e668fdace0295b37e2e259fd0fb152d354f900b4195440d78"
	rawtxBytes, _ := hex.DecodeString(rawtx)

	b := New(netmode.TestNet)

	assert.NoError(t, testserdes.DecodeBinary(rawtxBytes, b))
	expected := map[string]bool{ // 1 trans
		"58ea0709dac398c451fd51fdf4466f5257c77927c7909834a0ef3b469cd1a2ce": false,
	}

	var hashes []string

	for _, tx := range b.Transactions {
		hashes = append(hashes, tx.Hash().StringLE())
	}

	assert.Equal(t, len(expected), len(hashes))

	// changes value in map to true, if hash found
	for _, hash := range hashes {
		expected[hash] = true
	}

	// iterate map; all vlaues should be true
	val := true
	for _, v := range expected {
		if v == false {
			val = false
		}
	}
	assert.Equal(t, true, val)

	data, err := testserdes.EncodeBinary(b)
	assert.NoError(t, err)
	assert.Equal(t, rawtx, hex.EncodeToString(data))

	// update hidden hash value.
	_ = b.ConsensusData.Hash()

	testserdes.MarshalUnmarshalJSON(t, b, New(netmode.TestNet))
}

func TestBlockSizeCalculation(t *testing.T) {
	// block taken from testnet at height 61451: b2d9f9fe9860ff71a45c5278e2a84c22bdda84dd8e9313f1e440bea955ea540d
	// The Size in golang is given by counting the number of bytes of an object. (len(Bytes))
	// its implementation is different from the corresponding C# and python implementations. But the result should
	// should be the same.In this test we provide more details then necessary because in case of failure we can easily debug the
	// root cause of the size calculation missmatch.

	rawBlock := "0000000005440c786a66aaebf472aacb1d1db19d5b494c6a9226ea91bf5cf0e63a6605138cde5064efb81bc6539620b9e6d6d7c74f97d415b922c4fb4bb1833ce6a97a9d61f962fb7301000065f000005d12ac6c589d59f92e82d8bf60659cb716ffc1f101fd4a010c4011ff5d2138cf546d112ef712ee8a15277f7b6f1d5d2564b97497ac155782e6089cd3005dc9de81a8b22bb2f1c3a2edbac55e01581cb27980fdedf3a8bc57fa470c40657253c374a48da773fc653591f282a63a60695f29ab6c86300020ed505a019e5563e1be493efa71bdde37b16b4ec3f5f6dc2d2a2550151b020176b4dbe7afe40c403efdc559cb6bff135fd79138267db897c6fded01e3a0f15c0fb1c337359935d65e7ac49239f020951a74a96e11e73d225c9789953ffec40d5f7c9a84707b1d9a0c402804f24ab8034fa41223977ba48883eb94951184e31e5739872daf4f65461de3196ebf333f6d7dc4aff0b7b2143793179415f50a715484aba4e33b97dc636e150c40ed6b2ffeaef97eef746815ad16f5b8aed743892e93f7216bb744eb5c2f4cad91ae291919b61cd9a8d50fe85630d5e010c49a01ed687727c3ae5a7e17d4da213afdfd00150c2103009b7540e10f2562e5fd8fac9eaec25166a58b26e412348ff5a86927bfac22a20c21030205e9cefaea5a1dfc580af20c8d5aa2468bb0148f1a5e4605fc622c80e604ba0c210214baf0ceea3a66f17e7e1e839ea25fd8bed6cd82e6bb6e68250189065f44ff010c2103408dcd416396f64783ac587ea1e1593c57d9fea880c8a6a1920e92a2594778060c2102a7834be9b32e2981d157cb5bbd3acb42cfd11ea5c3b10224d7a44e98c5910f1b0c2102ba2c70f5996f357a43198705859fae2cfea13e1172962800772b3d588a9d4abd0c2102f889ecd43c5126ff1932d75fa87dea34fc95325fb724db93c8f79fe32cc3f180170b41138defaf0202c1353ed4e94d0cbc00be80024f7673890000000000261c130000000000e404210001f813c2cc8e18bbe4b3b87f8ef9105b50bb93918e01005d0300743ba40b0000000c14aa07cc3f2193a973904a09a6e60b87f1f96273970c14f813c2cc8e18bbe4b3b87f8ef9105b50bb93918e13c00c087472616e736665720c14bcaf41d684c7d4ad6ee0d99da9707b9d1f0c8e6641627d5b523801420c402360bbf64b9644c25f066dbd406454b07ab9f56e8e25d92d90c96c598f6c29d97eabdcf226f3575481662cfcdd064ee410978e5fae3f09a2f83129ba9cd82641290c2103caf763f91d3691cba5b5df3eb13e668fdace0295b37e2e259fd0fb152d354f900b4195440d78"
	rawBlockBytes, _ := hex.DecodeString(rawBlock)

	b := New(netmode.TestNet)
	assert.NoError(t, testserdes.DecodeBinary(rawBlockBytes, b))

	expected := []struct {
		ID            string
		Size          int
		Version       int
		SignersLen    int
		AttributesLen int
		WitnessesLen  int
	}{ // 1 trans
		{ID: "58ea0709dac398c451fd51fdf4466f5257c77927c7909834a0ef3b469cd1a2ce", Size: 252, Version: 0, SignersLen: 1, AttributesLen: 0, WitnessesLen: 1},
	}

	for i, tx := range b.Transactions {
		txID := tx.Hash()
		assert.Equal(t, expected[i].ID, txID.StringLE())

		assert.Equal(t, expected[i].Size, io.GetVarSize(tx))
		assert.Equal(t, expected[i].Version, int(tx.Version))
		assert.Equal(t, expected[i].SignersLen, len(tx.Signers))
		assert.Equal(t, expected[i].AttributesLen, len(tx.Attributes))
		assert.Equal(t, expected[i].WitnessesLen, len(tx.Scripts))
	}

	assert.Equal(t, len(expected), len(b.Transactions))

	// Block specific tests
	assert.Equal(t, 0, int(b.Version))
	assert.Equal(t, "1305663ae6f05cbf91ea26926a4c495b9db11d1dcbaa72f4ebaa666a780c4405", b.PrevHash.StringLE())
	assert.Equal(t, "9d7aa9e63c83b14bfbc422b915d4974fc7d7d6e6b9209653c61bb8ef6450de8c", b.MerkleRoot.StringLE())
	assert.Equal(t, 1597650434401, int(b.Timestamp))
	assert.Equal(t, 61541, int(b.Index))

	nextConsensus := address.Uint160ToString(b.NextConsensus)
	assert.Equal(t, "NUQ6Q4BWvHU71HNxPQ4LMSHPMK1jSz1nw4", nextConsensus)

	assert.Equal(t, "DEAR/10hOM9UbREu9xLuihUnf3tvHV0lZLl0l6wVV4LmCJzTAF3J3oGosiuy8cOi7brFXgFYHLJ5gP3t86i8V/pHDEBlclPDdKSNp3P8ZTWR8oKmOmBpXymrbIYwACDtUFoBnlVj4b5JPvpxvd43sWtOw/X23C0qJVAVGwIBdrTb56/kDEA+/cVZy2v/E1/XkTgmfbiXxv3tAeOg8VwPscM3NZk11l56xJI58CCVGnSpbhHnPSJcl4mVP/7EDV98moRwex2aDEAoBPJKuANPpBIjl3ukiIPrlJURhOMeVzmHLa9PZUYd4xluvzM/bX3Er/C3shQ3kxeUFfUKcVSEq6TjO5fcY24VDEDtay/+rvl+73RoFa0W9biu10OJLpP3IWu3ROtcL0ytka4pGRm2HNmo1Q/oVjDV4BDEmgHtaHcnw65afhfU2iE6", base64.StdEncoding.EncodeToString(b.Script.InvocationScript))
	assert.Equal(t, "FQwhAwCbdUDhDyVi5f2PrJ6uwlFmpYsm5BI0j/WoaSe/rCKiDCEDAgXpzvrqWh38WAryDI1aokaLsBSPGl5GBfxiLIDmBLoMIQIUuvDO6jpm8X5+HoOeol/YvtbNgua7bmglAYkGX0T/AQwhA0CNzUFjlvZHg6xYfqHhWTxX2f6ogMimoZIOkqJZR3gGDCECp4NL6bMuKYHRV8tbvTrLQs/RHqXDsQIk16ROmMWRDxsMIQK6LHD1mW81ekMZhwWFn64s/qE+EXKWKAB3Kz1Yip1KvQwhAviJ7NQ8USb/GTLXX6h96jT8lTJftyTbk8j3n+Msw/GAFwtBE43vrw==", base64.StdEncoding.EncodeToString(b.Script.VerificationScript))
	assert.Equal(t, "b2d9f9fe9860ff71a45c5278e2a84c22bdda84dd8e9313f1e440bea955ea540d", b.Hash().StringLE())

	benc, err := testserdes.EncodeBinary(b)
	assert.NoError(t, err)
	// test size of the block
	assert.Equal(t, 952, len(benc))
	assert.Equal(t, rawBlock, hex.EncodeToString(benc))
}

func TestBlockCompare(t *testing.T) {
	b1 := Block{Base: Base{Index: 1}}
	b2 := Block{Base: Base{Index: 2}}
	b3 := Block{Base: Base{Index: 3}}
	assert.Equal(t, 1, b2.Compare(&b1))
	assert.Equal(t, 0, b2.Compare(&b2))
	assert.Equal(t, -1, b2.Compare(&b3))
}
