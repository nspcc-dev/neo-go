package request

import (
	"encoding/base64"
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
		script: "wh8MCHRyYW5zZmVyDBRvRZFizuskiwcewVfZ5Pb9Jv2+UEFifVtS",
	}, {
		ps:     Params{{RawMessage: []byte(`42`)}},
		script: "wh8MAjQyDBRvRZFizuskiwcewVfZ5Pb9Jv2+UEFifVtS",
	}, {
		ps:     Params{{RawMessage: []byte(`"a"`)}, {RawMessage: []byte(`[]`)}},
		script: "wh8MAWEMFG9FkWLO6ySLBx7BV9nk9v0m/b5QQWJ9W1I=",
	}, {
		ps:     Params{{RawMessage: []byte(`"a"`)}, {RawMessage: []byte(`[{"type": "ByteArray", "value": "AwEtR+diEK7HO+Oas9GG4KQP6Nhr+j1Pq/2le6E7iPlq"}]`)}},
		script: "DCEDAS1H52IQrsc745qz0YbgpA/o2Gv6PU+r/aV7oTuI+WoRwB8MAWEMFG9FkWLO6ySLBx7BV9nk9v0m/b5QQWJ9W1I=",
	}, {
		ps:     Params{{RawMessage: []byte(`"a"`)}, {RawMessage: []byte(`[{"type": "Signature", "value": "4edf5005771de04619235d5a4c7a9a11bb78e008541f1da7725f654c33380a3c87e2959a025da706d7255cb3a3fa07ebe9c6559d0d9e6213c68049168eb1056f"}]`)}},
		script: "DGDh51/nTTnvvV17TjrX3bfl3lrhztr1rXVtvvx7TTznjV/V1rvvbl/rnhzfffzRrdzzt7b3n1rTbl1rvTp3vbnlxvdrd9rTt5t71zrnn13R317rbXdzrzTj3Xrx5vXTnp8RwB8MAWEMFG9FkWLO6ySLBx7BV9nk9v0m/b5QQWJ9W1I=",
	}, {
		ps:     Params{{RawMessage: []byte(`"a"`)}, {RawMessage: []byte(`[{"type": "Signature", "value": "Tt9QBXcd4EYZI11aTHqaEbt44AhUHx2ncl9lTDM4CjyH4pWaAl2nBtclXLOj+gfr6cZVnQ2eYhPGgEkWjrEFbw=="}]`)}},
		script: "DEBO31AFdx3gRhkjXVpMepoRu3jgCFQfHadyX2VMMzgKPIfilZoCXacG1yVcs6P6B+vpxlWdDZ5iE8aASRaOsQVvEcAfDAFhDBRvRZFizuskiwcewVfZ5Pb9Jv2+UEFifVtS",
	}, {
		ps:     Params{{RawMessage: []byte(`"a"`)}, {RawMessage: []byte(`[{"type": "String", "value": "50befd26fdf6e4d957c11e078b24ebce6291456f"}]`)}},
		script: "DCg1MGJlZmQyNmZkZjZlNGQ5NTdjMTFlMDc4YjI0ZWJjZTYyOTE0NTZmEcAfDAFhDBRvRZFizuskiwcewVfZ5Pb9Jv2+UEFifVtS",
	}, {
		ps:     Params{{RawMessage: []byte(`"a"`)}, {RawMessage: []byte(`[{"type": "Hash160", "value": "50befd26fdf6e4d957c11e078b24ebce6291456f"}]`)}},
		script: "DBRvRZFizuskiwcewVfZ5Pb9Jv2+UBHAHwwBYQwUb0WRYs7rJIsHHsFX2eT2/Sb9vlBBYn1bUg==",
	}, {
		ps:     Params{{RawMessage: []byte(`"a"`)}, {RawMessage: []byte(`[{"type": "Hash256", "value": "602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7"}]`)}},
		script: "DCDnLShpee5ssbfmXf3fsuOEEAuNFI53WN5C5BaLcXksYBHAHwwBYQwUb0WRYs7rJIsHHsFX2eT2/Sb9vlBBYn1bUg==",
	}, {
		ps:     Params{{RawMessage: []byte(`"a"`)}, {RawMessage: []byte(`[{"type": "PublicKey", "value": "03c089d7122b840a4935234e82e26ae5efd0c2acb627239dc9f207311337b6f2c1"}]`)}},
		script: "DCEDwInXEiuECkk1I06C4mrl79DCrLYnI53J8gcxEze28sERwB8MAWEMFG9FkWLO6ySLBx7BV9nk9v0m/b5QQWJ9W1I=",
	}, {
		ps:     Params{{RawMessage: []byte(`"a"`)}, {RawMessage: []byte(`[{"type": "Integer", "value": 42}]`)}},
		script: "ACoRwB8MAWEMFG9FkWLO6ySLBx7BV9nk9v0m/b5QQWJ9W1I=",
	}, {
		ps:     Params{{RawMessage: []byte(`"a"`)}, {RawMessage: []byte(`[{"type": "Integer", "value": "42"}]`)}}, // C# code doesn't use strict type assertions for JSON-ised params
		script: "ACoRwB8MAWEMFG9FkWLO6ySLBx7BV9nk9v0m/b5QQWJ9W1I=",
	}, {
		ps:     Params{{RawMessage: []byte(`"a"`)}, {RawMessage: []byte(`[{"type": "Integer", "value": true}]`)}}, // C# code doesn't use strict type assertions for JSON-ised params
		script: "ERHAHwwBYQwUb0WRYs7rJIsHHsFX2eT2/Sb9vlBBYn1bUg==",
	}, {
		ps:     Params{{RawMessage: []byte(`"a"`)}, {RawMessage: []byte(`[{"type": "Boolean", "value": true}]`)}},
		script: "ERHAHwwBYQwUb0WRYs7rJIsHHsFX2eT2/Sb9vlBBYn1bUg==",
	}, {
		ps:     Params{{RawMessage: []byte(`"a"`)}, {RawMessage: []byte(`[{"type": "Boolean", "value": false}]`)}},
		script: "EBHAHwwBYQwUb0WRYs7rJIsHHsFX2eT2/Sb9vlBBYn1bUg==",
	}, {
		ps:     Params{{RawMessage: []byte(`"a"`)}, {RawMessage: []byte(`[{"type": "Boolean", "value": "blah"}]`)}}, // C# code doesn't use strict type assertions for JSON-ised params
		script: "ERHAHwwBYQwUb0WRYs7rJIsHHsFX2eT2/Sb9vlBBYn1bUg==",
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
		assert.Equal(t, ps.script, base64.StdEncoding.EncodeToString(script), fmt.Sprintf("testcase #%d", i))
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
