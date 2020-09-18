package native

import (
	"crypto/elliptic"
	"errors"
	"math/big"
	"unicode/utf8"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// IDList is a list of oracle request IDs.
type IDList []uint64

// NodeList represents list or oracle nodes.
type NodeList keys.PublicKeys

// OracleRequest represents oracle request.
type OracleRequest struct {
	OriginalTxID     util.Uint256
	GasForResponse   uint64
	URL              string
	Filter           *string
	CallbackContract util.Uint160
	CallbackMethod   string
	UserData         []byte
}

// Bytes return l serizalized to a byte-slice.
func (l IDList) Bytes() []byte {
	w := io.NewBufBinWriter()
	l.EncodeBinary(w.BinWriter)
	return w.Bytes()
}

// EncodeBinary implements io.Serializable.
func (l IDList) EncodeBinary(w *io.BinWriter) {
	stackitem.EncodeBinaryStackItem(l.toStackItem(), w)
}

// DecodeBinary implements io.Serializable.
func (l *IDList) DecodeBinary(r *io.BinReader) {
	item := stackitem.DecodeBinaryStackItem(r)
	if r.Err != nil || item == nil {
		return
	}
	r.Err = l.fromStackItem(item)
}

func (l IDList) toStackItem() stackitem.Item {
	arr := make([]stackitem.Item, len(l))
	for i := range l {
		arr[i] = stackitem.NewBigInteger(new(big.Int).SetUint64(l[i]))
	}
	return stackitem.NewArray(arr)
}

func (l *IDList) fromStackItem(it stackitem.Item) error {
	arr, ok := it.Value().([]stackitem.Item)
	if !ok {
		return errors.New("not an array")
	}
	*l = make(IDList, len(arr))
	for i := range arr {
		bi, err := arr[i].TryInteger()
		if err != nil {
			return err
		}
		(*l)[i] = bi.Uint64()
	}
	return nil
}

// Remove removes id from list.
func (l *IDList) Remove(id uint64) bool {
	for i := range *l {
		if id == (*l)[i] {
			if i < len(*l) {
				copy((*l)[i:], (*l)[i+1:])
			}
			*l = (*l)[:len(*l)-1]
			return true
		}
	}
	return false
}

// Bytes return l serizalized to a byte-slice.
func (l NodeList) Bytes() []byte {
	w := io.NewBufBinWriter()
	l.EncodeBinary(w.BinWriter)
	return w.Bytes()
}

// EncodeBinary implements io.Serializable.
func (l NodeList) EncodeBinary(w *io.BinWriter) {
	stackitem.EncodeBinaryStackItem(l.toStackItem(), w)
}

// DecodeBinary implements io.Serializable.
func (l *NodeList) DecodeBinary(r *io.BinReader) {
	item := stackitem.DecodeBinaryStackItem(r)
	if r.Err != nil || item == nil {
		return
	}
	r.Err = l.fromStackItem(item)
}

func (l NodeList) toStackItem() stackitem.Item {
	arr := make([]stackitem.Item, len(l))
	for i := range l {
		arr[i] = stackitem.NewByteArray(l[i].Bytes())
	}
	return stackitem.NewArray(arr)
}

func (l *NodeList) fromStackItem(it stackitem.Item) error {
	arr, ok := it.Value().([]stackitem.Item)
	if !ok {
		return errors.New("not an array")
	}
	*l = make(NodeList, len(arr))
	for i := range arr {
		bs, err := arr[i].TryBytes()
		if err != nil {
			return err
		}
		pub, err := keys.NewPublicKeyFromBytes(bs, elliptic.P256())
		if err != nil {
			return err
		}
		(*l)[i] = pub
	}
	return nil
}

// Bytes return o serizalized to a byte-slice.
func (o *OracleRequest) Bytes() []byte {
	w := io.NewBufBinWriter()
	o.EncodeBinary(w.BinWriter)
	return w.Bytes()
}

// EncodeBinary implements io.Serializable.
func (o *OracleRequest) EncodeBinary(w *io.BinWriter) {
	stackitem.EncodeBinaryStackItem(o.toStackItem(), w)
}

// DecodeBinary implements io.Serializable.
func (o *OracleRequest) DecodeBinary(r *io.BinReader) {
	item := stackitem.DecodeBinaryStackItem(r)
	if r.Err != nil || item == nil {
		return
	}
	r.Err = o.fromStackItem(item)
}

func (o *OracleRequest) toStackItem() stackitem.Item {
	filter := stackitem.Item(stackitem.Null{})
	if o.Filter != nil {
		filter = stackitem.Make(*o.Filter)
	}
	return stackitem.NewArray([]stackitem.Item{
		stackitem.NewByteArray(o.OriginalTxID.BytesBE()),
		stackitem.NewBigInteger(new(big.Int).SetUint64(o.GasForResponse)),
		stackitem.Make(o.URL),
		filter,
		stackitem.NewByteArray(o.CallbackContract.BytesBE()),
		stackitem.Make(o.CallbackMethod),
		stackitem.NewByteArray(o.UserData),
	})
}

func (o *OracleRequest) fromStackItem(it stackitem.Item) error {
	arr, ok := it.Value().([]stackitem.Item)
	if !ok || len(arr) < 7 {
		return errors.New("not an array of needed length")
	}
	bs, err := arr[0].TryBytes()
	if err != nil {
		return err
	}
	o.OriginalTxID, err = util.Uint256DecodeBytesBE(bs)
	if err != nil {
		return err
	}

	gas, err := arr[1].TryInteger()
	if err != nil {
		return err
	}
	o.GasForResponse = gas.Uint64()

	s, isNull, ok := itemToString(arr[2])
	if !ok || isNull {
		return errors.New("invalid URL")
	}
	o.URL = s

	s, isNull, ok = itemToString(arr[3])
	if !ok {
		return errors.New("invalid filter")
	} else if !isNull {
		filter := s
		o.Filter = &filter
	}

	bs, err = arr[4].TryBytes()
	if err != nil {
		return err
	}
	o.CallbackContract, err = util.Uint160DecodeBytesBE(bs)
	if err != nil {
		return err
	}

	o.CallbackMethod, isNull, ok = itemToString(arr[5])
	if !ok || isNull {
		return errors.New("invalid callback method")
	}

	o.UserData, err = arr[6].TryBytes()
	return err
}

func itemToString(it stackitem.Item) (string, bool, bool) {
	_, ok := it.(stackitem.Null)
	if ok {
		return "", true, true
	}
	bs, err := it.TryBytes()
	if err != nil || !utf8.Valid(bs) {
		return "", false, false
	}
	return string(bs), false, true
}
