package neotest

import (
	"sort"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
)

func TestSingleSigner(t *testing.T) {
	a, err := wallet.NewAccount()
	require.NoError(t, err)

	s := NewSingleSigner(a)
	require.Equal(t, s.ScriptHash(), s.Account().Contract.ScriptHash())
}

func TestMultiSigner(t *testing.T) {
	const size = 4

	pubs := make(keys.PublicKeys, size)
	accs := make([]*wallet.Account, size)
	for i := range accs {
		a, err := wallet.NewAccount()
		require.NoError(t, err)

		accs[i] = a
		pubs[i] = a.PublicKey()
	}

	sort.Sort(pubs)
	m := smartcontract.GetDefaultHonestNodeCount(size)
	for i := range accs {
		require.NoError(t, accs[i].ConvertMultisig(m, pubs))
	}

	s := NewMultiSigner(accs...)
	for i := range pubs {
		for j := range accs {
			if pub := accs[j].PublicKey(); pub.Equal(pubs[i]) {
				require.Equal(t, pub, s.Single(i).Account().PublicKey())
			}
		}
	}
}
