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
	}

	store := storage.NewMemoryStore()
	states := AccountStates{}
	if err := states.processTXOutputs(store, outputs); err != nil {
		t.Fatal(err)
	}

	i := 0
	for k, _ := range states {
		assert.Equal(t, k, outputs[i].ScriptHash)
		i++
	}
}
