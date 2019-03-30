package syncmgr

import (
	"sort"

	"github.com/CityOfZion/neo-go/pkg/wire/payload"
)

func (s *Syncmgr) addToBlockPool(newBlock payload.Block) {
	s.poolLock.Lock()
	defer s.poolLock.Unlock()

	for _, block := range s.blockPool {
		if block.Index == newBlock.Index {
			return
		}
	}

	s.blockPool = append(s.blockPool, newBlock)

	// sort slice using block index
	sort.Slice(s.blockPool, func(i, j int) bool {
		return s.blockPool[i].Index < s.blockPool[j].Index
	})

}

func (s *Syncmgr) checkPool() error {
	// Assuming that the blocks are sorted in order

	var indexesToRemove = -1

	s.poolLock.Lock()
	defer func() {
		// removes all elements before this index, including the element at this index
		s.blockPool = s.blockPool[indexesToRemove+1:]
		s.poolLock.Unlock()
	}()

	// loop iterates through the cache, processing any
	// blocks that can be added to the chain
	for i, block := range s.blockPool {
		if s.nextBlockIndex != block.Index {
			break
		}

		// Save this block and save the indice location so we can remove it, when we defer
		err := s.processBlock(block)
		if err != nil {
			return err
		}

		indexesToRemove = i
	}

	return nil
}
