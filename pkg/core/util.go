package core

import (
	"bytes"
	"encoding/binary"
	"sort"

	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// Utilities for quick bootstrapping blockchains. Normally we should
// create the genisis block. For now (to speed up development) we will add
// The hashes manually.

func GenesisHashPrivNet() util.Uint256 {
	hash, _ := util.Uint256DecodeString("996e37358dc369912041f966f8c5d8d3a8255ba5dcbd3447f8a82b55db869099")
	return hash
}

func GenesisHashTestNet() util.Uint256 {
	hash, _ := util.Uint256DecodeString("b3181718ef6167105b70920e4a8fbbd0a0a56aacf460d70e10ba6fa1668f1fef")
	return hash
}

func GenesisHashMainNet() util.Uint256 {
	hash, _ := util.Uint256DecodeString("d42561e3d30e15be6400b6df2f328e02d2bf6354c41dce433bc57687c82144bf")
	return hash
}

// headerSliceReverse reverses the given slice of *Header.
func headerSliceReverse(dest []*Header) {
	for i, j := 0, len(dest)-1; i < j; i, j = i+1, j-1 {
		dest[i], dest[j] = dest[j], dest[i]
	}
}

// storeAsCurrentBlock stores the given block witch prefix
// SYSCurrentBlock.
func storeAsCurrentBlock(batch storage.Batch, block *Block) {
	buf := new(bytes.Buffer)
	buf.Write(block.Hash().BytesReverse())
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, block.Index)
	buf.Write(b)
	batch.Put(storage.SYSCurrentBlock.Bytes(), buf.Bytes())
}

// storeAsBlock stores the given block as DataBlock.
func storeAsBlock(batch storage.Batch, block *Block, sysFee uint32) error {
	var (
		key = storage.AppendPrefix(storage.DataBlock, block.Hash().BytesReverse())
		buf = new(bytes.Buffer)
	)

	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, sysFee)

	b, err := block.Trim()
	if err != nil {
		return err
	}
	buf.Write(b)
	batch.Put(key, buf.Bytes())
	return nil
}

// storeAsTransaction stores the given TX as DataTransaction.
func storeAsTransaction(batch storage.Batch, tx *transaction.Transaction, index uint32) error {
	key := storage.AppendPrefix(storage.DataTransaction, tx.Hash().BytesReverse())
	buf := new(bytes.Buffer)
	if err := tx.EncodeBinary(buf); err != nil {
		return err
	}

	dest := make([]byte, buf.Len()+4)
	binary.LittleEndian.PutUint32(dest[:4], index)
	copy(dest[4:], buf.Bytes()) 
	batch.Put(key, dest)

	return nil
}

// readStoredHeaderHashes returns a sorted list of header hashes
// retrieved from the given Store.
func readStoredHeaderHashes(store storage.Store) ([]util.Uint256, error) {
	hashMap := make(map[uint32][]util.Uint256)
	store.Seek(storage.IXHeaderHashList.Bytes(), func(k, v []byte) {
		storedCount := binary.LittleEndian.Uint32(k[1:])
		hashes, err := util.Read2000Uint256Hashes(v)
		if err != nil {
			panic(err)
		}
		hashMap[storedCount] = hashes
	})

	var (
		i          = 0
		sortedKeys = make([]int, len(hashMap))
	)

	for k, _ := range hashMap {
		sortedKeys[i] = int(k)
		i++
	}
	sort.Ints(sortedKeys)

	hashes := []util.Uint256{}
	for _, key := range sortedKeys {
		values := hashMap[uint32(key)]
		for _, hash := range values {
			hashes = append(hashes, hash)
		}
	}

	return hashes, nil
}
