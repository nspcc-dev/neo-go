package core

import (
	"bytes"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/crypto"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestDecodeEncodeAccountState(t *testing.T) {
	var (
		n        = 10
		balances = make(map[util.Uint256]util.Fixed8)
		votes    = make([]*crypto.PublicKey, n)
	)
	for i := 0; i < n; i++ {
		balances[util.RandomUint256()] = util.Fixed8(int64(util.RandomInt(1, 10000)))
		votes[i] = &crypto.PublicKey{crypto.RandomECPoint()}
	}

	a := &AccountState{
		ScriptHash: util.RandomUint160(),
		IsFrozen:   true,
		Votes:      votes,
		Balances:   balances,
	}

	buf := new(bytes.Buffer)
	if err := a.EncodeBinary(buf); err != nil {
		t.Fatal(err)
	}

	aDecode := &AccountState{}
	if err := aDecode.DecodeBinary(buf); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, a.ScriptHash, aDecode.ScriptHash)
	assert.Equal(t, a.IsFrozen, aDecode.IsFrozen)

	for i, vote := range a.Votes {
		assert.Equal(t, vote.X, aDecode.Votes[i].X)
	}
	assert.Equal(t, a.Balances, aDecode.Balances)
}

func TestProcessTXOutputs(t *testing.T) {
	outputs := []*transaction.Output{
		{
			util.RandomUint256(),
			80000,
			util.RandomUint160(),
		},
		{
			util.RandomUint256(),
			40000,
			util.RandomUint160(),
		},
		{
			util.RandomUint256(),
			393,
			util.RandomUint160(),
		},
	}

	store := storage.NewMemoryStore()
	states := AccountStates{}
	if err := states.processTXOutputs(store, outputs); err != nil {
		t.Fatal(err)
	}

	for _, out := range outputs {
		account := states[out.ScriptHash]
		assert.Equal(t, account.ScriptHash, out.ScriptHash)
		assert.Equal(t, false, account.IsFrozen)
		assert.Equal(t, 0, len(account.Votes))
		assert.Equal(t, 1, len(account.Balances))
		assert.Equal(t, out.Amount, account.Balances[out.AssetID])
	}

	batch := store.Batch()
	if err := states.Commit(batch); err != nil {
		t.Fatal(err)
	}
	store.PutBatch(batch)

	key := storage.AppendPrefix(storage.STAccount, outputs[0].ScriptHash.Bytes())
	if _, err := store.Get(key); err != nil {
		t.Fatal(err)
	}
}

func TestProcessTXOutputsWithExistingAccount(t *testing.T) {
	var (
		assetA  = util.RandomUint256()
		assetB  = util.RandomUint256()
		remitee = util.RandomUint160()
	)

	balances := make(map[util.Uint256]util.Fixed8)
	balances[assetA] = util.Fixed8(10)
	balances[assetB] = util.Fixed8(20)

	outputs := []*transaction.Output{
		{
			assetA,
			1,
			remitee,
		},
		{
			assetB,
			2,
			remitee,
		},
	}

	account := &AccountState{
		ScriptHash: remitee,
		Balances:   balances,
	}

	var (
		store  = storage.NewMemoryStore()
		buf    = new(bytes.Buffer)
		states = make(AccountStates)
	)

	assert.Nil(t, account.EncodeBinary(buf))
	key := storage.AppendPrefix(storage.STAccount, account.ScriptHash.Bytes())
	assert.Nil(t, store.Put(key, buf.Bytes()))
	assert.Nil(t, states.processTXOutputs(store, outputs))
	assert.Equal(t, util.Fixed8(11), states[remitee].Balances[assetA])
	assert.Equal(t, util.Fixed8(22), states[remitee].Balances[assetB])
}
