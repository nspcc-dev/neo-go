package chaindump

import (
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/io"
)

// Dump writes count blocks from start to the provided writer.
// Note: header needs to be written separately by client.
func Dump(bc blockchainer.Blockchainer, w *io.BinWriter, start, count uint32) error {
	for i := start; i < start+count; i++ {
		bh := bc.GetHeaderHash(int(i))
		b, err := bc.GetBlock(bh)
		if err != nil {
			return err
		}
		buf := io.NewBufBinWriter()
		b.EncodeBinary(buf.BinWriter)
		bytes := buf.Bytes()
		w.WriteU32LE(uint32(len(bytes)))
		w.WriteBytes(bytes)
		if w.Err != nil {
			return w.Err
		}
	}
	return nil
}

// Restore restores blocks from provided reader.
// f is called after addition of every block.
func Restore(bc blockchainer.Blockchainer, r *io.BinReader, skip, count uint32, f func(b *block.Block) error) error {
	readBlock := func(r *io.BinReader) ([]byte, error) {
		var size = r.ReadU32LE()
		buf := make([]byte, size)
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

	stateRootInHeader := bc.GetConfig().StateRootInHeader

	for ; i < skip+count; i++ {
		buf, err := readBlock(r)
		if err != nil {
			return err
		}
		b := block.New(stateRootInHeader)
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
