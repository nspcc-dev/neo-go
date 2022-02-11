package request

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvocationScriptCreationGood(t *testing.T) {
	p := Param{RawMessage: []byte(`"50befd26fdf6e4d957c11e078b24ebce6291456f"`)}
	contract, err := p.GetUint160FromHex()
	require.Nil(t, err)

	var paramScripts = []struct {
		ps     Params
		script string
	}{{
		ps:     Params{{RawMessage: []byte(`"transfer"`)}},
		script: "c21f0c087472616e736665720c146f459162ceeb248b071ec157d9e4f6fd26fdbe5041627d5b52",
	}, {
		ps:     Params{{RawMessage: []byte(`42`)}},
		script: "c21f0c0234320c146f459162ceeb248b071ec157d9e4f6fd26fdbe5041627d5b52",
	}, {
		ps:     Params{{RawMessage: []byte(`"a"`)}, {RawMessage: []byte(`[]`)}},
		script: "10c01f0c01610c146f459162ceeb248b071ec157d9e4f6fd26fdbe5041627d5b52",
	}, {
		ps:     Params{{RawMessage: []byte(`"a"`)}, {RawMessage: []byte(`[{"type": "ByteString", "value": "AwEtR+diEK7HO+Oas9GG4KQP6Nhr+j1Pq/2le6E7iPlq"}]`)}},
		script: "0c2103012d47e76210aec73be39ab3d186e0a40fe8d86bfa3d4fabfda57ba13b88f96a11c01f0c01610c146f459162ceeb248b071ec157d9e4f6fd26fdbe5041627d5b52",
	}, {
		ps:     Params{{RawMessage: []byte(`"a"`)}, {RawMessage: []byte(`[{"type": "Signature", "value": "4edf5005771de04619235d5a4c7a9a11bb78e008541f1da7725f654c33380a3c87e2959a025da706d7255cb3a3fa07ebe9c6559d0d9e6213c68049168eb1056f"}]`)}},
		script: "0c404edf5005771de04619235d5a4c7a9a11bb78e008541f1da7725f654c33380a3c87e2959a025da706d7255cb3a3fa07ebe9c6559d0d9e6213c68049168eb1056f11c01f0c01610c146f459162ceeb248b071ec157d9e4f6fd26fdbe5041627d5b52",
	}, {
		ps:     Params{{RawMessage: []byte(`"a"`)}, {RawMessage: []byte(`[{"type": "String", "value": "50befd26fdf6e4d957c11e078b24ebce6291456f"}]`)}},
		script: "0c283530626566643236666466366534643935376331316530373862323465626365363239313435366611c01f0c01610c146f459162ceeb248b071ec157d9e4f6fd26fdbe5041627d5b52",
	}, {
		ps:     Params{{RawMessage: []byte(`"a"`)}, {RawMessage: []byte(`[{"type": "Hash160", "value": "50befd26fdf6e4d957c11e078b24ebce6291456f"}]`)}},
		script: "0c146f459162ceeb248b071ec157d9e4f6fd26fdbe5011c01f0c01610c146f459162ceeb248b071ec157d9e4f6fd26fdbe5041627d5b52",
	}, {
		ps:     Params{{RawMessage: []byte(`"a"`)}, {RawMessage: []byte(`[{"type": "Hash256", "value": "602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7"}]`)}},
		script: "0c20e72d286979ee6cb1b7e65dfddfb2e384100b8d148e7758de42e4168b71792c6011c01f0c01610c146f459162ceeb248b071ec157d9e4f6fd26fdbe5041627d5b52",
	}, {
		ps:     Params{{RawMessage: []byte(`"a"`)}, {RawMessage: []byte(`[{"type": "PublicKey", "value": "03c089d7122b840a4935234e82e26ae5efd0c2acb627239dc9f207311337b6f2c1"}]`)}},
		script: "0c2103c089d7122b840a4935234e82e26ae5efd0c2acb627239dc9f207311337b6f2c111c01f0c01610c146f459162ceeb248b071ec157d9e4f6fd26fdbe5041627d5b52",
	}, {
		ps:     Params{{RawMessage: []byte(`"a"`)}, {RawMessage: []byte(`[{"type": "Integer", "value": 42}]`)}},
		script: "002a11c01f0c01610c146f459162ceeb248b071ec157d9e4f6fd26fdbe5041627d5b52",
	}, {
		ps:     Params{{RawMessage: []byte(`"a"`)}, {RawMessage: []byte(`[{"type": "Integer", "value": "42"}]`)}}, // C# code doesn't use strict type assertions for JSON-ised params
		script: "002a11c01f0c01610c146f459162ceeb248b071ec157d9e4f6fd26fdbe5041627d5b52",
	}, {
		ps:     Params{{RawMessage: []byte(`"a"`)}, {RawMessage: []byte(`[{"type": "Integer", "value": true}]`)}}, // C# code doesn't use strict type assertions for JSON-ised params
		script: "1111c01f0c01610c146f459162ceeb248b071ec157d9e4f6fd26fdbe5041627d5b52",
	}, {
		ps:     Params{{RawMessage: []byte(`"a"`)}, {RawMessage: []byte(`[{"type": "Boolean", "value": true}]`)}},
		script: "1111c01f0c01610c146f459162ceeb248b071ec157d9e4f6fd26fdbe5041627d5b52",
	}, {
		ps:     Params{{RawMessage: []byte(`"a"`)}, {RawMessage: []byte(`[{"type": "Boolean", "value": false}]`)}},
		script: "1011c01f0c01610c146f459162ceeb248b071ec157d9e4f6fd26fdbe5041627d5b52",
	}, {
		ps:     Params{{RawMessage: []byte(`"a"`)}, {RawMessage: []byte(`[{"type": "Boolean", "value": "blah"}]`)}}, // C# code doesn't use strict type assertions for JSON-ised params
		script: "1111c01f0c01610c146f459162ceeb248b071ec157d9e4f6fd26fdbe5041627d5b52",
	}}
	for i, ps := range paramScripts {
		method, err := ps.ps[0].GetString()
		require.NoError(t, err, fmt.Sprintf("testcase #%d", i))
		var p *Param
		if len(ps.ps) > 1 {
			p = &ps.ps[1]
		}
		script, err := CreateFunctionInvocationScript(contract, method, p)
		assert.Nil(t, err)
		assert.Equal(t, ps.script, hex.EncodeToString(script), fmt.Sprintf("testcase #%d", i))
	}
}

func TestInvocationScriptCreationBad(t *testing.T) {
	contract := util.Uint160{}

	var testParams = []Param{
		{RawMessage: []byte(`true`)},
		{RawMessage: []byte(`[{"type": "ByteArray", "value": "qwerty"}]`)},
		{RawMessage: []byte(`[{"type": "Signature", "value": "qwerty"}]`)},
		{RawMessage: []byte(`[{"type": "Hash160", "value": "qwerty"}]`)},
		{RawMessage: []byte(`[{"type": "Hash256", "value": "qwerty"}]`)},
		{RawMessage: []byte(`[{"type": "PublicKey", "value": 42}]`)},
		{RawMessage: []byte(`[{"type": "PublicKey", "value": "qwerty"}]`)},
		{RawMessage: []byte(`[{"type": "Integer", "value": "123q"}]`)},
		{RawMessage: []byte(`[{"type": "Unknown"}]`)},
	}
	for i, ps := range testParams {
		_, err := CreateFunctionInvocationScript(contract, "", &ps)
		assert.NotNil(t, err, fmt.Sprintf("testcase #%d", i))
	}
}

func TestExpandArrayIntoScript(t *testing.T) {
	bi := new(big.Int).Lsh(big.NewInt(1), 254)
	rawInt := make([]byte, 32)
	rawInt[31] = 0x40

	testCases := []struct {
		Input    []Param
		Expected []byte
	}{
		{
			Input:    []Param{{RawMessage: []byte(`{"type": "String", "value": "a"}`)}},
			Expected: []byte{byte(opcode.PUSHDATA1), 1, byte('a')},
		},
		{
			Input:    []Param{{RawMessage: []byte(`{"type": "Array", "value": [{"type": "String", "value": "a"}]}`)}},
			Expected: []byte{byte(opcode.PUSHDATA1), 1, byte('a'), byte(opcode.PUSH1), byte(opcode.PACK)},
		},
		{
			Input:    []Param{{RawMessage: []byte(`{"type": "Integer", "value": "` + bi.String() + `"}`)}},
			Expected: append([]byte{byte(opcode.PUSHINT256)}, rawInt...),
		},
	}
	for _, c := range testCases {
		script := io.NewBufBinWriter()
		err := ExpandArrayIntoScript(script.BinWriter, c.Input)
		require.NoError(t, err)
		require.Equal(t, c.Expected, script.Bytes())
	}
	errorCases := [][]Param{
		{
			{RawMessage: []byte(`{"type": "Array", "value": "a"}`)},
		},
		{
			{RawMessage: []byte(`{"type": "Array", "value": null}`)},
		},
		{
			{RawMessage: []byte(`{"type": "Integer", "value": "` +
				new(big.Int).Lsh(big.NewInt(1), 255).String() + `"}`)},
		},
	}
	for _, c := range errorCases {
		script := io.NewBufBinWriter()
		err := ExpandArrayIntoScript(script.BinWriter, c)
		require.Error(t, err)
	}
}
