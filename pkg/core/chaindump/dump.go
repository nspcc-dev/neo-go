package chaindump

import (
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// DumperRestorer is an interface to get/add blocks from/to.
type DumperRestorer interface {
	AddBlock(block *block.Block) error
	GetBlock(hash util.Uint256) (*block.Block, error)
	GetConfig() config.Blockchain
	GetHeaderHash(uint32) util.Uint256
}

// Dump writes count blocks from start to the provided writer.
// Note: header needs to be written separately by a client.
func Dump(bc DumperRestorer, w *io.BinWriter, start, count uint32) error {
	var buf = io.NewBufBinWriter()

	for i := start; i < start+count; i++ {
		bh := bc.GetHeaderHash(i)
		b, err := bc.GetBlock(bh)
		if err != nil {
			return err
		}
		b.EncodeBinary(buf.BinWriter)
		bytes := buf.Bytes()
		w.WriteU32LE(uint32(len(bytes)))
		w.WriteBytes(bytes)
		if w.Err != nil {
			return w.Err
		}
		buf.Reset()
	}
	return nil
}

// Restore restores blocks from the provided reader.
// f is called after addition of every block.
func Restore(bc DumperRestorer, r *io.BinReader, skip, count uint32, f func(b *block.Block) error) error {
	var buf []byte

	readBlock := func(r *io.BinReader) ([]byte, error) {
		var size = r.ReadU32LE()
		if uint32(cap(buf)) < size {
			buf = make([]byte, size)
		} else {
			buf = buf[:size]
		}
		r.ReadBytes(buf)
		return buf, r.Err
	}

	i := uint32(0)
	for ; i < skip; i++ {
		_, err := readBlock(r)
		if err != nil {
			return err
		}
	}

	for ; i < skip+count; i++ {
		buf, err := readBlock(r)
		if err != nil {
			return err
		}
		b := &block.Block{}
		r := io.NewBinReaderFromBuf(buf)
		b.DecodeBinary(r)
		if r.Err != nil {
			return r.Err
		}
		if b.Index != 0 || i != 0 || skip != 0 {
			err = bc.AddBlock(b)
			if err != nil {
				return fmt.Errorf("failed to add block %d: %w", i, err)
			}
		}
		if f != nil {
			if err := f(b); err != nil {
				return err
			}
		}
	}
	return nil
}
