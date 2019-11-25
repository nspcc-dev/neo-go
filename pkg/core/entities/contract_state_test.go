package entities

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/crypto/hash"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/smartcontract"
	"github.com/stretchr/testify/assert"
)

func TestEncodeDecodeContractState(t *testing.T) {
	script := []byte("testscript")

	contract := &ContractState{
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
	buf := io.NewBufBinWriter()
	contract.EncodeBinary(buf.BinWriter)
	assert.Nil(t, buf.Err)
	contractDecoded := &ContractState{}
	r := io.NewBinReaderFromBuf(buf.Bytes())
	contractDecoded.DecodeBinary(r)
	assert.Nil(t, r.Err)
	assert.Equal(t, contract, contractDecoded)
	assert.Equal(t, contract.ScriptHash(), contractDecoded.ScriptHash())
}

func TestContractStateProperties(t *testing.T) {
	flaggedContract := ContractState{
		Properties: smartcontract.HasStorage | smartcontract.HasDynamicInvoke | smartcontract.IsPayable,
	}
	nonFlaggedContract := ContractState{
		ReturnType: smartcontract.BoolType,
	}
	assert.Equal(t, true, flaggedContract.HasStorage())
	assert.Equal(t, true, flaggedContract.HasDynamicInvoke())
	assert.Equal(t, true, flaggedContract.IsPayable())
	assert.Equal(t, false, nonFlaggedContract.HasStorage())
	assert.Equal(t, false, nonFlaggedContract.HasDynamicInvoke())
	assert.Equal(t, false, nonFlaggedContract.IsPayable())
}
