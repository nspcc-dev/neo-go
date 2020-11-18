package state

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestEncodeDecodeContractState(t *testing.T) {
	script := []byte("testscript")

	h := hash.Hash160(script)
	m := manifest.NewManifest("Test")
	m.ABI.Methods = []manifest.Method{{
		Name: "main",
		Parameters: []manifest.Parameter{
			{
				Name: "amount",
				Type: smartcontract.IntegerType,
			},
			{
				Name: "hash",
				Type: smartcontract.Hash160Type,
			},
		},
		ReturnType: smartcontract.BoolType,
	}}
	contract := &Contract{
		ID:            123,
		UpdateCounter: 42,
		Hash:          h,
		Script:        script,
		Manifest:      *m,
	}

	t.Run("Serializable", func(t *testing.T) {
		contractDecoded := new(Contract)
		testserdes.EncodeDecodeBinary(t, contract, contractDecoded)
	})
	t.Run("JSON", func(t *testing.T) {
		contractDecoded := new(Contract)
		testserdes.MarshalUnmarshalJSON(t, contract, contractDecoded)
	})
}

func TestCreateContractHash(t *testing.T) {
	var script = []byte{1, 2, 3}
	var sender util.Uint160
	var err error

	require.Equal(t, "b4b7417195feca1cdb5a99504ab641d8c220ae99", CreateContractHash(sender, script).StringLE())
	sender, err = util.Uint160DecodeStringLE("a400ff00ff00ff00ff00ff00ff00ff00ff00ff01")
	require.NoError(t, err)
	require.Equal(t, "e56e4ee87f89a70e9138432c387ad49f2ee5b55f", CreateContractHash(sender, script).StringLE())
}
