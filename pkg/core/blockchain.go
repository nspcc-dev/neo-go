package core

import (
	"fmt"
	"log"
	"time"

	"github.com/CityOfZion/neo-go/pkg/util"
)

// tuning parameters
const (
	secondsPerBlock = 15
)

var (
	genAmount = []int{8, 7, 6, 5, 4, 3, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
)

// Blockchain holds the chain.
type Blockchain struct {
	// Any object that satisfies the BlockchainStorer interface.
	BlockchainStorer

	// index of the latest block.
	currentBlockHeight uint32

	// index of headers hashes
	headerIndex []util.Uint256
}

// NewBlockchain returns a pointer to a Blockchain.
func NewBlockchain(store BlockchainStorer) *Blockchain {
	hash, _ := util.Uint256DecodeFromString("0f654eb45164f08ddf296f7315d781f8b5a669c4d4b68f7265ffa79eeb455ed7")
	return &Blockchain{
		BlockchainStorer: store,
		headerIndex:      []util.Uint256{hash},
	}
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

// AddBlock ..
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
	var (
		count      = 0
		newHeaders = []*Header{}
	)

	fmt.Printf("received header, processing %d headers\n", len(headers))

	for i := 0; i < len(headers); i++ {
		h := headers[i]
		if int(h.Index-1) >= len(bc.headerIndex)+count {
			log.Printf("height of block higher then header index %d %d\n",
				h.Index, len(bc.headerIndex))
			break
		}

		if int(h.Index) < count+len(bc.headerIndex) {
			continue
		}

		count++

		newHeaders = append(newHeaders, h)
	}

	log.Println("done processing the headers")

	if len(newHeaders) > 0 {
		return bc.processHeaders(newHeaders)
	}

	return nil

	// hash, err := header.Hash()
	// if err != nil {
	// 	return err
	// }

	// bc.headerIndex = append(bc.headerIndex, hash)

	// return bc.Put(header)
}

func (bc *Blockchain) processHeaders(headers []*Header) error {
	lastHeader := headers[len(headers)-1:]

	for _, h := range headers {
		hash, err := h.Hash()
		if err != nil {
			return err
		}
		bc.headerIndex = append(bc.headerIndex, hash)
	}

	if lastHeader != nil {
		fmt.Println(lastHeader)
	}

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
