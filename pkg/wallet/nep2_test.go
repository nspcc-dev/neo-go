package wallet

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEncryptDecrypt(t *testing.T) {
	priv, err := NewPrivateKey()
	require.NoError(t, err)
	expectedWIF, err := priv.WIF()
	require.NoError(t, err)

	passphrase := "123456"
	enc, err := NEP2Encrypt(priv, passphrase)
	require.NoError(t, err)

	wif, err := NEP2Decrypt(enc, passphrase)
	require.NoError(t, err)
	require.Equal(t, expectedWIF, wif)
}
