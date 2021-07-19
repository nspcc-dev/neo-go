package stackitem

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var makeStackItemTestCases = []struct {
	input  interface{}
	result Item
}{
	{
		input:  int64(3),
		result: &BigInteger{value: big.NewInt(3)},
	},
	{
		input:  int16(3),
		result: &BigInteger{value: big.NewInt(3)},
	},
	{
		input:  3,
		result: &BigInteger{value: big.NewInt(3)},
	},
	{
		input:  uint8(3),
		result: &BigInteger{value: big.NewInt(3)},
	},
	{
		input:  uint16(3),
		result: &BigInteger{value: big.NewInt(3)},
	},
	{
		input:  uint32(3),
		result: &BigInteger{value: big.NewInt(3)},
	},
	{
		input:  uint64(3),
		result: &BigInteger{value: big.NewInt(3)},
	},
	{
		input:  big.NewInt(3),
		result: &BigInteger{value: big.NewInt(3)},
	},
	{
		input:  []byte{1, 2, 3, 4},
		result: &ByteArray{value: []byte{1, 2, 3, 4}},
	},
	{
		input:  []byte{},
		result: &ByteArray{value: []byte{}},
	},
	{
		input:  "bla",
		result: &ByteArray{value: []byte("bla")},
	},
	{
		input:  "",
		result: &ByteArray{value: []byte{}},
	},
	{
		input:  true,
		result: &Bool{value: true},
	},
	{
		input:  false,
		result: &Bool{value: false},
	},
	{
		input:  []Item{&BigInteger{value: big.NewInt(3)}, &ByteArray{value: []byte{1, 2, 3}}},
		result: &Array{value: []Item{&BigInteger{value: big.NewInt(3)}, &ByteArray{value: []byte{1, 2, 3}}}},
	},
	{
		input:  []int{1, 2, 3},
		result: &Array{value: []Item{&BigInteger{value: big.NewInt(1)}, &BigInteger{value: big.NewInt(2)}, &BigInteger{value: big.NewInt(3)}}},
	},
}

var makeStackItemErrorCases = []struct {
	input interface{}
}{
	{
		input: nil,
	},
}

func TestMakeStackItem(t *testing.T) {
	for _, testCase := range makeStackItemTestCases {
		assert.Equal(t, testCase.result, Make(testCase.input))
	}
	for _, errorCase := range makeStackItemErrorCases {
		assert.Panics(t, func() { Make(errorCase.input) })
	}
}

var stringerTestCases = []struct {
	input  Item
	result string
}{
	{
		input:  NewStruct([]Item{}),
		result: "Struct",
	},
	{
		input:  NewBigInteger(big.NewInt(3)),
		result: "BigInteger",
	},
	{
		input:  NewBool(true),
		result: "Boolean",
	},
	{
		input:  NewByteArray([]byte{}),
		result: "ByteString",
	},
	{
		input:  NewArray([]Item{}),
		result: "Array",
	},
	{
		input:  NewMap(),
		result: "Map",
	},
	{
		input:  NewInterop(nil),
		result: "Interop",
	},
	{
		input:  NewPointer(0, nil),
		result: "Pointer",
	},
}

func TestStringer(t *testing.T) {
	for _, testCase := range stringerTestCases {
		assert.Equal(t, testCase.result, testCase.input.String())
	}
}

var equalsTestCases = map[string][]struct {
	item1  Item
	item2  Item
	result bool
	panics bool
}{
	"struct": {
		{
			item1:  NewStruct(nil),
			item2:  nil,
			result: false,
		},
		{
			item1:  NewStruct(nil),
			item2:  NewBigInteger(big.NewInt(1)),
			result: false,
		},
		{
			item1:  NewStruct(nil),
			item2:  NewStruct([]Item{NewBigInteger(big.NewInt(1))}),
			result: false,
		},
		{
			item1:  NewStruct([]Item{NewBigInteger(big.NewInt(1))}),
			item2:  NewStruct([]Item{NewBigInteger(big.NewInt(2))}),
			result: false,
		},
		{
			item1:  NewStruct([]Item{NewBigInteger(big.NewInt(1))}),
			item2:  NewStruct([]Item{NewBigInteger(big.NewInt(1))}),
			result: true,
		},
		{
			item1:  NewStruct([]Item{NewBigInteger(big.NewInt(1)), NewStruct([]Item{})}),
			item2:  NewStruct([]Item{NewBigInteger(big.NewInt(1)), NewStruct([]Item{})}),
			result: true,
		},
	},
	"bigint": {
		{
			item1:  NewBigInteger(big.NewInt(2)),
			item2:  nil,
			result: false,
		},
		{
			item1:  NewBigInteger(big.NewInt(2)),
			item2:  NewBigInteger(big.NewInt(2)),
			result: true,
		},
		{
			item1:  NewBigInteger(big.NewInt(2)),
			item2:  NewBool(false),
			result: false,
		},
		{
			item1:  NewBigInteger(big.NewInt(0)),
			item2:  NewBool(false),
			result: false,
		},
		{
			item1:  NewBigInteger(big.NewInt(2)),
			item2:  Make(int32(2)),
			result: true,
		},
	},
	"bool": {
		{
			item1:  NewBool(true),
			item2:  nil,
			result: false,
		},
		{
			item1:  NewBool(true),
			item2:  NewBool(true),
			result: true,
		},
		{
			item1:  NewBool(true),
			item2:  NewBigInteger(big.NewInt(1)),
			result: false,
		},
		{
			item1:  NewBool(true),
			item2:  NewBool(false),
			result: false,
		},
		{
			item1:  NewBool(true),
			item2:  Make(true),
			result: true,
		},
	},
	"bytearray": {
		{
			item1:  NewByteArray(nil),
			item2:  nil,
			result: false,
		},
		{
			item1:  NewByteArray([]byte{1, 2, 3}),
			item2:  NewByteArray([]byte{1, 2, 3}),
			result: true,
		},
		{
			item1:  NewByteArray([]byte{1}),
			item2:  NewBigInteger(big.NewInt(1)),
			result: false,
		},
		{
			item1:  NewByteArray([]byte{1, 2, 3}),
			item2:  NewByteArray([]byte{1, 2, 4}),
			result: false,
		},
		{
			item1:  NewByteArray([]byte{1, 2, 3}),
			item2:  Make([]byte{1, 2, 3}),
			result: true,
		},
		{
			item1:  NewByteArray(make([]byte, MaxByteArrayComparableSize+1)),
			item2:  NewByteArray([]byte{1, 2, 3}),
			panics: true,
		},
		{
			item1:  NewByteArray([]byte{1, 2, 3}),
			item2:  NewByteArray(make([]byte, MaxByteArrayComparableSize+1)),
			panics: true,
		},
		{
			item1:  NewByteArray(make([]byte, MaxByteArrayComparableSize+1)),
			item2:  NewByteArray(make([]byte, MaxByteArrayComparableSize+1)),
			panics: true,
		},
	},
	"array": {
		{
			item1:  NewArray(nil),
			item2:  nil,
			result: false,
		},
		{
			item1:  NewArray([]Item{&BigInteger{big.NewInt(1)}, &BigInteger{big.NewInt(2)}, &BigInteger{big.NewInt(3)}}),
			item2:  NewArray([]Item{&BigInteger{big.NewInt(1)}, &BigInteger{big.NewInt(2)}, &BigInteger{big.NewInt(3)}}),
			result: false,
		},
		{
			item1:  NewArray([]Item{&BigInteger{big.NewInt(1)}}),
			item2:  NewBigInteger(big.NewInt(1)),
			result: false,
		},
		{
			item1:  NewArray([]Item{&BigInteger{big.NewInt(1)}, &BigInteger{big.NewInt(2)}, &BigInteger{big.NewInt(3)}}),
			item2:  NewArray([]Item{&BigInteger{big.NewInt(1)}, &BigInteger{big.NewInt(2)}, &BigInteger{big.NewInt(4)}}),
			result: false,
		},
	},
	"map": {
		{
			item1:  NewMap(),
			item2:  nil,
			result: false,
		},
		{
			item1:  NewMap(),
			item2:  NewMap(),
			result: false,
		},
		{
			item1:  &Map{value: []MapElement{{NewByteArray([]byte("first")), NewBigInteger(big.NewInt(1))}, {NewBool(true), NewByteArray([]byte{2})}}},
			item2:  &Map{value: []MapElement{{NewByteArray([]byte("first")), NewBigInteger(big.NewInt(1))}, {NewBool(true), NewByteArray([]byte{2})}}},
			result: false,
		},
		{
			item1:  &Map{value: []MapElement{{NewByteArray([]byte("first")), NewBigInteger(big.NewInt(1))}, {NewBool(true), NewByteArray([]byte{2})}}},
			item2:  &Map{value: []MapElement{{NewByteArray([]byte("first")), NewBigInteger(big.NewInt(1))}, {NewBool(true), NewByteArray([]byte{3})}}},
			result: false,
		},
	},
	"interop": {
		{
			item1:  NewInterop(nil),
			item2:  nil,
			result: false,
		},
		{
			item1:  NewInterop(nil),
			item2:  NewInterop(nil),
			result: true,
		},
		{
			item1:  NewInterop(2),
			item2:  NewInterop(3),
			result: false,
		},
		{
			item1:  NewInterop(3),
			item2:  NewInterop(3),
			result: true,
		},
	},
	"pointer": {
		{
			item1:  NewPointer(0, []byte{}),
			result: false,
		},
		{
			item1:  NewPointer(1, []byte{1}),
			item2:  NewPointer(1, []byte{1}),
			result: true,
		},
		{
			item1:  NewPointer(1, []byte{1}),
			item2:  NewPointer(2, []byte{1}),
			result: false,
		},
		{
			item1:  NewPointer(1, []byte{1}),
			item2:  NewPointer(1, []byte{2}),
			result: false,
		},
		{
			item1:  NewPointer(0, []byte{}),
			item2:  NewBigInteger(big.NewInt(0)),
			result: false,
		},
	},
}

func TestEquals(t *testing.T) {
	for name, testBatch := range equalsTestCases {
		for _, testCase := range testBatch {
			t.Run(name, func(t *testing.T) {
				if testCase.panics {
					assert.Panics(t, func() {
						testCase.item1.Equals(testCase.item2)
					})
				} else {
					assert.Equal(t, testCase.result, testCase.item1.Equals(testCase.item2))
					// Reference equals
					assert.Equal(t, true, testCase.item1.Equals(testCase.item1))
				}
			})
		}
	}
}

func TestEqualsDeepStructure(t *testing.T) {
	const perStruct = 4
	var items = []Item{}
	var num int
	for i := 0; i < perStruct; i++ {
		items = append(items, Make(0))
		num++
	}
	var layerUp = func(sa *Struct, num int) (*Struct, int) {
		items := []Item{}
		for i := 0; i < perStruct; i++ {
			clon, err := sa.Clone(100500)
			require.NoError(t, err)
			items = append(items, clon)
		}
		num *= perStruct
		num++
		return NewStruct(items), num
	}
	var sa = NewStruct(items)
	for i := 0; i < 4; i++ {
		sa, num = layerUp(sa, num)
	}
	require.Less(t, num, MaxComparableNumOfItems)
	sb, err := sa.Clone(num)
	require.NoError(t, err)
	require.True(t, sa.Equals(sb))
	sa, num = layerUp(sa, num)

	require.Less(t, MaxComparableNumOfItems, num)
	sb, err = sa.Clone(num)
	require.NoError(t, err)
	require.Panics(t, func() { sa.Equals(sb) })
}

var marshalJSONTestCases = []struct {
	input  Item
	result []byte
}{
	{
		input:  NewBigInteger(big.NewInt(2)),
		result: []byte(`2`),
	},
	{
		input:  NewBool(true),
		result: []byte(`true`),
	},
	{
		input:  NewByteArray([]byte{1, 2, 3}),
		result: []byte(`"010203"`),
	},
	{
		input:  NewBuffer([]byte{1, 2, 3}),
		result: []byte(`"010203"`),
	},
	{
		input:  &Array{value: []Item{&BigInteger{value: big.NewInt(3)}, &ByteArray{value: []byte{1, 2, 3}}}},
		result: []byte(`[3,"010203"]`),
	},
	{
		input:  &Interop{value: 3},
		result: []byte(`3`),
	},
}

func TestMarshalJSON(t *testing.T) {
	var (
		actual []byte
		err    error
	)
	for _, testCase := range marshalJSONTestCases {
		switch testCase.input.(type) {
		case *BigInteger:
			actual, err = testCase.input.(*BigInteger).MarshalJSON()
		case *Bool:
			actual, err = testCase.input.(*Bool).MarshalJSON()
		case *ByteArray:
			actual, err = testCase.input.(*ByteArray).MarshalJSON()
		case *Array:
			actual, err = testCase.input.(*Array).MarshalJSON()
		case *Interop:
			actual, err = testCase.input.(*Interop).MarshalJSON()
		default:
			continue
		}

		assert.NoError(t, err)
		assert.Equal(t, testCase.result, actual)
	}
}

func TestNewVeryBigInteger(t *testing.T) {
	check := func(ok bool, v *big.Int) {
		bs := bigint.ToBytes(v)
		if ok {
			assert.True(t, len(bs)*8 <= MaxBigIntegerSizeBits)
		} else {
			assert.True(t, len(bs)*8 > MaxBigIntegerSizeBits)
			assert.Panics(t, func() { NewBigInteger(v) })
		}
	}

	maxBitSet := big.NewInt(1)
	maxBitSet.Lsh(maxBitSet, MaxBigIntegerSizeBits-1)

	check(false, maxBitSet)
	check(true, new(big.Int).Neg(maxBitSet))

	minus1 := new(big.Int).Sub(maxBitSet, big.NewInt(1))
	check(true, minus1)
	check(true, new(big.Int).Neg(minus1))

	plus1 := new(big.Int).Add(maxBitSet, big.NewInt(1))
	check(false, plus1)
	check(false, new(big.Int).Neg(plus1))

	check(false, new(big.Int).Mul(maxBitSet, big.NewInt(2)))
}

func TestStructClone(t *testing.T) {
	st0 := Struct{}
	st := Struct{value: []Item{&st0}}
	_, err := st.Clone(1)
	require.NoError(t, err)
	_, err = st.Clone(0)
	require.Error(t, err)
}

func TestDeepCopy(t *testing.T) {
	testCases := []struct {
		name string
		item Item
	}{
		{"Integer", NewBigInteger(big.NewInt(1))},
		{"ByteArray", NewByteArray([]byte{1, 2, 3})},
		{"Buffer", NewBuffer([]byte{1, 2, 3})},
		{"Bool", NewBool(true)},
		{"Pointer", NewPointer(1, []byte{1, 2, 3})},
		{"Interop", NewInterop(&[]byte{1, 2})},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := DeepCopy(tc.item)
			require.Equal(t, tc.item, actual)
			require.False(t, actual == tc.item)
		})
	}

	t.Run("Null", func(t *testing.T) {
		require.Equal(t, Null{}, DeepCopy(Null{}))
	})

	t.Run("Array", func(t *testing.T) {
		arr := NewArray(make([]Item, 2))
		arr.value[0] = NewBool(true)
		arr.value[1] = arr

		actual := DeepCopy(arr)
		require.Equal(t, arr, actual)
		require.False(t, arr == actual)
		require.True(t, actual == actual.(*Array).value[1])
	})

	t.Run("Struct", func(t *testing.T) {
		arr := NewStruct(make([]Item, 2))
		arr.value[0] = NewBool(true)
		arr.value[1] = arr

		actual := DeepCopy(arr)
		require.Equal(t, arr, actual)
		require.False(t, arr == actual)
		require.True(t, actual == actual.(*Struct).value[1])
	})

	t.Run("Map", func(t *testing.T) {
		m := NewMapWithValue(make([]MapElement, 2))
		m.value[0] = MapElement{Key: NewBool(true), Value: m}
		m.value[1] = MapElement{Key: NewBigInteger(big.NewInt(1)), Value: NewByteArray([]byte{1, 2, 3})}

		actual := DeepCopy(m)
		require.Equal(t, m, actual)
		require.False(t, m == actual)
		require.True(t, actual == actual.(*Map).value[0].Value)
	})
}
