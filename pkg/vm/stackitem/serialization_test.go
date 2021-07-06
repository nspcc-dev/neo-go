package stackitem

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSerializationMaxErr(t *testing.T) {
	base := make([]byte, MaxSize/2+1)
	item := Make(base)

	arr := []Item{item, item.Dup()}
	aitem := Make(arr)

	_, err := SerializeItem(item)
	require.NoError(t, err)

	_, err = SerializeItem(aitem)
	require.Error(t, err)
}
