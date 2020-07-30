package stackitem

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFromString(t *testing.T) {
	typs := []Type{AnyT, PointerT, BooleanT, IntegerT, ByteArrayT, BufferT, ArrayT, StructT, MapT, InteropT}
	for _, typ := range typs {
		actual, err := FromString(typ.String())
		require.NoError(t, err)
		require.Equal(t, typ, actual)
	}
}
