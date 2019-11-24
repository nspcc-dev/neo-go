package util

import (
	"encoding/hex"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUint256UnmarshalJSON(t *testing.T) {
	str := "f037308fa0ab18155bccfc08485468c112409ea5064595699e98c545f245f32d"
	expected, err := Uint256DecodeReverseString(str)
	require.NoError(t, err)

	// UnmarshalJSON decodes hex-strings
	var u1, u2 Uint256

	require.NoError(t, u1.UnmarshalJSON([]byte(`"`+str+`"`)))
	assert.True(t, expected.Equals(u1))

	s, err := expected.MarshalJSON()
	require.NoError(t, err)

	// UnmarshalJSON decodes hex-strings prefixed by 0x
	require.NoError(t, u2.UnmarshalJSON(s))
	assert.True(t, expected.Equals(u1))
}

func TestUint256DecodeString(t *testing.T) {
	hexStr := "f037308fa0ab18155bccfc08485468c112409ea5064595699e98c545f245f32d"
	val, err := Uint256DecodeReverseString(hexStr)
	require.NoError(t, err)
	assert.Equal(t, hexStr, val.ReverseString())
}

func TestUint256DecodeBytes(t *testing.T) {
	hexStr := "f037308fa0ab18155bccfc08485468c112409ea5064595699e98c545f245f32d"
	b, err := hex.DecodeString(hexStr)
	require.NoError(t, err)

	val, err := Uint256DecodeReverseBytes(b)
	require.NoError(t, err)
	assert.Equal(t, hexStr, val.ReverseString())
}

func TestUInt256Equals(t *testing.T) {
	a := "f037308fa0ab18155bccfc08485468c112409ea5064595699e98c545f245f32d"
	b := "e287c5b29a1b66092be6803c59c765308ac20287e1b4977fd399da5fc8f66ab5"

	ua, err := Uint256DecodeReverseString(a)
	require.NoError(t, err)

	ub, err := Uint256DecodeReverseString(b)
	require.NoError(t, err)
	assert.False(t, ua.Equals(ub), "%s and %s cannot be equal", ua, ub)
	assert.True(t, ua.Equals(ua), "%s and %s must be equal", ua, ua)
}

func TestUint256_Serializable(t *testing.T) {
	a := Uint256{
		1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32,
	}

	w := io.NewBufBinWriter()
	a.EncodeBinary(w.BinWriter)
	require.NoError(t, w.Err)

	var b Uint256
	r := io.NewBinReaderFromBuf(w.Bytes())
	b.DecodeBinary(r)
	require.Equal(t, a, b)
}
