package transaction

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupInputsByPrevHash0(t *testing.T) {
	inputs := make([]Input, 0)
	res := GroupInputsByPrevHash(inputs)
	require.Equal(t, 0, len(res))
}

func TestGroupInputsByPrevHash1(t *testing.T) {
	inputs := make([]Input, 0)
	hash, err := util.Uint256DecodeStringLE("46168f963d6d8168a870405f66cc9e13a235791013b8ee2f90cc20a8293bd1af")
	require.NoError(t, err)
	inputs = append(inputs, Input{PrevHash: hash, PrevIndex: 42})
	res := GroupInputsByPrevHash(inputs)
	require.Equal(t, 1, len(res))
	require.Equal(t, 1, len(res[0]))
	assert.Equal(t, hash, res[0][0].PrevHash)
	assert.Equal(t, uint16(42), res[0][0].PrevIndex)
}

func TestGroupInputsByPrevHashMany(t *testing.T) {
	hash1, err := util.Uint256DecodeStringBE("a83ba6ede918a501558d3170a124324aedc89909e64c4ff2c6f863094f980b25")
	require.NoError(t, err)
	hash2, err := util.Uint256DecodeStringBE("629397158f852e838077bb2715b13a2e29b0a51c2157e5466321b70ed7904ce9")
	require.NoError(t, err)
	hash3, err := util.Uint256DecodeStringBE("caa41245c3e48ddc13dabe989ba8fbc59418e9228fef9efb62855b0b17d7448b")
	require.NoError(t, err)
	inputs := make([]Input, 0)
	for i := 0; i < 10; i++ {
		inputs = append(inputs, Input{PrevHash: hash1, PrevIndex: uint16(i)})
		inputs = append(inputs, Input{PrevHash: hash2, PrevIndex: uint16(i)})
		inputs = append(inputs, Input{PrevHash: hash3, PrevIndex: uint16(i)})
	}
	for i := 15; i < 20; i++ {
		inputs = append(inputs, Input{PrevHash: hash3, PrevIndex: uint16(i)})
	}
	for i := 10; i < 15; i++ {
		inputs = append(inputs, Input{PrevHash: hash2, PrevIndex: uint16(i)})
		inputs = append(inputs, Input{PrevHash: hash3, PrevIndex: uint16(i)})
	}
	seen := make(map[uint16]bool)
	res := GroupInputsByPrevHash(inputs)
	require.Equal(t, 3, len(res))
	assert.Equal(t, hash2, res[0][0].PrevHash)
	assert.Equal(t, 15, len(res[0]))
	for i := range res[0] {
		assert.Equal(t, res[0][i].PrevHash, res[0][0].PrevHash)
		assert.Equal(t, false, seen[res[0][i].PrevIndex])
		seen[res[0][i].PrevIndex] = true
	}
	seen = make(map[uint16]bool)
	assert.Equal(t, hash1, res[1][0].PrevHash)
	assert.Equal(t, 10, len(res[1]))
	for i := range res[1] {
		assert.Equal(t, res[1][i].PrevHash, res[1][0].PrevHash)
		assert.Equal(t, false, seen[res[1][i].PrevIndex])
		seen[res[1][i].PrevIndex] = true
	}
	seen = make(map[uint16]bool)
	assert.Equal(t, hash3, res[2][0].PrevHash)
	assert.Equal(t, 20, len(res[2]))
	for i := range res[2] {
		assert.Equal(t, res[2][i].PrevHash, res[2][0].PrevHash)
		assert.Equal(t, false, seen[res[2][i].PrevIndex])
		seen[res[2][i].PrevIndex] = true
	}
}
