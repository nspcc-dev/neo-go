package storage

import (
	"encoding/binary"
	"errors"
)

// KeyPrefix constants.
const (
	DataBlock         KeyPrefix = 0x01
	DataTransaction   KeyPrefix = 0x02
	STAccount         KeyPrefix = 0x40
	STCoin            KeyPrefix = 0x44
	STSpentCoin       KeyPrefix = 0x45
	STValidator       KeyPrefix = 0x48
	STAsset           KeyPrefix = 0x4c
	STContract        KeyPrefix = 0x50
	STStorage         KeyPrefix = 0x70
	IXHeaderHashList  KeyPrefix = 0x80
	IXValidatorsCount KeyPrefix = 0x90
	SYSCurrentBlock   KeyPrefix = 0xc0
	SYSCurrentHeader  KeyPrefix = 0xc1
	SYSVersion        KeyPrefix = 0xf0
)

// ErrKeyNotFound is an error returned by Store implementations
// when a certain key is not found.
var ErrKeyNotFound = errors.New("key not found")

type (
	// Store is anything that can persist and retrieve the blockchain.
	// information.
	Store interface {
		Batch() Batch
		Get([]byte) ([]byte, error)
		Put(k, v []byte) error
		PutBatch(Batch) error
		Seek(k []byte, f func(k, v []byte))
	}

	// Batch represents an abstraction on top of batch operations.
	// Each Store implementation is responsible of casting a Batch
	// to its appropriate type.
	Batch interface {
		Put(k, v []byte)
		Len() int
	}

	// KeyPrefix is a constant byte added as a prefix for each key
	// stored.
	KeyPrefix uint8
)

// Bytes returns the bytes representation of KeyPrefix.
func (k KeyPrefix) Bytes() []byte {
	return []byte{byte(k)}
}

// AppendPrefix append byteslice b to the given KeyPrefix.
// 	AppendKeyPrefix(SYSVersion, []byte{0x00, 0x01})
func AppendPrefix(k KeyPrefix, b []byte) []byte {
	dest := make([]byte, len(b)+1)
	dest[0] = byte(k)
	copy(dest[1:], b)
	return dest
}

// AppendPrefixInt append int n to the given KeyPrefix.
//	AppendPrefixInt(SYSCurrentHeader, 10001)
func AppendPrefixInt(k KeyPrefix, n int) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, uint32(n))
	return AppendPrefix(k, b)
}
