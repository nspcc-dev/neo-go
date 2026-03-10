package keys

import (
	"encoding/base64"
	"encoding/hex"
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

func TestSignWalletConnectPayload(t *testing.T) {
	// Cross-check with neofs-sdk-go's saltMessageWalletConnect output.
	// The payload for salt=0x...aabbcc (16 bytes) and data=[]byte("test") should be:
	//   010001f0 | VarUint(32+8) | hex(salt) | base64("test") | 0000
	// base64("test") = "dGVzdA==" (8 bytes)
	// hex(salt) = 32 hex chars
	salt := make([]byte, WalletConnectSaltLen)
	for i := range salt {
		salt[i] = byte(i)
	}
	data := []byte("test")
	b64 := make([]byte, base64.StdEncoding.EncodedLen(len(data)))
	base64.StdEncoding.Encode(b64, data)

	payload := walletConnectPayload(salt, b64)

	// Check prefix
	require.Equal(t, []byte{0x01, 0x00, 0x01, 0xf0}, payload[:4])

	// hex(salt) is 32 bytes, base64("test") is 8 bytes → saltedLen = 40
	saltedLen := hex.EncodedLen(WalletConnectSaltLen) + len(b64)
	require.Equal(t, 40, saltedLen)

	// VarUint(40) = 0x28 (1 byte since 40 < 0xfd)
	require.Equal(t, byte(40), payload[4])

	// hex(salt) starts at offset 5
	expectedHexSalt := hex.EncodeToString(salt)
	require.Equal(t, expectedHexSalt, string(payload[5:5+hex.EncodedLen(WalletConnectSaltLen)]))

	// base64("test") follows
	require.Equal(t, "dGVzdA==", string(payload[5+32:5+32+8]))

	// Suffix
	n := len(payload)
	require.Equal(t, []byte{0x00, 0x00}, payload[n-2:])
}

func TestWalletConnectEmptyMessage(t *testing.T) {
	priv, err := NewPrivateKey()
	require.NoError(t, err)

	msg := []byte{}
	sigWithSalt, err := priv.SignWalletConnect(msg)
	require.NoError(t, err)
	require.True(t, priv.PublicKey().VerifyWalletConnect(msg, sigWithSalt))
}
