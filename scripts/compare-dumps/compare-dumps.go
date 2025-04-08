package main

import (
	"bytes"
	"cmp"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/urfave/cli/v2"
)

var ledgerContractID = -4

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

func readFile(path string) (dump, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		var dErr error
		data, dErr = os.ReadFile(strings.TrimSuffix(path, ".json") + ".dump")
		if dErr != nil {
			return nil, fmt.Errorf("%w; %w", err, dErr)
		}
	}
	d := make(dump, 0)
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, err
	}
	return d, nil
}

func (d dump) normalize() {
	ledgerIDBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(ledgerIDBytes, uint32(ledgerContractID))
	for i := range d {
		var newStorage []storageOp
		for j := range d[i].Storage {
			keyBytes, err := base64.StdEncoding.DecodeString(d[i].Storage[j].Key)
			if err != nil {
				panic(fmt.Errorf("invalid key encoding: %w", err))
			}
			if bytes.HasPrefix(keyBytes, ledgerIDBytes) {
				continue
			}
			if d[i].Storage[j].State == "Changed" {
				d[i].Storage[j].State = "Added"
			}
			newStorage = append(newStorage, d[i].Storage[j])
		}
		slices.SortFunc(newStorage, func(a, b storageOp) int { return cmp.Compare(a.Key, b.Key) })
		d[i].Storage = newStorage
	}
	// assume that d is already sorted by Block
}

func compare(a, b string) error {
	dumpA, err := readFile(a)
	if err != nil {
		return fmt.Errorf("reading file %s: %w", a, err)
	}
	dumpB, err := readFile(b)
	if err != nil {
		return fmt.Errorf("reading file %s: %w", b, err)
	}
	dumpA.normalize()
	dumpB.normalize()
	if len(dumpA) != len(dumpB) {
		return fmt.Errorf("dump files differ in size: %d vs %d", len(dumpA), len(dumpB))
	}
	for i := range dumpA {
		blockA := &dumpA[i]
		blockB := &dumpB[i]
		if blockA.Block != blockB.Block {
			return fmt.Errorf("block number mismatch: %d vs %d", blockA.Block, blockB.Block)
		}
		if len(blockA.Storage) != len(blockB.Storage) {
			return fmt.Errorf("block %d, changes length mismatch: %d vs %d", blockA.Block, len(blockA.Storage), len(blockB.Storage))
		}
		fail := false
		for j := range blockA.Storage {
			if blockA.Storage[j].Key != blockB.Storage[j].Key {
				idA, prefixA := parseKey(blockA.Storage[j].Key)
				idB, prefixB := parseKey(blockB.Storage[j].Key)
				return fmt.Errorf("block %d: key mismatch:\n\tKey: %s\n\tContract ID: %d\n\tItem key (base64): %s\n\tItem key (hex): %s\n\tItem key (bytes): %v\nvs\n\tKey: %s\n\tContract ID: %d\n\tItem key (base64): %s\n\tItem key (hex): %s\n\tItem key (bytes): %v", blockA.Block, blockA.Storage[j].Key, idA, base64.StdEncoding.EncodeToString(prefixA), hex.EncodeToString(prefixA), prefixA, blockB.Storage[j].Key, idB, base64.StdEncoding.EncodeToString(prefixB), hex.EncodeToString(prefixB), prefixB)
			}
			if blockA.Storage[j].State != blockB.Storage[j].State {
				id, prefix := parseKey(blockA.Storage[j].Key)
				return fmt.Errorf("block %d: state mismatch for key %s:\n\tContract ID: %d\n\tItem key (base64): %s\n\tItem key (hex): %s\n\tItem key (bytes): %v\n\tDiff: %s vs %s", blockA.Block, blockA.Storage[j].Key, id, base64.StdEncoding.EncodeToString(prefix), hex.EncodeToString(prefix), prefix, blockA.Storage[j].State, blockB.Storage[j].State)
			}
			if blockA.Storage[j].Value != blockB.Storage[j].Value {
				fail = true
				id, prefix := parseKey(blockA.Storage[j].Key)
				fmt.Printf("block %d: value mismatch for key %s:\n\tContract ID: %d\n\tItem key (base64): %s\n\tItem key (hex): %s\n\tItem key (bytes): %v\n\tDiff: %s vs %s\n", blockA.Block, blockA.Storage[j].Key, id, base64.StdEncoding.EncodeToString(prefix), hex.EncodeToString(prefix), prefix, blockA.Storage[j].Value, blockB.Storage[j].Value)
			}
		}
		if fail {
			return errors.New("fail")
		}
	}
	return nil
}

// parseKey splits the provided storage item key into contract ID and contract storage item prefix.
func parseKey(key string) (int32, []byte) {
	keyBytes, _ := base64.StdEncoding.DecodeString(key) // ignore error, rely on proper storage dump state.
	id := int32(binary.LittleEndian.Uint32(keyBytes[:4]))
	prefix := keyBytes[4:]
	return id, prefix
}

func cliMain(c *cli.Context) error {
	a := c.Args().Get(0)
	b := c.Args().Get(1)
	if a == "" {
		return errors.New("no arguments given")
	}
	if b == "" {
		return errors.New("missing second argument")
	}
	fa, err := os.Open(a)
	if err != nil {
		return err
	}
	defer fa.Close()
	fb, err := os.Open(b)
	if err != nil {
		return err
	}
	defer fb.Close()

	astat, err := fa.Stat()
	if err != nil {
		return err
	}
	bstat, err := fb.Stat()
	if err != nil {
		return err
	}
	if astat.Mode().IsRegular() && bstat.Mode().IsRegular() {
		return compare(a, b)
	}
	if astat.Mode().IsDir() && bstat.Mode().IsDir() {
		for i := 0; i <= 6000000; i += 100000 {
			dir := fmt.Sprintf("BlockStorage_%d", i)
			fmt.Println("Processing directory", dir)
			for j := i - 99000; j <= i; j += 1000 {
				if j < 0 {
					continue
				}
				fname := fmt.Sprintf("%s/dump-block-%d.json", dir, j)

				aname := filepath.Join(a, fname)
				bname := filepath.Join(b, fname)
				err := compare(aname, bname)
				if err != nil {
					return fmt.Errorf("file %s: %w", fname, err)
				}
			}
		}
		return nil
	}
	return errors.New("both parameters must be either dump files or directories")
}

func main() {
	ctl := cli.NewApp()
	ctl.Name = "compare-dumps"
	ctl.Version = "1.0"
	ctl.Usage = "compare-dumps dumpDirA dumpDirB"
	ctl.Action = cliMain

	if err := ctl.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, ctl.Usage)
		os.Exit(1)
	}
}
