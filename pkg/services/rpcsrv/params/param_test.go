package params

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/neorpc"
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
				require.Equal(t, neorpc.SignerWithWitness{Signer: transaction.Signer{Account: expectedAcc}}, actual)
			},
			expectedRawMessage: []byte(`{"account": "0xcadb3dc2faa3ef14a13b619c9a43124755aa2569"}`),
		},
		{
			check: func(t *testing.T, p *Param) {
				actual, err := p.GetSignerWithWitness()
				require.NoError(t, err)
				expectedAcc, err := address.StringToUint160("NYxb4fSZVKAz8YsgaPK2WkT3KcAE9b3Vag")
				require.NoError(t, err)
				require.Equal(t, neorpc.SignerWithWitness{Signer: transaction.Signer{Account: expectedAcc, Scopes: transaction.Global}}, actual)
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

func TestGetBigInt(t *testing.T) {
	maxUint64 := new(big.Int).SetUint64(math.MaxUint64)
	minInt64 := big.NewInt(math.MinInt64)
	testCases := []struct {
		raw      string
		expected *big.Int
	}{
		{"true", big.NewInt(1)},
		{"false", new(big.Int)},
		{"42", big.NewInt(42)},
		{`"` + minInt64.String() + `"`, minInt64},
		{`"` + maxUint64.String() + `"`, maxUint64},
		{`"` + minInt64.String() + `000"`, new(big.Int).Mul(minInt64, big.NewInt(1000))},
		{`"` + maxUint64.String() + `000"`, new(big.Int).Mul(maxUint64, big.NewInt(1000))},
		{`"abc"`, nil},
		{`[]`, nil},
		{`null`, nil},
	}

	for _, tc := range testCases {
		var p Param
		require.NoError(t, json.Unmarshal([]byte(tc.raw), &p))

		actual, err := p.GetBigInt()
		if tc.expected == nil {
			require.Error(t, err)
			continue
		}
		require.NoError(t, err)
		require.Equal(t, tc.expected, actual)

		expected := tc.expected.Int64()
		actualInt, err := p.GetInt()
		if !actual.IsInt64() || int64(int(expected)) != expected {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.Equal(t, int(expected), actualInt)
		}
	}
}

func TestGetWitness(t *testing.T) {
	accountHash, err := util.Uint160DecodeStringLE("cadb3dc2faa3ef14a13b619c9a43124755aa2569")
	require.NoError(t, err)
	addrHash, err := address.StringToUint160("NYxb4fSZVKAz8YsgaPK2WkT3KcAE9b3Vag")
	require.NoError(t, err)

	testCases := []struct {
		raw        string
		expected   neorpc.SignerWithWitness
		shouldFail bool
	}{
		{
			raw: `{"account": "0xcadb3dc2faa3ef14a13b619c9a43124755aa2569"}`,
			expected: neorpc.SignerWithWitness{
				Signer: transaction.Signer{
					Account: accountHash,
					Scopes:  transaction.None,
				},
			},
		},
		{
			raw: `{"account": "NYxb4fSZVKAz8YsgaPK2WkT3KcAE9b3Vag", "scopes": "Global"}`,
			expected: neorpc.SignerWithWitness{
				Signer: transaction.Signer{
					Account: addrHash,
					Scopes:  transaction.Global,
				},
			},
		},
		{
			raw: `{"account": "0xcadb3dc2faa3ef14a13b619c9a43124755aa2569", "scopes": "Global"}`,
			expected: neorpc.SignerWithWitness{
				Signer: transaction.Signer{
					Account: accountHash,
					Scopes:  transaction.Global,
				},
			},
		},
		{
			raw: `{"account": "0xcadb3dc2faa3ef14a13b619c9a43124755aa2569", "scopes": 128}`,
			expected: neorpc.SignerWithWitness{
				Signer: transaction.Signer{
					Account: accountHash,
					Scopes:  transaction.Global,
				},
			},
		},
		{
			raw: `{"account": "0xcadb3dc2faa3ef14a13b619c9a43124755aa2569", "scopes": 0}`,
			expected: neorpc.SignerWithWitness{
				Signer: transaction.Signer{
					Account: accountHash,
					Scopes:  transaction.None,
				},
			},
		},
		{
			raw: `{"account": "0xcadb3dc2faa3ef14a13b619c9a43124755aa2569", "scopes": 1}`,
			expected: neorpc.SignerWithWitness{
				Signer: transaction.Signer{
					Account: accountHash,
					Scopes:  transaction.CalledByEntry,
				},
			},
		},
		{
			raw: `{"account": "0xcadb3dc2faa3ef14a13b619c9a43124755aa2569", "scopes": 17}`,
			expected: neorpc.SignerWithWitness{
				Signer: transaction.Signer{
					Account: accountHash,
					Scopes:  transaction.CalledByEntry | transaction.CustomContracts,
				},
			},
		},
		{
			raw:        `{"account": "0xcadb3dc2faa3ef14a13b619c9a43124755aa2569", "scopes": 178}`,
			shouldFail: true,
		},
		{
			raw:        `{"account": "0xcadb3dc2faa3ef14a13b619c9a43124755aa2569", "scopes": 2}`,
			shouldFail: true,
		},
	}

	for _, tc := range testCases {
		p := Param{RawMessage: json.RawMessage(tc.raw)}
		actual, err := p.GetSignerWithWitness()
		if tc.shouldFail {
			require.Error(t, err, tc.raw)
		} else {
			require.NoError(t, err, tc.raw)
			require.Equal(t, tc.expected, actual)

			actual, err = p.GetSignerWithWitness() // valid second invocation.
			require.NoError(t, err, tc.raw)
			require.Equal(t, tc.expected, actual)
		}
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
	c := neorpc.SignerWithWitness{
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

	t.Run("overflow", func(t *testing.T) {
		var hashes = make([]util.Uint256, transaction.MaxAttributes+1)
		msg, err := json.Marshal(hashes)
		require.NoError(t, err)
		p := Param{RawMessage: msg}
		_, _, err = p.GetSignersWithWitnesses()
		require.Error(t, err)
	})

	t.Run("bad format", func(t *testing.T) {
		p := Param{RawMessage: []byte(`"not a signer"`)}
		_, _, err := p.GetSignersWithWitnesses()
		require.Error(t, err)
	})
}

func TestParamGetUUID(t *testing.T) {
	t.Run("from null", func(t *testing.T) {
		p := Param{RawMessage: []byte("null")}
		_, err := p.GetUUID()
		require.ErrorIs(t, err, errNotAString)
	})
	t.Run("invalid uuid", func(t *testing.T) {
		p := Param{RawMessage: []byte(`"not-a-uuid"`)}
		_, err := p.GetUUID()
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "not a valid UUID"), err.Error())
	})
	t.Run("compat", func(t *testing.T) {
		expected := "2107da59-4f9c-462c-9c51-7666842519a9"
		p := Param{RawMessage: []byte(fmt.Sprintf(`"%s"`, expected))}
		id, err := p.GetUUID()
		require.NoError(t, err)
		require.Equal(t, id.String(), expected)
	})
}
