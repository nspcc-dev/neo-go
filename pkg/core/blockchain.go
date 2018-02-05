package core

import (
	"bytes"
	"encoding/binary"
	"log"
	"sync"
	"time"

	"github.com/CityOfZion/neo-go/pkg/util"
)

// tuning parameters
const (
	secondsPerBlock  = 15
	writeHdrBatchCnt = 2000
)

var (
	genAmount = []int{8, 7, 6, 5, 4, 3, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
)

// Blockchain holds the chain.
type Blockchain struct {
	logger *log.Logger

	// Any object that satisfies the BlockchainStorer interface.
	Store

	// current index of the heighest block
	currentBlockHeight uint32

	// number of headers stored
	storedHeaderCount uint32

	mtx sync.RWMutex

	// index of headers hashes
	headerIndex []util.Uint256
}

// NewBlockchain returns a pointer to a Blockchain.
func NewBlockchain(s Store, l *log.Logger, startHash util.Uint256) *Blockchain {
	bc := &Blockchain{
		logger: l,
		Store:  s,
	}

	// Starthash is 0, so we will create the genesis block.
	if startHash.Equals(util.Uint256{}) {
		bc.logger.Fatal("genesis block not yet implemented")
	}

	bc.headerIndex = []util.Uint256{startHash}

	return bc
}

// genesisBlock creates the genesis block for the chain.
// hash of the genesis block:
// d42561e3d30e15be6400b6df2f328e02d2bf6354c41dce433bc57687c82144bf
func (bc *Blockchain) genesisBlock() *Block {
	timestamp := uint32(time.Date(2016, 7, 15, 15, 8, 21, 0, time.UTC).Unix())

	// TODO: for testing I will hardcode the merkleroot.
	// This let's me focus on the bringing all the puzzle pieces
	// togheter much faster.
	// For more information about the genesis block:
	// https://neotracker.io/block/height/0
	mr, _ := util.Uint256DecodeFromString("803ff4abe3ea6533bcc0be574efa02f83ae8fdc651c879056b0d9be336c01bf4")

	return &Block{
		BlockBase: BlockBase{
			Version:       0,
			PrevHash:      util.Uint256{},
			MerkleRoot:    mr,
			Timestamp:     timestamp,
			Index:         0,
			ConsensusData: 2083236893,     // nioctib ^^
			NextConsensus: util.Uint160{}, // todo
		},
	}
}

// AddBlock (to be continued after headers is finished..)
func (bc *Blockchain) AddBlock(block *Block) error {
	// TODO: caching
	headerLen := len(bc.headerIndex)

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

func (bc *Blockchain) addHeader(header *Header) error {
	return bc.AddHeaders(header)
}

// AddHeaders processes the given headers.
func (bc *Blockchain) AddHeaders(headers ...*Header) error {
	start := time.Now()

	bc.mtx.Lock()
	defer bc.mtx.Unlock()

	batch := Batch{}
	for _, h := range headers {
		if int(h.Index-1) >= len(bc.headerIndex) {
			bc.logger.Printf("height of block higher then header index %d %d\n",
				h.Index, len(bc.headerIndex))
			break
		}
		if int(h.Index) < len(bc.headerIndex) {
			continue
		}
		if !h.Verify() {
			bc.logger.Printf("header %v is invalid", h)
			break
		}
		if err := bc.processHeader(h, batch); err != nil {
			return err
		}
	}

	// TODO: Implement caching strategy.
	if len(batch) > 0 {
		// Write all batches.
		if err := bc.writeBatch(batch); err != nil {
			return err
		}

		bc.logger.Printf("done processing headers up to index %d took %f Seconds",
			bc.HeaderHeight(), time.Since(start).Seconds())
	}

	return nil
}

// processHeader processes 1 header.
func (bc *Blockchain) processHeader(h *Header, batch Batch) error {
	hash, err := h.Hash()
	if err != nil {
		return err
	}
	bc.headerIndex = append(bc.headerIndex, hash)

	for int(h.Index)-writeHdrBatchCnt >= int(bc.storedHeaderCount) {
		// hdrsToWrite = bc.headerIndex[bc.storedHeaderCount : bc.storedHeaderCount+writeHdrBatchCnt]

		// NOTE: from original #c to be implemented:
		//
		// w.Write(header_index.Skip((int)stored_header_count).Take(2000).ToArray());
		// w.Flush();
		// batch.Put(SliceBuilder.Begin(DataEntryPrefix.IX_HeaderHashList).Add(stored_header_count), ms.ToArray());

		bc.storedHeaderCount += writeHdrBatchCnt
	}

	buf := new(bytes.Buffer)
	if err := h.EncodeBinary(buf); err != nil {
		return err
	}

	preBlock := preDataBlock.add(hash.ToSliceReverse())
	batch[&preBlock] = buf.Bytes()
	preHeader := preSYSCurrentHeader.toSlice()
	batch[&preHeader] = hashAndIndexToBytes(hash, h.Index)

	return nil
}

// CurrentBlockHash return the lastest hash in the header index.
func (bc *Blockchain) CurrentBlockHash() (hash util.Uint256) {
	if len(bc.headerIndex) == 0 {
		return
	}
	if len(bc.headerIndex) < int(bc.currentBlockHeight) {
		return
	}

	return bc.headerIndex[bc.currentBlockHeight]
}

// BlockHeight return the height/index of the latest block this node has.
func (bc *Blockchain) BlockHeight() uint32 {
	return bc.currentBlockHeight
}

// HeaderHeight returns the current index of the headers.
func (bc *Blockchain) HeaderHeight() uint32 {
	return uint32(len(bc.headerIndex)) - 1
}

func hashAndIndexToBytes(h util.Uint256, index uint32) []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, index)
	return append(h.ToSliceReverse(), buf...)
}
