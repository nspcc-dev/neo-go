package core

import (
	"bytes"
	"encoding/binary"
	"errors"
)

var errKeyNotFound = errors.New("key not found")

type (
	// Store is anything that can persist and retrieve the blockchain.
	// information.
	Store interface {
		Get([]byte) ([]byte, error)
		Put(k, v []byte) error
		PutBatch(batch Batch) error
		Find(k []byte, f func(k, v []byte))
	}

	// Batch represents an abstraction on top of batch operations.
	// Each Store implementation is responsible of casting a Batch
	// to its appropriate type.
	Batch interface {
		Put(k, v []byte)
	}

	// keyPrefix is a constant byte added as a prefix for each key
	// stored.
	keyPrefix uint8
)

func (k keyPrefix) bytes() []byte {
	return []byte{byte(k)}
}

// valid dataPrefix constants.
const (
	preDataBlock         keyPrefix = 0x01
	preDataTransaction   keyPrefix = 0x02
	preSTAccount         keyPrefix = 0x40
	preSTCoin            keyPrefix = 0x44
	preSTValidator       keyPrefix = 0x48
	preSTAsset           keyPrefix = 0x4c
	preSTContract        keyPrefix = 0x50
	preSTStorage         keyPrefix = 0x70
	preIXHeaderHashList  keyPrefix = 0x80
	preIXValidatorsCount keyPrefix = 0x90
	preSYSCurrentBlock   keyPrefix = 0xc0
	preSYSCurrentHeader  keyPrefix = 0xc1
	preSYSVersion        keyPrefix = 0xf0
)

func appendPrefix(k keyPrefix, b []byte) []byte {
	dest := make([]byte, len(b)+1)
	dest[0] = byte(k)
	copy(dest[1:], b)
	return dest
}

func appendPrefixInt(k keyPrefix, n int) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, uint32(n))
	return appendPrefix(k, b)
}

func storeAsCurrentBlock(batch Batch, block *Block) {
	buf := new(bytes.Buffer)
	buf.Write(block.Hash().BytesReverse())
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, block.Index)
	buf.Write(b)
	batch.Put(preSYSCurrentBlock.bytes(), buf.Bytes())
}

func storeAsBlock(batch Batch, block *Block, sysFee uint32) error {
	var (
		key = appendPrefix(preDataBlock, block.Hash().BytesReverse())
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
