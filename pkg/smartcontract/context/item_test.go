package context

import (
	"encoding/hex"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
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
		Script: []byte{1, 2, 3},
		Parameters: []smartcontract.Parameter{{
			Type:  smartcontract.SignatureType,
			Value: random.Bytes(keys.SignatureLen),
		}},
		Signatures: map[string][]byte{
			hex.EncodeToString(priv1.PublicKey().Bytes()): random.Bytes(keys.SignatureLen),
			hex.EncodeToString(priv2.PublicKey().Bytes()): random.Bytes(keys.SignatureLen),
		},
	}

	testserdes.MarshalUnmarshalJSON(t, expected, new(Item))
}
