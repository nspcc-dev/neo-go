package state

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestDecodeEncodeAccountState(t *testing.T) {
	var (
		n        = 10
		balances = make(map[util.Uint256][]UnspentBalance)
	)
	for i := 0; i < n; i++ {
		asset := random.Uint256()
		for j := 0; j < i+1; j++ {
			balances[asset] = append(balances[asset], UnspentBalance{
				Tx:    random.Uint256(),
				Index: uint16(random.Int(0, 65535)),
				Value: util.Fixed8(int64(random.Int(1, 10000))),
			})
		}
	}

	a := &Account{
		Version:    0,
		ScriptHash: random.Uint160(),
		IsFrozen:   true,
		Balances:   balances,
	}

	testserdes.EncodeDecodeBinary(t, a, new(Account))
}

func TestAccountStateBalanceValues(t *testing.T) {
	asset1 := random.Uint256()
	asset2 := random.Uint256()
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
