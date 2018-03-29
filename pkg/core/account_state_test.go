package core

import (
	"bytes"
	"testing"

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
		votes[i] = &crypto.PublicKey{
			ECPoint: crypto.RandomECPoint(),
		}
	}

	a := &AccountState{
		Version:    0,
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

	assert.Equal(t, a.Version, aDecode.Version)
	assert.Equal(t, a.ScriptHash, aDecode.ScriptHash)
	assert.Equal(t, a.IsFrozen, aDecode.IsFrozen)

	for i, vote := range a.Votes {
		assert.Equal(t, vote.X, aDecode.Votes[i].X)
	}
	assert.Equal(t, a.Balances, aDecode.Balances)
}
