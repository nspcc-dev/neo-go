package stackitem

import (
	"strconv"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/stretchr/testify/require"
)

func TestSerializationMaxErr(t *testing.T) {
	base := make([]byte, MaxSize/2+1)
	item := Make(base)

	// Pointer is unserializable, but we specifically want to catch ErrTooBig.
	arr := []Item{item, item.Dup(), NewPointer(0, []byte{})}
	aitem := Make(arr)

	_, err := Serialize(item)
	require.NoError(t, err)

	_, err = Serialize(aitem)
	require.ErrorIs(t, err, ErrTooBig)
}

func testSerialize(t *testing.T, expectedErr error, item Item) {
	testSerializeLimited(t, expectedErr, item, -1)
}

func testSerializeLimited(t *testing.T, expectedErr error, item Item, limit int) {
	var (
		data []byte
		err  error
	)
	if limit > 0 {
		data, err = SerializeLimited(item, limit)
	} else {
		data, err = Serialize(item)
	}
	if expectedErr != nil {
		require.ErrorIs(t, err, expectedErr)
		return
	}
	require.NoError(t, err)

	actual, err := Deserialize(data)
	require.NoError(t, err)
	require.Equal(t, item, actual)
}

func TestSerialize(t *testing.T) {
	bigByteArray := NewByteArray(make([]byte, MaxSize/2))
	smallByteArray := NewByteArray(make([]byte, MaxSize/4))
	zeroByteArray := NewByteArray(make([]byte, 0))
	testArray := func(t *testing.T, newItem func([]Item) Item) {
		arr := newItem([]Item{bigByteArray})
		testSerialize(t, nil, arr)
		testSerialize(t, ErrTooBig, newItem([]Item{bigByteArray, bigByteArray}))
		testSerialize(t, ErrTooBig, newItem([]Item{arr, arr}))

		arr.Value().([]Item)[0] = smallByteArray
		testSerialize(t, nil, newItem([]Item{arr, arr}))

		arr.Value().([]Item)[0] = arr
		testSerialize(t, ErrRecursive, arr)

		items := make([]Item, 0, MaxDeserialized-1)
		for range MaxDeserialized - 1 {
			items = append(items, zeroByteArray)
		}
		testSerialize(t, nil, newItem(items))

		items = append(items, zeroByteArray)
		_, err := Serialize(newItem(items))
		require.ErrorIs(t, err, errTooBigElements)
		data, err := SerializeLimited(newItem(items), MaxSerialized+1) // a tiny hack to check deserialization error.
		require.NoError(t, err)
		_, err = Deserialize(data)
		require.ErrorIs(t, err, ErrTooBig)
	}
	t.Run("array", func(t *testing.T) {
		testArray(t, func(items []Item) Item { return NewArray(items) })
	})
	t.Run("struct", func(t *testing.T) {
		testArray(t, func(items []Item) Item { return NewStruct(items) })
	})
	t.Run("buffer", func(t *testing.T) {
		testSerialize(t, nil, NewBuffer(make([]byte, MaxSize/2)))
		testSerialize(t, errTooBigSize, NewBuffer(make([]byte, MaxSize)))
	})
	t.Run("invalid", func(t *testing.T) {
		testSerialize(t, ErrUnserializable, NewInterop(42))
		testSerialize(t, ErrUnserializable, NewPointer(42, []byte{}))
		testSerialize(t, ErrUnserializable, nil)

		t.Run("protected interop", func(t *testing.T) {
			w := io.NewBufBinWriter()
			EncodeBinaryProtected(NewInterop(42), w.BinWriter)
			require.NoError(t, w.Err)

			data := w.Bytes()
			r := io.NewBinReaderFromBuf(data)
			DecodeBinary(r)
			require.Error(t, r.Err)

			r = io.NewBinReaderFromBuf(data)
			item := DecodeBinaryProtected(r)
			require.NoError(t, r.Err)
			require.IsType(t, (*Interop)(nil), item)
		})
		t.Run("protected pointer", func(t *testing.T) {
			w := io.NewBufBinWriter()
			EncodeBinaryProtected(NewPointer(42, []byte{}), w.BinWriter)
			require.NoError(t, w.Err)

			data := w.Bytes()
			r := io.NewBinReaderFromBuf(data)
			DecodeBinary(r)
			require.Error(t, r.Err)

			r = io.NewBinReaderFromBuf(data)
			item := DecodeBinaryProtected(r)
			require.NoError(t, r.Err)
			require.IsType(t, (*Pointer)(nil), item)
			require.Equal(t, 42, item.Value())
		})
		t.Run("protected nil", func(t *testing.T) {
			w := io.NewBufBinWriter()
			EncodeBinaryProtected(nil, w.BinWriter)
			require.NoError(t, w.Err)

			data := w.Bytes()
			r := io.NewBinReaderFromBuf(data)
			DecodeBinary(r)
			require.Error(t, r.Err)

			r = io.NewBinReaderFromBuf(data)
			item := DecodeBinaryProtected(r)
			require.NoError(t, r.Err)
			require.Nil(t, item)
		})
	})
	t.Run("bool", func(t *testing.T) {
		testSerialize(t, nil, NewBool(true))
		testSerialize(t, nil, NewBool(false))
	})
	t.Run("null", func(t *testing.T) {
		testSerialize(t, nil, Null{})
	})
	t.Run("integer", func(t *testing.T) {
		testSerialize(t, nil, Make(0xF))          // 1-byte
		testSerialize(t, nil, Make(0xFAB))        // 2-byte
		testSerialize(t, nil, Make(0xFABCD))      // 4-byte
		testSerialize(t, nil, Make(0xFABCDEFEDC)) // 8-byte
	})
	t.Run("map", func(t *testing.T) {
		one := Make(1)
		m := NewMap()
		m.Add(one, m)
		testSerialize(t, ErrRecursive, m)

		m.Add(one, bigByteArray)
		testSerialize(t, nil, m)

		m.Add(Make(2), bigByteArray)
		testSerialize(t, ErrTooBig, m)

		// Cover code path when result becomes too big after key encode.
		m = NewMap()
		m.Add(Make(0), NewByteArray(make([]byte, MaxSize-MaxKeySize)))
		m.Add(NewByteArray(make([]byte, MaxKeySize)), Make(1))
		testSerialize(t, ErrTooBig, m)

		m = NewMap()
		for i := range MaxDeserialized/2 - 1 {
			m.Add(Make(i), zeroByteArray)
		}
		testSerialize(t, nil, m)

		for i := range MaxDeserialized + 1 {
			m.Add(Make(i), zeroByteArray)
		}
		_, err := Serialize(m)
		require.ErrorIs(t, err, errTooBigElements)
		data, err := SerializeLimited(m, (MaxSerialized+1)*2+1) // a tiny hack to check deserialization error.
		require.NoError(t, err)
		_, err = Deserialize(data)
		require.ErrorIs(t, err, ErrTooBig)
	})
}

func TestSerializeLimited(t *testing.T) {
	const customLimit = 10

	smallArray := make([]Item, customLimit-1)
	for i := range smallArray {
		smallArray[i] = NewBool(true)
	}
	bigArray := make([]Item, customLimit)
	for i := range bigArray {
		bigArray[i] = NewBool(true)
	}
	t.Run("array", func(t *testing.T) {
		testSerializeLimited(t, nil, NewArray(smallArray), customLimit)
		testSerializeLimited(t, errTooBigElements, NewArray(bigArray), customLimit)
	})
	t.Run("struct", func(t *testing.T) {
		testSerializeLimited(t, nil, NewStruct(smallArray), customLimit)
		testSerializeLimited(t, errTooBigElements, NewStruct(bigArray), customLimit)
	})
	t.Run("map", func(t *testing.T) {
		smallMap := make([]MapElement, (customLimit-1)/2)
		for i := range smallMap {
			smallMap[i] = MapElement{
				Key:   NewByteArray([]byte(strconv.Itoa(i))),
				Value: NewBool(true),
			}
		}
		bigMap := make([]MapElement, customLimit/2)
		for i := range bigMap {
			bigMap[i] = MapElement{
				Key:   NewByteArray([]byte("key" + strconv.Itoa(i))),
				Value: NewBool(true),
			}
		}
		testSerializeLimited(t, nil, NewMapWithValue(smallMap), customLimit)
		testSerializeLimited(t, errTooBigElements, NewMapWithValue(bigMap), customLimit)
	})
	t.Run("seen", func(t *testing.T) {
		t.Run("OK", func(t *testing.T) {
			tinyArray := NewArray(make([]Item, (customLimit-3)/2)) // 1 for outer array, 1+1 for two inner arrays and the rest are for arrays' elements.
			for i := range tinyArray.value {
				tinyArray.value[i] = NewBool(true)
			}
			testSerializeLimited(t, nil, NewArray([]Item{tinyArray, tinyArray}), customLimit)
		})
		t.Run("big", func(t *testing.T) {
			tinyArray := NewArray(make([]Item, (customLimit-2)/2)) // should break on the second array serialisation.
			for i := range tinyArray.value {
				tinyArray.value[i] = NewBool(true)
			}
			testSerializeLimited(t, errTooBigElements, NewArray([]Item{tinyArray, tinyArray}), customLimit)
		})
	})
}

func TestEmptyDeserialization(t *testing.T) {
	empty := []byte{}
	_, err := Deserialize(empty)
	require.Error(t, err)
}

func TestMapDeserializationError(t *testing.T) {
	m := NewMap()
	m.Add(Make(1), Make(1))
	m.Add(Make(2), nil) // Bad value
	m.Add(Make(3), Make(3))

	w := io.NewBufBinWriter()
	EncodeBinaryProtected(m, w.BinWriter)
	require.NoError(t, w.Err)
	_, err := Deserialize(w.Bytes())
	require.ErrorIs(t, err, ErrInvalidType)
}

func TestDeserializeTooManyElements(t *testing.T) {
	item := Make(0)
	for range MaxDeserialized - 1 { // 1 for zero inner element.
		item = Make([]Item{item})
	}
	data, err := Serialize(item)
	require.NoError(t, err)
	_, err = Deserialize(data)
	require.NoError(t, err)

	item = Make([]Item{item})
	data, err = SerializeLimited(item, MaxSerialized+1) // tiny hack to avoid serialization error.
	require.NoError(t, err)
	_, err = Deserialize(data)
	require.ErrorIs(t, err, ErrTooBig)
}

func TestDeserializeLimited(t *testing.T) {
	customLimit := MaxDeserialized + 1
	item := Make(0)
	for range customLimit - 1 { // 1 for zero inner element.
		item = Make([]Item{item})
	}
	data, err := SerializeLimited(item, customLimit) // tiny hack to avoid serialization error.
	require.NoError(t, err)
	actual, err := DeserializeLimited(data, customLimit)
	require.NoError(t, err)
	require.Equal(t, item, actual)

	item = Make([]Item{item})
	data, err = SerializeLimited(item, customLimit+1) // tiny hack to avoid serialization error.
	require.NoError(t, err)
	_, err = DeserializeLimited(data, customLimit)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrTooBig)
}

func BenchmarkEncodeBinary(b *testing.B) {
	arr := getBigArray(15)

	w := io.NewBufBinWriter()

	b.ResetTimer()
	b.ReportAllocs()
	for range b.N {
		w.Reset()
		EncodeBinary(arr, w.BinWriter)
		if w.Err != nil {
			b.FailNow()
		}
	}
}

func BenchmarkSerializeSimple(b *testing.B) {
	s := NewStruct(nil)
	s.Append(Make(100500))
	s.Append(Make("1aada0032aba1ef6d1f0")) // Mimicking uint160.
	for range b.N {
		_, err := Serialize(s)
		if err != nil {
			b.FailNow()
		}
	}
}
