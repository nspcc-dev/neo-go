package state

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/stretchr/testify/assert"
)

func TestEncodeDecodeContractState(t *testing.T) {
	script := []byte("testscript")

	contract := &Contract{
		Script:      script,
		ParamList:   []smartcontract.ParamType{smartcontract.StringType, smartcontract.IntegerType, smartcontract.Hash160Type},
		ReturnType:  smartcontract.BoolType,
		Properties:  smartcontract.HasStorage,
		Name:        "Contrato",
		CodeVersion: "1.0.0",
		Author:      "Joe Random",
		Email:       "joe@example.com",
		Description: "Test contract",
	}

	assert.Equal(t, hash.Hash160(script), contract.ScriptHash())

	contractDecoded := &Contract{}
	testserdes.EncodeDecodeBinary(t, contract, contractDecoded)
	assert.Equal(t, contract.ScriptHash(), contractDecoded.ScriptHash())
}

func TestContractStateProperties(t *testing.T) {
	flaggedContract := Contract{
		Properties: smartcontract.HasStorage | smartcontract.HasDynamicInvoke | smartcontract.IsPayable,
	}
	nonFlaggedContract := Contract{
		ReturnType: smartcontract.BoolType,
	}
	assert.Equal(t, true, flaggedContract.HasStorage())
	assert.Equal(t, true, flaggedContract.HasDynamicInvoke())
	assert.Equal(t, true, flaggedContract.IsPayable())
	assert.Equal(t, false, nonFlaggedContract.HasStorage())
	assert.Equal(t, false, nonFlaggedContract.HasDynamicInvoke())
	assert.Equal(t, false, nonFlaggedContract.IsPayable())
}
