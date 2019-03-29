package peermgr

import (
	"errors"
	"sync"

	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

const cacheLimit = 20

var (
	//ErrCacheLimit is returned when the cache limit is reached
	ErrCacheLimit = errors.New("nomore items can be added to the cache")

	//ErrNoItems is returned when pickItem is called and there are no items in the cache
	ErrNoItems = errors.New("there are no items in the cache")

	//ErrDuplicateItem is returned when you try to add the same item, more than once to the cache
	ErrDuplicateItem = errors.New("this item is already in the cache")
)

//requestCache will cache any pending block requests
// for the node when there are no available nodes
type requestCache struct {
	cacheLock  sync.Mutex
	blockCache []util.Uint256
}

func (rc *requestCache) addBlock(newHash util.Uint256) error {
	if rc.cacheLen() == cacheLimit {
		return ErrCacheLimit
	}

	rc.cacheLock.Lock()
	defer rc.cacheLock.Unlock()

	// Check for duplicates. slice will always be small so a simple for loop will work
	for _, hash := range rc.blockCache {
		if hash.Equals(newHash) {
			return ErrDuplicateItem
		}
	}

	rc.blockCache = append(rc.blockCache, newHash)

	return nil
}

func (rc *requestCache) cacheLen() int {
	rc.cacheLock.Lock()
	defer rc.cacheLock.Unlock()
	return len(rc.blockCache)
}

func (rc *requestCache) pickItem() (util.Uint256, error) {
	if rc.cacheLen() < 1 {
		return util.Uint256{}, ErrNoItems
	}

	rc.cacheLock.Lock()
	defer rc.cacheLock.Unlock()

	item := rc.blockCache[0]
	rc.blockCache = rc.blockCache[1:]
	return item, nil
}
