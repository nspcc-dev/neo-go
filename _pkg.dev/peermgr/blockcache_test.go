package peermgr

import (
	"math/rand"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/wire/util"
	"github.com/stretchr/testify/assert"
)

func TestAddBlock(t *testing.T) {

	bc := &blockCache{
		cacheLimit: 20,
	}
	bi := randomBlockInfo(t)

	err := bc.addBlockInfo(bi)
	assert.Equal(t, nil, err)

	assert.Equal(t, 1, bc.cacheLen())

	err = bc.addBlockInfo(bi)
	assert.Equal(t, ErrDuplicateItem, err)

	assert.Equal(t, 1, bc.cacheLen())
}

func TestCacheLimit(t *testing.T) {

	bc := &blockCache{
		cacheLimit: 20,
	}

	for i := 0; i < bc.cacheLimit; i++ {
		err := bc.addBlockInfo(randomBlockInfo(t))
		assert.Equal(t, nil, err)
	}

	err := bc.addBlockInfo(randomBlockInfo(t))
	assert.Equal(t, ErrCacheLimit, err)

	assert.Equal(t, bc.cacheLimit, bc.cacheLen())
}
func TestPickItem(t *testing.T) {

	bc := &blockCache{
		cacheLimit: 20,
	}

	for i := 0; i < bc.cacheLimit; i++ {
		err := bc.addBlockInfo(randomBlockInfo(t))
		assert.Equal(t, nil, err)
	}

	for i := 0; i < bc.cacheLimit; i++ {
		_, err := bc.pickFirstItem()
		assert.Equal(t, nil, err)
	}

	assert.Equal(t, 0, bc.cacheLen())
}

func randomUint256(t *testing.T) util.Uint256 {
	rand32 := make([]byte, 32)
	rand.Read(rand32)

	u, err := util.Uint256DecodeBytes(rand32)
	assert.Equal(t, nil, err)

	return u
}

func randomBlockInfo(t *testing.T) BlockInfo {

	return BlockInfo{
		randomUint256(t),
		rand.Uint32(),
	}
}
