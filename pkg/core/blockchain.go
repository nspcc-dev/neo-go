package core

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/CityOfZion/neo-go/pkg/util"
	log "github.com/go-kit/kit/log"
)

// tuning parameters
const (
	secondsPerBlock  = 15
	headerBatchCount = 2000
)

var (
	genAmount = []int{8, 7, 6, 5, 4, 3, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
)

// Blockchain holds the chain.
type Blockchain struct {
	logger log.Logger

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

	// Only for operating on the headerList.
	headersOp     chan headersOpFunc
	headersOpDone chan struct{}

	// Whether we will verify received blocks.
	verifyBlocks bool
}

type headersOpFunc func(headerList *HeaderHashList)

// NewBlockchain creates a new Blockchain object.
func NewBlockchain(s Store, startHash util.Uint256) *Blockchain {
	logger := log.NewLogfmtLogger(os.Stderr)
	logger = log.With(logger, "component", "blockchain")

	bc := &Blockchain{
		logger:        logger,
		Store:         s,
		headersOp:     make(chan headersOpFunc),
		headersOpDone: make(chan struct{}),
		startHash:     startHash,
		blockCache:    NewCache(),
		verifyBlocks:  true,
	}
	go bc.run()
	bc.init()

	return bc
}

func (bc *Blockchain) init() {
	// for the initial header, for now
	bc.storedHeaderCount = 1
}

func (bc *Blockchain) run() {
	headerList := NewHeaderHashList(bc.startHash)
	for {
		select {
		case op := <-bc.headersOp:
			op(headerList)
			bc.headersOpDone <- struct{}{}
		}
	}
}

func (bc *Blockchain) AddBlock(block *Block) error {
	if !bc.blockCache.Has(block.Hash()) {
		bc.blockCache.Add(block.Hash(), block)
	}

	headerLen := int(bc.HeaderHeight() + 1)
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

func (bc *Blockchain) AddHeaders(headers ...*Header) (err error) {
	var (
		start = time.Now()
		batch = Batch{}
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

		// TODO: Implement caching strategy.
		if len(batch) > 0 {
			if err = bc.writeBatch(batch); err != nil {
				return
			}
			bc.logger.Log(
				"msg", "done processing headers",
				"index", headerList.Len()-1,
				"took", time.Since(start).Seconds(),
			)
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
		key := makeEntryPrefixInt(preIXHeaderHashList, int(bc.storedHeaderCount))
		batch[&key] = buf.Bytes()
		bc.storedHeaderCount += headerBatchCount
		buf.Reset()
	}

	buf.Reset()
	if err := h.EncodeBinary(buf); err != nil {
		return err
	}

	key := makeEntryPrefix(preDataBlock, h.Hash().BytesReverse())
	batch[&key] = buf.Bytes()
	key = preSYSCurrentHeader.bytes()
	batch[&key] = hashAndIndexToBytes(h.Hash(), h.Index)

	return nil
}

func (bc *Blockchain) persistBlock(block *Block) error {
	bc.blockHeight = block.Index
	return nil
}

func (bc *Blockchain) persist() (err error) {
	var (
		persisted = 0
		lenCache  = bc.blockCache.Len()
	)

	for lenCache > persisted {
		if bc.HeaderHeight()+1 <= bc.BlockHeight() {
			break
		}
		bc.headersOp <- func(headerList *HeaderHashList) {
			hash := headerList.Get(int(bc.BlockHeight() + 1))
			if block, ok := bc.blockCache.GetBlock(hash); ok {
				if err = bc.persistBlock(block); err != nil {
					return
				}
				bc.blockCache.Delete(hash)
				persisted++
			} else {
				bc.logger.Log(
					"msg", "block not found in cache",
					"hash", block.Hash(),
				)
			}
		}
		<-bc.headersOpDone
	}
	return
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

// BlockHeight returns the height/index of the highest block.
func (bc *Blockchain) BlockHeight() uint32 {
	return atomic.LoadUint32(&bc.blockHeight)
}

// HeaderHeight returns the index/height of the highest header.
func (bc *Blockchain) HeaderHeight() (n uint32) {
	bc.headersOp <- func(headerList *HeaderHashList) {
		n = uint32(headerList.Len() - 1)
	}
	<-bc.headersOpDone
	return
}

func hashAndIndexToBytes(h util.Uint256, index uint32) []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, index)
	return append(h.BytesReverse(), buf...)
}
