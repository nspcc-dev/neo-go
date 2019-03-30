package peermgr

import (
	"errors"
	"sort"
	"sync"

	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

var (
	//ErrCacheLimit is returned when the cache limit is reached
	ErrCacheLimit = errors.New("nomore items can be added to the cache")

	//ErrNoItems is returned when pickItem is called and there are no items in the cache
	ErrNoItems = errors.New("there are no items in the cache")

	//ErrDuplicateItem is returned when you try to add the same item, more than once to the cache
	ErrDuplicateItem = errors.New("this item is already in the cache")
)

//BlockInfo holds the necessary information that the cache needs
// to sort and store block requests
type BlockInfo struct {
	BlockHash  util.Uint256
	BlockIndex uint64
}

// Equals returns true if two blockInfo objects
// have the same hash and the same index
func (bi *BlockInfo) Equals(other BlockInfo) bool {
	return bi.BlockHash.Equals(other.BlockHash) && bi.BlockIndex == other.BlockIndex
}

// indexSorter sorts the blockInfos by blockIndex.
type indexSorter []BlockInfo

func (is indexSorter) Len() int           { return len(is) }
func (is indexSorter) Swap(i, j int)      { is[i], is[j] = is[j], is[i] }
func (is indexSorter) Less(i, j int) bool { return is[i].BlockIndex < is[j].BlockIndex }

//blockCache will cache any pending block requests
// for the node when there are no available nodes
type blockCache struct {
	cacheLimit int
	cacheLock  sync.Mutex
	cache      []BlockInfo
}

func newBlockCache(cacheLimit int) *blockCache {
	return &blockCache{
		cache:      make([]BlockInfo, 0, cacheLimit),
		cacheLimit: cacheLimit,
	}
}

func (bc *blockCache) addBlockInfo(bi BlockInfo) error {
	if bc.cacheLen() == bc.cacheLimit {
		return ErrCacheLimit
	}

	bc.cacheLock.Lock()
	defer bc.cacheLock.Unlock()

	// Check for duplicates. slice will always be small so a simple for loop will work
	for _, bInfo := range bc.cache {
		if bInfo.Equals(bi) {
			return ErrDuplicateItem
		}
	}
	bc.cache = append(bc.cache, bi)

	sort.Sort(indexSorter(bc.cache))

	return nil
}

func (bc *blockCache) addBlockInfos(bis []BlockInfo) error {

	if len(bis)+bc.cacheLen() > bc.cacheLimit {
		return errors.New("too many items to add, this will exceed the cache limit")
	}

	for _, bi := range bis {
		err := bc.addBlockInfo(bi)
		if err != nil {
			return err
		}
	}
	return nil
}

func (bc *blockCache) cacheLen() int {
	bc.cacheLock.Lock()
	defer bc.cacheLock.Unlock()
	return len(bc.cache)
}

func (bc *blockCache) pickFirstItem() (BlockInfo, error) {
	return bc.pickItem(0)
}

func (bc *blockCache) pickAllItems() ([]BlockInfo, error) {

	numOfItems := bc.cacheLen()

	items := make([]BlockInfo, 0, numOfItems)

	for i := 0; i < numOfItems; i++ {
		bi, err := bc.pickFirstItem()
		if err != nil {
			return nil, err
		}
		items = append(items, bi)
	}
	return items, nil
}

func (bc *blockCache) pickItem(i uint) (BlockInfo, error) {
	if bc.cacheLen() < 1 {
		return BlockInfo{}, ErrNoItems
	}

	if i >= uint(bc.cacheLen()) {
		return BlockInfo{}, errors.New("index out of range")
	}

	bc.cacheLock.Lock()
	defer bc.cacheLock.Unlock()

	item := bc.cache[i]
	bc.cache = append(bc.cache[:i], bc.cache[i+1:]...)
	return item, nil
}

func (bc *blockCache) removeHash(hashToRemove util.Uint256) error {
	index, err := bc.findHash(hashToRemove)
	if err != nil {
		return err
	}

	_, err = bc.pickItem(uint(index))
	return err
}

func (bc *blockCache) findHash(hashToFind util.Uint256) (int, error) {
	bc.cacheLock.Lock()
	defer bc.cacheLock.Unlock()
	for i, bInfo := range bc.cache {
		if bInfo.BlockHash.Equals(hashToFind) {
			return i, nil
		}
	}
	return -1, errors.New("hash cannot be found in the cache")
}
