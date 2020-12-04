package payload

import (
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	. "github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInventoryEncodeDecode(t *testing.T) {
	hashes := []Uint256{
		hash.Sha256([]byte("a")),
		hash.Sha256([]byte("b")),
	}
	inv := NewInventory(BlockType, hashes)

	testserdes.EncodeDecodeBinary(t, inv, new(Inventory))
}

func TestEmptyInv(t *testing.T) {
	msgInv := NewInventory(TXType, []Uint256{})

	data, err := testserdes.EncodeBinary(msgInv)
	assert.Nil(t, err)
	assert.Equal(t, []byte{byte(TXType), 0}, data)
	assert.Equal(t, 0, len(msgInv.Hashes))
}

func TestValid(t *testing.T) {
	require.True(t, TXType.Valid())
	require.True(t, BlockType.Valid())
	require.True(t, ConsensusType.Valid())
	require.False(t, InventoryType(0xFF).Valid())
}

func TestString(t *testing.T) {
	require.Equal(t, "TX", TXType.String())
	require.Equal(t, "block", BlockType.String())
	require.Equal(t, "consensus", ConsensusType.String())
	require.True(t, strings.Contains(InventoryType(0xFF).String(), "unknown"))
}
