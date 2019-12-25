package rpc

import (
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/encoding/address"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParam_UnmarshalJSON(t *testing.T) {
	msg := `["str1", 123, ["str2", 3], [{"type": "String", "value": "jajaja"}]]`
	expected := Params{
		{
			Type:  stringT,
			Value: "str1",
		},
		{
			Type:  numberT,
			Value: 123,
		},
		{
			Type: arrayT,
			Value: []Param{
				{
					Type:  stringT,
					Value: "str2",
				},
				{
					Type:  numberT,
					Value: 3,
				},
			},
		},
		{
			Type: arrayT,
			Value: []Param{
				{
					Type: funcParamT,
					Value: FuncParam{
						Type: String,
						Value: Param{
							Type:  stringT,
							Value: "jajaja",
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
}

func TestParamGetString(t *testing.T) {
	p := Param{stringT, "jajaja"}
	str, err := p.GetString()
	assert.Equal(t, "jajaja", str)
	require.Nil(t, err)

	p = Param{stringT, int(100500)}
	_, err = p.GetString()
	require.NotNil(t, err)
}

func TestParamGetInt(t *testing.T) {
	p := Param{numberT, int(100500)}
	i, err := p.GetInt()
	assert.Equal(t, 100500, i)
	require.Nil(t, err)

	p = Param{numberT, "jajaja"}
	_, err = p.GetInt()
	require.NotNil(t, err)
}

func TestParamGetArray(t *testing.T) {
	p := Param{arrayT, []Param{{numberT, 42}}}
	a, err := p.GetArray()
	assert.Equal(t, []Param{{numberT, 42}}, a)
	require.Nil(t, err)

	p = Param{arrayT, 42}
	_, err = p.GetArray()
	require.NotNil(t, err)
}

func TestParamGetUint256(t *testing.T) {
	gas := "602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7"
	u256, _ := util.Uint256DecodeStringLE(gas)
	p := Param{stringT, gas}
	u, err := p.GetUint256()
	assert.Equal(t, u256, u)
	require.Nil(t, err)

	p = Param{stringT, 42}
	_, err = p.GetUint256()
	require.NotNil(t, err)

	p = Param{stringT, "qq2c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7"}
	_, err = p.GetUint256()
	require.NotNil(t, err)
}

func TestParamGetUint160FromHex(t *testing.T) {
	in := "50befd26fdf6e4d957c11e078b24ebce6291456f"
	u160, _ := util.Uint160DecodeStringLE(in)
	p := Param{stringT, in}
	u, err := p.GetUint160FromHex()
	assert.Equal(t, u160, u)
	require.Nil(t, err)

	p = Param{stringT, 42}
	_, err = p.GetUint160FromHex()
	require.NotNil(t, err)

	p = Param{stringT, "wwbefd26fdf6e4d957c11e078b24ebce6291456f"}
	_, err = p.GetUint160FromHex()
	require.NotNil(t, err)
}

func TestParamGetUint160FromAddress(t *testing.T) {
	in := "AK2nJJpJr6o664CWJKi1QRXjqeic2zRp8y"
	u160, _ := address.StringToUint160(in)
	p := Param{stringT, in}
	u, err := p.GetUint160FromAddress()
	assert.Equal(t, u160, u)
	require.Nil(t, err)

	p = Param{stringT, 42}
	_, err = p.GetUint160FromAddress()
	require.NotNil(t, err)

	p = Param{stringT, "QK2nJJpJr6o664CWJKi1QRXjqeic2zRp8y"}
	_, err = p.GetUint160FromAddress()
	require.NotNil(t, err)
}

func TestParamGetFuncParam(t *testing.T) {
	fp := FuncParam{
		Type: String,
		Value: Param{
			Type:  stringT,
			Value: "jajaja",
		},
	}
	p := Param{
		Type:  funcParamT,
		Value: fp,
	}
	newfp, err := p.GetFuncParam()
	assert.Equal(t, fp, newfp)
	require.Nil(t, err)

	p = Param{funcParamT, 42}
	_, err = p.GetFuncParam()
	require.NotNil(t, err)
}

func TestParamGetBytesHex(t *testing.T) {
	in := "602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7"
	inb, _ := hex.DecodeString(in)
	p := Param{stringT, in}
	bh, err := p.GetBytesHex()
	assert.Equal(t, inb, bh)
	require.Nil(t, err)

	p = Param{stringT, 42}
	_, err = p.GetBytesHex()
	require.NotNil(t, err)

	p = Param{stringT, "qq2c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7"}
	_, err = p.GetBytesHex()
	require.NotNil(t, err)
}
