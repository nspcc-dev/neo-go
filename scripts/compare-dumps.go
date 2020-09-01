package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"

	"github.com/urfave/cli"
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

func readFile(path string) (dump, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	d := make(dump, 0)
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, err
	}
	return d, err
}

func (d dump) normalize() {
	for i := range d {
		for j := range d[i].Storage {
			if d[i].Storage[j].State == "Changed" {
				d[i].Storage[j].State = "Added"
			}
		}
		sort.Slice(d[i].Storage, func(k, l int) bool {
			return d[i].Storage[k].Key < d[i].Storage[l].Key
		})
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
		if blockA.Size != blockB.Size {
			return fmt.Errorf("block %d, changes number mismatch: %d vs %d", blockA.Block, blockA.Size, blockB.Size)
		}
		if len(blockA.Storage) != len(blockB.Storage) {
			return fmt.Errorf("block %d, changes length mismatch: %d vs %d", blockA.Block, len(blockA.Storage), len(blockB.Storage))
		}
		fail := false
		for j := range blockA.Storage {
			if blockA.Storage[j].Key != blockB.Storage[j].Key {
				return fmt.Errorf("block %d: key mismatch: %s vs %s", blockA.Block, blockA.Storage[j].Key, blockB.Storage[j].Key)
			}
			if blockA.Storage[j].State != blockB.Storage[j].State {
				return fmt.Errorf("block %d: state mismatch for key %s: %s vs %s", blockA.Block, blockA.Storage[j].Key, blockA.Storage[j].State, blockB.Storage[j].State)
			}
			if blockA.Storage[j].Value != blockB.Storage[j].Value {
				fail = true
				fmt.Printf("block %d: value mismatch for key %s: %s vs %s\n", blockA.Block, blockA.Storage[j].Key, blockA.Storage[j].Value, blockB.Storage[j].Value)
			}
		}
		if fail {
			return errors.New("fail")
		}
	}
	return nil
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
