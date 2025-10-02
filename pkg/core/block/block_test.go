package block

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func trim0x(value any) string {
	s := value.(string)
	return strings.TrimPrefix(s, "0x")
}

// Test blocks are blocks from testnet with their corresponding indices.
func TestDecodeBlock1(t *testing.T) {
	data, err := getBlockData(1)
	require.NoError(t, err)

	b, err := hex.DecodeString(data["raw"].(string))
	require.NoError(t, err)

	block := &Block{}
	assert.NoError(t, testserdes.DecodeBinary(b, block))

	assert.Equal(t, uint32(data["index"].(float64)), block.Index)
	assert.Equal(t, uint32(data["version"].(float64)), block.Version)
	assert.Equal(t, trim0x(data["hash"]), block.Hash().StringLE())
	assert.Equal(t, trim0x(data["previousblockhash"]), block.PrevHash.StringLE())
	assert.Equal(t, trim0x(data["merkleroot"]), block.MerkleRoot.StringLE())
	assert.Equal(t, trim0x(data["nextconsensus"]), address.Uint160ToString(block.NextConsensus))

	scripts := data["witnesses"].([]any)
	script := scripts[0].(map[string]any)
	assert.Equal(t, script["invocation"].(string), base64.StdEncoding.EncodeToString(block.Script.InvocationScript))
	assert.Equal(t, script["verification"].(string), base64.StdEncoding.EncodeToString(block.Script.VerificationScript))

	tx := data["tx"].([]any)
	tx0 := tx[0].(map[string]any)
	assert.Equal(t, len(tx), len(block.Transactions))
	assert.Equal(t, len(tx0["attributes"].([]any)), len(block.Transactions[0].Attributes))
}

func TestTrimmedBlock(t *testing.T) {
	block := getDecodedBlock(t, 1)

	buf := io.NewBufBinWriter()
	block.EncodeTrimmed(buf.BinWriter)
	require.NoError(t, buf.Err)

	r := io.NewBinReaderFromBuf(buf.Bytes())
	trimmedBlock, err := NewTrimmedFromReader(r)
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
	for i := range block.Transactions {
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

	assert.Equal(t, block.Hash(), block.Hash())
}

func TestBinBlockDecodeEncode(t *testing.T) {
	// block with two transfer transactions taken from C# privnet
	rawblock := "AAAAAAwIVa2D6Yha3tArd5XnwkAf7deJBsdyyvpYb2xMZGBbkOUNHAsfre0rKA/F+Ox05/bQSXmcRZnzK3M6Z+/TxJUh0MNFeAEAAAAAAAAAAAAAAQAAAADe7nnBifMAmLC6ai65CzqSWKbH/wHGDEDgwCcXkcaFw5MGOp1cpkgApzDTX2/RxKlmPeXTgWYtfEA8g9svUSbZA4TeoGyWvX8LiN0tJKrzajdMGvTVGqVmDEDp6PBmZmRx9CxswtLht6oWa2Uq4rl5diPsLtqXZeZepMlxUSbaCdlFTB7iWQG9yKXWR5hc0sScevvuVwwsUYdlDEDwlhwZrP07E5fEQKttVMYAiL7edd/eW2yoMGZe6Q95g7yXQ69edVHfQb61fBw3DjCpMDZ5lsxp3BgzXglJwMSKkxMMIQIQOn990BZVhZf3lg0nxRakOU/ZaLnmUVXrSwE+QEBAbgwhAqe8Vf6GhOARl2jRBLoweVvcyGYZ6GSt0mFWcj7Rhc1iDCECs2Ir9AF73+MXxYrtX0x1PyBrfbiWBG+n13S7xL9/jcIMIQPZDAffY+aQzneRLhCrUazJRLZoYCN7YIxPj4MJ5x7mmRRBe85spQIAWNC7C8DYpwAAAAAAIKpEAAAAAADoAwAAAd7uecGJ8wCYsLpqLrkLOpJYpsf/AQBbCwIA4fUFDBSAzse29bVvUFePc38WLTqxTUZlDQwU3u55wYnzAJiwumouuQs6klimx/8UwB8MCHRyYW5zZmVyDBT1Y+pAvCg9TQ4FxI6jBbPyoHNA70FifVtSOQHGDEC4UIzT61GYPx0LdksrF6C2ioYai6fbwpjv3BGAqiyagxiomYGZRLeXZyD67O5FJ86pXRFtSbVYu2YDG+T5ICIgDEDzm/wl+BnHvQXaHQ1rGLtdUMc41wN6I48kPPM7F23gL9sVxGziQIMRLnpTbWHrnzaU9Sy0fXkvIrdJy1KABkSQDEDBwuBuVK+nsZvn1oAscPj6d3FJiUGK9xiHpX9Ipp/5jTnXRBAyzyGc8IZMBVql4WS8kwFe6ojA/9BvFb5eWXnEkxMMIQIQOn990BZVhZf3lg0nxRakOU/ZaLnmUVXrSwE+QEBAbgwhAqe8Vf6GhOARl2jRBLoweVvcyGYZ6GSt0mFWcj7Rhc1iDCECs2Ir9AF73+MXxYrtX0x1PyBrfbiWBG+n13S7xL9/jcIMIQPZDAffY+aQzneRLhCrUazJRLZoYCN7YIxPj4MJ5x7mmRRBe85spQDYJLwZwNinAAAAAAAgqkQAAAAAAOgDAAAB3u55wYnzAJiwumouuQs6klimx/8BAF8LAwBA2d2ITQoADBSAzse29bVvUFePc38WLTqxTUZlDQwU3u55wYnzAJiwumouuQs6klimx/8UwB8MCHRyYW5zZmVyDBTPduKL0AYsSkeO41VhARMZ88+k0kFifVtSOQHGDEDWn0D7z2ELqpN8ghcM/PtfFwo56/BfEasfHuSKECJMYxvU47r2ZtSihg59lGxSZzHsvxTy6nsyvJ22ycNhINdJDECl61cg937N/HujKsLMu2wJMS7C54bzJ3q22Czqllvw3Yp809USgKDs+W+3QD7rI+SFs0OhIn0gooCUU6f/13WjDEDr9XdeT5CGTO8CL0JigzcTcucs0GBcqHs8fToO6zPuuCfS7Wh6dyxSCijT4A4S+7BUdW3dsO7828ke1fj8oNxmkxMMIQIQOn990BZVhZf3lg0nxRakOU/ZaLnmUVXrSwE+QEBAbgwhAqe8Vf6GhOARl2jRBLoweVvcyGYZ6GSt0mFWcj7Rhc1iDCECs2Ir9AF73+MXxYrtX0x1PyBrfbiWBG+n13S7xL9/jcIMIQPZDAffY+aQzneRLhCrUazJRLZoYCN7YIxPj4MJ5x7mmRRBe85spQ=="
	rawblockBytes, _ := base64.StdEncoding.DecodeString(rawblock)

	b := &Block{}

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

	// changes value in map to true, if hash is found
	for _, hash := range hashes {
		expected[hash] = true
	}

	// iterate map; all values should be true
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

	testserdes.MarshalUnmarshalJSON(t, b, &Block{})
}

func TestJSONEmptyTx(t *testing.T) {
	jblock := `{"hash":"0x5f3fbb43d1e516fe07771e2e873ebc9e2810662401acf603a775aace486220bd","version":0,"previousblockhash":"0x1f4d1defa46faa5e7b9b8d3f79a06bec777d7c26c4aa5f6f5899a291daa87c15","merkleroot":"0x0000000000000000000000000000000000000000000000000000000000000000","time":1627894840919,"nonce":"BA8E021F1AEEA3F6","index":1,"nextconsensus":"NVg7LjGcUSrgxgjX3zEgqaksfMaiS8Z6e1","primary":0,"witnesses":[{"invocation":"DEAq7W/jUhpMon1t9muqXKfBvNyGwLfFFM1vAxrMKvUl6MqK+LL/lJAJP9uAk/cberIWWhSsdcxUtltkBLemg/VuDECQZGuvP93JlZga2ml8cnbe5cNiGgO0EMrbGYyzvgr8calP5SwMNPSYms10gIHxlsuXDU++EQpZu/vKxfHoxdC5DEDgsA3POVZdfN+i5+ekvtsaIvif42n0GC+dZi3Rp37ETmt4NtkoK2I2UXi+WIjm5yXLJsPhAvEV6cJSrvqBdsQBDEDTS6NU+kB+tgeBe9lWv+6y0L2qcUBIaUxiTCaNWZtLPghQICBvjDz1/9ttJRXG3I5N9CFDjjLKCpdIY842HW4/DEC+wlWjkCzVqzKslvpCKZbEPUGIf87CFAD88xqzl26m/TpTUcT0+D5oI2bVzAk0mcdBTPnyjcNbv17BFmr63+09","verification":"FQwhAkhv0VcCxEkKJnAxEqXMHQkj/Wl6M0Br1aHADgATsJpwDCECTHt/tsMQ/M8bozsIJRnYKWTqk4aNZ2Zi1KWa1UjfDn0MIQKq7DhHD2qtAELG6HfP2Ah9Jnaw9Rb93TYoAbm9OTY5ngwhA7IJ/U9TpxcOpERODLCmu2pTwr0BaSaYnPhfmw+6F6cMDCEDuNnVdx2PUTqghpucyNUJhkA7eMbaNokGOMPUalrc4EoMIQLKDidpe5wkj28W4IX9AGHib0TahbWO6DXBEMql7DulVAwhAt9I9g6PPgHEj/QLm38TENeosqGTGIvv4cLj33QOiVCTF0Ge0Nw6"}],"tx":[]}`
	b := &Block{}
	require.NoError(t, json.Unmarshal([]byte(jblock), b))
	s, err := json.Marshal(b)
	require.NoError(t, err)
	require.JSONEq(t, jblock, string(s))
}

func TestBlockSizeCalculation(t *testing.T) {
	// block taken from C# privnet: 02d7c7801742cd404eb178780c840477f1eef4a771ecc8cc9434640fe8f2bb09
	// The Size in golang is given by counting the number of bytes of an object. (len(Bytes))
	// its implementation is different from the corresponding C# and python implementations. But the result should
	// be the same. In this test we provide more details then necessary because in case of failure we can easily debug the
	// root cause of the size calculation missmatch.

	rawBlock := "AAAAAAwIVa2D6Yha3tArd5XnwkAf7deJBsdyyvpYb2xMZGBbkOUNHAsfre0rKA/F+Ox05/bQSXmcRZnzK3M6Z+/TxJUh0MNFeAEAAAAAAAAAAAAAAQAAAADe7nnBifMAmLC6ai65CzqSWKbH/wHGDEDgwCcXkcaFw5MGOp1cpkgApzDTX2/RxKlmPeXTgWYtfEA8g9svUSbZA4TeoGyWvX8LiN0tJKrzajdMGvTVGqVmDEDp6PBmZmRx9CxswtLht6oWa2Uq4rl5diPsLtqXZeZepMlxUSbaCdlFTB7iWQG9yKXWR5hc0sScevvuVwwsUYdlDEDwlhwZrP07E5fEQKttVMYAiL7edd/eW2yoMGZe6Q95g7yXQ69edVHfQb61fBw3DjCpMDZ5lsxp3BgzXglJwMSKkxMMIQIQOn990BZVhZf3lg0nxRakOU/ZaLnmUVXrSwE+QEBAbgwhAqe8Vf6GhOARl2jRBLoweVvcyGYZ6GSt0mFWcj7Rhc1iDCECs2Ir9AF73+MXxYrtX0x1PyBrfbiWBG+n13S7xL9/jcIMIQPZDAffY+aQzneRLhCrUazJRLZoYCN7YIxPj4MJ5x7mmRRBe85spQIAWNC7C8DYpwAAAAAAIKpEAAAAAADoAwAAAd7uecGJ8wCYsLpqLrkLOpJYpsf/AQBbCwIA4fUFDBSAzse29bVvUFePc38WLTqxTUZlDQwU3u55wYnzAJiwumouuQs6klimx/8UwB8MCHRyYW5zZmVyDBT1Y+pAvCg9TQ4FxI6jBbPyoHNA70FifVtSOQHGDEC4UIzT61GYPx0LdksrF6C2ioYai6fbwpjv3BGAqiyagxiomYGZRLeXZyD67O5FJ86pXRFtSbVYu2YDG+T5ICIgDEDzm/wl+BnHvQXaHQ1rGLtdUMc41wN6I48kPPM7F23gL9sVxGziQIMRLnpTbWHrnzaU9Sy0fXkvIrdJy1KABkSQDEDBwuBuVK+nsZvn1oAscPj6d3FJiUGK9xiHpX9Ipp/5jTnXRBAyzyGc8IZMBVql4WS8kwFe6ojA/9BvFb5eWXnEkxMMIQIQOn990BZVhZf3lg0nxRakOU/ZaLnmUVXrSwE+QEBAbgwhAqe8Vf6GhOARl2jRBLoweVvcyGYZ6GSt0mFWcj7Rhc1iDCECs2Ir9AF73+MXxYrtX0x1PyBrfbiWBG+n13S7xL9/jcIMIQPZDAffY+aQzneRLhCrUazJRLZoYCN7YIxPj4MJ5x7mmRRBe85spQDYJLwZwNinAAAAAAAgqkQAAAAAAOgDAAAB3u55wYnzAJiwumouuQs6klimx/8BAF8LAwBA2d2ITQoADBSAzse29bVvUFePc38WLTqxTUZlDQwU3u55wYnzAJiwumouuQs6klimx/8UwB8MCHRyYW5zZmVyDBTPduKL0AYsSkeO41VhARMZ88+k0kFifVtSOQHGDEDWn0D7z2ELqpN8ghcM/PtfFwo56/BfEasfHuSKECJMYxvU47r2ZtSihg59lGxSZzHsvxTy6nsyvJ22ycNhINdJDECl61cg937N/HujKsLMu2wJMS7C54bzJ3q22Czqllvw3Yp809USgKDs+W+3QD7rI+SFs0OhIn0gooCUU6f/13WjDEDr9XdeT5CGTO8CL0JigzcTcucs0GBcqHs8fToO6zPuuCfS7Wh6dyxSCijT4A4S+7BUdW3dsO7828ke1fj8oNxmkxMMIQIQOn990BZVhZf3lg0nxRakOU/ZaLnmUVXrSwE+QEBAbgwhAqe8Vf6GhOARl2jRBLoweVvcyGYZ6GSt0mFWcj7Rhc1iDCECs2Ir9AF73+MXxYrtX0x1PyBrfbiWBG+n13S7xL9/jcIMIQPZDAffY+aQzneRLhCrUazJRLZoYCN7YIxPj4MJ5x7mmRRBe85spQ=="
	rawBlockBytes, _ := base64.StdEncoding.DecodeString(rawBlock)

	b := &Block{}
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
	assert.Equal(t, "0285238e7b5d3ca50ae8f8636dedb4fa486bc0536016e1f1edfd74442af7412e", b.Hash().StringLE())

	benc, err := testserdes.EncodeBinary(b)
	assert.NoError(t, err)
	// test size of the block
	assert.Equal(t, 1438, len(benc))
	assert.Equal(t, rawBlock, base64.StdEncoding.EncodeToString(benc))
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

		require.ErrorIs(t, testserdes.DecodeBinary(data, new(Block)), ErrMaxContentsPerBlock)
	})
}

func TestGetExpectedBlockSize(t *testing.T) {
	check := func(t *testing.T, stateRootEnabled bool) {
		t.Run("without transactions", func(t *testing.T) {
			b := newDumbBlock()
			if stateRootEnabled {
				b.Version = VersionFaun
			}
			b.Transactions = []*transaction.Transaction{}
			require.Equal(t, io.GetVarSize(b), b.GetExpectedBlockSize())
			require.Equal(t, io.GetVarSize(b), b.GetExpectedBlockSizeWithoutTransactions(0))
		})
		t.Run("with one transaction", func(t *testing.T) {
			b := newDumbBlock()
			if stateRootEnabled {
				b.Version = VersionFaun
			}
			expected := io.GetVarSize(b)
			require.Equal(t, expected, b.GetExpectedBlockSize())
			require.Equal(t, expected-b.Transactions[0].Size(), b.GetExpectedBlockSizeWithoutTransactions(len(b.Transactions)))
		})
		t.Run("with multiple transactions", func(t *testing.T) {
			b := newDumbBlock()
			if stateRootEnabled {
				b.Version = VersionFaun
			}
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
