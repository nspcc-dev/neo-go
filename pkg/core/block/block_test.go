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
	"github.com/nspcc-dev/neo-go/pkg/util"
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
			transaction.New([]byte{byte(opcode.PUSH1)}, 0),
		},
	}
}

func TestHashBlockEqualsHashHeader(t *testing.T) {
	block := newDumbBlock()

	assert.Equal(t, block.Hash(), block.Header.Hash())
}

func TestBinBlockDecodeEncode(t *testing.T) {
	// block with two transfer transactions taken from C# privnet
	rawblock := "AAAAAAwIVa2D6Yha3tArd5XnwkAf7deJBsdyyvpYb2xMZGBbkOUNHAsfre0rKA/F+Ox05/bQSXmcRZnzK3M6Z+/TxJUh0MNFeAEAAAEAAAAA3u55wYnzAJiwumouuQs6klimx/8BxgxA4MAnF5HGhcOTBjqdXKZIAKcw019v0cSpZj3l04FmLXxAPIPbL1Em2QOE3qBslr1/C4jdLSSq82o3TBr01RqlZgxA6ejwZmZkcfQsbMLS4beqFmtlKuK5eXYj7C7al2XmXqTJcVEm2gnZRUwe4lkBvcil1keYXNLEnHr77lcMLFGHZQxA8JYcGaz9OxOXxECrbVTGAIi+3nXf3ltsqDBmXukPeYO8l0OvXnVR30G+tXwcNw4wqTA2eZbMadwYM14JScDEipMTDCECEDp/fdAWVYWX95YNJ8UWpDlP2Wi55lFV60sBPkBAQG4MIQKnvFX+hoTgEZdo0QS6MHlb3MhmGehkrdJhVnI+0YXNYgwhArNiK/QBe9/jF8WK7V9MdT8ga324lgRvp9d0u8S/f43CDCED2QwH32PmkM53kS4Qq1GsyUS2aGAje2CMT4+DCece5pkUQXvObKUCAFjQuwvA2KcAAAAAACCqRAAAAAAA6AMAAAHe7nnBifMAmLC6ai65CzqSWKbH/wEAWwsCAOH1BQwUgM7HtvW1b1BXj3N/Fi06sU1GZQ0MFN7uecGJ8wCYsLpqLrkLOpJYpsf/FMAfDAh0cmFuc2ZlcgwU9WPqQLwoPU0OBcSOowWz8qBzQO9BYn1bUjkBxgxAuFCM0+tRmD8dC3ZLKxegtoqGGoun28KY79wRgKosmoMYqJmBmUS3l2cg+uzuRSfOqV0RbUm1WLtmAxvk+SAiIAxA85v8JfgZx70F2h0Naxi7XVDHONcDeiOPJDzzOxdt4C/bFcRs4kCDES56U21h6582lPUstH15LyK3SctSgAZEkAxAwcLgblSvp7Gb59aALHD4+ndxSYlBivcYh6V/SKaf+Y0510QQMs8hnPCGTAVapeFkvJMBXuqIwP/QbxW+Xll5xJMTDCECEDp/fdAWVYWX95YNJ8UWpDlP2Wi55lFV60sBPkBAQG4MIQKnvFX+hoTgEZdo0QS6MHlb3MhmGehkrdJhVnI+0YXNYgwhArNiK/QBe9/jF8WK7V9MdT8ga324lgRvp9d0u8S/f43CDCED2QwH32PmkM53kS4Qq1GsyUS2aGAje2CMT4+DCece5pkUQXvObKUA2CS8GcDYpwAAAAAAIKpEAAAAAADoAwAAAd7uecGJ8wCYsLpqLrkLOpJYpsf/AQBfCwMAQNndiE0KAAwUgM7HtvW1b1BXj3N/Fi06sU1GZQ0MFN7uecGJ8wCYsLpqLrkLOpJYpsf/FMAfDAh0cmFuc2ZlcgwUz3bii9AGLEpHjuNVYQETGfPPpNJBYn1bUjkBxgxA1p9A+89hC6qTfIIXDPz7XxcKOevwXxGrHx7kihAiTGMb1OO69mbUooYOfZRsUmcx7L8U8up7MrydtsnDYSDXSQxApetXIPd+zfx7oyrCzLtsCTEuwueG8yd6ttgs6pZb8N2KfNPVEoCg7Plvt0A+6yPkhbNDoSJ9IKKAlFOn/9d1owxA6/V3Xk+QhkzvAi9CYoM3E3LnLNBgXKh7PH06Dusz7rgn0u1oencsUgoo0+AOEvuwVHVt3bDu/NvJHtX4/KDcZpMTDCECEDp/fdAWVYWX95YNJ8UWpDlP2Wi55lFV60sBPkBAQG4MIQKnvFX+hoTgEZdo0QS6MHlb3MhmGehkrdJhVnI+0YXNYgwhArNiK/QBe9/jF8WK7V9MdT8ga324lgRvp9d0u8S/f43CDCED2QwH32PmkM53kS4Qq1GsyUS2aGAje2CMT4+DCece5pkUQXvObKU="
	rawblockBytes, _ := base64.StdEncoding.DecodeString(rawblock)

	b := New(netmode.PrivNet, false)

	assert.NoError(t, testserdes.DecodeBinary(rawblockBytes, b))
	expected := map[string]bool{ // 1 trans
		"5a30127b16a628de6aa6823418d76214b97a0fc6db865024d3d2c6e939ce3433": false,
		"25426643feed564cd3e57f346d6c68692f5622b3063da11c5572d99ee1a5b49a": false,
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

	testserdes.MarshalUnmarshalJSON(t, b, New(netmode.PrivNet, false))
}

func TestBlockSizeCalculation(t *testing.T) {
	// block taken from C# privnet: 02d7c7801742cd404eb178780c840477f1eef4a771ecc8cc9434640fe8f2bb09
	// The Size in golang is given by counting the number of bytes of an object. (len(Bytes))
	// its implementation is different from the corresponding C# and python implementations. But the result should
	// should be the same.In this test we provide more details then necessary because in case of failure we can easily debug the
	// root cause of the size calculation missmatch.

	rawBlock := "AAAAAAwIVa2D6Yha3tArd5XnwkAf7deJBsdyyvpYb2xMZGBbkOUNHAsfre0rKA/F+Ox05/bQSXmcRZnzK3M6Z+/TxJUh0MNFeAEAAAEAAAAA3u55wYnzAJiwumouuQs6klimx/8BxgxA4MAnF5HGhcOTBjqdXKZIAKcw019v0cSpZj3l04FmLXxAPIPbL1Em2QOE3qBslr1/C4jdLSSq82o3TBr01RqlZgxA6ejwZmZkcfQsbMLS4beqFmtlKuK5eXYj7C7al2XmXqTJcVEm2gnZRUwe4lkBvcil1keYXNLEnHr77lcMLFGHZQxA8JYcGaz9OxOXxECrbVTGAIi+3nXf3ltsqDBmXukPeYO8l0OvXnVR30G+tXwcNw4wqTA2eZbMadwYM14JScDEipMTDCECEDp/fdAWVYWX95YNJ8UWpDlP2Wi55lFV60sBPkBAQG4MIQKnvFX+hoTgEZdo0QS6MHlb3MhmGehkrdJhVnI+0YXNYgwhArNiK/QBe9/jF8WK7V9MdT8ga324lgRvp9d0u8S/f43CDCED2QwH32PmkM53kS4Qq1GsyUS2aGAje2CMT4+DCece5pkUQXvObKUCAFjQuwvA2KcAAAAAACCqRAAAAAAA6AMAAAHe7nnBifMAmLC6ai65CzqSWKbH/wEAWwsCAOH1BQwUgM7HtvW1b1BXj3N/Fi06sU1GZQ0MFN7uecGJ8wCYsLpqLrkLOpJYpsf/FMAfDAh0cmFuc2ZlcgwU9WPqQLwoPU0OBcSOowWz8qBzQO9BYn1bUjkBxgxAuFCM0+tRmD8dC3ZLKxegtoqGGoun28KY79wRgKosmoMYqJmBmUS3l2cg+uzuRSfOqV0RbUm1WLtmAxvk+SAiIAxA85v8JfgZx70F2h0Naxi7XVDHONcDeiOPJDzzOxdt4C/bFcRs4kCDES56U21h6582lPUstH15LyK3SctSgAZEkAxAwcLgblSvp7Gb59aALHD4+ndxSYlBivcYh6V/SKaf+Y0510QQMs8hnPCGTAVapeFkvJMBXuqIwP/QbxW+Xll5xJMTDCECEDp/fdAWVYWX95YNJ8UWpDlP2Wi55lFV60sBPkBAQG4MIQKnvFX+hoTgEZdo0QS6MHlb3MhmGehkrdJhVnI+0YXNYgwhArNiK/QBe9/jF8WK7V9MdT8ga324lgRvp9d0u8S/f43CDCED2QwH32PmkM53kS4Qq1GsyUS2aGAje2CMT4+DCece5pkUQXvObKUA2CS8GcDYpwAAAAAAIKpEAAAAAADoAwAAAd7uecGJ8wCYsLpqLrkLOpJYpsf/AQBfCwMAQNndiE0KAAwUgM7HtvW1b1BXj3N/Fi06sU1GZQ0MFN7uecGJ8wCYsLpqLrkLOpJYpsf/FMAfDAh0cmFuc2ZlcgwUz3bii9AGLEpHjuNVYQETGfPPpNJBYn1bUjkBxgxA1p9A+89hC6qTfIIXDPz7XxcKOevwXxGrHx7kihAiTGMb1OO69mbUooYOfZRsUmcx7L8U8up7MrydtsnDYSDXSQxApetXIPd+zfx7oyrCzLtsCTEuwueG8yd6ttgs6pZb8N2KfNPVEoCg7Plvt0A+6yPkhbNDoSJ9IKKAlFOn/9d1owxA6/V3Xk+QhkzvAi9CYoM3E3LnLNBgXKh7PH06Dusz7rgn0u1oencsUgoo0+AOEvuwVHVt3bDu/NvJHtX4/KDcZpMTDCECEDp/fdAWVYWX95YNJ8UWpDlP2Wi55lFV60sBPkBAQG4MIQKnvFX+hoTgEZdo0QS6MHlb3MhmGehkrdJhVnI+0YXNYgwhArNiK/QBe9/jF8WK7V9MdT8ga324lgRvp9d0u8S/f43CDCED2QwH32PmkM53kS4Qq1GsyUS2aGAje2CMT4+DCece5pkUQXvObKU="
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
	}{ // 2 transactions
		{ID: "5a30127b16a628de6aa6823418d76214b97a0fc6db865024d3d2c6e939ce3433", Size: 488, Version: 0, SignersLen: 1, AttributesLen: 0, WitnessesLen: 1},
		{ID: "25426643feed564cd3e57f346d6c68692f5622b3063da11c5572d99ee1a5b49a", Size: 492, Version: 0, SignersLen: 1, AttributesLen: 0, WitnessesLen: 1},
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
	assert.Equal(t, "5b60644c6c6f58faca72c70689d7ed1f40c2e795772bd0de5a88e983ad55080c", b.PrevHash.StringLE())
	assert.Equal(t, "95c4d3ef673a732bf399459c7949d0f6e774ecf8c50f282bedad1f0b1c0de590", b.MerkleRoot.StringLE())
	assert.Equal(t, 1616078164001, int(b.Timestamp))
	assert.Equal(t, 1, int(b.Index))

	nextConsensus := address.Uint160ToString(b.NextConsensus)
	assert.Equal(t, "NgEisvCqr2h8wpRxQb7bVPWUZdbVCY8Uo6", nextConsensus)

	assert.Equal(t, "DEDgwCcXkcaFw5MGOp1cpkgApzDTX2/RxKlmPeXTgWYtfEA8g9svUSbZA4TeoGyWvX8LiN0tJKrzajdMGvTVGqVmDEDp6PBmZmRx9CxswtLht6oWa2Uq4rl5diPsLtqXZeZepMlxUSbaCdlFTB7iWQG9yKXWR5hc0sScevvuVwwsUYdlDEDwlhwZrP07E5fEQKttVMYAiL7edd/eW2yoMGZe6Q95g7yXQ69edVHfQb61fBw3DjCpMDZ5lsxp3BgzXglJwMSK", base64.StdEncoding.EncodeToString(b.Script.InvocationScript))
	assert.Equal(t, "EwwhAhA6f33QFlWFl/eWDSfFFqQ5T9loueZRVetLAT5AQEBuDCECp7xV/oaE4BGXaNEEujB5W9zIZhnoZK3SYVZyPtGFzWIMIQKzYiv0AXvf4xfFiu1fTHU/IGt9uJYEb6fXdLvEv3+NwgwhA9kMB99j5pDOd5EuEKtRrMlEtmhgI3tgjE+PgwnnHuaZFEF7zmyl", base64.StdEncoding.EncodeToString(b.Script.VerificationScript))
	assert.Equal(t, "02d7c7801742cd404eb178780c840477f1eef4a771ecc8cc9434640fe8f2bb09", b.Hash().StringLE())

	benc, err := testserdes.EncodeBinary(b)
	assert.NoError(t, err)
	// test size of the block
	assert.Equal(t, 1430, len(benc))
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

func TestGetExpectedBlockSize(t *testing.T) {
	check := func(t *testing.T, stateRootEnabled bool) {

		t.Run("without transactions", func(t *testing.T) {
			b := newDumbBlock()
			b.StateRootEnabled = stateRootEnabled
			b.Transactions = []*transaction.Transaction{}
			require.Equal(t, io.GetVarSize(b), b.GetExpectedBlockSize())
			require.Equal(t, io.GetVarSize(b), b.GetExpectedBlockSizeWithoutTransactions(0))
		})
		t.Run("with one transaction", func(t *testing.T) {
			b := newDumbBlock()
			b.StateRootEnabled = stateRootEnabled
			expected := io.GetVarSize(b)
			require.Equal(t, expected, b.GetExpectedBlockSize())
			require.Equal(t, expected-b.Transactions[0].Size(), b.GetExpectedBlockSizeWithoutTransactions(len(b.Transactions)))
		})
		t.Run("with multiple transactions", func(t *testing.T) {
			b := newDumbBlock()
			b.StateRootEnabled = stateRootEnabled
			b.Transactions = make([]*transaction.Transaction, 123)
			for i := range b.Transactions {
				tx := transaction.New([]byte{byte(opcode.RET)}, int64(i))
				tx.Signers = []transaction.Signer{{Account: util.Uint160{1, 2, 3}}}
				tx.Scripts = []transaction.Witness{{}}
				b.Transactions[i] = tx
			}
			expected := io.GetVarSize(b)
			require.Equal(t, expected, b.GetExpectedBlockSize())
			for _, tx := range b.Transactions {
				expected -= tx.Size()
			}
			require.Equal(t, expected, b.GetExpectedBlockSizeWithoutTransactions(len(b.Transactions)))
		})
	}
	t.Run("StateRoot enabled", func(t *testing.T) {
		check(t, true)
	})
	t.Run("StateRoot disabled", func(t *testing.T) {
		check(t, false)
	})
}
