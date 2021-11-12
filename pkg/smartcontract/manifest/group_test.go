package manifest

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestGroupJSONInOut(t *testing.T) {
	priv, err := keys.NewPrivateKey()
	require.NoError(t, err)
	pub := priv.PublicKey()
	sig := make([]byte, keys.SignatureLen)
	g := Group{pub, sig}
	testserdes.MarshalUnmarshalJSON(t, &g, new(Group))
}

func TestGroupsAreValid(t *testing.T) {
	h := util.Uint160{42, 42, 42}
	priv, err := keys.NewPrivateKey()
	require.NoError(t, err)
	priv2, err := keys.NewPrivateKey()
	require.NoError(t, err)
	pub := priv.PublicKey()
	pub2 := priv2.PublicKey()
	gcorrect := Group{pub, priv.Sign(h.BytesBE())}
	gcorrect2 := Group{pub2, priv2.Sign(h.BytesBE())}
	gincorrect := Group{pub, priv.Sign(h.BytesLE())}
	gps := Groups{gcorrect}
	require.NoError(t, gps.AreValid(h))

	gps = Groups{gincorrect}
	require.Error(t, gps.AreValid(h))

	gps = Groups{gcorrect, gcorrect2}
	require.NoError(t, gps.AreValid(h))

	gps = Groups{gcorrect, gcorrect}
	require.Error(t, gps.AreValid(h))
}

func TestGroupsContains(t *testing.T) {
	priv, err := keys.NewPrivateKey()
	require.NoError(t, err)
	priv2, err := keys.NewPrivateKey()
	require.NoError(t, err)
	priv3, err := keys.NewPrivateKey()
	require.NoError(t, err)
	g1 := Group{priv.PublicKey(), nil}
	g2 := Group{priv2.PublicKey(), nil}
	gps := Groups{g1, g2}
	require.True(t, gps.Contains(priv2.PublicKey()))
	require.False(t, gps.Contains(priv3.PublicKey()))
}
