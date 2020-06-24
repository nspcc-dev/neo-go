package server

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/nspcc-dev/neo-go/pkg/core/storage"
)

type dump []blockDump

type blockDump struct {
	Block   uint32      `json:"block"`
	Size    int         `json:"size"`
	Storage []storageOp `json:"storage"`
}

type storageOp struct {
	State string `json:"state"`
	Key   string `json:"key"`
	Value string `json:"value,omitempty"`
}

// NEO has some differences of key storing (that should go away post-preview2).
// ours format: contract's ID (uint32le) + key
// theirs format: contract's ID (uint32le) + key with 16 between every 16 bytes, padded to len 16.
func toNeoStorageKey(key []byte) []byte {
	if len(key) < 4 {
		panic("invalid key in storage")
	}

	// Prefix is a contract's ID in LE.
	nkey := make([]byte, 4, len(key))
	copy(nkey, key[:4])
	key = key[4:]

	index := 0
	remain := len(key)
	for remain >= 16 {
		nkey = append(nkey, key[index:index+16]...)
		nkey = append(nkey, 16)
		index += 16
		remain -= 16
	}

	if remain > 0 {
		nkey = append(nkey, key[index:]...)
	}

	padding := 16 - remain
	for i := 0; i < padding; i++ {
		nkey = append(nkey, 0)
	}

	nkey = append(nkey, byte(remain))

	return nkey
}

// batchToMap converts batch to a map so that JSON is compatible
// with https://github.com/NeoResearch/neo-storage-audit/
func batchToMap(index uint32, batch *storage.MemBatch) blockDump {
	size := len(batch.Put) + len(batch.Deleted)
	ops := make([]storageOp, 0, size)
	for i := range batch.Put {
		key := batch.Put[i].Key
		if len(key) == 0 || key[0] != byte(storage.STStorage) {
			continue
		}

		op := "Added"
		if batch.Put[i].Exists {
			op = "Changed"
		}

		key = toNeoStorageKey(key[1:])
		ops = append(ops, storageOp{
			State: op,
			Key:   hex.EncodeToString(key),
			Value: hex.EncodeToString(batch.Put[i].Value),
		})
	}

	for i := range batch.Deleted {
		key := batch.Deleted[i].Key
		if len(key) == 0 || key[0] != byte(storage.STStorage) || !batch.Deleted[i].Exists {
			continue
		}

		key = toNeoStorageKey(key[1:])
		ops = append(ops, storageOp{
			State: "Deleted",
			Key:   hex.EncodeToString(key),
		})
	}

	return blockDump{
		Block:   index,
		Size:    len(ops),
		Storage: ops,
	}
}

func newDump() *dump {
	return new(dump)
}

func (d *dump) add(index uint32, batch *storage.MemBatch) {
	m := batchToMap(index, batch)
	*d = append(*d, m)
}

func (d *dump) tryPersist(prefix string, index uint32) error {
	if len(*d) == 0 {
		return nil
	}
	path, err := getPath(prefix, index)
	if err != nil {
		return err
	}
	old, err := readFile(path)
	if err == nil {
		*old = append(*old, *d...)
	} else {
		old = d
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", " ")
	if err := enc.Encode(*old); err != nil {
		return err
	}

	*d = (*d)[:0]

	return nil
}

func readFile(path string) (*dump, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	d := newDump()
	if err := json.Unmarshal(data, d); err != nil {
		return nil, err
	}
	return d, err
}

// getPath returns filename for storing blocks up to index.
// Directory structure is the following:
// https://github.com/NeoResearch/neo-storage-audit#folder-organization-where-to-find-the-desired-block
// Dir `BlockStorage_$DIRNO` contains blocks up to $DIRNO (from $DIRNO-100k)
// Inside it there are files grouped by 1k blocks.
// File dump-block-$FILENO.json contains blocks from $FILENO-999, $FILENO
// Example: file `BlockStorage_100000/dump-block-6000.json` contains blocks from 5001 to 6000.
func getPath(prefix string, index uint32) (string, error) {
	dirN := ((index + 99999) / 100000) * 100000
	dir := fmt.Sprintf("BlockStorage_%d", dirN)

	path := filepath.Join(prefix, dir)
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		err := os.MkdirAll(path, os.ModePerm)
		if err != nil {
			return "", err
		}
	} else if !info.IsDir() {
		return "", fmt.Errorf("file `%s` is not a directory", path)
	}

	fileN := ((index + 999) / 1000) * 1000
	file := fmt.Sprintf("dump-block-%d.json", fileN)
	return filepath.Join(path, file), nil
}
