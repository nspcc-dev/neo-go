package state

import (
	"errors"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
)

// NEP5Tracker contains info about a single account in a NEP5 contract.
type NEP5Tracker struct {
	// Balance is the current balance of the account.
	Balance *big.Int
	// LastUpdatedBlock is a number of block when last `transfer` to or from the
	// account occured.
	LastUpdatedBlock uint32
}

// TransferLog is a log of NEP5 token transfers for the specific command.
type TransferLog struct {
	Raw []byte
}

// NEP5TransferSize is a size of a marshaled NEP5Transfer struct in bytes.
const NEP5TransferSize = util.Uint160Size*3 + amountSize + 4 + 4 + util.Uint256Size + 4

// NEP5Transfer represents a single NEP5 Transfer event.
type NEP5Transfer struct {
	// Asset is a NEP5 contract hash.
	Asset util.Uint160
	// Address is the address of the sender.
	From util.Uint160
	// To is the address of the receiver.
	To util.Uint160
	// Amount is the amount of tokens transferred.
	// It is negative when tokens are sent and positive if they are received.
	Amount *big.Int
	// Block is a number of block when the event occured.
	Block uint32
	// Timestamp is the timestamp of the block where transfer occured.
	Timestamp uint32
	// Tx is a hash the transaction.
	Tx util.Uint256
	// Index is the index of this transfer in the corresponding tx.
	Index uint32
}

const amountSize = 32

// NEP5Balances is a map of the NEP5 contract hashes
// to the corresponding structures.
type NEP5Balances struct {
	Trackers map[util.Uint160]NEP5Tracker
	// NextTransferBatch stores an index of the next transfer batch.
	NextTransferBatch uint32
}

// NEP5Metadata is a metadata for NEP5 contracts.
type NEP5Metadata struct {
	Decimals int64
}

// NewNEP5Balances returns new NEP5Balances.
func NewNEP5Balances() *NEP5Balances {
	return &NEP5Balances{
		Trackers: make(map[util.Uint160]NEP5Tracker),
	}
}

// DecodeBinary implements io.Serializable interface.
func (bs *NEP5Balances) DecodeBinary(r *io.BinReader) {
	bs.NextTransferBatch = r.ReadU32LE()
	lenBalances := r.ReadVarUint()
	m := make(map[util.Uint160]NEP5Tracker, lenBalances)
	for i := 0; i < int(lenBalances); i++ {
		var key util.Uint160
		var tr NEP5Tracker
		r.ReadBytes(key[:])
		tr.DecodeBinary(r)
		m[key] = tr
	}
	bs.Trackers = m
}

// EncodeBinary implements io.Serializable interface.
func (bs *NEP5Balances) EncodeBinary(w *io.BinWriter) {
	w.WriteU32LE(bs.NextTransferBatch)
	w.WriteVarUint(uint64(len(bs.Trackers)))
	for k, v := range bs.Trackers {
		w.WriteBytes(k[:])
		v.EncodeBinary(w)
	}
}

// DecodeBinary implements io.Serializable interface.
func (bs *NEP5Metadata) DecodeBinary(r *io.BinReader) {
	bs.Decimals = int64(r.ReadU64LE())
}

// EncodeBinary implements io.Serializable interface.
func (bs *NEP5Metadata) EncodeBinary(w *io.BinWriter) {
	w.WriteU64LE(uint64(bs.Decimals))
}

// Append appends single transfer to a log.
func (lg *TransferLog) Append(tr io.Serializable) error {
	w := io.NewBufBinWriter()
	tr.EncodeBinary(w.BinWriter)
	if w.Err != nil {
		return w.Err
	}
	lg.Raw = append(lg.Raw, w.Bytes()...)
	return nil
}

// ForEach iterates over transfer log returning on first error.
func (lg *TransferLog) ForEach(size int, tr io.Serializable, f func() (bool, error)) (bool, error) {
	if lg == nil {
		return true, nil
	}
	for i := len(lg.Raw); i > 0; i -= size {
		r := io.NewBinReaderFromBuf(lg.Raw[i-size : i])
		tr.DecodeBinary(r)
		if r.Err != nil {
			return false, r.Err
		}
		cont, err := f()
		if err != nil {
			return false, err
		}
		if !cont {
			return false, nil
		}
	}
	return true, nil
}

// Size returns an amount of transfer written in log provided size of a single transfer.
func (lg *TransferLog) Size() int {
	return len(lg.Raw)
}

// EncodeBinary implements io.Serializable interface.
func (t *NEP5Tracker) EncodeBinary(w *io.BinWriter) {
	w.WriteVarBytes(emit.IntToBytes(t.Balance))
	w.WriteU32LE(t.LastUpdatedBlock)
}

// DecodeBinary implements io.Serializable interface.
func (t *NEP5Tracker) DecodeBinary(r *io.BinReader) {
	t.Balance = emit.BytesToInt(r.ReadVarBytes(amountSize))
	t.LastUpdatedBlock = r.ReadU32LE()
}

func parseUint160(addr []byte) util.Uint160 {
	if u, err := util.Uint160DecodeBytesBE(addr); err == nil {
		return u
	}
	return util.Uint160{}
}

// NEP5TransferFromNotification creates NEP5Transfer structure from the given
// notification (and using given context) if it's possible to parse it as
// NEP5 transfer.
func NEP5TransferFromNotification(ne NotificationEvent, txHash util.Uint256, height uint32, time uint32, index uint32) (*NEP5Transfer, error) {
	arr, ok := ne.Item.Value().([]vm.StackItem)
	if !ok || len(arr) != 4 {
		return nil, errors.New("no array or wrong element count")
	}
	op, ok := arr[0].Value().([]byte)
	if !ok || string(op) != "transfer" {
		return nil, errors.New("not a 'transfer' event")
	}
	from, ok := arr[1].Value().([]byte)
	if !ok {
		return nil, errors.New("wrong 'from' type")
	}
	to, ok := arr[2].Value().([]byte)
	if !ok {
		return nil, errors.New("wrong 'to' type")
	}
	amount, ok := arr[3].Value().(*big.Int)
	if !ok {
		bs, ok := arr[3].Value().([]byte)
		if !ok {
			return nil, errors.New("wrong amount type")
		}
		amount = emit.BytesToInt(bs)
	}
	toAddr := parseUint160(to)
	fromAddr := parseUint160(from)
	transfer := &NEP5Transfer{
		Asset:     ne.ScriptHash,
		From:      fromAddr,
		To:        toAddr,
		Amount:    amount,
		Block:     height,
		Timestamp: time,
		Tx:        txHash,
		Index:     index,
	}
	return transfer, nil
}

// EncodeBinary implements io.Serializable interface.
// Note: change NEP5TransferSize constant when changing this function.
func (t *NEP5Transfer) EncodeBinary(w *io.BinWriter) {
	w.WriteBytes(t.Asset[:])
	w.WriteBytes(t.Tx[:])
	w.WriteBytes(t.From[:])
	w.WriteBytes(t.To[:])
	w.WriteU32LE(t.Block)
	w.WriteU32LE(t.Timestamp)
	am := emit.IntToBytes(t.Amount)
	if len(am) > amountSize {
		panic("bad integer length")
	}
	fillerLen := amountSize - len(am)
	w.WriteBytes(am)
	var filler byte
	if t.Amount.Sign() < 0 {
		filler = 0xff
	}
	for i := 0; i < fillerLen; i++ {
		w.WriteB(filler)
	}
	w.WriteU32LE(t.Index)
}

// DecodeBinary implements io.Serializable interface.
func (t *NEP5Transfer) DecodeBinary(r *io.BinReader) {
	r.ReadBytes(t.Asset[:])
	r.ReadBytes(t.Tx[:])
	r.ReadBytes(t.From[:])
	r.ReadBytes(t.To[:])
	t.Block = r.ReadU32LE()
	t.Timestamp = r.ReadU32LE()
	amount := make([]byte, amountSize)
	r.ReadBytes(amount)
	t.Amount = emit.BytesToInt(amount)
	t.Index = r.ReadU32LE()
}
