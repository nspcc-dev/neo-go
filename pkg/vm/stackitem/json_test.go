package stackitem

import (
	"encoding/base64"
	"math/big"
	"testing"

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
	var testBase64 = base64.StdEncoding.EncodeToString([]byte("test"))
	t.Run("ByteString", func(t *testing.T) {
		t.Run("Empty", getTestDecodeFunc(`""`, []byte{}))
		t.Run("Base64", getTestDecodeFunc(`"`+testBase64+`"`, "test"))
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
		t.Run("Simple", getTestDecodeFunc((`[1,"`+testBase64+`",true,null]`),
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
		t.Run("Big", getTestDecodeFunc(`{"3":{"a":3},"arr":["`+testBase64+`"]}`, large))
	})
	t.Run("Invalid", func(t *testing.T) {
		t.Run("Empty", getTestDecodeFunc(``, nil))
		t.Run("InvalidString", getTestDecodeFunc(`"not a base64"`, nil))
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
			l := base64.StdEncoding.DecodedLen(MaxSize + 8)
			require.True(t, l < MaxSize) // check if test makes sense
			item := NewByteArray(make([]byte, l))
			_, err := ToJSON(item)
			require.Error(t, err)
		})
		t.Run("BigNestedArray", getTestDecodeFunc(`[[[[[[[[[[[]]]]]]]]]]]`, nil))
		t.Run("EncodeRecursive", func(t *testing.T) {
			// add this item to speed up test a bit
			item := NewByteArray(make([]byte, MaxSize/100))
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
