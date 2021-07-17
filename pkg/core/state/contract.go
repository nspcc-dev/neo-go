package state

import (
	"errors"
	"math"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Contract holds information about a smart contract in the NEO blockchain.
type Contract struct {
	ContractBase
	UpdateCounter uint16 `json:"updatecounter"`
}

// ContractBase represents part shared by native and user-deployed contracts.
type ContractBase struct {
	ID       int32             `json:"id"`
	Hash     util.Uint160      `json:"hash"`
	NEF      nef.File          `json:"nef"`
	Manifest manifest.Manifest `json:"manifest"`
}

// NativeContract holds information about native contract.
type NativeContract struct {
	ContractBase
	UpdateHistory []uint32 `json:"updatehistory"`
}

// ToStackItem converts state.Contract to stackitem.Item.
func (c *Contract) ToStackItem() (stackitem.Item, error) {
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

// FromStackItem fills Contract's data from given stack itemized contract
// representation.
func (c *Contract) FromStackItem(item stackitem.Item) error {
	arr, ok := item.Value().([]stackitem.Item)
	if !ok {
		return errors.New("not an array")
	}
	bi, ok := arr[0].Value().(*big.Int)
	if !ok {
		return errors.New("ID is not an integer")
	}
	if !bi.IsInt64() || bi.Int64() > math.MaxInt32 || bi.Int64() < math.MinInt32 {
		return errors.New("ID not in int32 range")
	}
	c.ID = int32(bi.Int64())
	bi, ok = arr[1].Value().(*big.Int)
	if !ok {
		return errors.New("UpdateCounter is not an integer")
	}
	if !bi.IsInt64() || bi.Int64() > math.MaxUint16 || bi.Int64() < 0 {
		return errors.New("UpdateCounter not in uint16 range")
	}
	c.UpdateCounter = uint16(bi.Int64())
	bytes, err := arr[2].TryBytes()
	if err != nil {
		return err
	}
	c.Hash, err = util.Uint160DecodeBytesBE(bytes)
	if err != nil {
		return err
	}
	bytes, err = arr[3].TryBytes()
	if err != nil {
		return err
	}
	c.NEF, err = nef.FileFromBytes(bytes)
	if err != nil {
		return err
	}
	m := new(manifest.Manifest)
	err = m.FromStackItem(arr[4])
	if err != nil {
		return err
	}
	c.Manifest = *m
	return nil
}

// CreateContractHash creates deployed contract hash from transaction sender
// and contract script.
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
