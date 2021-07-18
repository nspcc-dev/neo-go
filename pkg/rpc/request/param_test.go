package request

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParam_UnmarshalJSON(t *testing.T) {
	msg := `["str1", 123, null, ["str2", 3], [{"type": "String", "value": "jajaja"}],
                 {"primary": 1},
                 {"sender": "f84d6a337fbc3d3a201d41da99e86b479e7a2554"},
                 {"signer": "f84d6a337fbc3d3a201d41da99e86b479e7a2554"},
                 {"sender": "f84d6a337fbc3d3a201d41da99e86b479e7a2554", "signer": "f84d6a337fbc3d3a201d41da99e86b479e7a2554"},
                 {"contract": "f84d6a337fbc3d3a201d41da99e86b479e7a2554"},
                 {"name": "my_pretty_notification"},
                 {"contract": "f84d6a337fbc3d3a201d41da99e86b479e7a2554", "name":"my_pretty_notification"},
                 {"state": "HALT"},
                 {"account": "0xcadb3dc2faa3ef14a13b619c9a43124755aa2569"},
                 {"account": "NYxb4fSZVKAz8YsgaPK2WkT3KcAE9b3Vag", "scopes": "Global"},
                 [{"account": "0xcadb3dc2faa3ef14a13b619c9a43124755aa2569", "scopes": "Global"}]]`
	contr, err := util.Uint160DecodeStringLE("f84d6a337fbc3d3a201d41da99e86b479e7a2554")
	require.NoError(t, err)
	name := "my_pretty_notification"
	accountHash, err := util.Uint160DecodeStringLE("cadb3dc2faa3ef14a13b619c9a43124755aa2569")
	require.NoError(t, err)
	addrHash, err := address.StringToUint160("NYxb4fSZVKAz8YsgaPK2WkT3KcAE9b3Vag")
	require.NoError(t, err)
	expected := Params{
		{
			Type:  StringT,
			Value: "str1",
		},
		{
			Type:  NumberT,
			Value: 123,
		},
		{
			Type: defaultT,
		},
		{
			Type: ArrayT,
			Value: []Param{
				{
					Type:  StringT,
					Value: "str2",
				},
				{
					Type:  NumberT,
					Value: 3,
				},
			},
		},
		{
			Type: ArrayT,
			Value: []Param{
				{
					Type: FuncParamT,
					Value: FuncParam{
						Type: smartcontract.StringType,
						Value: Param{
							Type:  StringT,
							Value: "jajaja",
						},
					},
				},
			},
		},
		{
			Type:  BlockFilterT,
			Value: BlockFilter{Primary: 1},
		},
		{
			Type:  TxFilterT,
			Value: TxFilter{Sender: &contr},
		},
		{
			Type:  TxFilterT,
			Value: TxFilter{Signer: &contr},
		},
		{
			Type:  TxFilterT,
			Value: TxFilter{Sender: &contr, Signer: &contr},
		},
		{
			Type:  NotificationFilterT,
			Value: NotificationFilter{Contract: &contr},
		},
		{
			Type:  NotificationFilterT,
			Value: NotificationFilter{Name: &name},
		},
		{
			Type:  NotificationFilterT,
			Value: NotificationFilter{Contract: &contr, Name: &name},
		},
		{
			Type:  ExecutionFilterT,
			Value: ExecutionFilter{State: "HALT"},
		},
		{
			Type: SignerWithWitnessT,
			Value: SignerWithWitness{
				Signer: transaction.Signer{
					Account: accountHash,
					Scopes:  transaction.None,
				},
			},
		},
		{
			Type: SignerWithWitnessT,
			Value: SignerWithWitness{
				Signer: transaction.Signer{
					Account: addrHash,
					Scopes:  transaction.Global,
				},
			},
		},
		{
			Type: ArrayT,
			Value: []Param{
				{
					Type: SignerWithWitnessT,
					Value: SignerWithWitness{
						Signer: transaction.Signer{
							Account: accountHash,
							Scopes:  transaction.Global,
						},
					},
				},
			},
		},
	}

	var ps Params
	require.NoError(t, json.Unmarshal([]byte(msg), &ps))
	require.Equal(t, expected, ps)

	msg = `[{"2": 3}]`
	require.Error(t, json.Unmarshal([]byte(msg), &ps))
	msg = `[{"account": "notanaccount", "scopes": "Global"}]`
	require.Error(t, json.Unmarshal([]byte(msg), &ps))
}

func TestParamGetString(t *testing.T) {
	p := Param{StringT, "jajaja"}
	str, err := p.GetString()
	assert.Equal(t, "jajaja", str)
	require.Nil(t, err)

	p = Param{StringT, int(100500)}
	_, err = p.GetString()
	require.NotNil(t, err)
}

func TestParamGetInt(t *testing.T) {
	p := Param{NumberT, int(100500)}
	i, err := p.GetInt()
	assert.Equal(t, 100500, i)
	require.Nil(t, err)

	p = Param{NumberT, "jajaja"}
	_, err = p.GetInt()
	require.NotNil(t, err)
}

func TestParamGetArray(t *testing.T) {
	p := Param{ArrayT, []Param{{NumberT, 42}}}
	a, err := p.GetArray()
	assert.Equal(t, []Param{{NumberT, 42}}, a)
	require.Nil(t, err)

	p = Param{ArrayT, 42}
	_, err = p.GetArray()
	require.NotNil(t, err)
}

func TestParamGetUint256(t *testing.T) {
	gas := "602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7"
	u256, _ := util.Uint256DecodeStringLE(gas)
	p := Param{StringT, gas}
	u, err := p.GetUint256()
	assert.Equal(t, u256, u)
	require.Nil(t, err)

	p = Param{StringT, "0x" + gas}
	u, err = p.GetUint256()
	require.NoError(t, err)
	assert.Equal(t, u256, u)

	p = Param{StringT, 42}
	_, err = p.GetUint256()
	require.NotNil(t, err)

	p = Param{StringT, "qq2c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7"}
	_, err = p.GetUint256()
	require.NotNil(t, err)
}

func TestParamGetUint160FromHex(t *testing.T) {
	in := "50befd26fdf6e4d957c11e078b24ebce6291456f"
	u160, _ := util.Uint160DecodeStringLE(in)
	p := Param{StringT, in}
	u, err := p.GetUint160FromHex()
	assert.Equal(t, u160, u)
	require.Nil(t, err)

	p = Param{StringT, 42}
	_, err = p.GetUint160FromHex()
	require.NotNil(t, err)

	p = Param{StringT, "wwbefd26fdf6e4d957c11e078b24ebce6291456f"}
	_, err = p.GetUint160FromHex()
	require.NotNil(t, err)
}

func TestParamGetUint160FromAddress(t *testing.T) {
	in := "NPAsqZkx9WhNd4P72uhZxBhLinSuNkxfB8"
	u160, _ := address.StringToUint160(in)
	p := Param{StringT, in}
	u, err := p.GetUint160FromAddress()
	assert.Equal(t, u160, u)
	require.Nil(t, err)

	p = Param{StringT, 42}
	_, err = p.GetUint160FromAddress()
	require.NotNil(t, err)

	p = Param{StringT, "QK2nJJpJr6o664CWJKi1QRXjqeic2zRp8y"}
	_, err = p.GetUint160FromAddress()
	require.NotNil(t, err)
}

func TestParam_GetUint160FromAddressOrHex(t *testing.T) {
	in := "NPAsqZkx9WhNd4P72uhZxBhLinSuNkxfB8"
	inHex, _ := address.StringToUint160(in)

	t.Run("Address", func(t *testing.T) {
		p := Param{StringT, in}
		u, err := p.GetUint160FromAddressOrHex()
		require.NoError(t, err)
		require.Equal(t, inHex, u)
	})

	t.Run("Hex", func(t *testing.T) {
		p := Param{StringT, inHex.StringLE()}
		u, err := p.GetUint160FromAddressOrHex()
		require.NoError(t, err)
		require.Equal(t, inHex, u)
	})
}

func TestParamGetFuncParam(t *testing.T) {
	fp := FuncParam{
		Type: smartcontract.StringType,
		Value: Param{
			Type:  StringT,
			Value: "jajaja",
		},
	}
	p := Param{
		Type:  FuncParamT,
		Value: fp,
	}
	newfp, err := p.GetFuncParam()
	assert.Equal(t, fp, newfp)
	require.Nil(t, err)

	p = Param{FuncParamT, 42}
	_, err = p.GetFuncParam()
	require.NotNil(t, err)
}

func TestParamGetBytesHex(t *testing.T) {
	in := "602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7"
	inb, _ := hex.DecodeString(in)
	p := Param{StringT, in}
	bh, err := p.GetBytesHex()
	assert.Equal(t, inb, bh)
	require.Nil(t, err)

	p = Param{StringT, 42}
	_, err = p.GetBytesHex()
	require.NotNil(t, err)

	p = Param{StringT, "qq2c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7"}
	_, err = p.GetBytesHex()
	require.NotNil(t, err)
}

func TestParamGetBytesBase64(t *testing.T) {
	in := "Aj4A8DoW6HB84EXrQu6A05JFFUHuUQ3BjhyL77rFTXQm"
	inb, err := base64.StdEncoding.DecodeString(in)
	require.NoError(t, err)
	p := Param{StringT, in}
	bh, err := p.GetBytesBase64()
	assert.Equal(t, inb, bh)
	require.Nil(t, err)

	p = Param{StringT, 42}
	_, err = p.GetBytesBase64()
	require.NotNil(t, err)

	p = Param{StringT, "@j4A8DoW6HB84EXrQu6A05JFFUHuUQ3BjhyL77rFTXQm"}
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
	p := Param{Type: SignerWithWitnessT, Value: c}
	actual, err := p.GetSignerWithWitness()
	require.NoError(t, err)
	require.Equal(t, c, actual)

	p = Param{Type: SignerWithWitnessT, Value: `{"account": "0xcadb3dc2faa3ef14a13b619c9a43124755aa2569", "scopes": 0}`}
	_, err = p.GetSignerWithWitness()
	require.Error(t, err)
}

func TestParamGetSigners(t *testing.T) {
	u1 := util.Uint160{1, 2, 3, 4}
	u2 := util.Uint160{5, 6, 7, 8}
	t.Run("from hashes", func(t *testing.T) {
		p := Param{ArrayT, []Param{
			{Type: StringT, Value: u1.StringLE()},
			{Type: StringT, Value: u2.StringLE()},
		}}
		actual, _, err := p.GetSignersWithWitnesses()
		require.NoError(t, err)
		require.Equal(t, 2, len(actual))
		require.True(t, u1.Equals(actual[0].Account))
		require.True(t, u2.Equals(actual[1].Account))
	})

	t.Run("from signers", func(t *testing.T) {
		c1 := SignerWithWitness{
			Signer: transaction.Signer{
				Account: u1,
				Scopes:  transaction.Global,
			},
			Witness: transaction.Witness{
				InvocationScript:   []byte{1, 2, 3},
				VerificationScript: []byte{1, 2, 3},
			},
		}
		c2 := SignerWithWitness{
			Signer: transaction.Signer{
				Account: u2,
				Scopes:  transaction.CustomContracts,
				AllowedContracts: []util.Uint160{
					{1, 2, 3},
					{4, 5, 6},
				},
			},
		}
		p := Param{ArrayT, []Param{
			{Type: SignerWithWitnessT, Value: c1},
			{Type: SignerWithWitnessT, Value: c2},
		}}
		actualS, actualW, err := p.GetSignersWithWitnesses()
		require.NoError(t, err)
		require.Equal(t, 2, len(actualS))
		require.Equal(t, transaction.Signer{
			Account: c1.Account,
			Scopes:  c1.Scopes,
		}, actualS[0])
		require.Equal(t, transaction.Signer{
			Account:          c2.Account,
			Scopes:           c2.Scopes,
			AllowedContracts: c2.AllowedContracts,
		}, actualS[1])
		require.EqualValues(t, 2, len(actualW))
		require.EqualValues(t, transaction.Witness{
			InvocationScript:   c1.InvocationScript,
			VerificationScript: c1.VerificationScript,
		}, actualW[0])
		require.Equal(t, transaction.Witness{}, actualW[1])
	})

	t.Run("bad format", func(t *testing.T) {
		p := Param{ArrayT, []Param{
			{Type: StringT, Value: u1.StringLE()},
			{Type: StringT, Value: "bla"},
		}}
		_, _, err := p.GetSignersWithWitnesses()
		require.Error(t, err)
	})
}
