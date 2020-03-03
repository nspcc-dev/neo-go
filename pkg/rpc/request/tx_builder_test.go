package request

import (
	"encoding/hex"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
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
		script: "676f459162ceeb248b071ec157d9e4f6fd26fdbe50",
	}, {
		ps:     Params{{Type: StringT, Value: "transfer"}},
		script: "087472616e73666572676f459162ceeb248b071ec157d9e4f6fd26fdbe50",
	}, {
		ps:     Params{{Type: NumberT, Value: 42}},
		script: "023432676f459162ceeb248b071ec157d9e4f6fd26fdbe50",
	}, {
		ps:     Params{{Type: StringT, Value: "a"}, {Type: ArrayT, Value: []Param{}}},
		script: "00c10161676f459162ceeb248b071ec157d9e4f6fd26fdbe50",
	}, {
		ps:     Params{{Type: StringT, Value: "a"}, {Type: ArrayT, Value: []Param{{Type: FuncParamT, Value: FuncParam{Type: smartcontract.ByteArrayType, Value: Param{Type: StringT, Value: "50befd26fdf6e4d957c11e078b24ebce6291456f"}}}}}},
		script: "1450befd26fdf6e4d957c11e078b24ebce6291456f51c10161676f459162ceeb248b071ec157d9e4f6fd26fdbe50",
	}, {
		ps:     Params{{Type: StringT, Value: "a"}, {Type: ArrayT, Value: []Param{{Type: FuncParamT, Value: FuncParam{Type: smartcontract.SignatureType, Value: Param{Type: StringT, Value: "4edf5005771de04619235d5a4c7a9a11bb78e008541f1da7725f654c33380a3c87e2959a025da706d7255cb3a3fa07ebe9c6559d0d9e6213c68049168eb1056f"}}}}}},
		script: "404edf5005771de04619235d5a4c7a9a11bb78e008541f1da7725f654c33380a3c87e2959a025da706d7255cb3a3fa07ebe9c6559d0d9e6213c68049168eb1056f51c10161676f459162ceeb248b071ec157d9e4f6fd26fdbe50",
	}, {
		ps:     Params{{Type: StringT, Value: "a"}, {Type: ArrayT, Value: []Param{{Type: FuncParamT, Value: FuncParam{Type: smartcontract.StringType, Value: Param{Type: StringT, Value: "50befd26fdf6e4d957c11e078b24ebce6291456f"}}}}}},
		script: "283530626566643236666466366534643935376331316530373862323465626365363239313435366651c10161676f459162ceeb248b071ec157d9e4f6fd26fdbe50",
	}, {
		ps:     Params{{Type: StringT, Value: "a"}, {Type: ArrayT, Value: []Param{{Type: FuncParamT, Value: FuncParam{Type: smartcontract.Hash160Type, Value: Param{Type: StringT, Value: "50befd26fdf6e4d957c11e078b24ebce6291456f"}}}}}},
		script: "146f459162ceeb248b071ec157d9e4f6fd26fdbe5051c10161676f459162ceeb248b071ec157d9e4f6fd26fdbe50",
	}, {
		ps:     Params{{Type: StringT, Value: "a"}, {Type: ArrayT, Value: []Param{{Type: FuncParamT, Value: FuncParam{Type: smartcontract.Hash256Type, Value: Param{Type: StringT, Value: "602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7"}}}}}},
		script: "20e72d286979ee6cb1b7e65dfddfb2e384100b8d148e7758de42e4168b71792c6051c10161676f459162ceeb248b071ec157d9e4f6fd26fdbe50",
	}, {
		ps:     Params{{Type: StringT, Value: "a"}, {Type: ArrayT, Value: []Param{{Type: FuncParamT, Value: FuncParam{Type: smartcontract.PublicKeyType, Value: Param{Type: StringT, Value: "03c089d7122b840a4935234e82e26ae5efd0c2acb627239dc9f207311337b6f2c1"}}}}}},
		script: "2103c089d7122b840a4935234e82e26ae5efd0c2acb627239dc9f207311337b6f2c151c10161676f459162ceeb248b071ec157d9e4f6fd26fdbe50",
	}, {
		ps:     Params{{Type: StringT, Value: "a"}, {Type: ArrayT, Value: []Param{{Type: FuncParamT, Value: FuncParam{Type: smartcontract.IntegerType, Value: Param{Type: NumberT, Value: 42}}}}}},
		script: "012a51c10161676f459162ceeb248b071ec157d9e4f6fd26fdbe50",
	}, {
		ps:     Params{{Type: StringT, Value: "a"}, {Type: ArrayT, Value: []Param{{Type: FuncParamT, Value: FuncParam{Type: smartcontract.BoolType, Value: Param{Type: StringT, Value: "true"}}}}}},
		script: "5151c10161676f459162ceeb248b071ec157d9e4f6fd26fdbe50",
	}, {
		ps:     Params{{Type: StringT, Value: "a"}, {Type: ArrayT, Value: []Param{{Type: FuncParamT, Value: FuncParam{Type: smartcontract.BoolType, Value: Param{Type: StringT, Value: "false"}}}}}},
		script: "0051c10161676f459162ceeb248b071ec157d9e4f6fd26fdbe50",
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
