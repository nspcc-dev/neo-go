package state

import (
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Contract holds information about a smart contract in the Neo blockchain.
type Contract struct {
	ContractBase
	UpdateCounter uint16 `json:"updatecounter"`
}

// ContractBase represents a part shared by native and user-deployed contracts.
type ContractBase struct {
	ID       int32             `json:"id"`
	Hash     util.Uint160      `json:"hash"`
	NEF      nef.File          `json:"nef"`
	Manifest manifest.Manifest `json:"manifest"`
}

// ToStackItem converts state.Contract to stackitem.Item.
func (c *Contract) ToStackItem() (stackitem.Item, error) {
	// Do not skip the NEF size check, it won't affect native Management related
	// states as the same checked is performed during contract deploy/update.
	rawNef, err := c.NEF.Bytes()
	if err != nil {
		return nil, err
	}
	m, err := c.Manifest.ToStackItem()
	if err != nil {
		return nil, err
	}
	return stackitem.NewArray([]stackitem.Item{
		stackitem.Make(c.ID),
		stackitem.Make(c.UpdateCounter),
		stackitem.NewByteArray(c.Hash.BytesBE()),
		stackitem.NewByteArray(rawNef),
		m,
	}), nil
}

// FromStackItem fills Contract's data from the given stack itemized contract
// representation.
func (c *Contract) FromStackItem(item stackitem.Item) error {
	arr, ok := item.Value().([]stackitem.Item)
	if !ok {
		return errors.New("not an array")
	}
	if len(arr) != 5 {
		return errors.New("invalid structure")
	}
	var err error
	c.ID, err = stackitem.ToInt32(arr[0])
	if err != nil {
		return fmt.Errorf("invalid ID: %w", err)
	}
	c.UpdateCounter, err = stackitem.ToUint16(arr[1])
	if err != nil {
		return fmt.Errorf("invalid update counter: %w", err)
	}
	c.Hash, err = stackitem.ToUint160(arr[2])
	if err != nil {
		return fmt.Errorf("invalid hash: %w", err)
	}
	bytes, err := arr[3].TryBytes()
	if err != nil {
		return fmt.Errorf("invalid NEF: %w", err)
	}
	c.NEF, err = nef.FileFromBytes(bytes)
	if err != nil {
		return fmt.Errorf("failed to decode NEF: %w", err)
	}
	m := new(manifest.Manifest)
	err = m.FromStackItem(arr[4])
	if err != nil {
		return fmt.Errorf("invalid manifest")
	}
	c.Manifest = *m
	return nil
}

// CreateContractHash creates a deployed contract hash from the transaction sender
// and the contract script.
func CreateContractHash(sender util.Uint160, checksum uint32, name string) util.Uint160 {
	w := io.NewBufBinWriter()
	emit.Opcodes(w.BinWriter, opcode.ABORT)
	emit.Bytes(w.BinWriter, sender.BytesBE())
	emit.Int(w.BinWriter, int64(checksum))
	emit.String(w.BinWriter, name)
	if w.Err != nil {
		panic(w.Err)
	}
	return hash.Hash160(w.Bytes())
}

// CreateNativeContractHash calculates the hash for the native contract with the
// given name.
func CreateNativeContractHash(name string) util.Uint160 {
	return CreateContractHash(util.Uint160{}, 0, name)
}
