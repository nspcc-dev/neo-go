package keys

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSignVerifyWalletConnect(t *testing.T) {
	priv, err := NewPrivateKey()
	require.NoError(t, err)

	msg := []byte("hello, world")
	sigWithSalt, err := priv.SignWalletConnect(msg)
	require.NoError(t, err)
	require.Equal(t, SignatureLen+WalletConnectSaltLen, len(sigWithSalt))

	pub := priv.PublicKey()
	require.True(t, pub.VerifyWalletConnect(msg, sigWithSalt))

	// Wrong message.
	require.False(t, pub.VerifyWalletConnect([]byte("wrong"), sigWithSalt))

	// Wrong key.
	priv2, err := NewPrivateKey()
	require.NoError(t, err)
	require.False(t, priv2.PublicKey().VerifyWalletConnect(msg, sigWithSalt))

	// Truncated signature.
	require.False(t, pub.VerifyWalletConnect(msg, sigWithSalt[:SignatureLen]))

	// Two different signs of same message should produce different salts.
	sigWithSalt2, err := priv.SignWalletConnect(msg)
	require.NoError(t, err)
	require.NotEqual(t, sigWithSalt, sigWithSalt2)

	// But both should verify successfully.
	require.True(t, pub.VerifyWalletConnect(msg, sigWithSalt2))
}

func TestWalletConnectEmptyMessage(t *testing.T) {
	priv, err := NewPrivateKey()
	require.NoError(t, err)

	msg := []byte{}
	sigWithSalt, err := priv.SignWalletConnect(msg)
	require.NoError(t, err)
	require.True(t, priv.PublicKey().VerifyWalletConnect(msg, sigWithSalt))
}
