package ledger

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Transaction is a summary representation of a transaction as returned by
// the native Ledger contract's getTransaction/getTransactionFromBlock
// methods. Unlike [pkg/core/transaction.Transaction], it doesn't contain
// signers (only sender's hash), attributes and witnesses.
type Transaction struct {
	Hash            util.Uint256
	Version         uint8
	Nonce           uint32
	Sender          util.Uint160
	SystemFee       int64
	NetworkFee      int64
	ValidUntilBlock uint32
	Script          []byte
}

// ToStackItem creates [stackitem.Item] representing [Transaction]. It
// never returns an error. It implements [stackitem.Convertible] interface.
func (t *Transaction) ToStackItem() (stackitem.Item, error) {
	return stackitem.NewArray([]stackitem.Item{
		stackitem.NewByteArray(t.Hash.BytesBE()),
		stackitem.NewBigInteger(big.NewInt(int64(t.Version))),
		stackitem.NewBigInteger(big.NewInt(int64(t.Nonce))),
		stackitem.NewByteArray(t.Sender.BytesBE()),
		stackitem.NewBigInteger(big.NewInt(t.SystemFee)),
		stackitem.NewBigInteger(big.NewInt(t.NetworkFee)),
		stackitem.NewBigInteger(big.NewInt(int64(t.ValidUntilBlock))),
		stackitem.NewByteArray(t.Script),
	}), nil
}

// FromStackItem retrieves fields of [Transaction] from the given
// [stackitem.Item] or returns an error if it's not possible to do so. It
// implements [stackitem.Convertible] interface.
func (t *Transaction) FromStackItem(item stackitem.Item) error {
	arr, ok := item.Value().([]stackitem.Item)
	if !ok {
		return errors.New("not an array")
	}
	if len(arr) != 8 {
		return fmt.Errorf("wrong number of structure elements: expected %d, got %d", 8, len(arr))
	}
	hash, err := stackitem.ToUint256(arr[0])
	if err != nil {
		return fmt.Errorf("field Hash: %w", err)
	}
	version, err := stackitem.ToUint8(arr[1])
	if err != nil {
		return fmt.Errorf("field Version: %w", err)
	}
	nonce, err := stackitem.ToUint32(arr[2])
	if err != nil {
		return fmt.Errorf("field Nonce: %w", err)
	}
	sender, err := stackitem.ToUint160(arr[3])
	if err != nil {
		return fmt.Errorf("field Sender: %w", err)
	}
	sysFee, err := stackitem.ToInt64(arr[4])
	if err != nil {
		return fmt.Errorf("field SystemFee: %w", err)
	}
	netFee, err := stackitem.ToInt64(arr[5])
	if err != nil {
		return fmt.Errorf("field NetworkFee: %w", err)
	}
	vub, err := stackitem.ToUint32(arr[6])
	if err != nil {
		return fmt.Errorf("field ValidUntilBlock: %w", err)
	}
	script, err := arr[7].TryBytes()
	if err != nil {
		return fmt.Errorf("field Script: %w", err)
	}

	t.Hash = hash
	t.Version = version
	t.Nonce = nonce
	t.Sender = sender
	t.SystemFee = sysFee
	t.NetworkFee = netFee
	t.ValidUntilBlock = vub
	t.Script = script

	return nil
}

// ToSCParameter creates [smartcontract.Parameter] representing
// [Transaction]. It implements [smartcontract.Convertible] interface so
// that [Transaction] could be used with invokers.
func (t *Transaction) ToSCParameter() (smartcontract.Parameter, error) {
	if t == nil {
		return smartcontract.Parameter{Type: smartcontract.AnyType}, nil
	}

	var (
		err  error
		prm  smartcontract.Parameter
		prms = make([]smartcontract.Parameter, 0, 8)
	)
	prm, err = smartcontract.NewParameterFromValue(t.Hash)
	if err != nil {
		return smartcontract.Parameter{}, fmt.Errorf("field Hash: %w", err)
	}
	prms = append(prms, prm)

	prm, err = smartcontract.NewParameterFromValue(t.Version)
	if err != nil {
		return smartcontract.Parameter{}, fmt.Errorf("field Version: %w", err)
	}
	prms = append(prms, prm)

	prm, err = smartcontract.NewParameterFromValue(t.Nonce)
	if err != nil {
		return smartcontract.Parameter{}, fmt.Errorf("field Nonce: %w", err)
	}
	prms = append(prms, prm)

	prm, err = smartcontract.NewParameterFromValue(t.Sender)
	if err != nil {
		return smartcontract.Parameter{}, fmt.Errorf("field Sender: %w", err)
	}
	prms = append(prms, prm)

	prm, err = smartcontract.NewParameterFromValue(t.SystemFee)
	if err != nil {
		return smartcontract.Parameter{}, fmt.Errorf("field SystemFee: %w", err)
	}
	prms = append(prms, prm)

	prm, err = smartcontract.NewParameterFromValue(t.NetworkFee)
	if err != nil {
		return smartcontract.Parameter{}, fmt.Errorf("field NetworkFee: %w", err)
	}
	prms = append(prms, prm)

	prm, err = smartcontract.NewParameterFromValue(t.ValidUntilBlock)
	if err != nil {
		return smartcontract.Parameter{}, fmt.Errorf("field ValidUntilBlock: %w", err)
	}
	prms = append(prms, prm)

	prm, err = smartcontract.NewParameterFromValue(t.Script)
	if err != nil {
		return smartcontract.Parameter{}, fmt.Errorf("field Script: %w", err)
	}
	prms = append(prms, prm)

	return smartcontract.Parameter{Type: smartcontract.ArrayType, Value: prms}, nil
}
