package storage

import (
	"bytes"
	"encoding/binary"
	"sort"

	"github.com/CityOfZion/neo-go/pkg/util"
)

// Version will attempt to get the current version stored in the
// underlying Store.
func Version(s Store) (string, error) {
	version, err := s.Get(SYSVersion.Bytes())
	return string(version), err
}

// PutVersion will store the given version in the underlying Store.
func PutVersion(s Store, v string) error {
	return s.Put(SYSVersion.Bytes(), []byte(v))
}

// CurrentBlockHeight returns the current block height found in the
// underlying Store.
func CurrentBlockHeight(s Store) (uint32, error) {
	b, err := s.Get(SYSCurrentBlock.Bytes())
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(b[32:36]), nil
}

// CurrentHeaderHeight returns the current header height and hash from
// the underlying Store.
func CurrentHeaderHeight(s Store) (i uint32, h util.Uint256, err error) {
	var b []byte
	b, err = s.Get(SYSCurrentHeader.Bytes())
	if err != nil {
		return
	}
	i = binary.LittleEndian.Uint32(b[32:36])
	h, err = util.Uint256DecodeReverseBytes(b[:32])
	return
}

// uint32Slice attaches the methods of Interface to []int, sorting in increasing order.
type uint32Slice []uint32

func (p uint32Slice) Len() int           { return len(p) }
func (p uint32Slice) Less(i, j int) bool { return p[i] < p[j] }
func (p uint32Slice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// HeaderHashes returns a sorted list of header hashes retrieved from
// the given underlying Store.
func HeaderHashes(s Store) ([]util.Uint256, error) {
	hashMap := make(map[uint32][]util.Uint256)
	s.Seek(IXHeaderHashList.Bytes(), func(k, v []byte) {
		storedCount := binary.LittleEndian.Uint32(k[1:])
		hashes, err := read2000Uint256Hashes(v)
		if err != nil {
			panic(err)
		}
		hashMap[storedCount] = hashes
	})

	var (
		hashes     = make([]util.Uint256, 0, len(hashMap))
		sortedKeys = make([]uint32, 0, len(hashMap))
	)

	for k := range hashMap {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Sort(uint32Slice(sortedKeys))

	for _, key := range sortedKeys {
		hashes = append(hashes[:key], hashMap[key]...)
	}

	return hashes, nil
}

// read2000Uint256Hashes attempts to read 2000 Uint256 hashes from
// the given byte array.
func read2000Uint256Hashes(b []byte) ([]util.Uint256, error) {
	r := bytes.NewReader(b)
	br := util.NewBinReaderFromIO(r)
	lenHashes := br.ReadVarUint()
	hashes := make([]util.Uint256, lenHashes)
	br.ReadLE(hashes)
	if br.Err != nil {
		return nil, br.Err
	}
	return hashes, nil
}
