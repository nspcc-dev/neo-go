package request

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParam_UnmarshalJSON(t *testing.T) {
	type testCase struct {
		check              func(t *testing.T, p *Param)
		expectedRawMessage []byte
	}
	msg := `["123", 123, null, ["str2", 3], [{"type": "String", "value": "jajaja"}],
	  {"account": "0xcadb3dc2faa3ef14a13b619c9a43124755aa2569"},
	  {"account": "NYxb4fSZVKAz8YsgaPK2WkT3KcAE9b3Vag", "scopes": "Global"},
	  [{"account": "0xcadb3dc2faa3ef14a13b619c9a43124755aa2569", "scopes": "Global"}]]`
	expected := []testCase{
		{
			check: func(t *testing.T, p *Param) {
				expectedS := "123"
				actualS, err := p.GetStringStrict()
				require.NoError(t, err)
				require.Equal(t, expectedS, actualS)
				actualS, err = p.GetString()
				require.NoError(t, err)
				require.Equal(t, expectedS, actualS)

				expectedI := 123
				_, err = p.GetIntStrict()
				require.Error(t, err)
				actualI, err := p.GetInt()
				require.NoError(t, err)
				require.Equal(t, expectedI, actualI)

				expectedB := true
				_, err = p.GetBooleanStrict()
				require.Error(t, err)
				actualB, err := p.GetBoolean()
				require.NoError(t, err)
				require.Equal(t, expectedB, actualB)

				_, err = p.GetArray()
				require.Error(t, err)
			},
			expectedRawMessage: []byte(`"123"`),
		},
		{
			check: func(t *testing.T, p *Param) {
				expectedS := "123"
				_, err := p.GetStringStrict()
				require.Error(t, err)
				actualS, err := p.GetString()
				require.NoError(t, err)
				require.Equal(t, expectedS, actualS)

				expectedI := 123
				actualI, err := p.GetIntStrict()
				require.NoError(t, err)
				require.Equal(t, expectedI, actualI)
				actualI, err = p.GetInt()
				require.NoError(t, err)
				require.Equal(t, expectedI, actualI)

				expectedB := true
				_, err = p.GetBooleanStrict()
				require.Error(t, err)
				actualB, err := p.GetBoolean()
				require.NoError(t, err)
				require.Equal(t, expectedB, actualB)

				_, err = p.GetArray()
				require.Error(t, err)
			},
			expectedRawMessage: []byte(`123`),
		},
		{
			check: func(t *testing.T, p *Param) {
				_, err := p.GetStringStrict()
				require.Error(t, err)
				_, err = p.GetString()
				require.Error(t, err)

				_, err = p.GetIntStrict()
				require.Error(t, err)
				_, err = p.GetInt()
				require.Error(t, err)

				_, err = p.GetBooleanStrict()
				require.Error(t, err)
				_, err = p.GetBoolean()
				require.Error(t, err)

				_, err = p.GetArray()
				require.Error(t, err)
			},
			expectedRawMessage: []byte(`null`),
		},
		{
			check: func(t *testing.T, p *Param) {
				_, err := p.GetStringStrict()
				require.Error(t, err)
				_, err = p.GetString()
				require.Error(t, err)

				_, err = p.GetIntStrict()
				require.Error(t, err)
				_, err = p.GetInt()
				require.Error(t, err)

				_, err = p.GetBooleanStrict()
				require.Error(t, err)
				_, err = p.GetBoolean()
				require.Error(t, err)

				a, err := p.GetArray()
				require.NoError(t, err)
				require.Equal(t, []Param{
					{RawMessage: json.RawMessage(`"str2"`)},
					{RawMessage: json.RawMessage(`3`)},
				}, a)
			},
			expectedRawMessage: []byte(`["str2", 3]`),
		},
		{
			check: func(t *testing.T, p *Param) {
				a, err := p.GetArray()
				require.NoError(t, err)
				require.Equal(t, 1, len(a))
				fp, err := a[0].GetFuncParam()
				require.NoError(t, err)
				require.Equal(t, smartcontract.StringType, fp.Type)
				strVal, err := fp.Value.GetStringStrict()
				require.NoError(t, err)
				require.Equal(t, "jajaja", strVal)
			},
			expectedRawMessage: []byte(`[{"type": "String", "value": "jajaja"}]`),
		},
		{
			check: func(t *testing.T, p *Param) {
				actual, err := p.GetSignerWithWitness()
				require.NoError(t, err)
				expectedAcc, err := util.Uint160DecodeStringLE("cadb3dc2faa3ef14a13b619c9a43124755aa2569")
				require.NoError(t, err)
				require.Equal(t, SignerWithWitness{Signer: transaction.Signer{Account: expectedAcc}}, actual)
			},
			expectedRawMessage: []byte(`{"account": "0xcadb3dc2faa3ef14a13b619c9a43124755aa2569"}`),
		},
		{
			check: func(t *testing.T, p *Param) {
				actual, err := p.GetSignerWithWitness()
				require.NoError(t, err)
				expectedAcc, err := address.StringToUint160("NYxb4fSZVKAz8YsgaPK2WkT3KcAE9b3Vag")
				require.NoError(t, err)
				require.Equal(t, SignerWithWitness{Signer: transaction.Signer{Account: expectedAcc, Scopes: transaction.Global}}, actual)
			},
			expectedRawMessage: []byte(`{"account": "NYxb4fSZVKAz8YsgaPK2WkT3KcAE9b3Vag", "scopes": "Global"}`),
		},
		{
			check: func(t *testing.T, p *Param) {
				actualSigs, actualWtns, err := p.GetSignersWithWitnesses()
				require.NoError(t, err)
				require.Equal(t, 1, len(actualSigs))
				require.Equal(t, 1, len(actualWtns))
				expectedAcc, err := util.Uint160DecodeStringLE("cadb3dc2faa3ef14a13b619c9a43124755aa2569")
				require.NoError(t, err)
				require.Equal(t, transaction.Signer{Account: expectedAcc, Scopes: transaction.Global}, actualSigs[0])
				require.Equal(t, transaction.Witness{}, actualWtns[0])
			},
			expectedRawMessage: []byte(`[{"account": "0xcadb3dc2faa3ef14a13b619c9a43124755aa2569", "scopes": "Global"}]`),
		},
	}

	var ps Params
	require.NoError(t, json.Unmarshal([]byte(msg), &ps))
	require.Equal(t, len(expected), len(ps))
	for i, tc := range expected {
		require.NotNil(t, ps[i])
		require.Equal(t, json.RawMessage(tc.expectedRawMessage), ps[i].RawMessage, i)
		tc.check(t, &ps[i])
	}
}

func TestGetWitness(t *testing.T) {
	accountHash, err := util.Uint160DecodeStringLE("cadb3dc2faa3ef14a13b619c9a43124755aa2569")
	require.NoError(t, err)
	addrHash, err := address.StringToUint160("NYxb4fSZVKAz8YsgaPK2WkT3KcAE9b3Vag")
	require.NoError(t, err)

	testCases := []struct {
		raw      string
		expected SignerWithWitness
	}{
		{`{"account": "0xcadb3dc2faa3ef14a13b619c9a43124755aa2569"}`, SignerWithWitness{
			Signer: transaction.Signer{
				Account: accountHash,
				Scopes:  transaction.None,
			}},
		},
		{`{"account": "NYxb4fSZVKAz8YsgaPK2WkT3KcAE9b3Vag", "scopes": "Global"}`, SignerWithWitness{
			Signer: transaction.Signer{
				Account: addrHash,
				Scopes:  transaction.Global,
			}},
		},
		{`{"account": "0xcadb3dc2faa3ef14a13b619c9a43124755aa2569", "scopes": "Global"}`, SignerWithWitness{
			Signer: transaction.Signer{
				Account: accountHash,
				Scopes:  transaction.Global,
			}},
		},
	}

	for _, tc := range testCases {
		p := Param{RawMessage: json.RawMessage(tc.raw)}
		actual, err := p.GetSignerWithWitness()
		require.NoError(t, err)
		require.Equal(t, tc.expected, actual)

		actual, err = p.GetSignerWithWitness() // valid second invocation.
		require.NoError(t, err)
		require.Equal(t, tc.expected, actual)
	}
}

func TestParamGetUint256(t *testing.T) {
	gas := "602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7"
	u256, _ := util.Uint256DecodeStringLE(gas)
	p := Param{RawMessage: []byte(fmt.Sprintf(`"%s"`, gas))}
	u, err := p.GetUint256()
	assert.Equal(t, u256, u)
	require.Nil(t, err)

	p = Param{RawMessage: []byte(fmt.Sprintf(`"0x%s"`, gas))}
	u, err = p.GetUint256()
	require.NoError(t, err)
	assert.Equal(t, u256, u)

	p = Param{RawMessage: []byte(`42`)}
	_, err = p.GetUint256()
	require.NotNil(t, err)

	p = Param{RawMessage: []byte(`"qq2c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7"`)}
	_, err = p.GetUint256()
	require.NotNil(t, err)
}

func TestParamGetUint160FromHex(t *testing.T) {
	in := "50befd26fdf6e4d957c11e078b24ebce6291456f"
	u160, _ := util.Uint160DecodeStringLE(in)
	p := Param{RawMessage: []byte(fmt.Sprintf(`"%s"`, in))}
	u, err := p.GetUint160FromHex()
	assert.Equal(t, u160, u)
	require.Nil(t, err)

	p = Param{RawMessage: []byte(`42`)}
	_, err = p.GetUint160FromHex()
	require.NotNil(t, err)

	p = Param{RawMessage: []byte(`"wwbefd26fdf6e4d957c11e078b24ebce6291456f"`)}
	_, err = p.GetUint160FromHex()
	require.NotNil(t, err)
}

func TestParamGetUint160FromAddress(t *testing.T) {
	in := "NPAsqZkx9WhNd4P72uhZxBhLinSuNkxfB8"
	u160, _ := address.StringToUint160(in)
	p := Param{RawMessage: []byte(fmt.Sprintf(`"%s"`, in))}
	u, err := p.GetUint160FromAddress()
	assert.Equal(t, u160, u)
	require.Nil(t, err)

	p = Param{RawMessage: []byte(`42`)}
	_, err = p.GetUint160FromAddress()
	require.NotNil(t, err)

	p = Param{RawMessage: []byte(`"QK2nJJpJr6o664CWJKi1QRXjqeic2zRp8y"`)}
	_, err = p.GetUint160FromAddress()
	require.NotNil(t, err)
}

func TestParam_GetUint160FromAddressOrHex(t *testing.T) {
	in := "NPAsqZkx9WhNd4P72uhZxBhLinSuNkxfB8"
	inHex, _ := address.StringToUint160(in)

	t.Run("Address", func(t *testing.T) {
		p := Param{RawMessage: []byte(fmt.Sprintf(`"%s"`, in))}
		u, err := p.GetUint160FromAddressOrHex()
		require.NoError(t, err)
		require.Equal(t, inHex, u)
	})

	t.Run("Hex", func(t *testing.T) {
		p := Param{RawMessage: []byte(fmt.Sprintf(`"%s"`, inHex.StringLE()))}
		u, err := p.GetUint160FromAddressOrHex()
		require.NoError(t, err)
		require.Equal(t, inHex, u)
	})
}

func TestParamGetFuncParam(t *testing.T) {
	fp := FuncParam{
		Type:  smartcontract.StringType,
		Value: Param{RawMessage: []byte(`"jajaja"`)},
	}
	p := Param{RawMessage: []byte(`{"type": "String", "value": "jajaja"}`)}
	newfp, err := p.GetFuncParam()
	assert.Equal(t, fp, newfp)
	require.Nil(t, err)

	p = Param{RawMessage: []byte(`42`)}
	_, err = p.GetFuncParam()
	require.NotNil(t, err)
}

func TestParamGetBytesHex(t *testing.T) {
	in := "602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7"
	inb, _ := hex.DecodeString(in)
	p := Param{RawMessage: []byte(fmt.Sprintf(`"%s"`, in))}
	bh, err := p.GetBytesHex()
	assert.Equal(t, inb, bh)
	require.Nil(t, err)

	p = Param{RawMessage: []byte(`42`)}
	h, err := p.GetBytesHex()
	assert.Equal(t, []byte{0x42}, h) // that's the way how C# works: 42 -> "42" -> []byte{0x42}
	require.Nil(t, err)

	p = Param{RawMessage: []byte(`"qq2c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7"`)}
	_, err = p.GetBytesHex()
	require.NotNil(t, err)
}

func TestParamGetBytesBase64(t *testing.T) {
	in := "Aj4A8DoW6HB84EXrQu6A05JFFUHuUQ3BjhyL77rFTXQm"
	inb, err := base64.StdEncoding.DecodeString(in)
	require.NoError(t, err)
	p := Param{RawMessage: []byte(fmt.Sprintf(`"%s"`, in))}
	bh, err := p.GetBytesBase64()
	assert.Equal(t, inb, bh)
	require.Nil(t, err)

	p = Param{RawMessage: []byte(`42`)}
	_, err = p.GetBytesBase64()
	require.NotNil(t, err)

	p = Param{RawMessage: []byte("@j4A8DoW6HB84EXrQu6A05JFFUHuUQ3BjhyL77rFTXQm")}
	_, err = p.GetBytesBase64()
	require.NotNil(t, err)
}

func TestParamGetSigner(t *testing.T) {
	c := SignerWithWitness{
		Signer: transaction.Signer{
			Account: util.Uint160{1, 2, 3, 4},
			Scopes:  transaction.Global,
		},
		Witness: transaction.Witness{

			InvocationScript:   []byte{1, 2, 3},
			VerificationScript: []byte{1, 2, 3},
		},
	}
	p := Param{RawMessage: []byte(`{"account": "0x0000000000000000000000000000000004030201", "scopes": "Global", "invocation": "AQID", "verification": "AQID"}`)}
	actual, err := p.GetSignerWithWitness()
	require.NoError(t, err)
	require.Equal(t, c, actual)

	p = Param{RawMessage: []byte(`"not a signer"`)}
	_, err = p.GetSignerWithWitness()
	require.Error(t, err)
}

func TestParamGetSigners(t *testing.T) {
	u1 := util.Uint160{1, 2, 3, 4}
	u2 := util.Uint160{5, 6, 7, 8}
	t.Run("from hashes", func(t *testing.T) {
		p := Param{RawMessage: []byte(fmt.Sprintf(`["%s", "%s"]`, u1.StringLE(), u2.StringLE()))}
		actual, _, err := p.GetSignersWithWitnesses()
		require.NoError(t, err)
		require.Equal(t, 2, len(actual))
		require.True(t, u1.Equals(actual[0].Account))
		require.True(t, u2.Equals(actual[1].Account))
	})

	t.Run("bad format", func(t *testing.T) {
		p := Param{RawMessage: []byte(`"not a signer"`)}
		_, _, err := p.GetSignersWithWitnesses()
		require.Error(t, err)
	})
}
