package consensus

import (
	"crypto/sha256"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/stretchr/testify/require"
)

func TestCrypt(t *testing.T) {
	key, err := keys.NewPrivateKey()
	require.NoError(t, err)

	priv := privateKey{key}

	pub := key.PublicKey()

	data := []byte{1, 2, 3, 4}
	hash := sha256.Sum256(data)

	sign, err := priv.Sign(data)
	require.NoError(t, err)
	require.True(t, pub.Verify(sign, hash[:]))

	sign[0] = ^sign[0]
	require.False(t, pub.Verify(sign, hash[:]))
}
