package util

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUint160UnmarshalJSON(t *testing.T) {
	str := "2d3b96ae1bcc5a585e075e3b81920210dec16302"
	expected, err := Uint160DecodeString(str)
	assert.NoError(t, err)

	// UnmarshalJSON decodes hex-strings
	var u1, u2 Uint160

	assert.NoError(t, u1.UnmarshalJSON([]byte(`"`+str+`"`)))
	assert.True(t, expected.Equals(u1))

	s, err := expected.MarshalJSON()
	require.NoError(t, err)

	// UnmarshalJSON decodes hex-strings prefixed by 0x
	assert.NoError(t, u2.UnmarshalJSON(s))
	assert.True(t, expected.Equals(u1))
}

func TestUInt160DecodeString(t *testing.T) {
	hexStr := "2d3b96ae1bcc5a585e075e3b81920210dec16302"
	val, err := Uint160DecodeString(hexStr)
	assert.NoError(t, err)
	assert.Equal(t, hexStr, val.String())
}

func TestUint160DecodeBytes(t *testing.T) {
	hexStr := "2d3b96ae1bcc5a585e075e3b81920210dec16302"
	b, err := hex.DecodeString(hexStr)
	require.NoError(t, err)

	val, err := Uint160DecodeBytes(b)
	assert.NoError(t, err)
	assert.Equal(t, hexStr, val.String())
}

func TestUInt160Equals(t *testing.T) {
	a := "2d3b96ae1bcc5a585e075e3b81920210dec16302"
	b := "4d3b96ae1bcc5a585e075e3b81920210dec16302"

	ua, err := Uint160DecodeString(a)
	require.NoError(t, err)

	ub, err := Uint160DecodeString(b)
	require.NoError(t, err)
	assert.False(t, ua.Equals(ub), "%s and %s cannot be equal", ua, ub)
	assert.True(t, ua.Equals(ua), "%s and %s must be equal", ua, ua)
}

func TestUInt160Less(t *testing.T) {
	a := "2d3b96ae1bcc5a585e075e3b81920210dec16302"
	b := "2d3b96ae1bcc5a585e075e3b81920210dec16303"

	ua, err := Uint160DecodeString(a)
	assert.Nil(t, err)
	ua2, err := Uint160DecodeString(a)
	assert.Nil(t, err)
	ub, err := Uint160DecodeString(b)
	assert.Nil(t, err)
	assert.Equal(t, true, ua.Less(ub))
	assert.Equal(t, false, ua.Less(ua2))
	assert.Equal(t, false, ub.Less(ua))
}

func TestUInt160String(t *testing.T) {
	hexStr := "b28427088a3729b2536d10122960394e8be6721f"
	hexRevStr := "1f72e68b4e39602912106d53b229378a082784b2"

	val, err := Uint160DecodeString(hexStr)
	assert.Nil(t, err)

	assert.Equal(t, hexStr, val.String())
	assert.Equal(t, hexRevStr, val.ReverseString())
}
