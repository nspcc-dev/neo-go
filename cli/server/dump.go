package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/storage/dboper"
)

type dump []blockDump

type blockDump struct {
	Block   uint32             `json:"block"`
	Size    int                `json:"size"`
	Storage []dboper.Operation `json:"storage"`
}

func newDump() *dump {
	return new(dump)
}

func (d *dump) addBlockDump(entry blockDump) {
	*d = append(*d, entry)
}

func (d *dump) add(index uint32, batch *storage.MemBatch) {
	ops := storage.BatchToOperations(batch)
	*d = append(*d, blockDump{
		Block:   index,
		Size:    len(ops),
		Storage: ops,
	})
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
	for _, l := range *old {
		if err := enc.Encode(l); err != nil {
			return err
		}
	}

	*d = (*d)[:0]

	return nil
}

func readFile(path string) (*dump, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	dec := json.NewDecoder(f)
	d := newDump()
	for i := 0; ; i++ {
		bD := new(blockDump)
		err := dec.Decode(bD)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("failed to unmarshal entry #%d from %s: %w", i, path, err)
		}
		d.addBlockDump(*bD)
	}
	return d, err
}

// getPath returns filename for storing blocks up to index.
// Directory structure is the following:
// Dir `BlockStorage_$DIRNO` contains storage diffs for blocks with indexes [$DIRNO, $DIRNO+99999].
// Inside it there are files grouped by 1k blocks, every file contains raws where
// every raw is a JSON object containing storage diff for the corresponding block.
// File dump-block-$FILENO.dump contains blocks with indexes [$FILENO, $FILENO+999].
// Example: file `BlockStorage_100000/dump-block-6000.dump` contains blocks from 6000 to 6999.
func getPath(prefix string, index uint32) (string, error) {
	dirN := index / 100000 * 100000
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

	fileN := index / 1000 * 1000
	file := fmt.Sprintf("dump-block-%d.dump", fileN)
	return filepath.Join(path, file), nil
}
