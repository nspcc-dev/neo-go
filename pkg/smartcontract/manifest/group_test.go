package manifest

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
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
