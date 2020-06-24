package native

import (
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// BlockedAccounts represents a slice of blocked accounts hashes.
type BlockedAccounts []util.Uint160

// Bytes returns serialized BlockedAccounts.
func (ba *BlockedAccounts) Bytes() []byte {
	w := io.NewBufBinWriter()
	ba.EncodeBinary(w.BinWriter)
	if w.Err != nil {
		panic(w.Err)
	}
	return w.Bytes()
}

// EncodeBinary implements io.Serializable interface.
func (ba *BlockedAccounts) EncodeBinary(w *io.BinWriter) {
	w.WriteArray(*ba)
}

// BlockedAccountsFromBytes converts serialized BlockedAccounts to structure.
func BlockedAccountsFromBytes(b []byte) (BlockedAccounts, error) {
	ba := new(BlockedAccounts)
	if len(b) == 0 {
		return *ba, nil
	}
	r := io.NewBinReaderFromBuf(b)
	ba.DecodeBinary(r)
	if r.Err != nil {
		return nil, r.Err
	}
	return *ba, nil
}

// DecodeBinary implements io.Serializable interface.
func (ba *BlockedAccounts) DecodeBinary(r *io.BinReader) {
	r.ReadArray(ba)
}

// ToStackItem converts BlockedAccounts to stackitem.Item
func (ba *BlockedAccounts) ToStackItem() stackitem.Item {
	result := make([]stackitem.Item, len(*ba))
	for i, account := range *ba {
		result[i] = stackitem.NewByteArray(account.BytesLE())
	}
	return stackitem.NewArray(result)
}
