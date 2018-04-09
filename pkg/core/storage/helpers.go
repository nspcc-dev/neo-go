package storage

import (
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
	h, err = util.Uint256DecodeBytes(b[:32])
	return
}

// HeaderHashes returns a sorted list of header hashes retrieved from
// the given underlying Store.
func HeaderHashes(s Store) ([]util.Uint256, error) {
	hashMap := make(map[uint32][]util.Uint256)
	s.Seek(IXHeaderHashList.Bytes(), func(k, v []byte) {
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
