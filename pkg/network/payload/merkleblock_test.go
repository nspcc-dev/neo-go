package payload

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

func newDumbBlock() *block.Header {
	return &block.Header{
		Version:       0,
		PrevHash:      hash.Sha256([]byte("a")),
		MerkleRoot:    hash.Sha256([]byte("b")),
		Timestamp:     100500,
		Index:         1,
		NextConsensus: hash.Hash160([]byte("a")),
		Script: transaction.Witness{
			VerificationScript: []byte{0x51}, // PUSH1
			InvocationScript:   []byte{0x61}, // NOP
		},
	}
}

func TestMerkleBlock_EncodeDecodeBinary(t *testing.T) {
	t.Run("positive", func(t *testing.T) {
		b := newDumbBlock()
		_ = b.Hash()
		expected := &MerkleBlock{
			Header:  b,
			TxCount: 0,
			Hashes:  []util.Uint256{},
			Flags:   []byte{},
		}
		testserdes.EncodeDecodeBinary(t, expected, new(MerkleBlock))
	})

	t.Run("bad contents count", func(t *testing.T) {
		b := newDumbBlock()
		_ = b.Hash()
		expected := &MerkleBlock{
			Header:  b,
			TxCount: block.MaxTransactionsPerBlock + 1,
			Hashes:  make([]util.Uint256, block.MaxTransactionsPerBlock),
			Flags:   []byte{},
		}
		data, err := testserdes.EncodeBinary(expected)
		require.NoError(t, err)
		require.ErrorIs(t, testserdes.DecodeBinary(data, new(MerkleBlock)), block.ErrMaxContentsPerBlock)
	})

	t.Run("bad flags size", func(t *testing.T) {
		b := newDumbBlock()
		_ = b.Hash()
		expected := &MerkleBlock{
			Header:  b,
			TxCount: 0,
			Hashes:  []util.Uint256{},
			Flags:   []byte{1, 2, 3, 4, 5},
		}
		data, err := testserdes.EncodeBinary(expected)
		require.NoError(t, err)
		require.Error(t, testserdes.DecodeBinary(data, new(MerkleBlock)))
	})
}

func TestRoundToNextPowerOfTwo(t *testing.T) {
	for _, tc := range []struct {
		in       uint
		expected uint
	}{
		{in: 0, expected: 0},
		{in: 1, expected: 1},
		{in: 2, expected: 2},
		{in: 3, expected: 4},
		{in: 4, expected: 4},
		{in: 5, expected: 8},
		{in: MaxHashesCount, expected: 512},
	} {
		t.Run(strconv.Itoa(int(tc.in)), func(t *testing.T) {
			require.Equal(t, tc.expected, roundToNextPowerOfTwo(tc.in))
		})
	}
}
