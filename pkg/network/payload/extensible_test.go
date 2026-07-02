package payload

import (
	gio "io"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestExtensible_Serializable(t *testing.T) {
	pk, err := keys.NewPrivateKey()
	require.NoError(t, err)
	sender := pk.PublicKey().GetScriptHash()
	expected := &Extensible{
		Category:        "test",
		ValidBlockStart: 12,
		ValidBlockEnd:   1234,
		Sender:          sender,
		Data:            random.Bytes(4),
		Witness: transaction.Witness{
			InvocationScript:   []byte{1, 2, 3},
			VerificationScript: pk.PublicKey().GetVerificationScript(),
		},
	}

	testserdes.EncodeDecodeBinary(t, expected, new(Extensible))

	t.Run("invalid", func(t *testing.T) {
		w := io.NewBufBinWriter()
		expected.encodeBinaryUnsigned(w.BinWriter)
		unsigned := w.Bytes()

		t.Run("unexpected EOF", func(t *testing.T) {
			err := testserdes.DecodeBinary(unsigned, new(Extensible))
			require.ErrorIs(t, err, gio.EOF)
		})
		t.Run("invalid padding", func(t *testing.T) {
			err := testserdes.DecodeBinary(append(unsigned, 42), new(Extensible))
			require.ErrorIs(t, err, errInvalidPadding)
		})
		t.Run("too large data size", func(t *testing.T) {
			oldData := expected.Data
			expected.Data = make([]byte, MaxSize+1)
			t.Cleanup(func() {
				expected.Data = oldData
			})
			w := io.NewBufBinWriter()
			expected.encodeBinaryUnsigned(w.BinWriter)
			unsigned = w.Bytes()
			err := testserdes.DecodeBinary(unsigned, new(Extensible))
			require.ErrorContains(t, err, "byte-slice is too big")
		})
		t.Run("scripthash mismatch", func(t *testing.T) {
			expected.Sender = util.Uint160{1, 2, 3}
			w := io.NewBufBinWriter()
			expected.EncodeBinary(w.BinWriter)
			signed := w.Bytes()
			err := testserdes.DecodeBinary(signed, new(Extensible))
			require.ErrorContains(t, err, "witness script hash doesn't match sender")
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
