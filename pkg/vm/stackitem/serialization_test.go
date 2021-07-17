package stackitem

import (
	"errors"
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
	require.True(t, errors.Is(err, ErrTooBig), err)
}

func testSerialize(t *testing.T, expectedErr error, item Item) {
	data, err := Serialize(item)
	if expectedErr != nil {
		require.True(t, errors.Is(err, expectedErr), err)
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

		items := make([]Item, 0, MaxArraySize)
		for i := 0; i < MaxArraySize; i++ {
			items = append(items, zeroByteArray)
		}
		testSerialize(t, nil, newItem(items))

		items = append(items, zeroByteArray)
		data, err := Serialize(newItem(items))
		require.NoError(t, err)
		_, err = Deserialize(data)
		require.True(t, errors.Is(err, ErrTooBig), err)
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
		for i := 0; i < MaxArraySize; i++ {
			m.Add(Make(i), zeroByteArray)
		}
		// testSerialize(t, nil, m) // It contains too many elements already, so ErrTooBig.

		m.Add(Make(100500), zeroByteArray)
		data, err := Serialize(m)
		require.NoError(t, err)
		_, err = Deserialize(data)
		require.True(t, errors.Is(err, ErrTooBig), err)
	})
}

func TestDeserializeTooManyElements(t *testing.T) {
	item := Make(0)
	for i := 0; i < MaxDeserialized-1; i++ { // 1 for zero inner element.
		item = Make([]Item{item})
	}
	data, err := Serialize(item)
	require.NoError(t, err)
	_, err = Deserialize(data)
	require.NoError(t, err)

	item = Make([]Item{item})
	data, err = Serialize(item)
	require.NoError(t, err)
	_, err = Deserialize(data)
	require.True(t, errors.Is(err, ErrTooBig), err)
}

func BenchmarkEncodeBinary(b *testing.B) {
	arr := getBigArray(15)

	w := io.NewBufBinWriter()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w.Reset()
		EncodeBinary(arr, w.BinWriter)
		if w.Err != nil {
			b.FailNow()
		}
	}
}
