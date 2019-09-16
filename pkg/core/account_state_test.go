package core

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestDecodeEncodeAccountState(t *testing.T) {
	var (
		n        = 10
		balances = make(map[util.Uint256]util.Fixed8)
		votes    = make([]*keys.PublicKey, n)
	)
	for i := 0; i < n; i++ {
		balances[randomUint256()] = util.Fixed8(int64(randomInt(1, 10000)))
		k, err := keys.NewPrivateKey()
		assert.Nil(t, err)
		votes[i] = k.PublicKey()
	}

	a := &AccountState{
		Version:    0,
		ScriptHash: randomUint160(),
		IsFrozen:   true,
		Votes:      votes,
		Balances:   balances,
	}

	buf := io.NewBufBinWriter()
	if err := a.EncodeBinary(buf.BinWriter); err != nil {
		t.Fatal(err)
	}

	aDecode := &AccountState{}
	if err := aDecode.DecodeBinary(io.NewBinReaderFromBuf(buf.Bytes())); err != nil {
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
