package native

import (
	"encoding/base64"
	"encoding/hex"
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/mr-tron/base58"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	base58neogo "github.com/nspcc-dev/neo-go/pkg/encoding/base58"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStdLibItoaAtoi(t *testing.T) {
	s := newStd()
	ic := &interop.Context{VM: vm.New()}
	var actual stackitem.Item

	t.Run("itoa-atoi", func(t *testing.T) {
		var testCases = []struct {
			num    *big.Int
			base   *big.Int
			result string
		}{
			{big.NewInt(0), big.NewInt(10), "0"},
			{big.NewInt(0), big.NewInt(16), "0"},
			{big.NewInt(1), big.NewInt(10), "1"},
			{big.NewInt(-1), big.NewInt(10), "-1"},
			{big.NewInt(1), big.NewInt(16), "1"},
			{big.NewInt(7), big.NewInt(16), "7"},
			{big.NewInt(8), big.NewInt(16), "08"},
			{big.NewInt(65535), big.NewInt(16), "0FFFF"},
			{big.NewInt(15), big.NewInt(16), "0F"},
			{big.NewInt(-1), big.NewInt(16), "F"},
		}

		for _, tc := range testCases {
			require.NotPanics(t, func() {
				actual = s.itoa(ic, []stackitem.Item{stackitem.Make(tc.num), stackitem.Make(tc.base)})
			})
			require.Equal(t, stackitem.Make(tc.result), actual)

			require.NotPanics(t, func() {
				actual = s.atoi(ic, []stackitem.Item{stackitem.Make(tc.result), stackitem.Make(tc.base)})
			})
			require.Equal(t, stackitem.Make(tc.num), actual)

			if tc.base.Int64() == 10 {
				require.NotPanics(t, func() {
					actual = s.itoa10(ic, []stackitem.Item{stackitem.Make(tc.num)})
				})
				require.Equal(t, stackitem.Make(tc.result), actual)

				require.NotPanics(t, func() {
					actual = s.atoi10(ic, []stackitem.Item{stackitem.Make(tc.result)})
				})
				require.Equal(t, stackitem.Make(tc.num), actual)
			}
		}

		t.Run("-1", func(t *testing.T) {
			for _, str := range []string{"FF", "FFF", "FFFF"} {
				require.NotPanics(t, func() {
					actual = s.atoi(ic, []stackitem.Item{stackitem.Make(str), stackitem.Make(16)})
				})

				require.Equal(t, stackitem.Make(-1), actual)
			}
		})
	})

	t.Run("itoa error", func(t *testing.T) {
		var testCases = []struct {
			num  *big.Int
			base *big.Int
			err  error
		}{
			{big.NewInt(1), big.NewInt(13), ErrInvalidBase},
			{big.NewInt(-1), new(big.Int).Add(big.NewInt(math.MaxInt64), big.NewInt(10)), ErrInvalidBase},
		}

		for _, tc := range testCases {
			require.PanicsWithError(t, tc.err.Error(), func() {
				_ = s.itoa(ic, []stackitem.Item{stackitem.Make(tc.num), stackitem.Make(tc.base)})
			})
		}
	})

	t.Run("atoi error", func(t *testing.T) {
		var testCases = []struct {
			num  string
			base *big.Int
			err  error
		}{
			{"1", big.NewInt(13), ErrInvalidBase},
			{"1", new(big.Int).Add(big.NewInt(math.MaxInt64), big.NewInt(16)), ErrInvalidBase},
			{"1_000", big.NewInt(10), ErrInvalidFormat},
			{"FE", big.NewInt(10), ErrInvalidFormat},
			{"XD", big.NewInt(16), ErrInvalidFormat},
			{strings.Repeat("0", stdMaxInputLength+1), big.NewInt(10), ErrTooBigInput},
		}

		for _, tc := range testCases {
			require.PanicsWithError(t, tc.err.Error(), func() {
				_ = s.atoi(ic, []stackitem.Item{stackitem.Make(tc.num), stackitem.Make(tc.base)})
			})
		}
	})
}

func TestStdLibJSON(t *testing.T) {
	s := newStd()
	ic := &interop.Context{VM: vm.New()}
	var actual stackitem.Item

	t.Run("JSONSerialize", func(t *testing.T) {
		t.Run("Good", func(t *testing.T) {
			require.NotPanics(t, func() {
				actual = s.jsonSerialize(ic, []stackitem.Item{stackitem.Make(42)})
			})

			require.Equal(t, stackitem.Make([]byte("42")), actual)
		})

		t.Run("Bad", func(t *testing.T) {
			arr := stackitem.NewArray([]stackitem.Item{
				stackitem.NewByteArray(make([]byte, stackitem.MaxSize/2)),
				stackitem.NewByteArray(make([]byte, stackitem.MaxSize/2)),
			})
			require.Panics(t, func() {
				_ = s.jsonSerialize(ic, []stackitem.Item{arr})
			})
		})
	})

	t.Run("JSONDeserialize", func(t *testing.T) {
		t.Run("Good", func(t *testing.T) {
			require.NotPanics(t, func() {
				actual = s.jsonDeserialize(ic, []stackitem.Item{stackitem.Make("42")})
			})

			require.Equal(t, stackitem.Make(42), actual)
		})
		t.Run("Bad", func(t *testing.T) {
			require.Panics(t, func() {
				_ = s.jsonDeserialize(ic, []stackitem.Item{stackitem.Make("{]")})
			})
			require.Panics(t, func() {
				_ = s.jsonDeserialize(ic, []stackitem.Item{stackitem.NewInterop(nil)})
			})
		})
	})
}

func TestStdLibEncodeDecode(t *testing.T) {
	s := newStd()
	original := []byte("my pretty string")
	encoded64 := base64.StdEncoding.EncodeToString(original)
	encoded58 := base58.Encode(original)
	encoded58Check := base58neogo.CheckEncode(original)
	ic := &interop.Context{VM: vm.New()}
	var actual stackitem.Item

	bigInputArgs := []stackitem.Item{stackitem.Make(strings.Repeat("6", stdMaxInputLength+1))}

	t.Run("Encode64", func(t *testing.T) {
		require.NotPanics(t, func() {
			actual = s.base64Encode(ic, []stackitem.Item{stackitem.Make(original)})
		})
		require.Equal(t, stackitem.Make(encoded64), actual)
	})
	t.Run("Encode64/error", func(t *testing.T) {
		require.PanicsWithError(t, ErrTooBigInput.Error(),
			func() { s.base64Encode(ic, bigInputArgs) })
	})
	t.Run("Encode58", func(t *testing.T) {
		require.NotPanics(t, func() {
			actual = s.base58Encode(ic, []stackitem.Item{stackitem.Make(original)})
		})
		require.Equal(t, stackitem.Make(encoded58), actual)
	})
	t.Run("Encode58/error", func(t *testing.T) {
		require.PanicsWithError(t, ErrTooBigInput.Error(),
			func() { s.base58Encode(ic, bigInputArgs) })
	})
	t.Run("CheckEncode58", func(t *testing.T) {
		require.NotPanics(t, func() {
			actual = s.base58CheckEncode(ic, []stackitem.Item{stackitem.Make(original)})
		})
		require.Equal(t, stackitem.Make(encoded58Check), actual)
	})
	t.Run("CheckEncode58/error", func(t *testing.T) {
		require.PanicsWithError(t, ErrTooBigInput.Error(),
			func() { s.base58CheckEncode(ic, bigInputArgs) })
	})
	t.Run("Decode64/positive", func(t *testing.T) {
		require.NotPanics(t, func() {
			actual = s.base64Decode(ic, []stackitem.Item{stackitem.Make(encoded64)})
		})
		require.Equal(t, stackitem.Make(original), actual)
	})
	t.Run("Decode64/error", func(t *testing.T) {
		require.Panics(t, func() {
			_ = s.base64Decode(ic, []stackitem.Item{stackitem.Make(encoded64 + "%")})
		})
		require.Panics(t, func() {
			_ = s.base64Decode(ic, []stackitem.Item{stackitem.NewInterop(nil)})
		})
		require.PanicsWithError(t, ErrTooBigInput.Error(),
			func() { s.base64Decode(ic, bigInputArgs) })
	})
	t.Run("Decode58/positive", func(t *testing.T) {
		require.NotPanics(t, func() {
			actual = s.base58Decode(ic, []stackitem.Item{stackitem.Make(encoded58)})
		})
		require.Equal(t, stackitem.Make(original), actual)
	})
	t.Run("Decode58/error", func(t *testing.T) {
		require.Panics(t, func() {
			_ = s.base58Decode(ic, []stackitem.Item{stackitem.Make(encoded58 + "%")})
		})
		require.Panics(t, func() {
			_ = s.base58Decode(ic, []stackitem.Item{stackitem.NewInterop(nil)})
		})
		require.PanicsWithError(t, ErrTooBigInput.Error(),
			func() { s.base58Decode(ic, bigInputArgs) })
	})
	t.Run("CheckDecode58/positive", func(t *testing.T) {
		require.NotPanics(t, func() {
			actual = s.base58CheckDecode(ic, []stackitem.Item{stackitem.Make(encoded58Check)})
		})
		require.Equal(t, stackitem.Make(original), actual)
	})
	t.Run("CheckDecode58/error", func(t *testing.T) {
		require.Panics(t, func() {
			_ = s.base58CheckDecode(ic, []stackitem.Item{stackitem.Make(encoded58 + "%")})
		})
		require.Panics(t, func() {
			_ = s.base58CheckDecode(ic, []stackitem.Item{stackitem.NewInterop(nil)})
		})
		require.PanicsWithError(t, ErrTooBigInput.Error(),
			func() { s.base58CheckDecode(ic, bigInputArgs) })
	})
}

func TestStdLibSerialize(t *testing.T) {
	s := newStd()
	ic := &interop.Context{VM: vm.New()}

	t.Run("recursive", func(t *testing.T) {
		arr := stackitem.NewArray(nil)
		arr.Append(arr)
		require.Panics(t, func() {
			_ = s.serialize(ic, []stackitem.Item{arr})
		})
	})
	t.Run("big item", func(t *testing.T) {
		require.Panics(t, func() {
			_ = s.serialize(ic, []stackitem.Item{stackitem.NewByteArray(make([]byte, stackitem.MaxSize))})
		})
	})
	t.Run("good", func(t *testing.T) {
		var (
			actualSerialized   stackitem.Item
			actualDeserialized stackitem.Item
		)
		require.NotPanics(t, func() {
			actualSerialized = s.serialize(ic, []stackitem.Item{stackitem.Make(42)})
		})

		encoded, err := stackitem.Serialize(stackitem.Make(42))
		require.NoError(t, err)
		require.Equal(t, stackitem.Make(encoded), actualSerialized)

		require.NotPanics(t, func() {
			actualDeserialized = s.deserialize(ic, []stackitem.Item{actualSerialized})
		})
		require.Equal(t, stackitem.Make(42), actualDeserialized)

		t.Run("bad", func(t *testing.T) {
			encoded[0] ^= 0xFF
			require.Panics(t, func() {
				_ = s.deserialize(ic, []stackitem.Item{stackitem.Make(encoded)})
			})
		})
	})
}

func TestStdLibSerializeDeserialize(t *testing.T) {
	s := newStd()
	ic := &interop.Context{VM: vm.New()}
	var actual stackitem.Item

	checkSerializeDeserialize := func(t *testing.T, value interface{}, expected stackitem.Item) {
		require.NotPanics(t, func() {
			actual = s.serialize(ic, []stackitem.Item{stackitem.Make(value)})
		})
		require.NotPanics(t, func() {
			actual = s.deserialize(ic, []stackitem.Item{actual})
		})
		require.Equal(t, expected, actual)
	}

	t.Run("Bool", func(t *testing.T) {
		checkSerializeDeserialize(t, true, stackitem.NewBool(true))
	})
	t.Run("ByteArray", func(t *testing.T) {
		checkSerializeDeserialize(t, []byte{1, 2, 3}, stackitem.NewByteArray([]byte{1, 2, 3}))
	})
	t.Run("Integer", func(t *testing.T) {
		checkSerializeDeserialize(t, 48, stackitem.NewBigInteger(big.NewInt(48)))
	})
	t.Run("Array", func(t *testing.T) {
		arr := stackitem.NewArray([]stackitem.Item{
			stackitem.Make(true),
			stackitem.Make(123),
			stackitem.NewMap()})
		checkSerializeDeserialize(t, arr, arr)
	})
	t.Run("Struct", func(t *testing.T) {
		st := stackitem.NewStruct([]stackitem.Item{
			stackitem.Make(true),
			stackitem.Make(123),
			stackitem.NewMap(),
		})
		checkSerializeDeserialize(t, st, st)
	})
	t.Run("Map", func(t *testing.T) {
		item := stackitem.NewMap()
		item.Add(stackitem.Make(true), stackitem.Make([]byte{1, 2, 3}))
		item.Add(stackitem.Make([]byte{0}), stackitem.Make(false))
		checkSerializeDeserialize(t, item, item)
	})
	t.Run("Serialize MapCompat", func(t *testing.T) {
		resHex := "480128036b6579280576616c7565"
		res, err := hex.DecodeString(resHex)
		require.NoError(t, err)

		item := stackitem.NewMap()
		item.Add(stackitem.Make([]byte("key")), stackitem.Make([]byte("value")))
		require.NotPanics(t, func() {
			actual = s.serialize(ic, []stackitem.Item{stackitem.Make(item)})
		})
		bytes, err := actual.TryBytes()
		require.NoError(t, err)
		assert.Equal(t, res, bytes)
	})
	t.Run("Serialize Interop", func(t *testing.T) {
		require.Panics(t, func() {
			actual = s.serialize(ic, []stackitem.Item{stackitem.NewInterop("kek")})
		})
	})
	t.Run("Serialize Array bad", func(t *testing.T) {
		item := stackitem.NewArray([]stackitem.Item{stackitem.NewBool(true), stackitem.NewBool(true)})
		item.Value().([]stackitem.Item)[1] = item
		require.Panics(t, func() {
			actual = s.serialize(ic, []stackitem.Item{item})
		})
	})
	t.Run("Deserialize unknown", func(t *testing.T) {
		data, err := stackitem.Serialize(stackitem.NewBigInteger(big.NewInt(123)))
		require.NoError(t, err)

		data[0] = 0xFF
		require.Panics(t, func() {
			actual = s.deserialize(ic, []stackitem.Item{stackitem.Make(data)})
		})
	})
	t.Run("Deserialize not a byte array", func(t *testing.T) {
		require.Panics(t, func() {
			actual = s.deserialize(ic, []stackitem.Item{stackitem.NewInterop(nil)})
		})
	})
}

func TestMemoryCompare(t *testing.T) {
	s := newStd()
	ic := &interop.Context{VM: vm.New()}

	check := func(t *testing.T, result int64, s1, s2 string) {
		actual := s.memoryCompare(ic, []stackitem.Item{stackitem.Make(s1), stackitem.Make(s2)})
		require.Equal(t, big.NewInt(result), actual.Value())
	}

	check(t, -1, "a", "ab")
	check(t, 1, "ab", "a")
	check(t, 0, "ab", "ab")
	check(t, -1, "", "a")
	check(t, 0, "", "")

	t.Run("C# compatibility", func(t *testing.T) {
		// These tests are taken from C# node.
		check(t, -1, "abc", "c")
		check(t, -1, "abc", "d")
		check(t, 0, "abc", "abc")
		check(t, -1, "abc", "abcd")
	})

	t.Run("big arguments", func(t *testing.T) {
		s1 := stackitem.Make(strings.Repeat("x", stdMaxInputLength+1))
		s2 := stackitem.Make("xxx")

		require.PanicsWithError(t, ErrTooBigInput.Error(),
			func() { s.memoryCompare(ic, []stackitem.Item{s1, s2}) })

		require.PanicsWithError(t, ErrTooBigInput.Error(),
			func() { s.memoryCompare(ic, []stackitem.Item{s2, s1}) })
	})
}

func TestMemorySearch(t *testing.T) {
	s := newStd()
	ic := &interop.Context{VM: vm.New()}

	check := func(t *testing.T, result int64, args ...interface{}) {
		items := make([]stackitem.Item, len(args))
		for i := range args {
			items[i] = stackitem.Make(args[i])
		}

		var actual stackitem.Item
		switch len(items) {
		case 2:
			actual = s.memorySearch2(ic, items)
		case 3:
			actual = s.memorySearch3(ic, items)
		case 4:
			actual = s.memorySearch4(ic, items)
		default:
			panic("invalid args length")
		}
		require.Equal(t, big.NewInt(result), actual.Value())
	}

	t.Run("C# compatibility", func(t *testing.T) {
		// These tests are taken from C# node.
		check(t, 2, "abc", "c", 0)
		check(t, 2, "abc", "c", 1)
		check(t, 2, "abc", "c", 2)
		check(t, -1, "abc", "c", 3)
		check(t, -1, "abc", "d", 0)

		check(t, 2, "abc", "c", 0, false)
		check(t, 2, "abc", "c", 1, false)
		check(t, 2, "abc", "c", 2, false)
		check(t, -1, "abc", "c", 3, false)
		check(t, -1, "abc", "d", 0, false)

		check(t, -1, "abc", "c", 0, true)
		check(t, -1, "abc", "c", 1, true)
		check(t, -1, "abc", "c", 2, true)
		check(t, 2, "abc", "c", 3, true)
		check(t, -1, "abc", "d", 0, true)
	})

	t.Run("boundary indices", func(t *testing.T) {
		arg := stackitem.Make("aaa")
		require.Panics(t, func() {
			s.memorySearch3(ic, []stackitem.Item{arg, arg, stackitem.Make(-1)})
		})
		require.Panics(t, func() {
			s.memorySearch3(ic, []stackitem.Item{arg, arg, stackitem.Make(4)})
		})
		t.Run("still in capacity", func(t *testing.T) {
			require.Panics(t, func() {
				arr := stackitem.NewByteArray(make([]byte, 5, 10))
				s.memorySearch3(ic, []stackitem.Item{arr, arg, stackitem.Make(7)})
			})
			require.Panics(t, func() {
				arr := stackitem.NewByteArray(make([]byte, 5, 10))
				s.memorySearch4(ic, []stackitem.Item{arr, arg,
					stackitem.Make(7), stackitem.Make(true)})
			})
		})
	})

	t.Run("big arguments", func(t *testing.T) {
		s1 := stackitem.Make(strings.Repeat("x", stdMaxInputLength+1))
		s2 := stackitem.Make("xxx")
		start := stackitem.Make(1)
		b := stackitem.Make(true)

		require.PanicsWithError(t, ErrTooBigInput.Error(),
			func() { s.memorySearch2(ic, []stackitem.Item{s1, s2}) })

		require.PanicsWithError(t, ErrTooBigInput.Error(),
			func() { s.memorySearch2(ic, []stackitem.Item{s2, s1}) })

		require.PanicsWithError(t, ErrTooBigInput.Error(),
			func() { s.memorySearch3(ic, []stackitem.Item{s1, s2, start}) })

		require.PanicsWithError(t, ErrTooBigInput.Error(),
			func() { s.memorySearch3(ic, []stackitem.Item{s2, s1, start}) })

		require.PanicsWithError(t, ErrTooBigInput.Error(),
			func() { s.memorySearch4(ic, []stackitem.Item{s1, s2, start, b}) })

		require.PanicsWithError(t, ErrTooBigInput.Error(),
			func() { s.memorySearch4(ic, []stackitem.Item{s2, s1, start, b}) })
	})
}

func TestStringSplit(t *testing.T) {
	s := newStd()
	ic := &interop.Context{VM: vm.New()}

	check := func(t *testing.T, result []string, str, sep string, remove interface{}) {
		args := []stackitem.Item{stackitem.Make(str), stackitem.Make(sep)}
		var actual stackitem.Item
		if remove == nil {
			actual = s.stringSplit2(ic, args)
		} else {
			args = append(args, stackitem.NewBool(remove.(bool)))
			actual = s.stringSplit3(ic, args)
		}

		arr, ok := actual.Value().([]stackitem.Item)
		require.True(t, ok)
		require.Equal(t, len(result), len(arr))
		for i := range result {
			require.Equal(t, stackitem.Make(result[i]), arr[i])
		}
	}

	check(t, []string{"a", "b", "c"}, "abc", "", nil)
	check(t, []string{"a", "b", "c"}, "abc", "", true)
	check(t, []string{"a", "c", "", "", "d"}, "abcbbbd", "b", nil)
	check(t, []string{"a", "c", "", "", "d"}, "abcbbbd", "b", false)
	check(t, []string{"a", "c", "d"}, "abcbbbd", "b", true)
	check(t, []string{""}, "", "abc", nil)
	check(t, []string{}, "", "abc", true)

	t.Run("C# compatibility", func(t *testing.T) {
		// These tests are taken from C# node.
		check(t, []string{"a", "b"}, "a,b", ",", nil)
	})

	t.Run("big arguments", func(t *testing.T) {
		s1 := stackitem.Make(strings.Repeat("x", stdMaxInputLength+1))
		s2 := stackitem.Make("xxx")

		require.PanicsWithError(t, ErrTooBigInput.Error(),
			func() { s.stringSplit2(ic, []stackitem.Item{s1, s2}) })
	})
}
