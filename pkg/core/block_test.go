package core

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/stretchr/testify/assert"
)

// Test blocks are blocks from mainnet.

func TestDecodeBlock1(t *testing.T) {
	data, err := getBlockData(1)
	if err != nil {
		t.Fatal(err)
	}

	block := &Block{}
	b, err := hex.DecodeString(data["raw"].(string))
	if err != nil {
		t.Fatal(err)
	}

	if err := block.DecodeBinary(bytes.NewReader(b)); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, uint32(data["index"].(float64)), block.Index)
	assert.Equal(t, uint32(data["version"].(float64)), block.Version)
	assert.Equal(t, data["hash"].(string), block.Hash().String())
	assert.Equal(t, data["previousblockhash"].(string), block.PrevHash.String())
	assert.Equal(t, data["merkleroot"].(string), block.MerkleRoot.String())
	assert.Equal(t, data["nextconsensus"].(string), block.NextConsensus.Address())

	script := data["script"].(map[string]interface{})
	assert.Equal(t, script["invocation"].(string), hex.EncodeToString(block.Script.InvocationScript))
	assert.Equal(t, script["verification"].(string), hex.EncodeToString(block.Script.VerificationScript))

	tx := data["tx"].([]interface{})
	minerTX := tx[0].(map[string]interface{})
	assert.Equal(t, len(tx), len(block.Transactions))
	assert.Equal(t, minerTX["type"].(string), block.Transactions[0].Type.String())
}

func TestHashBlockEqualsHashHeader(t *testing.T) {
	block := newBlock(0)
	assert.Equal(t, block.Hash(), block.Header().Hash())
}

func TestBlockVerify(t *testing.T) {
	block := newBlock(
		0,
		newTX(transaction.MinerType),
		newTX(transaction.IssueType),
	)
	assert.True(t, block.Verify(false))

	block.Transactions = []*transaction.Transaction{
		{Type: transaction.IssueType},
		{Type: transaction.MinerType},
	}
	assert.False(t, block.Verify(false))

	block.Transactions = []*transaction.Transaction{
		{Type: transaction.MinerType},
		{Type: transaction.MinerType},
	}
	assert.False(t, block.Verify(false))
}
