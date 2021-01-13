package state

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
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
		NEF: nef.File{
			Header: nef.Header{
				Magic:    nef.Magic,
				Compiler: "neo-go.test",
				Version:  "test",
			},
			Script:   script,
			Checksum: 0,
		},
		Manifest: *m,
	}
	contract.NEF.Checksum = contract.NEF.CalculateChecksum()

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

func TestContractFromStackItem(t *testing.T) {
	var (
		id           = stackitem.Make(42)
		counter      = stackitem.Make(11)
		chash        = stackitem.Make(util.Uint160{1, 2, 3}.BytesBE())
		script       = []byte{0, 9, 8}
		nefFile, _   = nef.NewFile(script)
		rawNef, _    = nefFile.Bytes()
		nefItem      = stackitem.NewByteArray(rawNef)
		manifest     = manifest.DefaultManifest("stack item")
		manifestB, _ = json.Marshal(manifest)
		manifItem    = stackitem.Make(manifestB)

		badCases = []struct {
			name string
			item stackitem.Item
		}{
			{"not an array", stackitem.Make(1)},
			{"id is not a number", stackitem.Make([]stackitem.Item{manifItem, counter, chash, nefItem, manifItem})},
			{"id is out of range", stackitem.Make([]stackitem.Item{stackitem.Make(math.MaxUint32), counter, chash, nefItem, manifItem})},
			{"counter is not a number", stackitem.Make([]stackitem.Item{id, manifItem, chash, nefItem, manifItem})},
			{"counter is out of range", stackitem.Make([]stackitem.Item{id, stackitem.Make(100500), chash, nefItem, manifItem})},
			{"hash is not a byte string", stackitem.Make([]stackitem.Item{id, counter, stackitem.NewArray(nil), nefItem, manifItem})},
			{"hash is not a hash", stackitem.Make([]stackitem.Item{id, counter, stackitem.Make([]byte{1, 2, 3}), nefItem, manifItem})},
			{"nef is not a byte string", stackitem.Make([]stackitem.Item{id, counter, chash, stackitem.NewArray(nil), manifItem})},
			{"manifest is not a byte string", stackitem.Make([]stackitem.Item{id, counter, chash, nefItem, stackitem.NewArray(nil)})},
			{"manifest is not correct", stackitem.Make([]stackitem.Item{id, counter, chash, nefItem, stackitem.Make(100500)})},
		}
	)
	for _, cs := range badCases {
		t.Run(cs.name, func(t *testing.T) {
			var c = new(Contract)
			err := c.FromStackItem(cs.item)
			require.Error(t, err)
		})
	}
	var c = new(Contract)
	err := c.FromStackItem(stackitem.Make([]stackitem.Item{id, counter, chash, nefItem, manifItem}))
	require.NoError(t, err)
}
