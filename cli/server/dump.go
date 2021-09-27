package server

import (
	"encoding/base64"
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

// batchToMap converts batch to a map so that JSON is compatible
// with https://github.com/NeoResearch/neo-storage-audit/
func batchToMap(index uint32, batch *storage.MemBatch) blockDump {
	size := len(batch.Put) + len(batch.Deleted)
	ops := make([]storageOp, 0, size)
	for i := range batch.Put {
		key := batch.Put[i].Key
		if len(key) == 0 || key[0] != byte(storage.STStorage) && key[0] != byte(storage.STTempStorage) {
			continue
		}

		op := "Added"
		if batch.Put[i].Exists {
			op = "Changed"
		}

		ops = append(ops, storageOp{
			State: op,
			Key:   base64.StdEncoding.EncodeToString(key[1:]),
			Value: base64.StdEncoding.EncodeToString(batch.Put[i].Value),
		})
	}

	for i := range batch.Deleted {
		key := batch.Deleted[i].Key
		if len(key) == 0 || !batch.Deleted[i].Exists ||
			key[0] != byte(storage.STStorage) && key[0] != byte(storage.STTempStorage) {
			continue
		}

		ops = append(ops, storageOp{
			State: "Deleted",
			Key:   base64.StdEncoding.EncodeToString(key[1:]),
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
