package state

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/stretchr/testify/assert"
)

func TestEncodeDecodeContractState(t *testing.T) {
	script := []byte("testscript")

	h := hash.Hash160(script)
	m := manifest.NewManifest(h)
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
	m.Features = smartcontract.HasStorage
	contract := &Contract{
		ID:       123,
		Script:   script,
		Manifest: *m,
	}

	assert.Equal(t, h, contract.ScriptHash())

	t.Run("Serializable", func(t *testing.T) {
		contractDecoded := new(Contract)
		testserdes.EncodeDecodeBinary(t, contract, contractDecoded)
		assert.Equal(t, contract.ScriptHash(), contractDecoded.ScriptHash())
	})
	t.Run("JSON", func(t *testing.T) {
		contractDecoded := new(Contract)
		testserdes.MarshalUnmarshalJSON(t, contract, contractDecoded)
		assert.Equal(t, contract.ScriptHash(), contractDecoded.ScriptHash())
	})
}
