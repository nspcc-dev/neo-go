package io_test

import (
	"fmt"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
)

// Mock structure to test getting size of an array of serializable things.
type smthSerializable struct {
	some [42]byte
}

func (*smthSerializable) DecodeBinary(*io.BinReader) {}

func (ss *smthSerializable) EncodeBinary(bw io.BinaryWriter) {
	bw.WriteBytes(ss.some[:])
}

// Mock structure that gives error in EncodeBinary().
type smthNotReallySerializable struct{}

func (*smthNotReallySerializable) DecodeBinary(*io.BinReader) {}

func (*smthNotReallySerializable) EncodeBinary(bw io.BinaryWriter) {
	bw.SetError(fmt.Errorf("smth bad happened in smthNotReallySerializable"))
}

func TestVarSize(t *testing.T) {
	testCases := []struct {
		variable interface{}
		name     string
		expected int
	}{
		{
			252,
			"test_int_1",
			1,
		},
		{
			253,
			"test_int_2",
			3,
		},
		{
			65535,
			"test_int_3",
			3,
		},
		{
			65536,
			"test_int_4",
			5,
		},
		{
			4294967295,
			"test_int_5",
			5,
		},
		{
			uint(252),
			"test_uint_1",
			1,
		},
		{
			uint(253),
			"test_uint_2",
			3,
		},
		{
			uint(65535),
			"test_uint_3",
			3,
		},
		{
			uint(65536),
			"test_uint_4",
			5,
		},
		{
			uint(4294967295),
			"test_uint_5",
			5,
		},
		{
			[]byte{1, 2, 4, 5, 6},
			"test_[]byte_1",
			6,
		},
		{
			// The neo C# implementation doe not allowed this!
			util.Uint160{1, 2, 4, 5, 6},
			"test_Uint160_1",
			21,
		},

		{[20]uint8{1, 2, 3, 4, 5, 6},
			"test_uint8_1",
			21,
		},
		{[20]uint8{1, 2, 3, 4, 5, 6, 8, 9},
			"test_uint8_2",
			21,
		},

		{[32]uint8{1, 2, 3, 4, 5, 6},
			"test_uint8_3",
			33,
		},
		{[10]uint16{1, 2, 3, 4, 5, 6},
			"test_uint16_1",
			21,
		},

		{[10]uint16{1, 2, 3, 4, 5, 6, 10, 21},
			"test_uint16_2",
			21,
		},
		{[30]uint32{1, 2, 3, 4, 5, 6, 10, 21},
			"test_uint32_2",
			121,
		},
		{[30]uint64{1, 2, 3, 4, 5, 6, 10, 21},
			"test_uint64_2",
			241,
		},
		{[20]int8{1, 2, 3, 4, 5, 6},
			"test_int8_1",
			21,
		},
		{[20]int8{-1, 2, 3, 4, 5, 6, 8, 9},
			"test_int8_2",
			21,
		},

		{[32]int8{-1, 2, 3, 4, 5, 6},
			"test_int8_3",
			33,
		},
		{[10]int16{-1, 2, 3, 4, 5, 6},
			"test_int16_1",
			21,
		},

		{[10]int16{-1, 2, 3, 4, 5, 6, 10, 21},
			"test_int16_2",
			21,
		},
		{[30]int32{-1, 2, 3, 4, 5, 6, 10, 21},
			"test_int32_2",
			121,
		},
		{[30]int64{-1, 2, 3, 4, 5, 6, 10, 21},
			"test_int64_2",
			241,
		},
		// The neo C# implementation doe not allowed this!
		{util.Uint256{1, 2, 3, 4, 5, 6},
			"test_Uint256_1",
			33,
		},

		{"abc",
			"test_string_1",
			4,
		},
		{"abcà",
			"test_string_2",
			6,
		},
		{"2d3b96ae1bcc5a585e075e3b81920210dec16302",
			"test_string_3",
			41,
		},
		{[]*smthSerializable{{}, {}},
			"test_Serializable",
			2*42 + 1,
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("run: %s", tc.name), func(t *testing.T) {
			result := io.GetVarSize(tc.variable)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func panicVarSize(t *testing.T, v interface{}) {
	defer func() {
		r := recover()
		assert.NotNil(t, r)
	}()

	_ = io.GetVarSize(v)
	// this should never execute
	assert.Nil(t, t)
}

func TestVarSizePanic(t *testing.T) {
	panicVarSize(t, t)
	panicVarSize(t, struct{}{})
	panicVarSize(t, &smthNotReallySerializable{})
}
