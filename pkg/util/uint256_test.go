package util_test

import (
	"encoding/hex"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUint256UnmarshalJSON(t *testing.T) {
	str := "f037308fa0ab18155bccfc08485468c112409ea5064595699e98c545f245f32d"
	expected, err := util.Uint256DecodeStringLE(str)
	require.NoError(t, err)

	// UnmarshalJSON decodes hex-strings
	var u1, u2 util.Uint256

	require.NoError(t, u1.UnmarshalJSON([]byte(`"`+str+`"`)))
	assert.True(t, expected.Equals(u1))

	testserdes.MarshalUnmarshalJSON(t, &expected, &u2)

	// UnmarshalJSON does not accepts numbers
	assert.Error(t, u2.UnmarshalJSON([]byte("123")))
}

func TestUint256DecodeString(t *testing.T) {
	hexStr := "f037308fa0ab18155bccfc08485468c112409ea5064595699e98c545f245f32d"
	val, err := util.Uint256DecodeStringLE(hexStr)
	require.NoError(t, err)
	assert.Equal(t, hexStr, val.StringLE())

	valBE, err := util.Uint256DecodeStringBE(hexStr)
	require.NoError(t, err)
	assert.Equal(t, val, valBE.Reverse())

	bs, err := hex.DecodeString(hexStr)
	require.NoError(t, err)

	val1, err := util.Uint256DecodeBytesBE(bs)
	assert.NoError(t, err)
	assert.Equal(t, hexStr, val1.String())
	assert.Equal(t, val, val1.Reverse())

	_, err = util.Uint256DecodeStringLE(hexStr[1:])
	assert.Error(t, err)

	_, err = util.Uint256DecodeStringBE(hexStr[1:])
	assert.Error(t, err)

	hexStr = "zzz7308fa0ab18155bccfc08485468c112409ea5064595699e98c545f245f32d"
	_, err = util.Uint256DecodeStringLE(hexStr)
	assert.Error(t, err)

	_, err = util.Uint256DecodeStringBE(hexStr)
	assert.Error(t, err)
}

func TestUint256DecodeBytes(t *testing.T) {
	hexStr := "f037308fa0ab18155bccfc08485468c112409ea5064595699e98c545f245f32d"
	b, err := hex.DecodeString(hexStr)
	require.NoError(t, err)

	val, err := util.Uint256DecodeBytesLE(b)
	require.NoError(t, err)
	assert.Equal(t, hexStr, val.StringLE())

	_, err = util.Uint256DecodeBytesBE(b[1:])
	assert.Error(t, err)
}

func TestUInt256Equals(t *testing.T) {
	a := "f037308fa0ab18155bccfc08485468c112409ea5064595699e98c545f245f32d"
	b := "e287c5b29a1b66092be6803c59c765308ac20287e1b4977fd399da5fc8f66ab5"

	ua, err := util.Uint256DecodeStringLE(a)
	require.NoError(t, err)

	ub, err := util.Uint256DecodeStringLE(b)
	require.NoError(t, err)
	assert.False(t, ua.Equals(ub), "%s and %s cannot be equal", ua, ub)
	assert.True(t, ua.Equals(ua), "%s and %s must be equal", ua, ua)
	assert.Zero(t, ua.CompareTo(ua), "%s and %s must be equal", ua, ua)
}

func TestUint256_Serializable(t *testing.T) {
	a := util.Uint256{
		1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32,
	}

	var b util.Uint256
	testserdes.EncodeDecodeBinary(t, &a, &b)
}
