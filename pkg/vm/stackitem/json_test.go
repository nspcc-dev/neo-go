package stackitem

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getTestDecodeFunc(js string, expected ...interface{}) func(t *testing.T) {
	return func(t *testing.T) {
		actual, err := FromJSON([]byte(js))
		if expected[0] == nil {
			require.Error(t, err)
			return
		}
		require.NoError(t, err)
		require.Equal(t, Make(expected[0]), actual)

		if len(expected) == 1 {
			encoded, err := ToJSON(actual)
			require.NoError(t, err)
			require.Equal(t, js, string(encoded))
		}
	}
}

func TestFromToJSON(t *testing.T) {
	t.Run("ByteString", func(t *testing.T) {
		t.Run("Empty", getTestDecodeFunc(`""`, []byte{}))
		t.Run("Base64", getTestDecodeFunc(`"test"`, "test"))
		t.Run("Escape", getTestDecodeFunc(`"\"quotes\""`, `"quotes"`))
	})
	t.Run("BigInteger", func(t *testing.T) {
		t.Run("ZeroFloat", getTestDecodeFunc(`12.000`, 12, nil))
		t.Run("NonZeroFloat", getTestDecodeFunc(`12.01`, nil))
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

// getBigArray returns array takes up a lot of storage when serialized.
func getBigArray(depth int) *Array {
	arr := NewArray([]Item{})
	for i := 0; i < depth; i++ {
		arr = NewArray([]Item{arr, arr})
	}
	return arr
}

func BenchmarkToJSON(b *testing.B) {
	arr := getBigArray(15)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
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

func TestFromJSONWithTypes(t *testing.T) {
	testCases := []struct {
		name string
		json string
		item Item
	}{
		{"Pointer", `{"type":"Pointer","value":3}`, NewPointer(3, nil)},
		{"Interop", `{"type":"Interop"}`, NewInterop(nil)},
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
