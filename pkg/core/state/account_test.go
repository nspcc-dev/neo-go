package state

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/core/testutil"
	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestDecodeEncodeAccountState(t *testing.T) {
	var (
		n        = 10
		balances = make(map[util.Uint256][]UnspentBalance)
		votes    = make([]*keys.PublicKey, n)
	)
	for i := 0; i < n; i++ {
		asset := testutil.RandomUint256()
		for j := 0; j < i+1; j++ {
			balances[asset] = append(balances[asset], UnspentBalance{
				Tx:    testutil.RandomUint256(),
				Index: uint16(testutil.RandomInt(0, 65535)),
				Value: util.Fixed8(int64(testutil.RandomInt(1, 10000))),
			})
		}
		k, err := keys.NewPrivateKey()
		assert.Nil(t, err)
		votes[i] = k.PublicKey()
	}

	a := &Account{
		Version:    0,
		ScriptHash: testutil.RandomUint160(),
		IsFrozen:   true,
		Votes:      votes,
		Balances:   balances,
	}

	buf := io.NewBufBinWriter()
	a.EncodeBinary(buf.BinWriter)
	assert.Nil(t, buf.Err)

	aDecode := &Account{}
	r := io.NewBinReaderFromBuf(buf.Bytes())
	aDecode.DecodeBinary(r)
	assert.Nil(t, r.Err)

	assert.Equal(t, a.Version, aDecode.Version)
	assert.Equal(t, a.ScriptHash, aDecode.ScriptHash)
	assert.Equal(t, a.IsFrozen, aDecode.IsFrozen)

	for i, vote := range a.Votes {
		assert.Equal(t, vote.X, aDecode.Votes[i].X)
	}
	assert.Equal(t, a.Balances, aDecode.Balances)
}

func TestAccountStateBalanceValues(t *testing.T) {
	asset1 := testutil.RandomUint256()
	asset2 := testutil.RandomUint256()
	as := Account{Balances: make(map[util.Uint256][]UnspentBalance)}
	ref := 0
	for i := 0; i < 10; i++ {
		ref += i
		as.Balances[asset1] = append(as.Balances[asset1], UnspentBalance{Value: util.Fixed8(i)})
		as.Balances[asset2] = append(as.Balances[asset2], UnspentBalance{Value: util.Fixed8(i * 10)})
	}
	bVals := as.GetBalanceValues()
	assert.Equal(t, util.Fixed8(ref), bVals[asset1])
	assert.Equal(t, util.Fixed8(ref*10), bVals[asset2])
}
