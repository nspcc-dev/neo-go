package core

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"time"

	"github.com/CityOfZion/neo-go/pkg/util"
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
	logger *log.Logger

	// Any object that satisfies the BlockchainStorer interface.
	Store

	// Current index/height of the heighest block.
	blockHeight uint32

	// Number of headers stored.
	storedHeaderCount uint32

	// List of known headers.
	headerList []util.Uint256

	addHeadersCh chan addHeadersTuple
}

type addHeadersTuple struct {
	headers []*Header
	err     chan error
}

// NewBlockchain creates a new Blockchain object.
func NewBlockchain(s Store, l *log.Logger, startHash util.Uint256) *Blockchain {
	bc := &Blockchain{
		logger:       l,
		Store:        s,
		addHeadersCh: make(chan addHeadersTuple),
	}
	go bc.run()

	bc.headerList = []util.Uint256{startHash}

	return bc
}

func (bc *Blockchain) run() {
	for {
		select {
		case t := <-bc.addHeadersCh:
			t.err <- bc.addHeaders(t.headers...)
		}
	}
}

// AddBlock (to be continued after headers is finished..)
func (bc *Blockchain) AddBlock(block *Block) error {
	// TODO: caching
	headerLen := len(bc.headerList)

	if int(block.Index-1) >= headerLen {
		return nil
	}
	if int(block.Index) == headerLen {
		// todo: if (VerifyBlocks && !block.Verify()) return false;
	}
	if int(block.Index) < headerLen {
		return nil
	}

	return nil
}

// AddHeaders processes the given headers.
func (bc *Blockchain) AddHeaders(headers ...*Header) error {
	t := addHeadersTuple{
		headers: headers,
		err:     make(chan error),
	}
	bc.addHeadersCh <- t
	return <-t.err
}

func (bc *Blockchain) addHeaders(headers ...*Header) (err error) {
	var (
		start = time.Now()
		batch = Batch{}
	)

	for _, h := range headers {
		if int(h.Index-1) >= len(bc.headerList) {
			err = fmt.Errorf(
				"height of block higher then headerList %d > %d\n",
				h.Index, len(bc.headerList),
			)
			break
		}
		if int(h.Index) < len(bc.headerList) {
			continue
		}
		if !h.Verify() {
			err = fmt.Errorf("header %v is invalid", h)
			break
		}
		if err = bc.processHeader(h, batch); err != nil {
			break
		}
	}

	// TODO: Implement caching strategy.
	if len(batch) > 0 {
		if err = bc.writeBatch(batch); err != nil {
			return
		}
		bc.logger.Printf(
			"done processing headers up to index %d took %f Seconds",
			bc.HeaderHeight(), time.Since(start).Seconds(),
		)
	}

	return err
}

// processHeader processes the given header. Note that this is only thread
// safe if executed in headers operation.
func (bc *Blockchain) processHeader(h *Header, batch Batch) error {
	bc.headerList = append(bc.headerList, h.Hash())

	for int(h.Index)-headerBatchCount >= int(bc.storedHeaderCount) {
		// hdrsToWrite = bc.headerList[bc.storedHeaderCount : bc.storedHeaderCount+writeHdrBatchCnt]

		// NOTE: from original #c to be implemented:
		//
		// w.Write(header_index.Skip((int)stored_header_count).Take(2000).ToArray());
		// w.Flush();
		// batch.Put(SliceBuilder.Begin(DataEntryPrefix.IX_HeaderHashList).Add(stored_header_count), ms.ToArray());

		bc.storedHeaderCount += headerBatchCount
	}

	buf := new(bytes.Buffer)
	if err := h.EncodeBinary(buf); err != nil {
		return err
	}

	preBlock := preDataBlock.add(h.Hash().BytesReverse())
	batch[&preBlock] = buf.Bytes()
	preHeader := preSYSCurrentHeader.toSlice()
	batch[&preHeader] = hashAndIndexToBytes(h.Hash(), h.Index)

	return nil
}

// CurrentBlockHash return the lastest hash in the header index.
func (bc *Blockchain) CurrentBlockHash() (hash util.Uint256) {
	if len(bc.headerList) == 0 {
		return
	}
	if len(bc.headerList) < int(bc.blockHeight) {
		return
	}

	return bc.headerList[bc.blockHeight]
}

// CurrentHeaderHash returns the hash of the latest known header.
func (bc *Blockchain) CurrentHeaderHash() util.Uint256 {
	return bc.headerList[len(bc.headerList)-1]
}

// BlockHeight return the height/index of the latest block this node has.
func (bc *Blockchain) BlockHeight() uint32 {
	return bc.blockHeight
}

// HeaderHeight returns the index/height of the heighest header.
func (bc *Blockchain) HeaderHeight() uint32 {
	return uint32(len(bc.headerList)) - 1
}

func hashAndIndexToBytes(h util.Uint256, index uint32) []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, index)
	return append(h.BytesReverse(), buf...)
}
