package core

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
		Script:           script,
		ParamList:        []smartcontract.ParamType{smartcontract.StringType, smartcontract.IntegerType, smartcontract.Hash160Type},
		ReturnType:       smartcontract.BoolType,
		Properties:       []byte("smth"),
		Name:             "Contracto",
		CodeVersion:      "1.0.0",
		Author:           "Joe Random",
		Email:            "joe@example.com",
		Description:      "Test contract",
		HasStorage:       true,
		HasDynamicInvoke: false,
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
