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
	require.True(t, TXType.Valid(false))
	require.True(t, TXType.Valid(true))
	require.True(t, BlockType.Valid(false))
	require.True(t, BlockType.Valid(true))
	require.True(t, ExtensibleType.Valid(false))
	require.True(t, ExtensibleType.Valid(true))
	require.False(t, P2PNotaryRequestType.Valid(false))
	require.True(t, P2PNotaryRequestType.Valid(true))
	require.False(t, InventoryType(0xFF).Valid(false))
	require.False(t, InventoryType(0xFF).Valid(true))
}

func TestString(t *testing.T) {
	require.Equal(t, "TX", TXType.String())
	require.Equal(t, "block", BlockType.String())
	require.Equal(t, "extensible", ExtensibleType.String())
	require.Equal(t, "p2pNotaryRequest", P2PNotaryRequestType.String())
	require.True(t, strings.Contains(InventoryType(0xFF).String(), "unknown"))
}
