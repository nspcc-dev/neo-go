package stackitem

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getTestDecodeFunc(js string, expected ...any) func(t *testing.T) {
	return getTestDecodeEncodeFunc(js, true, expected...)
}

func getTestDecodeEncodeFunc(js string, needEncode bool, expected ...any) func(t *testing.T) {
	return func(t *testing.T) {
		actual, err := FromJSON([]byte(js), 20, true)
		if expected[0] == nil {
			require.Error(t, err)
			return
		}
		require.NoError(t, err)
		require.Equal(t, Make(expected[0]), actual)

		if needEncode && len(expected) == 1 {
			encoded, err := ToJSON(actual)
			require.NoError(t, err)
			require.Equal(t, js, string(encoded))
		}
	}
}

func TestFromToJSON(t *testing.T) {
	bigInt, ok := new(big.Int).SetString("28000000000000000000000", 10)
	require.True(t, ok)
	t.Run("ByteString", func(t *testing.T) {
		t.Run("Empty", getTestDecodeFunc(`""`, []byte{}))
		t.Run("Base64", getTestDecodeFunc(`"test"`, "test"))
		t.Run("Escape", getTestDecodeFunc(`"\"quotes\""`, `"quotes"`))
	})
	t.Run("BigInteger", func(t *testing.T) {
		t.Run("ZeroFloat", getTestDecodeFunc(`12.000`, 12, nil))
		t.Run("NonZeroFloat", getTestDecodeFunc(`12.01`, nil))
		t.Run("ExpInteger", getTestDecodeEncodeFunc(`2.8e+22`, false, bigInt))
		t.Run("ExpFloat", getTestDecodeEncodeFunc(`1.2345e+3`, false, nil)) // float value, parsing should fail for it.
		t.Run("Negative", getTestDecodeFunc(`-4`, -4))
		t.Run("Positive", getTestDecodeFunc(`123`, 123))
	})
	t.Run("Bool", func(t *testing.T) {
		t.Run("True", getTestDecodeFunc(`true`, true))
		t.Run("False", getTestDecodeFunc(`false`, false))
	})
	t.Run("Null", getTestDecodeFunc(`null`, Null{}))
	t.Run("Array", func(t *testing.T) {
		t.Run("Empty", getTestDecodeFunc(`[]`, NewArray([]Item{})))
		t.Run("Simple", getTestDecodeFunc((`[1,"test",true,null]`),
			NewArray([]Item{NewBigInteger(big.NewInt(1)), NewByteArray([]byte("test")), NewBool(true), Null{}})))
		t.Run("Nested", getTestDecodeFunc(`[[],[{},null]]`,
			NewArray([]Item{NewArray([]Item{}), NewArray([]Item{NewMap(), Null{}})})))
		t.Run("ManyElements", func(t *testing.T) {
			js := `[1, 2, 3]` // 3 elements + array itself
			_, err := FromJSON([]byte(js), 4, true)
			require.NoError(t, err)

			_, err = FromJSON([]byte(js), 3, true)
			require.ErrorIs(t, err, errTooBigElements)
		})
	})
	t.Run("Map", func(t *testing.T) {
		small := NewMap()
		small.Add(NewByteArray([]byte("a")), NewBigInteger(big.NewInt(3)))
		large := NewMap()
		large.Add(NewByteArray([]byte("3")), small)
		large.Add(NewByteArray([]byte("arr")), NewArray([]Item{NewByteArray([]byte("test"))}))
		t.Run("Empty", getTestDecodeFunc(`{}`, NewMap()))
		t.Run("Small", getTestDecodeFunc(`{"a":3}`, small))
		t.Run("Big", getTestDecodeFunc(`{"3":{"a":3},"arr":["test"]}`, large))

		m := NewMap()
		m.Add(NewByteArray([]byte("\t")), NewBool(true))
		t.Run("escape keys", getTestDecodeFunc(`{"\t":true}`, m))

		t.Run("ManyElements", func(t *testing.T) {
			js := `{"a":1,"b":3}` // 4 elements + map itself
			_, err := FromJSON([]byte(js), 5, true)
			require.NoError(t, err)

			_, err = FromJSON([]byte(js), 4, true)
			require.ErrorIs(t, err, errTooBigElements)
		})
	})
	t.Run("Invalid", func(t *testing.T) {
		t.Run("Empty", getTestDecodeFunc(``, nil))
		t.Run("InvalidArray", getTestDecodeFunc(`[}`, nil))
		t.Run("InvalidMap", getTestDecodeFunc(`{]`, nil))
		t.Run("InvalidMapValue", getTestDecodeFunc(`{"a":{]}`, nil))
		t.Run("AfterArray", getTestDecodeFunc(`[]XX`, nil))
		t.Run("EncodeBigInteger", func(t *testing.T) {
			item := NewBigInteger(big.NewInt(MaxAllowedInteger + 1))
			_, err := ToJSON(item)
			require.Error(t, err)
		})
		t.Run("EncodeInvalidItemType", func(t *testing.T) {
			item := NewPointer(1, []byte{1, 2, 3})
			_, err := ToJSON(item)
			require.Error(t, err)
		})
		t.Run("BigByteArray", func(t *testing.T) {
			item := NewByteArray(make([]byte, MaxSize))
			_, err := ToJSON(item)
			require.Error(t, err)
		})
		t.Run("BigNestedArray", getTestDecodeFunc(`[[[[[[[[[[[]]]]]]]]]]]`, nil))
		t.Run("EncodeRecursive", func(t *testing.T) {
			// add this item to speed up test a bit
			item := NewByteArray(make([]byte, MaxKeySize))
			t.Run("Array", func(t *testing.T) {
				arr := NewArray([]Item{item})
				arr.Append(arr)
				_, err := ToJSON(arr)
				require.Error(t, err)
			})
			t.Run("Map", func(t *testing.T) {
				m := NewMap()
				m.Add(item, m)
				_, err := ToJSON(m)
				require.Error(t, err)
			})
		})
	})
}

// TestFromJSON_CompatBigInt ensures that maximum BigInt parsing precision matches
// the C# one, ref. https://github.com/neo-project/neo/issues/2879.
func TestFromJSON_CompatBigInt(t *testing.T) {
	tcs := map[string]struct {
		bestPrec   string
		compatPrec string
	}{
		`9.05e+28`: {
			bestPrec:   "90500000000000000000000000000",
			compatPrec: "90499999999999993918259200000",
		},
		`1.871e+21`: {
			bestPrec:   "1871000000000000000000",
			compatPrec: "1871000000000000000000",
		},
		`3.0366e+32`: {
			bestPrec:   "303660000000000000000000000000000",
			compatPrec: "303660000000000004445016810323968",
		},
		`1e+30`: {
			bestPrec:   "1000000000000000000000000000000",
			compatPrec: "1000000000000000019884624838656",
		},
	}
	for in, expected := range tcs {
		t.Run(in, func(t *testing.T) {
			// Best precision.
			actual, err := FromJSON([]byte(in), 5, true)
			require.NoError(t, err)
			require.Equal(t, expected.bestPrec, actual.Value().(*big.Int).String())

			// Compatible precision.
			actual, err = FromJSON([]byte(in), 5, false)
			require.NoError(t, err)
			require.Equal(t, expected.compatPrec, actual.Value().(*big.Int).String())
		})
	}
}

func testToJSON(t *testing.T, expectedErr error, item Item) {
	data, err := ToJSON(item)
	if expectedErr != nil {
		require.ErrorIs(t, err, expectedErr)
		return
	}
	require.NoError(t, err)

	actual, err := FromJSON(data, 1024, true)
	require.NoError(t, err)
	require.Equal(t, item, actual)
}

func TestToJSONCornerCases(t *testing.T) {
	// base64 encoding increases size by a factor of ~256/64 = 4
	const maxSize = MaxSize / 4

	bigByteArray := NewByteArray(make([]byte, maxSize/2))
	smallByteArray := NewByteArray(make([]byte, maxSize/4))
	t.Run("Array", func(t *testing.T) {
		arr := NewArray([]Item{bigByteArray})
		testToJSON(t, ErrTooBig, NewArray([]Item{arr, arr}))

		arr.value[0] = smallByteArray
		testToJSON(t, nil, NewArray([]Item{arr, arr}))
	})
	t.Run("big ByteArray", func(t *testing.T) {
		testToJSON(t, ErrTooBig, NewByteArray(make([]byte, maxSize+4)))
	})
	t.Run("invalid Map key", func(t *testing.T) {
		m := NewMap()
		m.Add(Make([]byte{0xe9}), Make(true))
		testToJSON(t, ErrInvalidValue, m)
	})
	t.Run("circular reference", func(t *testing.T) {
		m := NewMap()
		m.Add(Make("one"), Make(true))

		// No circular reference, ensure it can be properly serialized.
		arr := NewArray([]Item{m, m})
		testToJSON(t, nil, arr)

		// With circular reference, error expected.
		m.Add(Make("two"), arr)
		testToJSON(t, ErrTooBig, arr)
	})
}

// getBigArray returns array takes up a lot of storage when serialized.
func getBigArray(depth int) *Array {
	arr := NewArray([]Item{})
	for range depth {
		arr = NewArray([]Item{arr, arr})
	}
	return arr
}

func BenchmarkToJSON(b *testing.B) {
	arr := getBigArray(15)

	b.ReportAllocs()
	for b.Loop() {
		_, err := ToJSON(arr)
		if err != nil {
			b.FailNow()
		}
	}
}

// This test is taken from the C# code
// https://github.com/neo-project/neo/blob/master/tests/neo.UnitTests/VM/UT_Helper.cs#L30
func TestToJSONWithTypeCompat(t *testing.T) {
	items := []Item{
		Make(5), Make("hello world"),
		Make([]byte{1, 2, 3}), Make(true),
	}

	// Note: we use `Equal` and not `JSONEq` because there are no spaces and maps so the order is well-defined.
	s, err := ToJSONWithTypes(items[0])
	assert.NoError(t, err)
	assert.Equal(t, `{"type":"Integer","value":"5"}`, string(s))

	s, err = ToJSONWithTypes(items[1])
	assert.NoError(t, err)
	assert.Equal(t, `{"type":"ByteString","value":"aGVsbG8gd29ybGQ="}`, string(s))

	s, err = ToJSONWithTypes(items[2])
	assert.NoError(t, err)
	assert.Equal(t, `{"type":"ByteString","value":"AQID"}`, string(s))

	s, err = ToJSONWithTypes(items[3])
	assert.NoError(t, err)
	assert.Equal(t, `{"type":"Boolean","value":true}`, string(s))

	s, err = ToJSONWithTypes(NewArray(items))
	assert.NoError(t, err)
	assert.Equal(t, `{"type":"Array","value":[{"type":"Integer","value":"5"},{"type":"ByteString","value":"aGVsbG8gd29ybGQ="},{"type":"ByteString","value":"AQID"},{"type":"Boolean","value":true}]}`, string(s))

	item := NewMap()
	item.Add(Make(1), NewPointer(0, []byte{0}))
	s, err = ToJSONWithTypes(item)
	assert.NoError(t, err)
	assert.Equal(t, `{"type":"Map","value":[{"key":{"type":"Integer","value":"1"},"value":{"type":"Pointer","value":0}}]}`, string(s))
}

func TestToJSONWithTypes(t *testing.T) {
	testCases := []struct {
		name   string
		item   Item
		result string
	}{
		{"Null", Null{}, `{"type":"Any"}`},
		{"Integer", NewBigInteger(big.NewInt(42)), `{"type":"Integer","value":"42"}`},
		{"ByteString", NewByteArray([]byte{1, 2, 3}), `{"type":"ByteString","value":"AQID"}`},
		{"Buffer", NewBuffer([]byte{1, 2, 3}), `{"type":"Buffer","value":"AQID"}`},
		{"BoolTrue", NewBool(true), `{"type":"Boolean","value":true}`},
		{"BoolFalse", NewBool(false), `{"type":"Boolean","value":false}`},
		{"Struct", NewStruct([]Item{Make(11)}),
			`{"type":"Struct","value":[{"type":"Integer","value":"11"}]}`},
		{"Map", NewMapWithValue([]MapElement{{Key: NewBigInteger(big.NewInt(42)), Value: NewBool(false)}}),
			`{"type":"Map","value":[{"key":{"type":"Integer","value":"42"},` +
				`"value":{"type":"Boolean","value":false}}]}`},
		{"Interop", NewInterop(nil),
			`{"type":"InteropInterface"}`},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s, err := ToJSONWithTypes(tc.item)
			require.NoError(t, err)
			require.Equal(t, tc.result, string(s))

			item, err := FromJSONWithTypes(s)
			require.NoError(t, err)
			require.Equal(t, tc.item, item)
		})
	}

	t.Run("shared sub struct", func(t *testing.T) {
		t.Run("Buffer", func(t *testing.T) {
			shared := NewBuffer([]byte{1, 2, 3})
			a := NewArray([]Item{shared, shared})
			data, err := ToJSONWithTypes(a)
			require.NoError(t, err)
			expected := `{"type":"Array","value":[` +
				`{"type":"Buffer","value":"AQID"},{"type":"Buffer","value":"AQID"}]}`
			require.Equal(t, expected, string(data))
		})
		t.Run("Array", func(t *testing.T) {
			shared := NewArray([]Item{})
			a := NewArray([]Item{shared, shared})
			data, err := ToJSONWithTypes(a)
			require.NoError(t, err)
			expected := `{"type":"Array","value":[` +
				`{"type":"Array","value":[]},{"type":"Array","value":[]}]}`
			require.Equal(t, expected, string(data))
		})
		t.Run("Map", func(t *testing.T) {
			shared := NewMap()
			m := NewMapWithValue([]MapElement{
				{NewBool(true), shared},
				{NewBool(false), shared},
			})
			data, err := ToJSONWithTypes(m)
			require.NoError(t, err)
			expected := `{"type":"Map","value":[` +
				`{"key":{"type":"Boolean","value":true},"value":{"type":"Map","value":[]}},` +
				`{"key":{"type":"Boolean","value":false},"value":{"type":"Map","value":[]}}]}`
			require.Equal(t, expected, string(data))
		})
	})

	t.Run("Invalid", func(t *testing.T) {
		t.Run("RecursiveArray", func(t *testing.T) {
			arr := NewArray(nil)
			arr.value = []Item{Make(5), arr, Make(true)}

			_, err := ToJSONWithTypes(arr)
			require.Error(t, err)
		})
		t.Run("RecursiveMap", func(t *testing.T) {
			m := NewMap()
			m.Add(Make(3), Make(true))
			m.Add(Make(5), m)

			_, err := ToJSONWithTypes(m)
			require.Error(t, err)
		})
	})
}

func TestToJSONWithTypesBadCases(t *testing.T) {
	bigBuf := make([]byte, MaxSize)

	t.Run("issue 2385", func(t *testing.T) {
		const maxStackSize = 2 * 1024

		items := make([]Item, maxStackSize)
		for i := range items {
			items[i] = NewBuffer(bigBuf)
		}
		_, err := ToJSONWithTypes(NewArray(items))
		require.ErrorIs(t, err, errTooBigSize)
	})
	t.Run("overflow on primitive item", func(t *testing.T) {
		_, err := ToJSONWithTypes(NewBuffer(bigBuf))
		require.ErrorIs(t, err, errTooBigSize)
	})
	t.Run("overflow on array element", func(t *testing.T) {
		b := NewBuffer(bigBuf[:MaxSize/2])
		_, err := ToJSONWithTypes(NewArray([]Item{b, b}))
		require.ErrorIs(t, err, errTooBigSize)
	})
	t.Run("overflow on map key", func(t *testing.T) {
		m := NewMapWithValue([]MapElement{
			{NewBool(true), NewBool(true)},
			{NewByteArray(bigBuf), NewBool(true)},
		})
		_, err := ToJSONWithTypes(m)
		require.ErrorIs(t, err, errTooBigSize)
	})
	t.Run("overflow on the last byte of array", func(t *testing.T) {
		// Construct big enough buffer and pad with integer digits
		// until the necessary branch is covered #ididthemath.
		arr := NewArray([]Item{
			NewByteArray(bigBuf[:MaxSize/4*3-70]),
			NewBigInteger(big.NewInt(123456)),
		})
		_, err := ToJSONWithTypes(arr)
		require.ErrorIs(t, err, errTooBigSize)
	})
	t.Run("overflow on the item prefix", func(t *testing.T) {
		arr := NewArray([]Item{
			NewByteArray(bigBuf[:MaxSize/4*3-60]),
			NewBool(true),
		})
		_, err := ToJSONWithTypes(arr)
		require.ErrorIs(t, err, errTooBigSize)
	})
	t.Run("overflow on null", func(t *testing.T) {
		arr := NewArray([]Item{
			NewByteArray(bigBuf[:MaxSize/4*3-52]),
			Null{},
		})
		_, err := ToJSONWithTypes(arr)
		require.ErrorIs(t, err, errTooBigSize)
	})
	t.Run("overflow on interop", func(t *testing.T) {
		arr := NewArray([]Item{
			NewByteArray(bigBuf[:MaxSize/4*3-52]),
			NewInterop(42),
		})
		_, err := ToJSONWithTypes(arr)
		require.ErrorIs(t, err, errTooBigSize)
	})
	t.Run("overflow on cached item", func(t *testing.T) {
		b := NewArray([]Item{NewByteArray(bigBuf[:MaxSize/2])})
		arr := NewArray([]Item{b, b})
		_, err := ToJSONWithTypes(arr)
		require.ErrorIs(t, err, errTooBigSize)
	})
	t.Run("invalid type", func(t *testing.T) {
		_, err := ToJSONWithTypes(nil)
		require.ErrorIs(t, err, ErrUnserializable)
	})
}

func TestFromJSONWithTypes(t *testing.T) {
	testCases := []struct {
		name string
		json string
		item Item
	}{
		{"Pointer", `{"type":"Pointer","value":3}`, NewPointer(3, nil)},
		{"Interop", `{"type":"InteropInterface"}`, NewInterop(nil)},
		{"Null", `{"type":"Any"}`, Null{}},
		{"Array", `{"type":"Array","value":[{"type":"Any"}]}`, NewArray([]Item{Null{}})},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			item, err := FromJSONWithTypes([]byte(tc.json))
			require.NoError(t, err)
			require.Equal(t, tc.item, item)
		})
	}

	t.Run("Invalid", func(t *testing.T) {
		errCases := []struct {
			name string
			json string
		}{
			{"InvalidType", `{"type":int,"value":"4"`},
			{"UnexpectedType", `{"type":"int","value":"4"}`},
			{"IntegerValue1", `{"type":"Integer","value": 4}`},
			{"IntegerValue2", `{"type":"Integer","value": "a"}`},
			{"BoolValue", `{"type":"Boolean","value": "str"}`},
			{"PointerValue", `{"type":"Pointer","value": "str"}`},
			{"BufferValue1", `{"type":"Buffer","value":"not a base 64"}`},
			{"BufferValue2", `{"type":"Buffer","value":123}`},
			{"ArrayValue", `{"type":"Array","value":3}`},
			{"ArrayElement", `{"type":"Array","value":[3]}`},
			{"MapValue", `{"type":"Map","value":3}`},
			{"MapElement", `{"type":"Map","value":[{"key":"value"}]}`},
			{"MapElementKeyNotPrimitive", `{"type":"Map","value":[{"key":{"type":"Any"}}]}`},
			{"MapElementValue", `{"type":"Map","value":[` +
				`{"key":{"type":"Integer","value":"3"},"value":3}]}`},
		}
		for _, tc := range errCases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := FromJSONWithTypes([]byte(tc.json))
				require.Error(t, err)
			})
		}
	})
}
