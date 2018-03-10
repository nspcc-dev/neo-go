package core

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestNewBlockchain(t *testing.T) {
	startHash, _ := util.Uint256DecodeString("996e37358dc369912041f966f8c5d8d3a8255ba5dcbd3447f8a82b55db869099")
	bc := NewBlockchain(nil, startHash)

	assert.Equal(t, uint32(0), bc.BlockHeight())
	assert.Equal(t, uint32(0), bc.HeaderHeight())
	assert.Equal(t, uint32(1), bc.storedHeaderCount)
	assert.Equal(t, startHash, bc.startHash)
}

func TestAddHeaders(t *testing.T) {
	bc := newTestBC()
	h1 := newBlock(1).Header()
	h2 := newBlock(2).Header()
	h3 := newBlock(3).Header()

	if err := bc.AddHeaders(h1, h2, h3); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 0, bc.blockCache.Len())
	assert.Equal(t, h3.Index, bc.HeaderHeight())
	assert.Equal(t, uint32(1), bc.storedHeaderCount)
	assert.Equal(t, uint32(0), bc.BlockHeight())
	assert.Equal(t, h3.Hash(), bc.CurrentHeaderHash())

	// Add them again, they should not be added.
	if err := bc.AddHeaders(h3, h2, h1); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, h3.Index, bc.HeaderHeight())
	assert.Equal(t, uint32(1), bc.storedHeaderCount)
	assert.Equal(t, uint32(0), bc.BlockHeight())
	assert.Equal(t, h3.Hash(), bc.CurrentHeaderHash())
}

func TestAddBlock(t *testing.T) {
	bc := newTestBC()
	blocks := []*Block{
		newBlock(1),
		newBlock(2),
		newBlock(3),
	}

	for i := 0; i < len(blocks); i++ {
		if err := bc.AddBlock(blocks[i]); err != nil {
			t.Fatal(err)
		}
	}

	lastBlock := blocks[len(blocks)-1]
	assert.Equal(t, 3, bc.blockCache.Len())
	assert.Equal(t, lastBlock.Index, bc.HeaderHeight())
	assert.Equal(t, lastBlock.Hash(), bc.CurrentHeaderHash())
	assert.Equal(t, uint32(1), bc.storedHeaderCount)

	if err := bc.persist(); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, lastBlock.Index, bc.BlockHeight())
	assert.Equal(t, lastBlock.Hash(), bc.CurrentHeaderHash())
	assert.Equal(t, 0, bc.blockCache.Len())
}

func newTestBC() *Blockchain {
	startHash, _ := util.Uint256DecodeString("a")
	bc := NewBlockchain(NewMemoryStore(), startHash)
	bc.verifyBlocks = false
	return bc
}
