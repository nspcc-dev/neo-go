package request

import (
	"encoding/hex"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvocationScriptCreationGood(t *testing.T) {
	p := Param{Type: StringT, Value: "50befd26fdf6e4d957c11e078b24ebce6291456f"}
	contract, err := p.GetUint160FromHex()
	require.Nil(t, err)

	var paramScripts = []struct {
		ps     Params
		script string
	}{{
		script: "0c146f459162ceeb248b071ec157d9e4f6fd26fdbe5041627d5b52",
	}, {
		ps:     Params{{Type: StringT, Value: "transfer"}},
		script: "0c087472616e736665720c146f459162ceeb248b071ec157d9e4f6fd26fdbe5041627d5b52",
	}, {
		ps:     Params{{Type: NumberT, Value: 42}},
		script: "0c0234320c146f459162ceeb248b071ec157d9e4f6fd26fdbe5041627d5b52",
	}, {
		ps:     Params{{Type: StringT, Value: "a"}, {Type: ArrayT, Value: []Param{}}},
		script: "10c00c01610c146f459162ceeb248b071ec157d9e4f6fd26fdbe5041627d5b52",
	}, {
		ps:     Params{{Type: StringT, Value: "a"}, {Type: ArrayT, Value: []Param{{Type: FuncParamT, Value: FuncParam{Type: smartcontract.ByteArrayType, Value: Param{Type: StringT, Value: "AwEtR+diEK7HO+Oas9GG4KQP6Nhr+j1Pq/2le6E7iPlq"}}}}}},
		script: "0c2103012d47e76210aec73be39ab3d186e0a40fe8d86bfa3d4fabfda57ba13b88f96a11c00c01610c146f459162ceeb248b071ec157d9e4f6fd26fdbe5041627d5b52",
	}, {
		ps:     Params{{Type: StringT, Value: "a"}, {Type: ArrayT, Value: []Param{{Type: FuncParamT, Value: FuncParam{Type: smartcontract.SignatureType, Value: Param{Type: StringT, Value: "4edf5005771de04619235d5a4c7a9a11bb78e008541f1da7725f654c33380a3c87e2959a025da706d7255cb3a3fa07ebe9c6559d0d9e6213c68049168eb1056f"}}}}}},
		script: "0c404edf5005771de04619235d5a4c7a9a11bb78e008541f1da7725f654c33380a3c87e2959a025da706d7255cb3a3fa07ebe9c6559d0d9e6213c68049168eb1056f11c00c01610c146f459162ceeb248b071ec157d9e4f6fd26fdbe5041627d5b52",
	}, {
		ps:     Params{{Type: StringT, Value: "a"}, {Type: ArrayT, Value: []Param{{Type: FuncParamT, Value: FuncParam{Type: smartcontract.StringType, Value: Param{Type: StringT, Value: "50befd26fdf6e4d957c11e078b24ebce6291456f"}}}}}},
		script: "0c283530626566643236666466366534643935376331316530373862323465626365363239313435366611c00c01610c146f459162ceeb248b071ec157d9e4f6fd26fdbe5041627d5b52",
	}, {
		ps:     Params{{Type: StringT, Value: "a"}, {Type: ArrayT, Value: []Param{{Type: FuncParamT, Value: FuncParam{Type: smartcontract.Hash160Type, Value: Param{Type: StringT, Value: "50befd26fdf6e4d957c11e078b24ebce6291456f"}}}}}},
		script: "0c146f459162ceeb248b071ec157d9e4f6fd26fdbe5011c00c01610c146f459162ceeb248b071ec157d9e4f6fd26fdbe5041627d5b52",
	}, {
		ps:     Params{{Type: StringT, Value: "a"}, {Type: ArrayT, Value: []Param{{Type: FuncParamT, Value: FuncParam{Type: smartcontract.Hash256Type, Value: Param{Type: StringT, Value: "602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7"}}}}}},
		script: "0c20e72d286979ee6cb1b7e65dfddfb2e384100b8d148e7758de42e4168b71792c6011c00c01610c146f459162ceeb248b071ec157d9e4f6fd26fdbe5041627d5b52",
	}, {
		ps:     Params{{Type: StringT, Value: "a"}, {Type: ArrayT, Value: []Param{{Type: FuncParamT, Value: FuncParam{Type: smartcontract.PublicKeyType, Value: Param{Type: StringT, Value: "03c089d7122b840a4935234e82e26ae5efd0c2acb627239dc9f207311337b6f2c1"}}}}}},
		script: "0c2103c089d7122b840a4935234e82e26ae5efd0c2acb627239dc9f207311337b6f2c111c00c01610c146f459162ceeb248b071ec157d9e4f6fd26fdbe5041627d5b52",
	}, {
		ps:     Params{{Type: StringT, Value: "a"}, {Type: ArrayT, Value: []Param{{Type: FuncParamT, Value: FuncParam{Type: smartcontract.IntegerType, Value: Param{Type: NumberT, Value: 42}}}}}},
		script: "002a11c00c01610c146f459162ceeb248b071ec157d9e4f6fd26fdbe5041627d5b52",
	}, {
		ps:     Params{{Type: StringT, Value: "a"}, {Type: ArrayT, Value: []Param{{Type: FuncParamT, Value: FuncParam{Type: smartcontract.BoolType, Value: Param{Type: StringT, Value: "true"}}}}}},
		script: "1111c00c01610c146f459162ceeb248b071ec157d9e4f6fd26fdbe5041627d5b52",
	}, {
		ps:     Params{{Type: StringT, Value: "a"}, {Type: ArrayT, Value: []Param{{Type: FuncParamT, Value: FuncParam{Type: smartcontract.BoolType, Value: Param{Type: StringT, Value: "false"}}}}}},
		script: "1011c00c01610c146f459162ceeb248b071ec157d9e4f6fd26fdbe5041627d5b52",
	}}
	for _, ps := range paramScripts {
		script, err := CreateFunctionInvocationScript(contract, ps.ps)
		assert.Nil(t, err)
		assert.Equal(t, ps.script, hex.EncodeToString(script))
	}
}

func TestInvocationScriptCreationBad(t *testing.T) {
	contract := util.Uint160{}

	var testParams = []Params{
		{{Type: NumberT, Value: "qwerty"}},
		{{Type: ArrayT, Value: 42}},
		{{Type: ArrayT, Value: []Param{{Type: NumberT, Value: 42}}}},
		{{Type: ArrayT, Value: []Param{{Type: FuncParamT, Value: FuncParam{Type: smartcontract.ByteArrayType, Value: Param{Type: StringT, Value: "qwerty"}}}}}},
		{{Type: ArrayT, Value: []Param{{Type: FuncParamT, Value: FuncParam{Type: smartcontract.SignatureType, Value: Param{Type: StringT, Value: "qwerty"}}}}}},
		{{Type: ArrayT, Value: []Param{{Type: FuncParamT, Value: FuncParam{Type: smartcontract.StringType, Value: Param{Type: NumberT, Value: 42}}}}}},
		{{Type: ArrayT, Value: []Param{{Type: FuncParamT, Value: FuncParam{Type: smartcontract.Hash160Type, Value: Param{Type: StringT, Value: "qwerty"}}}}}},
		{{Type: ArrayT, Value: []Param{{Type: FuncParamT, Value: FuncParam{Type: smartcontract.Hash256Type, Value: Param{Type: StringT, Value: "qwerty"}}}}}},
		{{Type: ArrayT, Value: []Param{{Type: FuncParamT, Value: FuncParam{Type: smartcontract.PublicKeyType, Value: Param{Type: NumberT, Value: 42}}}}}},
		{{Type: ArrayT, Value: []Param{{Type: FuncParamT, Value: FuncParam{Type: smartcontract.PublicKeyType, Value: Param{Type: StringT, Value: "qwerty"}}}}}},
		{{Type: ArrayT, Value: []Param{{Type: FuncParamT, Value: FuncParam{Type: smartcontract.IntegerType, Value: Param{Type: StringT, Value: "qwerty"}}}}}},
		{{Type: ArrayT, Value: []Param{{Type: FuncParamT, Value: FuncParam{Type: smartcontract.BoolType, Value: Param{Type: NumberT, Value: 42}}}}}},
		{{Type: ArrayT, Value: []Param{{Type: FuncParamT, Value: FuncParam{Type: smartcontract.BoolType, Value: Param{Type: StringT, Value: "qwerty"}}}}}},
		{{Type: ArrayT, Value: []Param{{Type: FuncParamT, Value: FuncParam{Type: smartcontract.UnknownType, Value: Param{}}}}}},
	}
	for _, ps := range testParams {
		_, err := CreateFunctionInvocationScript(contract, ps)
		assert.NotNil(t, err)
	}
}

func TestExpandArrayIntoScript(t *testing.T) {
	testCases := []struct {
		Input    []Param
		Expected []byte
	}{
		{
			Input:    []Param{{Type: FuncParamT, Value: FuncParam{Type: smartcontract.StringType, Value: Param{Value: "a"}}}},
			Expected: []byte{byte(opcode.PUSHDATA1), 1, byte('a')},
		},
		{
			Input:    []Param{{Type: FuncParamT, Value: FuncParam{Type: smartcontract.ArrayType, Value: Param{Value: []Param{{Type: FuncParamT, Value: FuncParam{Type: smartcontract.StringType, Value: Param{Value: "a"}}}}}}}},
			Expected: []byte{byte(opcode.PUSHDATA1), 1, byte('a'), byte(opcode.PUSH1), byte(opcode.PACK)},
		},
	}
	for _, c := range testCases {
		script := io.NewBufBinWriter()
		err := expandArrayIntoScript(script.BinWriter, c.Input)
		require.NoError(t, err)
		require.Equal(t, c.Expected, script.Bytes())
	}
	errorCases := [][]Param{
		{
			{Type: FuncParamT, Value: FuncParam{Type: smartcontract.ArrayType, Value: Param{Value: "a"}}},
		},
		{
			{Type: FuncParamT, Value: FuncParam{Type: smartcontract.ArrayType, Value: Param{Value: []Param{{Type: FuncParamT, Value: nil}}}}},
		},
	}
	for _, c := range errorCases {
		script := io.NewBufBinWriter()
		err := expandArrayIntoScript(script.BinWriter, c)
		require.Error(t, err)
	}
}
