package state

import (
	"bytes"
	"encoding/binary"

	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// UnclaimedBalanceSize is a size of the UnclaimedBalance struct in bytes.
const UnclaimedBalanceSize = util.Uint256Size + 2 + 4 + 4 + 8

// UnclaimedBalances is a slice of UnclaimedBalance.
type UnclaimedBalances struct {
	Raw []byte
}

// Size returns an amount of store unclaimed balances.
func (bs *UnclaimedBalances) Size() int {
	return len(bs.Raw) / UnclaimedBalanceSize
}

// ForEach iterates over all unclaimed balances.
func (bs *UnclaimedBalances) ForEach(f func(*UnclaimedBalance) error) error {
	b := new(UnclaimedBalance)
	for i := 0; i < len(bs.Raw); i += UnclaimedBalanceSize {
		r := io.NewBinReaderFromBuf(bs.Raw[i : i+UnclaimedBalanceSize])
		b.DecodeBinary(r)
		if r.Err != nil {
			return r.Err
		} else if err := f(b); err != nil {
			return err
		}
	}
	return nil
}

// Remove removes specified unclaim from the list and returns
// false if it wasn't found.
func (bs *UnclaimedBalances) Remove(tx util.Uint256, index uint16) bool {
	const keySize = util.Uint256Size + 2
	key := make([]byte, keySize)
	copy(key, tx[:])
	binary.LittleEndian.PutUint16(key[util.Uint256Size:], index)

	for i := 0; i < len(bs.Raw); i += UnclaimedBalanceSize {
		if bytes.Equal(bs.Raw[i:i+keySize], key) {
			lastIndex := len(bs.Raw) - UnclaimedBalanceSize
			if i != lastIndex {
				copy(bs.Raw[i:i+UnclaimedBalanceSize], bs.Raw[lastIndex:])
			}
			bs.Raw = bs.Raw[:lastIndex]
			return true
		}
	}
	return false
}

// Put puts new unclaim in a list.
func (bs *UnclaimedBalances) Put(b *UnclaimedBalance) error {
	w := io.NewBufBinWriter()
	b.EncodeBinary(w.BinWriter)
	if w.Err != nil {
		return w.Err
	}
	bs.Raw = append(bs.Raw, w.Bytes()...)
	return nil
}
