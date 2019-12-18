package consensus

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/nspcc-dev/dbft/crypto"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/stretchr/testify/require"
)

func TestCrypt(t *testing.T) {
	key, err := keys.NewPrivateKey()
	require.NoError(t, err)

	priv := privateKey{key}
	data, err := priv.MarshalBinary()
	require.NoError(t, err)

	key1, err := keys.NewPrivateKey()
	require.NoError(t, err)

	priv1 := privateKey{key1}
	require.NotEqual(t, priv, priv1)
	require.NoError(t, priv1.UnmarshalBinary(data))
	require.Equal(t, priv, priv1)

	pub := publicKey{key.PublicKey()}
	data, err = pub.MarshalBinary()
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

func Test1(t *testing.T) {
	for i := 0; i < 4; i++ {
		priv, pub := crypto.GenerateWith(crypto.SuiteBLS, rand.Reader)
		data, _ := priv.MarshalBinary()
		fmt.Printf("pri %d: %s\n", i, hex.EncodeToString(data))

		data, _ = pub.MarshalBinary()
		fmt.Printf("pub %d: %s\n", i, hex.EncodeToString(data))
	}
}