package context

import (
	"encoding/hex"
	"io"
	"math/rand"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestContextItem_AddSignature(t *testing.T) {
	item := &Item{Signatures: make(map[string][]byte)}

	priv1, err := keys.NewPrivateKey()
	require.NoError(t, err)

	pub1 := priv1.PublicKey()
	sig1 := []byte{1, 2, 3}
	item.AddSignature(pub1, sig1)
	require.Equal(t, sig1, item.GetSignature(pub1))

	priv2, err := keys.NewPrivateKey()
	require.NoError(t, err)

	pub2 := priv2.PublicKey()
	sig2 := []byte{5, 6, 7}
	item.AddSignature(pub2, sig2)
	require.Equal(t, sig2, item.GetSignature(pub2))
	require.Equal(t, sig1, item.GetSignature(pub1))
}

func TestContextItem_MarshalJSON(t *testing.T) {
	priv1, err := keys.NewPrivateKey()
	require.NoError(t, err)

	priv2, err := keys.NewPrivateKey()
	require.NoError(t, err)

	expected := &Item{
		Script: util.Uint160{1, 2, 3},
		Parameters: []smartcontract.Parameter{{
			Type:  smartcontract.SignatureType,
			Value: getRandomSlice(t, 64),
		}},
		Signatures: map[string][]byte{
			hex.EncodeToString(priv1.PublicKey().Bytes()): getRandomSlice(t, 64),
			hex.EncodeToString(priv2.PublicKey().Bytes()): getRandomSlice(t, 64),
		},
	}

	testserdes.MarshalUnmarshalJSON(t, expected, new(Item))
}

func getRandomSlice(t *testing.T, n int) []byte {
	src := rand.NewSource(time.Now().UnixNano())
	r := rand.New(src)
	data := make([]byte, n)
	_, err := io.ReadFull(r, data)
	require.NoError(t, err)
	return data
}
