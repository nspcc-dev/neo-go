package core

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/CityOfZion/neo-go/pkg/util"
	log "github.com/sirupsen/logrus"
	"github.com/syndtr/goleveldb/leveldb"
)

// tuning parameters
const (
	secondsPerBlock  = 15
	headerBatchCount = 2000
)

var (
	genAmount       = []int{8, 7, 6, 5, 4, 3, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	persistInterval = 5 * time.Second
)

// Blockchain holds the chain.
type Blockchain struct {
	// Any object that satisfies the BlockchainStorer interface.
	Store

	// Current index/height of the highest block.
	// Read access should always be called by BlockHeight().
	// Writes access should only happen in persist().
	blockHeight uint32

	// Number of headers stored.
	storedHeaderCount uint32

	blockCache *Cache

	startHash util.Uint256

	// All operation on headerList must be called from an
	// headersOp to be routine safe.
	headerList *HeaderHashList

	// Only for operating on the headerList.
	headersOp     chan headersOpFunc
	headersOpDone chan struct{}

	// Whether we will verify received blocks.
	verifyBlocks bool
}

type headersOpFunc func(headerList *HeaderHashList)

// NewBlockchain creates a new Blockchain object.
func NewBlockchain(s Store, startHash util.Uint256) (*Blockchain, error) {
	bc := &Blockchain{
		Store:         s,
		headersOp:     make(chan headersOpFunc),
		headersOpDone: make(chan struct{}),
		startHash:     startHash,
		blockCache:    NewCache(),
		verifyBlocks:  false,
	}
	go bc.run()

	if err := bc.init(); err != nil {
		return nil, err
	}

	return bc, nil
}

func (bc *Blockchain) init() error {
	currBlockBytes, err := bc.Get(preSYSCurrentBlock.bytes())
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			// for the initial header, for now
			bc.headerList = NewHeaderHashList(bc.startHash)
			bc.storedHeaderCount = 1
			return nil
		}
		return err
	}

	bc.blockHeight = binary.LittleEndian.Uint32(currBlockBytes[32:36])
	headerList, err := readStoredHeaderList(bc.Store)
	if err != nil {
		return err
	}
	bc.headerList = NewHeaderHashList(headerList...)
	bc.storedHeaderCount = uint32(bc.headerList.Len())

	return nil
}

func (bc *Blockchain) run() {
	persistTimer := time.NewTimer(persistInterval)
	for {
		select {
		case op := <-bc.headersOp:
			op(bc.headerList)
			bc.headersOpDone <- struct{}{}
		case <-persistTimer.C:
			go bc.persist()
			persistTimer.Reset(persistInterval)
		}
	}
}

// AddBlock processes the given block and will add it to the cache so it
// can be persisted.
func (bc *Blockchain) AddBlock(block *Block) error {
	if !bc.blockCache.Has(block.Hash()) {
		bc.blockCache.Add(block.Hash(), block)
	}

	headerLen := bc.headerListLen()
	if int(block.Index-1) >= headerLen {
		return nil
	}
	if int(block.Index) == headerLen {
		if bc.verifyBlocks && !block.Verify(false) {
			return fmt.Errorf("block %s is invalid", block.Hash())
		}
		return bc.AddHeaders(block.Header())
	}
	return nil
}

// AddHeaders will process the given headers and add them to the
// HeaderHashList.
func (bc *Blockchain) AddHeaders(headers ...*Header) (err error) {
	var (
		start = time.Now()
		batch = new(leveldb.Batch)
	)

	bc.headersOp <- func(headerList *HeaderHashList) {
		for _, h := range headers {
			if int(h.Index-1) >= headerList.Len() {
				err = fmt.Errorf(
					"height of block higher then current header height %d > %d\n",
					h.Index, headerList.Len(),
				)
				return
			}
			if int(h.Index) < headerList.Len() {
				continue
			}
			if !h.Verify() {
				err = fmt.Errorf("header %v is invalid", h)
				return
			}
			if err = bc.processHeader(h, batch, headerList); err != nil {
				return
			}
		}

		if batch.Len() > 0 {
			if err = bc.PutBatch(batch); err != nil {
				return
			}
			log.WithFields(log.Fields{
				"headerIndex": headerList.Len() - 1,
				"blockHeight": bc.BlockHeight(),
				"took":        time.Since(start),
			}).Debug("done processing headers")
		}
	}
	<-bc.headersOpDone
	return err
}

// processHeader processes the given header. Note that this is only thread safe
// if executed in headers operation.
func (bc *Blockchain) processHeader(h *Header, batch Batch, headerList *HeaderHashList) error {
	headerList.Add(h.Hash())

	buf := new(bytes.Buffer)
	for int(h.Index)-headerBatchCount >= int(bc.storedHeaderCount) {
		if err := headerList.Write(buf, int(bc.storedHeaderCount), headerBatchCount); err != nil {
			return err
		}
		key := appendPrefixInt(preIXHeaderHashList, int(bc.storedHeaderCount))
		batch.Put(key, buf.Bytes())
		bc.storedHeaderCount += headerBatchCount
		buf.Reset()
	}

	buf.Reset()
	if err := h.EncodeBinary(buf); err != nil {
		return err
	}

	key := appendPrefix(preDataBlock, h.Hash().BytesReverse())
	batch.Put(key, buf.Bytes())
	batch.Put(preSYSCurrentHeader.bytes(), hashAndIndexToBytes(h.Hash(), h.Index))

	return nil
}

func (bc *Blockchain) persistBlock(block *Block) error {
	batch := new(leveldb.Batch)

	storeAsBlock(batch, block, 0)
	storeAsCurrentBlock(batch, block)

	if err := bc.PutBatch(batch); err != nil {
		return err
	}

	atomic.AddUint32(&bc.blockHeight, 1)
	return nil
}

func (bc *Blockchain) persist() (err error) {
	var (
		start     = time.Now()
		persisted = 0
		lenCache  = bc.blockCache.Len()
	)

	bc.headersOp <- func(headerList *HeaderHashList) {
		for i := 0; i < lenCache; i++ {
			if uint32(headerList.Len()) <= bc.BlockHeight() {
				return
			}
			hash := headerList.Get(int(bc.BlockHeight() + 1))
			if block, ok := bc.blockCache.GetBlock(hash); ok {
				if err = bc.persistBlock(block); err != nil {
					return
				}
				bc.blockCache.Delete(hash)
				persisted++
			}
		}
	}
	<-bc.headersOpDone

	if persisted > 0 {
		log.WithFields(log.Fields{
			"persisted":   persisted,
			"blockHeight": bc.BlockHeight(),
			"took":        time.Since(start),
		}).Info("blockchain persist completed")
	}

	return
}

func (bc *Blockchain) headerListLen() (n int) {
	bc.headersOp <- func(headerList *HeaderHashList) {
		n = headerList.Len()
	}
	<-bc.headersOpDone
	return
}

// GetBlock returns a Block by the given hash.
func (bc *Blockchain) GetBlock(hash util.Uint256) (*Block, error) {
	return nil, nil
}

// HasBlock return true if the blockchain contains he given
// transaction hash.
func (bc *Blockchain) HasTransaction(hash util.Uint256) bool {
	return false
}

// HasBlock return true if the blockchain contains the given
// block hash.
func (bc *Blockchain) HasBlock(hash util.Uint256) bool {
	return false
}

// CurrentBlockHash returns the heighest processed block hash.
func (bc *Blockchain) CurrentBlockHash() (hash util.Uint256) {
	bc.headersOp <- func(headerList *HeaderHashList) {
		hash = headerList.Get(int(bc.BlockHeight()))
	}
	<-bc.headersOpDone
	return
}

// CurrentHeaderHash returns the hash of the latest known header.
func (bc *Blockchain) CurrentHeaderHash() (hash util.Uint256) {
	bc.headersOp <- func(headerList *HeaderHashList) {
		hash = headerList.Last()
	}
	<-bc.headersOpDone
	return
}

// GetHeaderHash return the hash from the headerList by its
// height/index.
func (bc *Blockchain) GetHeaderHash(i int) (hash util.Uint256) {
	bc.headersOp <- func(headerList *HeaderHashList) {
		hash = headerList.Get(i)
	}
	<-bc.headersOpDone
	return
}

// BlockHeight returns the height/index of the highest block.
func (bc *Blockchain) BlockHeight() uint32 {
	return atomic.LoadUint32(&bc.blockHeight)
}

// HeaderHeight returns the index/height of the highest header.
func (bc *Blockchain) HeaderHeight() uint32 {
	return uint32(bc.headerListLen() - 1)
}

func hashAndIndexToBytes(h util.Uint256, index uint32) []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, index)
	return append(h.BytesReverse(), buf...)
}
