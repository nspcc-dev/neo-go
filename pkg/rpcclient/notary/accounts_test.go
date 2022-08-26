package notary

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/stretchr/testify/require"
)

func TestFakeAccounts(t *testing.T) {
	k, err := keys.NewPrivateKey()
	require.NoError(t, err)

	fac := FakeSimpleAccount(k.PublicKey())
	require.False(t, fac.CanSign())

	sh := k.PublicKey().GetScriptHash()
	tx := transaction.New([]byte{1, 2, 3}, 1)
	tx.Signers = append(tx.Signers, transaction.Signer{Account: sh})
	require.NoError(t, fac.SignTx(0, tx))

	fac = FakeContractAccount(sh)
	require.False(t, fac.CanSign())
	require.NoError(t, fac.SignTx(0, tx))

	_, err = FakeMultisigAccount(0, keys.PublicKeys{k.PublicKey()})
	require.Error(t, err)

	fac, err = FakeMultisigAccount(1, keys.PublicKeys{k.PublicKey()})
	require.NoError(t, err)
	require.False(t, fac.CanSign())
	tx.Signers[0].Account = hash.Hash160(fac.Contract.Script)
	require.NoError(t, fac.SignTx(0, tx))
}
