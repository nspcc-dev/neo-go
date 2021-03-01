package block

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
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

	block := New(netmode.TestNet, false)
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

	trimmedBlock, err := NewBlockFromTrimmedBytes(netmode.TestNet, false, b)
	require.NoError(t, err)

	assert.True(t, trimmedBlock.Trimmed)
	assert.Equal(t, block.Version, trimmedBlock.Version)
	assert.Equal(t, block.PrevHash, trimmedBlock.PrevHash)
	assert.Equal(t, block.MerkleRoot, trimmedBlock.MerkleRoot)
	assert.Equal(t, block.Timestamp, trimmedBlock.Timestamp)
	assert.Equal(t, block.Index, trimmedBlock.Index)
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
		Header: Header{
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
		Transactions: []*transaction.Transaction{
			transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0),
		},
	}
}

func TestHashBlockEqualsHashHeader(t *testing.T) {
	block := newDumbBlock()

	assert.Equal(t, block.Hash(), block.Header.Hash())
}

func TestBinBlockDecodeEncode(t *testing.T) {
	// block taken from testnet: 256
	rawblock := "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"
	rawblockBytes, _ := base64.StdEncoding.DecodeString(rawblock)

	b := New(netmode.TestNet, false)

	assert.NoError(t, testserdes.DecodeBinary(rawblockBytes, b))
	expected := map[string]bool{ // 1 trans
		"affad44bb6acacabc058db0bf1e12ab1239ae5e04007b4d4a2ea0cda868e284a": false,
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
	assert.Equal(t, rawblock, base64.StdEncoding.EncodeToString(data))

	testserdes.MarshalUnmarshalJSON(t, b, New(netmode.TestNet, false))
}

func TestBlockSizeCalculation(t *testing.T) {
	// block taken from testnet at height 256: 51ee44e12cdc1d3041a50d352063127fa65d86670686f14cc08f01b3cee7de17
	// The Size in golang is given by counting the number of bytes of an object. (len(Bytes))
	// its implementation is different from the corresponding C# and python implementations. But the result should
	// should be the same.In this test we provide more details then necessary because in case of failure we can easily debug the
	// root cause of the size calculation missmatch.

	rawBlock := "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"
	rawBlockBytes, _ := base64.StdEncoding.DecodeString(rawBlock)

	b := New(netmode.TestNet, false)
	assert.NoError(t, testserdes.DecodeBinary(rawBlockBytes, b))

	expected := []struct {
		ID            string
		Size          int
		Version       int
		SignersLen    int
		AttributesLen int
		WitnessesLen  int
	}{ // 1 trans
		{ID: "affad44bb6acacabc058db0bf1e12ab1239ae5e04007b4d4a2ea0cda868e284a", Size: 864, Version: 0, SignersLen: 2, AttributesLen: 0, WitnessesLen: 2},
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
	assert.Equal(t, "fd2b68ac95461aa8ecd9ce7817bc0dea12985fc5ca52cfb5c39426e4071c1952", b.PrevHash.StringLE())
	assert.Equal(t, "0a4ce89ab8b057ff5f3b421f5c85d3a90db9e613c020b9916096fb5282f719a9", b.MerkleRoot.StringLE())
	assert.Equal(t, 1612366104874, int(b.Timestamp))
	assert.Equal(t, 256, int(b.Index))

	nextConsensus := address.Uint160ToString(b.NextConsensus)
	assert.Equal(t, "NgPkjjLTNcQad99iRYeXRUuowE4gxLAnDL", nextConsensus)

	assert.Equal(t, "DEAZyIVwKWzpgvm8bSzWuVF1awS0QBguORR682YoVfkJ05KGG7pz9OsLBZ9/F1BHO7TCIcEcc8Sr9AcqbonrSRoCDEDhhZnqTUi8tXYbrTBrVj8i9Yxl8NraAMID4DJryWdD6u42OzAHr0of27qc5nojujhLffBfljuBSyQpyeQcUOpyDEAZr7sLz6n4TgNApkwATGXiP/DZ8AummAGDtznrW6Y8XWHngB7F0j3NKvUQ7lyRt11rT+zNjz5R5vIbBrs8C2ktDEDgNW0xGP/OB8CRHXX/l/+0tPO3drkyCddpG9PKCgyd4oEhQu8cwbSv4Q0V1Y0dZcJBoxUB5Qus/U+Xli7jb6OJDEADaXZxdEb56jlYhF4oM8I8KnthEzGw1Q7pfeI97fehTm78ZOJXWIxESThZzI3D2ndVz0Rcdr5MeA/espG6J2ao", base64.StdEncoding.EncodeToString(b.Script.InvocationScript))
	assert.Equal(t, "FQwhAwCbdUDhDyVi5f2PrJ6uwlFmpYsm5BI0j/WoaSe/rCKiDCEDAgXpzvrqWh38WAryDI1aokaLsBSPGl5GBfxiLIDmBLoMIQIUuvDO6jpm8X5+HoOeol/YvtbNgua7bmglAYkGX0T/AQwhAj6bMuqJuU0GbmSbEk/VDjlu6RNp6OKmrhsRwXDQIiVtDCEDQI3NQWOW9keDrFh+oeFZPFfZ/qiAyKahkg6SollHeAYMIQKng0vpsy4pgdFXy1u9OstCz9EepcOxAiTXpE6YxZEPGwwhAroscPWZbzV6QxmHBYWfriz+oT4RcpYoAHcrPViKnUq9FwtBE43vrw==", base64.StdEncoding.EncodeToString(b.Script.VerificationScript))
	assert.Equal(t, "63d18734a3edbe92a9480b690734b85aaf9c24361d034afbea37d749cfc62d6a", b.Hash().StringLE())

	benc, err := testserdes.EncodeBinary(b)
	assert.NoError(t, err)
	// test size of the block
	assert.Equal(t, 1556, len(benc))
	assert.Equal(t, rawBlock, base64.StdEncoding.EncodeToString(benc))
}

func TestBlockCompare(t *testing.T) {
	b1 := Block{Header: Header{Index: 1}}
	b2 := Block{Header: Header{Index: 2}}
	b3 := Block{Header: Header{Index: 3}}
	assert.Equal(t, 1, b2.Compare(&b1))
	assert.Equal(t, 0, b2.Compare(&b2))
	assert.Equal(t, -1, b2.Compare(&b3))
}

func TestBlockEncodeDecode(t *testing.T) {
	t.Run("positive", func(t *testing.T) {
		b := newDumbBlock()
		b.Transactions = []*transaction.Transaction{}
		_ = b.Hash()
		testserdes.EncodeDecodeBinary(t, b, new(Block))
	})

	t.Run("bad contents count", func(t *testing.T) {
		b := newDumbBlock()
		b.Transactions = make([]*transaction.Transaction, MaxTransactionsPerBlock+1)
		for i := range b.Transactions {
			b.Transactions[i] = &transaction.Transaction{
				Script: []byte("my_pretty_script"),
			}
		}
		_ = b.Hash()
		data, err := testserdes.EncodeBinary(b)
		require.NoError(t, err)

		require.True(t, errors.Is(testserdes.DecodeBinary(data, new(Block)), ErrMaxContentsPerBlock))
	})
}
