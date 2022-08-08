//go:build go1.18

package transaction

import (
	"encoding/base64"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func FuzzNewTransactionFromBytes(f *testing.F) {
	b, err := base64.StdEncoding.DecodeString(rawInvocationTX)
	require.NoError(f, err)
	f.Add(b)
	tx := New([]byte{0x51}, 1)
	tx.Signers = []Signer{{Account: util.Uint160{1, 2, 3}}}
	tx.Scripts = []Witness{{InvocationScript: []byte{}, VerificationScript: []byte{}}}
	f.Add(tx.Bytes())
	f.Fuzz(func(t *testing.T, b []byte) {
		require.NotPanics(t, func() {
			_, _ = NewTransactionFromBytes(b)
		})
	})
}
