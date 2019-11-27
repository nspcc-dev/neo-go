package rpc

import (
	"encoding/hex"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvocationScriptCreationGood(t *testing.T) {
	p := Param{stringT, "50befd26fdf6e4d957c11e078b24ebce6291456f"}
	contract, err := p.GetUint160FromHex()
	require.Nil(t, err)

	var paramScripts = []struct {
		ps     Params
		script string
	}{{
		script: "676f459162ceeb248b071ec157d9e4f6fd26fdbe50",
	}, {
		ps:     Params{{stringT, "transfer"}},
		script: "087472616e73666572676f459162ceeb248b071ec157d9e4f6fd26fdbe50",
	}, {
		ps:     Params{{numberT, 42}},
		script: "023432676f459162ceeb248b071ec157d9e4f6fd26fdbe50",
	}, {
		ps:     Params{{stringT, "a"}, {arrayT, []Param{}}},
		script: "00c10161676f459162ceeb248b071ec157d9e4f6fd26fdbe50",
	}, {
		ps:     Params{{stringT, "a"}, {arrayT, []Param{{funcParamT, FuncParam{ByteArray, Param{stringT, "50befd26fdf6e4d957c11e078b24ebce6291456f"}}}}}},
		script: "1450befd26fdf6e4d957c11e078b24ebce6291456f51c10161676f459162ceeb248b071ec157d9e4f6fd26fdbe50",
	}, {
		ps:     Params{{stringT, "a"}, {arrayT, []Param{{funcParamT, FuncParam{Signature, Param{stringT, "4edf5005771de04619235d5a4c7a9a11bb78e008541f1da7725f654c33380a3c87e2959a025da706d7255cb3a3fa07ebe9c6559d0d9e6213c68049168eb1056f"}}}}}},
		script: "404edf5005771de04619235d5a4c7a9a11bb78e008541f1da7725f654c33380a3c87e2959a025da706d7255cb3a3fa07ebe9c6559d0d9e6213c68049168eb1056f51c10161676f459162ceeb248b071ec157d9e4f6fd26fdbe50",
	}, {
		ps:     Params{{stringT, "a"}, {arrayT, []Param{{funcParamT, FuncParam{String, Param{stringT, "50befd26fdf6e4d957c11e078b24ebce6291456f"}}}}}},
		script: "283530626566643236666466366534643935376331316530373862323465626365363239313435366651c10161676f459162ceeb248b071ec157d9e4f6fd26fdbe50",
	}, {
		ps:     Params{{stringT, "a"}, {arrayT, []Param{{funcParamT, FuncParam{Hash160, Param{stringT, "50befd26fdf6e4d957c11e078b24ebce6291456f"}}}}}},
		script: "146f459162ceeb248b071ec157d9e4f6fd26fdbe5051c10161676f459162ceeb248b071ec157d9e4f6fd26fdbe50",
	}, {
		ps:     Params{{stringT, "a"}, {arrayT, []Param{{funcParamT, FuncParam{Hash256, Param{stringT, "602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7"}}}}}},
		script: "20e72d286979ee6cb1b7e65dfddfb2e384100b8d148e7758de42e4168b71792c6051c10161676f459162ceeb248b071ec157d9e4f6fd26fdbe50",
	}, {
		ps:     Params{{stringT, "a"}, {arrayT, []Param{{funcParamT, FuncParam{PublicKey, Param{stringT, "03c089d7122b840a4935234e82e26ae5efd0c2acb627239dc9f207311337b6f2c1"}}}}}},
		script: "2103c089d7122b840a4935234e82e26ae5efd0c2acb627239dc9f207311337b6f2c151c10161676f459162ceeb248b071ec157d9e4f6fd26fdbe50",
	}, {
		ps:     Params{{stringT, "a"}, {arrayT, []Param{{funcParamT, FuncParam{Integer, Param{numberT, 42}}}}}},
		script: "012a51c10161676f459162ceeb248b071ec157d9e4f6fd26fdbe50",
	}, {
		ps:     Params{{stringT, "a"}, {arrayT, []Param{{funcParamT, FuncParam{Boolean, Param{stringT, "true"}}}}}},
		script: "5151c10161676f459162ceeb248b071ec157d9e4f6fd26fdbe50",
	}, {
		ps:     Params{{stringT, "a"}, {arrayT, []Param{{funcParamT, FuncParam{Boolean, Param{stringT, "false"}}}}}},
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
		Params{{numberT, "qwerty"}},
		Params{{arrayT, 42}},
		Params{{arrayT, []Param{{numberT, 42}}}},
		Params{{arrayT, []Param{{funcParamT, FuncParam{ByteArray, Param{stringT, "qwerty"}}}}}},
		Params{{arrayT, []Param{{funcParamT, FuncParam{Signature, Param{stringT, "qwerty"}}}}}},
		Params{{arrayT, []Param{{funcParamT, FuncParam{String, Param{numberT, 42}}}}}},
		Params{{arrayT, []Param{{funcParamT, FuncParam{Hash160, Param{stringT, "qwerty"}}}}}},
		Params{{arrayT, []Param{{funcParamT, FuncParam{Hash256, Param{stringT, "qwerty"}}}}}},
		Params{{arrayT, []Param{{funcParamT, FuncParam{PublicKey, Param{numberT, 42}}}}}},
		Params{{arrayT, []Param{{funcParamT, FuncParam{PublicKey, Param{stringT, "qwerty"}}}}}},
		Params{{arrayT, []Param{{funcParamT, FuncParam{Integer, Param{stringT, "qwerty"}}}}}},
		Params{{arrayT, []Param{{funcParamT, FuncParam{Boolean, Param{numberT, 42}}}}}},
		Params{{arrayT, []Param{{funcParamT, FuncParam{Boolean, Param{stringT, "qwerty"}}}}}},
		Params{{arrayT, []Param{{funcParamT, FuncParam{Unknown, Param{}}}}}},
	}
	for _, ps := range testParams {
		_, err := CreateFunctionInvocationScript(contract, ps)
		assert.NotNil(t, err)
	}
}
