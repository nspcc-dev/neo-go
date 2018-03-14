package core

import (
	"bytes"
	"encoding/binary"

	"github.com/syndtr/goleveldb/leveldb"
)

type dataEntry uint8

func (e dataEntry) bytes() []byte {
	return []byte{byte(e)}
}

// Storage data entry prefixes.
const (
	preDataBlock         dataEntry = 0x01
	preDataTransaction   dataEntry = 0x02
	preSTAccount         dataEntry = 0x40
	preSTCoin            dataEntry = 0x44
	preSTValidator       dataEntry = 0x48
	preSTAsset           dataEntry = 0x4c
	preSTContract        dataEntry = 0x50
	preSTStorage         dataEntry = 0x70
	preIXHeaderHashList  dataEntry = 0x80
	preIXValidatorsCount dataEntry = 0x90
	preSYSCurrentBlock   dataEntry = 0xc0
	preSYSCurrentHeader  dataEntry = 0xc1
	preSYSVersion        dataEntry = 0xf0
)

func makeEntryPrefixInt(e dataEntry, n int) []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, n)
	return makeEntryPrefix(e, buf.Bytes())
}

func makeEntryPrefix(e dataEntry, b []byte) []byte {
	dest := make([]byte, len(b)+1)
	dest[0] = byte(e)
	for i := 1; i < len(b); i++ {
		dest[i] = b[i]
	}
	return dest
}

// Store is anything that can persist and retrieve the blockchain.
type Store interface {
	get(k []byte) ([]byte, error)
	write(k, v []byte) error
	writeBatch(batch *leveldb.Batch) error
}
