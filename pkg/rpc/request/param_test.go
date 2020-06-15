package request

import (
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
	msg := `["str1", 123, ["str2", 3], [{"type": "String", "value": "jajaja"}],
                 {"primary": 1},
                 {"sender": "f84d6a337fbc3d3a201d41da99e86b479e7a2554"},
                 {"cosigner": "f84d6a337fbc3d3a201d41da99e86b479e7a2554"},
                 {"sender": "f84d6a337fbc3d3a201d41da99e86b479e7a2554", "cosigner": "f84d6a337fbc3d3a201d41da99e86b479e7a2554"},
                 {"contract": "f84d6a337fbc3d3a201d41da99e86b479e7a2554"},
                 {"state": "HALT"},
                 {"account": "0xcadb3dc2faa3ef14a13b619c9a43124755aa2569"},
                 [{"account": "0xcadb3dc2faa3ef14a13b619c9a43124755aa2569", "scopes": "Global"}]]`
	contr, err := util.Uint160DecodeStringLE("f84d6a337fbc3d3a201d41da99e86b479e7a2554")
	require.NoError(t, err)
	accountHash, err := util.Uint160DecodeStringLE("cadb3dc2faa3ef14a13b619c9a43124755aa2569")
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
			Value: TxFilter{Cosigner: &contr},
		},
		{
			Type:  TxFilterT,
			Value: TxFilter{Sender: &contr, Cosigner: &contr},
		},
		{
			Type:  NotificationFilterT,
			Value: NotificationFilter{Contract: contr},
		},
		{
			Type:  ExecutionFilterT,
			Value: ExecutionFilter{State: "HALT"},
		},
		{
			Type: Cosigner,
			Value: transaction.Cosigner{
				Account: accountHash,
				Scopes:  transaction.Global,
			},
		},
		{
			Type: ArrayT,
			Value: []Param{
				{
					Type: Cosigner,
					Value: transaction.Cosigner{
						Account: accountHash,
						Scopes:  transaction.Global,
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
	in := "AK2nJJpJr6o664CWJKi1QRXjqeic2zRp8y"
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

func TestParamGetCosigner(t *testing.T) {
	c := transaction.Cosigner{
		Account: util.Uint160{1, 2, 3, 4},
		Scopes:  transaction.Global,
	}
	p := Param{Type: Cosigner, Value: c}
	actual, err := p.GetCosigner()
	require.NoError(t, err)
	require.Equal(t, c, actual)

	p = Param{Type: Cosigner, Value: `{"account": "0xcadb3dc2faa3ef14a13b619c9a43124755aa2569", "scopes": 0}`}
	_, err = p.GetCosigner()
	require.Error(t, err)
}

func TestParamGetCosigners(t *testing.T) {
	u1 := util.Uint160{1, 2, 3, 4}
	u2 := util.Uint160{5, 6, 7, 8}
	t.Run("from hashes", func(t *testing.T) {
		p := Param{ArrayT, []Param{
			{Type: StringT, Value: u1.StringLE()},
			{Type: StringT, Value: u2.StringLE()},
		}}
		actual, err := p.GetCosigners()
		require.NoError(t, err)
		require.Equal(t, 2, len(actual))
		require.True(t, u1.Equals(actual[0].Account))
		require.True(t, u2.Equals(actual[1].Account))
	})

	t.Run("from cosigners", func(t *testing.T) {
		c1 := transaction.Cosigner{
			Account: u1,
			Scopes:  transaction.Global,
		}
		c2 := transaction.Cosigner{
			Account: u2,
			Scopes:  transaction.CustomContracts,
			AllowedContracts: []util.Uint160{
				{1, 2, 3},
				{4, 5, 6},
			},
		}
		p := Param{ArrayT, []Param{
			{Type: Cosigner, Value: c1},
			{Type: Cosigner, Value: c2},
		}}
		actual, err := p.GetCosigners()
		require.NoError(t, err)
		require.Equal(t, 2, len(actual))
		require.Equal(t, c1, actual[0])
		require.Equal(t, c2, actual[1])
	})

	t.Run("bad format", func(t *testing.T) {
		p := Param{ArrayT, []Param{
			{Type: StringT, Value: u1.StringLE()},
			{Type: StringT, Value: "bla"},
		}}
		_, err := p.GetCosigners()
		require.Error(t, err)
	})
}
