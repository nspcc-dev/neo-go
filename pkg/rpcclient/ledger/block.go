/*
Package ledger provides RPC wrappers for types used by smart contracts that
import pkg/interop/native/ledger to access the native LedgerContract.
*/
package ledger

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Block is a summary representation of a block as returned by the native
// Ledger contract's getBlock method. Unlike [pkg/core/block.Block], it
// doesn't contain full set of transactions, only their count.
type Block struct {
	Hash               util.Uint256
	Version            uint32
	PrevHash           util.Uint256
	MerkleRoot         util.Uint256
	Timestamp          uint64
	Nonce              uint64
	Index              uint32
	PrimaryIndex       uint8
	NextConsensus      util.Uint160
	TransactionsLength uint16
	StateRootEnabled   bool
	PrevStateRoot      util.Uint256
}

// ToStackItem creates [stackitem.Item] representing [Block]. It never
// returns an error. It implements [stackitem.Convertible] interface.
func (b *Block) ToStackItem() (stackitem.Item, error) {
	items := []stackitem.Item{
		stackitem.NewByteArray(b.Hash.BytesBE()),
		stackitem.NewBigInteger(big.NewInt(int64(b.Version))),
		stackitem.NewByteArray(b.PrevHash.BytesBE()),
		stackitem.NewByteArray(b.MerkleRoot.BytesBE()),
		stackitem.NewBigInteger(new(big.Int).SetUint64(b.Timestamp)),
		stackitem.NewBigInteger(new(big.Int).SetUint64(b.Nonce)),
		stackitem.NewBigInteger(big.NewInt(int64(b.Index))),
		stackitem.NewBigInteger(big.NewInt(int64(b.PrimaryIndex))),
		stackitem.NewByteArray(b.NextConsensus.BytesBE()),
		stackitem.NewBigInteger(big.NewInt(int64(b.TransactionsLength))),
	}
	if b.StateRootEnabled {
		items = append(items, stackitem.NewByteArray(b.PrevStateRoot.BytesBE()))
	}

	return stackitem.NewArray(items), nil
}

// FromStackItem retrieves fields of [Block] from the given [stackitem.Item]
// or returns an error if it's not possible to do so. It implements
// [stackitem.Convertible] interface.
func (b *Block) FromStackItem(item stackitem.Item) error {
	arr, ok := item.Value().([]stackitem.Item)
	if !ok {
		return errors.New("not an array")
	}
	expectedLen := 10
	if b.StateRootEnabled {
		expectedLen = 11
	}
	if len(arr) != expectedLen {
		return fmt.Errorf("wrong number of structure elements: expected %d, got %d", expectedLen, len(arr))
	}
	hash, err := stackitem.ToUint256(arr[0])
	if err != nil {
		return fmt.Errorf("field Hash: %w", err)
	}
	version, err := stackitem.ToUint32(arr[1])
	if err != nil {
		return fmt.Errorf("field Version: %w", err)
	}
	prevHash, err := stackitem.ToUint256(arr[2])
	if err != nil {
		return fmt.Errorf("field PrevHash: %w", err)
	}
	merkleRoot, err := stackitem.ToUint256(arr[3])
	if err != nil {
		return fmt.Errorf("field MerkleRoot: %w", err)
	}
	timestamp, err := stackitem.ToUint64(arr[4])
	if err != nil {
		return fmt.Errorf("field Timestamp: %w", err)
	}
	nonce, err := stackitem.ToUint64(arr[5])
	if err != nil {
		return fmt.Errorf("field Nonce: %w", err)
	}
	index, err := stackitem.ToUint32(arr[6])
	if err != nil {
		return fmt.Errorf("field Index: %w", err)
	}
	primaryIndex, err := stackitem.ToUint8(arr[7])
	if err != nil {
		return fmt.Errorf("field PrimaryIndex: %w", err)
	}
	nextConsensus, err := stackitem.ToUint160(arr[8])
	if err != nil {
		return fmt.Errorf("field NextConsensus: %w", err)
	}
	txLen, err := stackitem.ToUint16(arr[9])
	if err != nil {
		return fmt.Errorf("field TransactionsLength: %w", err)
	}

	b.Hash = hash
	b.Version = version
	b.PrevHash = prevHash
	b.MerkleRoot = merkleRoot
	b.Timestamp = timestamp
	b.Nonce = nonce
	b.Index = index
	b.PrimaryIndex = primaryIndex
	b.NextConsensus = nextConsensus
	b.TransactionsLength = txLen

	if b.StateRootEnabled {
		prevStateRoot, err := stackitem.ToUint256(arr[10])
		if err != nil {
			return fmt.Errorf("field PrevStateRoot: %w", err)
		}
		b.PrevStateRoot = prevStateRoot
	} else {
		b.PrevStateRoot = util.Uint256{}
	}

	return nil
}

// ToSCParameter creates [smartcontract.Parameter] representing [Block]. It
// implements [smartcontract.Convertible] interface so that [Block] could be
// used with invokers.
func (b *Block) ToSCParameter() (smartcontract.Parameter, error) {
	if b == nil {
		return smartcontract.Parameter{Type: smartcontract.AnyType}, nil
	}

	var (
		err  error
		prm  smartcontract.Parameter
		prms = make([]smartcontract.Parameter, 0, 11)
	)
	prm, err = smartcontract.NewParameterFromValue(b.Hash)
	if err != nil {
		return smartcontract.Parameter{}, fmt.Errorf("field Hash: %w", err)
	}
	prms = append(prms, prm)

	prm, err = smartcontract.NewParameterFromValue(b.Version)
	if err != nil {
		return smartcontract.Parameter{}, fmt.Errorf("field Version: %w", err)
	}
	prms = append(prms, prm)

	prm, err = smartcontract.NewParameterFromValue(b.PrevHash)
	if err != nil {
		return smartcontract.Parameter{}, fmt.Errorf("field PrevHash: %w", err)
	}
	prms = append(prms, prm)

	prm, err = smartcontract.NewParameterFromValue(b.MerkleRoot)
	if err != nil {
		return smartcontract.Parameter{}, fmt.Errorf("field MerkleRoot: %w", err)
	}
	prms = append(prms, prm)

	prm, err = smartcontract.NewParameterFromValue(b.Timestamp)
	if err != nil {
		return smartcontract.Parameter{}, fmt.Errorf("field Timestamp: %w", err)
	}
	prms = append(prms, prm)

	prm, err = smartcontract.NewParameterFromValue(b.Nonce)
	if err != nil {
		return smartcontract.Parameter{}, fmt.Errorf("field Nonce: %w", err)
	}
	prms = append(prms, prm)

	prm, err = smartcontract.NewParameterFromValue(b.Index)
	if err != nil {
		return smartcontract.Parameter{}, fmt.Errorf("field Index: %w", err)
	}
	prms = append(prms, prm)

	prm, err = smartcontract.NewParameterFromValue(b.PrimaryIndex)
	if err != nil {
		return smartcontract.Parameter{}, fmt.Errorf("field PrimaryIndex: %w", err)
	}
	prms = append(prms, prm)

	prm, err = smartcontract.NewParameterFromValue(b.NextConsensus)
	if err != nil {
		return smartcontract.Parameter{}, fmt.Errorf("field NextConsensus: %w", err)
	}
	prms = append(prms, prm)

	prm, err = smartcontract.NewParameterFromValue(b.TransactionsLength)
	if err != nil {
		return smartcontract.Parameter{}, fmt.Errorf("field TransactionsLength: %w", err)
	}
	prms = append(prms, prm)

	if b.StateRootEnabled {
		prm, err = smartcontract.NewParameterFromValue(b.PrevStateRoot)
		if err != nil {
			return smartcontract.Parameter{}, fmt.Errorf("field PrevStateRoot: %w", err)
		}
		prms = append(prms, prm)
	}

	return smartcontract.Parameter{Type: smartcontract.ArrayType, Value: prms}, nil
}
