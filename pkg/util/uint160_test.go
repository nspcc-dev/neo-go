package util_test

import (
	"encoding/hex"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUint160UnmarshalJSON(t *testing.T) {
	str := "0263c1de100292813b5e075e585acc1bae963b2d"
	expected, err := util.Uint160DecodeStringLE(str)
	assert.NoError(t, err)

	// UnmarshalJSON decodes hex-strings
	var u1, u2 util.Uint160

	assert.NoError(t, u1.UnmarshalJSON([]byte(`"`+str+`"`)))
	assert.True(t, expected.Equals(u1))

	testserdes.MarshalUnmarshalJSON(t, &expected, &u2)

	assert.Error(t, u2.UnmarshalJSON([]byte(`123`)))
}

func TestUInt160DecodeString(t *testing.T) {
	hexStr := "2d3b96ae1bcc5a585e075e3b81920210dec16302"
	val, err := util.Uint160DecodeStringBE(hexStr)
	assert.NoError(t, err)
	assert.Equal(t, hexStr, val.String())

	valLE, err := util.Uint160DecodeStringLE(hexStr)
	assert.NoError(t, err)
	assert.Equal(t, val, valLE.Reverse())

	_, err = util.Uint160DecodeStringBE(hexStr[1:])
	assert.Error(t, err)

	_, err = util.Uint160DecodeStringLE(hexStr[1:])
	assert.Error(t, err)

	hexStr = "zz3b96ae1bcc5a585e075e3b81920210dec16302"
	_, err = util.Uint160DecodeStringBE(hexStr)
	assert.Error(t, err)

	_, err = util.Uint160DecodeStringLE(hexStr)
	assert.Error(t, err)
}

func TestUint160DecodeBytes(t *testing.T) {
	hexStr := "2d3b96ae1bcc5a585e075e3b81920210dec16302"
	b, err := hex.DecodeString(hexStr)
	require.NoError(t, err)

	val, err := util.Uint160DecodeBytesBE(b)
	assert.NoError(t, err)
	assert.Equal(t, hexStr, val.String())

	valLE, err := util.Uint160DecodeBytesLE(b)
	assert.NoError(t, err)
	assert.Equal(t, val, valLE.Reverse())

	_, err = util.Uint160DecodeBytesLE(b[1:])
	assert.Error(t, err)

	_, err = util.Uint160DecodeBytesBE(b[1:])
	assert.Error(t, err)
}

func TestUInt160Equals(t *testing.T) {
	a := "2d3b96ae1bcc5a585e075e3b81920210dec16302"
	b := "4d3b96ae1bcc5a585e075e3b81920210dec16302"

	ua, err := util.Uint160DecodeStringBE(a)
	require.NoError(t, err)

	ub, err := util.Uint160DecodeStringBE(b)
	require.NoError(t, err)
	assert.False(t, ua.Equals(ub), "%s and %s cannot be equal", ua, ub)
	assert.True(t, ua.Equals(ua), "%s and %s must be equal", ua, ua)
}

func TestUInt160Less(t *testing.T) {
	a := "2d3b96ae1bcc5a585e075e3b81920210dec16302"
	b := "2d3b96ae1bcc5a585e075e3b81920210dec16303"

	ua, err := util.Uint160DecodeStringBE(a)
	assert.Nil(t, err)
	ua2, err := util.Uint160DecodeStringBE(a)
	assert.Nil(t, err)
	ub, err := util.Uint160DecodeStringBE(b)
	assert.Nil(t, err)
	assert.Equal(t, true, ua.Less(ub))
	assert.Equal(t, false, ua.Less(ua2))
	assert.Equal(t, false, ub.Less(ua))
}

func TestUInt160String(t *testing.T) {
	hexStr := "b28427088a3729b2536d10122960394e8be6721f"
	hexRevStr := "1f72e68b4e39602912106d53b229378a082784b2"

	val, err := util.Uint160DecodeStringBE(hexStr)
	assert.Nil(t, err)

	assert.Equal(t, hexStr, val.String())
	assert.Equal(t, hexRevStr, val.StringLE())
}

func TestUint160_Reverse(t *testing.T) {
	hexStr := "b28427088a3729b2536d10122960394e8be6721f"
	val, err := util.Uint160DecodeStringBE(hexStr)

	require.NoError(t, err)
	assert.Equal(t, hexStr, val.Reverse().StringLE())
	assert.Equal(t, val, val.Reverse().Reverse())
}
