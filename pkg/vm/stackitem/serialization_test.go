package stackitem

import (
	"errors"
	"testing"

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
