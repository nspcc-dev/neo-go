package payload

import (
	"errors"
	gio "io"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/stretchr/testify/require"
)

func TestExtensible_Serializable(t *testing.T) {
	expected := &Extensible{
		Category:        "test",
		ValidBlockStart: 12,
		ValidBlockEnd:   1234,
		Sender:          random.Uint160(),
		Data:            random.Bytes(4),
		Witness: transaction.Witness{
			InvocationScript:   random.Bytes(3),
			VerificationScript: random.Bytes(3),
		},
	}

	testserdes.EncodeDecodeBinary(t, expected, new(Extensible))

	t.Run("invalid", func(t *testing.T) {
		w := io.NewBufBinWriter()
		expected.encodeBinaryUnsigned(w.BinWriter)
		unsigned := w.Bytes()

		t.Run("unexpected EOF", func(t *testing.T) {
			err := testserdes.DecodeBinary(unsigned, new(Extensible))
			require.True(t, errors.Is(err, gio.EOF))
		})
		t.Run("invalid padding", func(t *testing.T) {
			err := testserdes.DecodeBinary(append(unsigned, 42), new(Extensible))
			require.True(t, errors.Is(err, errInvalidPadding))
		})
		t.Run("too large data size", func(t *testing.T) {
			expected.Data = make([]byte, MaxSize+1)
			w := io.NewBufBinWriter()
			expected.encodeBinaryUnsigned(w.BinWriter)
			unsigned = w.Bytes()
			err := testserdes.DecodeBinary(unsigned, new(Extensible))
			require.NotNil(t, err)
		})
	})
}

func TestExtensible_Hashes(t *testing.T) {
	getExtensiblePair := func() (*Extensible, *Extensible) {
		p1 := NewExtensible()
		p1.Data = []byte{1, 2, 3}
		p2 := NewExtensible()
		p2.Data = []byte{3, 2, 1}
		return p1, p2
	}

	t.Run("Hash", func(t *testing.T) {
		p1, p2 := getExtensiblePair()
		require.NotEqual(t, p1.Hash(), p2.Hash())
		require.NotEqual(t, p1.Hash(), p2.Hash())
	})
}
