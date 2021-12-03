package neotest

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
)

func TestSingleSigner(t *testing.T) {
	a, err := wallet.NewAccount()
	require.NoError(t, err)

	s := NewSingleSigner(a)
	require.Equal(t, s.ScriptHash(), s.Account().Contract.ScriptHash())
}
