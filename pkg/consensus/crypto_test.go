package consensus

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/stretchr/testify/require"
)

func TestCrypt(t *testing.T) {
	key, err := keys.NewPrivateKey()
	require.NoError(t, err)

	priv := privateKey{key}

	key1, err := keys.NewPrivateKey()
	require.NoError(t, err)

	pub := publicKey{key.PublicKey()}
	data, err := pub.MarshalBinary()
	require.NoError(t, err)

	pub1 := publicKey{key1.PublicKey()}
	require.NotEqual(t, pub, pub1)
	require.NoError(t, pub1.UnmarshalBinary(data))
	require.Equal(t, pub, pub1)

	data = []byte{1, 2, 3, 4}

	sign, err := priv.Sign(data)
	require.NoError(t, err)
	require.NoError(t, pub.Verify(data, sign))

	sign[0] = ^sign[0]
	require.Error(t, pub.Verify(data, sign))
}
