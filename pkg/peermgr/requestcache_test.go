package peermgr

import (
	"crypto/rand"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/wire/util"
	"github.com/stretchr/testify/assert"
)

func TestAddBlock(t *testing.T) {

	rc := &requestCache{}
	hash := util.Uint256{0, 2, 3, 4}

	err := rc.addBlock(hash)
	assert.Equal(t, nil, err)

	assert.Equal(t, 1, rc.cacheLen())

	err = rc.addBlock(hash)
	assert.Equal(t, ErrDuplicateItem, err)

	assert.Equal(t, 1, rc.cacheLen())
}

func TestCacheLimit(t *testing.T) {

	rc := &requestCache{}

	for i := 0; i < cacheLimit; i++ {
		err := rc.addBlock(randomUint256(t))
		assert.Equal(t, nil, err)
	}

	err := rc.addBlock(randomUint256(t))
	assert.Equal(t, ErrCacheLimit, err)

	assert.Equal(t, cacheLimit, rc.cacheLen())
}
func TestPickItem(t *testing.T) {

	rc := &requestCache{}

	for i := 0; i < cacheLimit; i++ {
		err := rc.addBlock(randomUint256(t))
		assert.Equal(t, nil, err)
	}

	for i := 0; i < cacheLimit; i++ {
		_, err := rc.pickItem()
		assert.Equal(t, nil, err)
	}

	assert.Equal(t, 0, rc.cacheLen())
}

func randomUint256(t *testing.T) util.Uint256 {
	rand32 := make([]byte, 32)
	rand.Read(rand32)

	u, err := util.Uint256DecodeBytes(rand32)
	assert.Equal(t, nil, err)

	return u
}
