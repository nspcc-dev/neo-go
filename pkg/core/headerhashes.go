package core

import (
	"fmt"
	"slices"
	"sync"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

const (
	headerBatchCount = 2000
	pagesCache       = 8
)

// HeaderHashes is a header hash manager part of the Blockchain. It can't be used
// without Blockchain.
type HeaderHashes struct {
	// Backing storage.
	dao *dao.Simple

	// Lock for all internal state fields.
	lock sync.RWMutex

	// The latest header hashes (storedHeaderCount+).
	latest []util.Uint256

	// Previously completed page of header hashes (pre-storedHeaderCount).
	previous []util.Uint256

	// Number of headers stored in the chain file.
	storedHeaderCount uint32

	// Cache for accessed pages of header hashes.
	cache *lru.Cache[uint32, []util.Uint256]
}

func (h *HeaderHashes) initMinTrustedHeader(dao *dao.Simple, trusted config.HashIndex) {
	h.dao = dao
	h.cache, _ = lru.New[uint32, []util.Uint256](pagesCache) // Never errors for positive size.
	h.previous = make([]util.Uint256, headerBatchCount)
	h.latest = make([]util.Uint256, 0, headerBatchCount)

	// For non-genesis block, trusted header is not yet in the storage. Use a
	// stub and pretend that the latest header is `trusted.Index-1` to fetch
	// the next one (trusted) from the network.
	if trusted.Index > 0 {
		trusted.Index--
		trusted.Hash = util.Uint256{}
	}

	for range trusted.Index % headerBatchCount {
		h.latest = append(h.latest, util.Uint256{})
	}
	h.latest = append(h.latest, trusted.Hash)
	dao.PutCurrentHeader(trusted.Hash, trusted.Index)
	h.storedHeaderCount = (trusted.Index /*trusted header is not yet in the storage*/ / headerBatchCount) * headerBatchCount

	// Store trusted header if it's the last header in the batch and update storedHeaderCount.
	_ = h.tryStoreBatch(dao) // ignore serialization error.

	updateHeaderHeightMetric(trusted.Index)
}

func (h *HeaderHashes) init(dao *dao.Simple, trusted config.HashIndex) error {
	currHeaderHeight, currHeaderHash, err := dao.GetCurrentHeaderHeight()
	if err != nil {
		return fmt.Errorf("failed to retrieve current header info: %w", err)
	}
	if currHeaderHeight < trusted.Index {
		h.initMinTrustedHeader(dao, trusted)
		return nil
	}

	h.dao = dao
	h.cache, _ = lru.New[uint32, []util.Uint256](pagesCache) // Never errors for positive size.
	h.storedHeaderCount = ((currHeaderHeight + 1) / headerBatchCount) * headerBatchCount
	missingHeaderCount := ((trusted.Index + 1) / headerBatchCount) * headerBatchCount
	if h.storedHeaderCount >= headerBatchCount &&
		((h.storedHeaderCount > missingHeaderCount && h.storedHeaderCount-missingHeaderCount >= headerBatchCount) ||
			currHeaderHeight%headerBatchCount != trusted.Index%headerBatchCount) {
		h.previous, err = h.dao.GetHeaderHashes(h.storedHeaderCount - headerBatchCount)
		if err != nil {
			return fmt.Errorf("failed to retrieve header hash page %d: %w; stored: %d, missing: %d, trusted: %d, curr: %d", h.storedHeaderCount-headerBatchCount, err, h.storedHeaderCount, missingHeaderCount, trusted.Index, currHeaderHeight)
		}
	} else {
		h.previous = make([]util.Uint256, headerBatchCount)
	}
	h.latest = make([]util.Uint256, 0, headerBatchCount)

	// There is a high chance that the Node is stopped before the next
	// batch of 2000 headers was stored. Via the currentHeaders stored we can sync
	// that with stored blocks.
	if currHeaderHeight >= h.storedHeaderCount {
		var (
			hash       = currHeaderHash
			targetHash util.Uint256
			padLeft    bool
		)
		if h.storedHeaderCount >= headerBatchCount {
			targetHash = h.previous[len(h.previous)-1]
		}
		if targetHash.Equals(util.Uint256{}) && trusted.Index > 0 {
			// Don't retrieve header hashes prior to trusted header (if set) since sometimes
			// these blocks may be missing from the storage (if previously node was started
			// from existing DB with some trusted point higher than header height). These hashes
			// are useless for further node operation anyway.
			targetHash = trusted.Hash
			padLeft = true
		}
		headers := make([]util.Uint256, 0, headerBatchCount)

		for hash != targetHash {
			blk, err := h.dao.GetBlock(hash)
			if err != nil {
				return fmt.Errorf("could not get header %s: %w", hash, err)
			}
			headers = append(headers, blk.Hash())
			hash = blk.PrevHash
		}
		slices.Reverse(headers)
		if padLeft {
			h.latest = h.latest[:currHeaderHeight-uint32(len(headers))]
			h.latest = append(h.latest, trusted.Hash)
		}
		h.latest = append(h.latest, headers...)
	}
	return nil
}

func (h *HeaderHashes) lastHeaderIndex() uint32 {
	return h.storedHeaderCount + uint32(len(h.latest)) - 1
}

// HeaderHeight returns the index/height of the highest header.
func (h *HeaderHashes) HeaderHeight() uint32 {
	h.lock.RLock()
	n := h.lastHeaderIndex()
	h.lock.RUnlock()
	return n
}

func (h *HeaderHashes) addHeaders(headers ...*block.Header) error {
	var (
		batch      = h.dao.GetPrivate()
		lastHeader *block.Header
		err        error
	)

	h.lock.Lock()
	defer h.lock.Unlock()

	for _, head := range headers {
		if head.Index != h.lastHeaderIndex()+1 {
			continue
		}
		err = batch.StoreHeader(head)
		if err != nil {
			return err
		}
		lastHeader = head
		h.latest = append(h.latest, head.Hash())
		err = h.tryStoreBatch(batch)
		if err != nil {
			return fmt.Errorf("failed to store batch of header hashes: %w", err)
		}
	}
	if lastHeader != nil {
		batch.PutCurrentHeader(lastHeader.Hash(), lastHeader.Index)
		updateHeaderHeightMetric(lastHeader.Index)
		if _, err = batch.Persist(); err != nil {
			return err
		}
	}
	return nil
}

// tryStoreBatch stores the current batch of header hashes to the storage in
// case if batch is full. It also initializes HeaderHashes for the next batch
// processing. It does not persist dao.
func (h *HeaderHashes) tryStoreBatch(d *dao.Simple) error {
	if len(h.latest) == headerBatchCount {
		err := d.StoreHeaderHashes(h.latest, h.storedHeaderCount)
		if err != nil {
			return err
		}
		copy(h.previous, h.latest)
		h.latest = h.latest[:0]
		h.storedHeaderCount += headerBatchCount
	}
	return nil
}

// CurrentHeaderHash returns the hash of the latest known header.
func (h *HeaderHashes) CurrentHeaderHash() util.Uint256 {
	var hash util.Uint256

	h.lock.RLock()
	if len(h.latest) > 0 {
		hash = h.latest[len(h.latest)-1]
	} else {
		hash = h.previous[len(h.previous)-1]
	}
	h.lock.RUnlock()
	return hash
}

// GetHeaderHash returns hash of the header/block with specified index, if
// HeaderHashes doesn't have a hash for this height, zero Uint256 value is returned.
func (h *HeaderHashes) GetHeaderHash(i uint32) util.Uint256 {
	h.lock.RLock()
	res, ok := h.getLocalHeaderHash(i)
	h.lock.RUnlock()
	if ok {
		return res
	}
	// If it's not in the latest/previous, then it's in the cache or DB, those
	// need no additional locks.
	page := (i / headerBatchCount) * headerBatchCount
	hashes, ok := h.cache.Get(page)
	if ok {
		return hashes[i-page]
	}
	hashes, err := h.dao.GetHeaderHashes(page)
	if err != nil {
		return util.Uint256{}
	}
	_ = h.cache.Add(page, hashes)
	return hashes[i-page]
}

// getLocalHeaderHash looks for the index in the latest and previous caches.
// Locking is left to the user.
func (h *HeaderHashes) getLocalHeaderHash(i uint32) (util.Uint256, bool) {
	if i > h.lastHeaderIndex() {
		return util.Uint256{}, false
	}
	if i >= h.storedHeaderCount {
		return h.latest[i-h.storedHeaderCount], true
	}
	previousStored := h.storedHeaderCount - headerBatchCount
	if i >= previousStored {
		return h.previous[i-previousStored], true
	}
	return util.Uint256{}, false
}

func (h *HeaderHashes) haveRecentHash(hash util.Uint256, i uint32) bool {
	h.lock.RLock()
	defer h.lock.RUnlock()
	for ; i > 0; i-- {
		lh, ok := h.getLocalHeaderHash(i)
		if ok && hash.Equals(lh) {
			return true
		}
	}
	return false
}
