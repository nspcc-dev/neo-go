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
