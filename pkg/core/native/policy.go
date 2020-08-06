package native

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"sync"

	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

const (
	policyName       = "Policy"
	policyContractID = -3

	defaultMaxBlockSize            = 1024 * 256
	defaultMaxTransactionsPerBlock = 512
	defaultFeePerByte              = 1000
	defaultMaxVerificationGas      = 50000000
	defaultMaxBlockSystemFee       = 9000 * GASFactor
	// minBlockSystemFee is the minimum allowed system fee per block.
	minBlockSystemFee = 4007600
)

var (
	// maxTransactionsPerBlockKey is a key used to store the maximum number of
	// transactions allowed in block.
	maxTransactionsPerBlockKey = []byte{23}
	// feePerByteKey is a key used to store the minimum fee per byte for
	// transaction.
	feePerByteKey = []byte{10}
	// blockedAccountsKey is a key used to store the list of blocked accounts.
	blockedAccountsKey = []byte{15}
	// maxBlockSizeKey is a key used to store the maximum block size value.
	maxBlockSizeKey = []byte{12}
	// maxBlockSystemFeeKey is a key used to store the maximum block system fee value.
	maxBlockSystemFeeKey = []byte{17}
)

// Policy represents Policy native contract.
type Policy struct {
	interop.ContractMD
	lock sync.RWMutex
	// isValid defies whether cached values were changed during the current
	// consensus iteration. If false, these values will be updated after
	// blockchain DAO persisting. If true, we can safely use cached values.
	isValid                 bool
	maxTransactionsPerBlock uint32
	maxBlockSize            uint32
	feePerByte              int64
	maxBlockSystemFee       int64
	maxVerificationGas      int64
}

var _ interop.Contract = (*Policy)(nil)

// newPolicy returns Policy native contract.
func newPolicy() *Policy {
	p := &Policy{ContractMD: *interop.NewContractMD(policyName)}

	p.ContractID = policyContractID
	p.Manifest.Features |= smartcontract.HasStorage

	desc := newDescriptor("getMaxTransactionsPerBlock", smartcontract.IntegerType)
	md := newMethodAndPrice(p.getMaxTransactionsPerBlock, 1000000, smartcontract.AllowStates)
	p.AddMethod(md, desc, true)

	desc = newDescriptor("getMaxBlockSize", smartcontract.IntegerType)
	md = newMethodAndPrice(p.getMaxBlockSize, 1000000, smartcontract.AllowStates)
	p.AddMethod(md, desc, true)

	desc = newDescriptor("getFeePerByte", smartcontract.IntegerType)
	md = newMethodAndPrice(p.getFeePerByte, 1000000, smartcontract.AllowStates)
	p.AddMethod(md, desc, true)

	desc = newDescriptor("getBlockedAccounts", smartcontract.ArrayType)
	md = newMethodAndPrice(p.getBlockedAccounts, 1000000, smartcontract.AllowStates)
	p.AddMethod(md, desc, true)

	desc = newDescriptor("getMaxBlockSystemFee", smartcontract.IntegerType)
	md = newMethodAndPrice(p.getMaxBlockSystemFee, 1000000, smartcontract.AllowStates)
	p.AddMethod(md, desc, true)

	desc = newDescriptor("setMaxBlockSize", smartcontract.BoolType,
		manifest.NewParameter("value", smartcontract.IntegerType))
	md = newMethodAndPrice(p.setMaxBlockSize, 3000000, smartcontract.AllowModifyStates)
	p.AddMethod(md, desc, false)

	desc = newDescriptor("setMaxTransactionsPerBlock", smartcontract.BoolType,
		manifest.NewParameter("value", smartcontract.IntegerType))
	md = newMethodAndPrice(p.setMaxTransactionsPerBlock, 3000000, smartcontract.AllowModifyStates)
	p.AddMethod(md, desc, false)

	desc = newDescriptor("setFeePerByte", smartcontract.BoolType,
		manifest.NewParameter("value", smartcontract.IntegerType))
	md = newMethodAndPrice(p.setFeePerByte, 3000000, smartcontract.AllowModifyStates)
	p.AddMethod(md, desc, false)

	desc = newDescriptor("setMaxBlockSystemFee", smartcontract.BoolType,
		manifest.NewParameter("value", smartcontract.IntegerType))
	md = newMethodAndPrice(p.setMaxBlockSystemFee, 3000000, smartcontract.AllowModifyStates)
	p.AddMethod(md, desc, false)

	desc = newDescriptor("blockAccount", smartcontract.BoolType,
		manifest.NewParameter("account", smartcontract.Hash160Type))
	md = newMethodAndPrice(p.blockAccount, 3000000, smartcontract.AllowModifyStates)
	p.AddMethod(md, desc, false)

	desc = newDescriptor("unblockAccount", smartcontract.BoolType,
		manifest.NewParameter("account", smartcontract.Hash160Type))
	md = newMethodAndPrice(p.unblockAccount, 3000000, smartcontract.AllowModifyStates)
	p.AddMethod(md, desc, false)

	desc = newDescriptor("onPersist", smartcontract.VoidType)
	md = newMethodAndPrice(getOnPersistWrapper(p.OnPersist), 0, smartcontract.AllowModifyStates)
	p.AddMethod(md, desc, false)
	return p
}

// Metadata implements Contract interface.
func (p *Policy) Metadata() *interop.ContractMD {
	return &p.ContractMD
}

// Initialize initializes Policy native contract and implements Contract interface.
func (p *Policy) Initialize(ic *interop.Context) error {
	si := &state.StorageItem{
		Value: make([]byte, 4, 8),
	}
	binary.LittleEndian.PutUint32(si.Value, defaultMaxBlockSize)
	err := ic.DAO.PutStorageItem(p.ContractID, maxBlockSizeKey, si)
	if err != nil {
		return err
	}

	binary.LittleEndian.PutUint32(si.Value, defaultMaxTransactionsPerBlock)
	err = ic.DAO.PutStorageItem(p.ContractID, maxTransactionsPerBlockKey, si)
	if err != nil {
		return err
	}

	si.Value = si.Value[:8]
	binary.LittleEndian.PutUint64(si.Value, defaultFeePerByte)
	err = ic.DAO.PutStorageItem(p.ContractID, feePerByteKey, si)
	if err != nil {
		return err
	}

	binary.LittleEndian.PutUint64(si.Value, defaultMaxBlockSystemFee)
	err = ic.DAO.PutStorageItem(p.ContractID, maxBlockSystemFeeKey, si)
	if err != nil {
		return err
	}

	ba := new(BlockedAccounts)
	si.Value = ba.Bytes()
	err = ic.DAO.PutStorageItem(p.ContractID, blockedAccountsKey, si)
	if err != nil {
		return err
	}

	p.isValid = true
	p.maxTransactionsPerBlock = defaultMaxTransactionsPerBlock
	p.maxBlockSize = defaultMaxBlockSize
	p.feePerByte = defaultFeePerByte
	p.maxBlockSystemFee = defaultMaxBlockSystemFee
	p.maxVerificationGas = defaultMaxVerificationGas

	return nil
}

// OnPersist implements Contract interface.
func (p *Policy) OnPersist(ic *interop.Context) error {
	return nil
}

// OnPersistEnd updates cached Policy values if they've been changed
func (p *Policy) OnPersistEnd(dao dao.DAO) {
	if p.isValid {
		return
	}
	p.lock.Lock()
	defer p.lock.Unlock()

	maxTxPerBlock := p.getUint32WithKey(dao, maxTransactionsPerBlockKey)
	p.maxTransactionsPerBlock = maxTxPerBlock

	maxBlockSize := p.getUint32WithKey(dao, maxBlockSizeKey)
	p.maxBlockSize = maxBlockSize

	feePerByte := p.getInt64WithKey(dao, feePerByteKey)
	p.feePerByte = feePerByte

	maxBlockSystemFee := p.getInt64WithKey(dao, maxBlockSystemFeeKey)
	p.maxBlockSystemFee = maxBlockSystemFee

	p.maxVerificationGas = defaultMaxVerificationGas

	p.isValid = true
}

// getMaxTransactionsPerBlock is Policy contract method and returns the upper
// limit of transactions per block.
func (p *Policy) getMaxTransactionsPerBlock(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	return stackitem.NewBigInteger(big.NewInt(int64(p.GetMaxTransactionsPerBlockInternal(ic.DAO))))
}

// GetMaxTransactionsPerBlockInternal returns the upper limit of transactions per
// block.
func (p *Policy) GetMaxTransactionsPerBlockInternal(dao dao.DAO) uint32 {
	p.lock.RLock()
	defer p.lock.RUnlock()
	if p.isValid {
		return p.maxTransactionsPerBlock
	}
	return p.getUint32WithKey(dao, maxTransactionsPerBlockKey)
}

// getMaxBlockSize is Policy contract method and returns maximum block size.
func (p *Policy) getMaxBlockSize(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	return stackitem.NewBigInteger(big.NewInt(int64(p.GetMaxBlockSizeInternal(ic.DAO))))
}

// GetMaxBlockSizeInternal returns maximum block size.
func (p *Policy) GetMaxBlockSizeInternal(dao dao.DAO) uint32 {
	p.lock.RLock()
	defer p.lock.RUnlock()
	if p.isValid {
		return p.maxBlockSize
	}
	return p.getUint32WithKey(dao, maxBlockSizeKey)
}

// getFeePerByte is Policy contract method and returns required transaction's fee
// per byte.
func (p *Policy) getFeePerByte(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	return stackitem.NewBigInteger(big.NewInt(p.GetFeePerByteInternal(ic.DAO)))
}

// GetFeePerByteInternal returns required transaction's fee per byte.
func (p *Policy) GetFeePerByteInternal(dao dao.DAO) int64 {
	p.lock.RLock()
	defer p.lock.RUnlock()
	if p.isValid {
		return p.feePerByte
	}
	return p.getInt64WithKey(dao, feePerByteKey)
}

// GetMaxVerificationGas returns maximum gas allowed to be burned during verificaion.
func (p *Policy) GetMaxVerificationGas(_ dao.DAO) int64 {
	if p.isValid {
		return p.maxVerificationGas
	}
	return defaultMaxVerificationGas
}

// getMaxBlockSystemFee is Policy contract method and returns the maximum overall
// system fee per block.
func (p *Policy) getMaxBlockSystemFee(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	return stackitem.NewBigInteger(big.NewInt(p.GetMaxBlockSystemFeeInternal(ic.DAO)))
}

// GetMaxBlockSystemFeeInternal the maximum overall system fee per block.
func (p *Policy) GetMaxBlockSystemFeeInternal(dao dao.DAO) int64 {
	p.lock.RLock()
	defer p.lock.RUnlock()
	if p.isValid {
		return p.maxBlockSystemFee
	}
	return p.getInt64WithKey(dao, maxBlockSystemFeeKey)
}

// getBlockedAccounts is Policy contract method and returns list of blocked
// accounts hashes.
func (p *Policy) getBlockedAccounts(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	ba, err := p.GetBlockedAccountsInternal(ic.DAO)
	if err != nil {
		panic(err)
	}
	return ba.ToStackItem()
}

// GetBlockedAccountsInternal returns list of blocked accounts hashes.
func (p *Policy) GetBlockedAccountsInternal(dao dao.DAO) (BlockedAccounts, error) {
	si := dao.GetStorageItem(p.ContractID, blockedAccountsKey)
	if si == nil {
		return nil, errors.New("BlockedAccounts uninitialized")
	}
	ba, err := BlockedAccountsFromBytes(si.Value)
	if err != nil {
		return nil, err
	}
	return ba, nil
}

// setMaxTransactionsPerBlock is Policy contract method and  sets the upper limit
// of transactions per block.
func (p *Policy) setMaxTransactionsPerBlock(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	ok, err := p.checkValidators(ic)
	if err != nil {
		panic(err)
	}
	if !ok {
		return stackitem.NewBool(false)
	}
	value := uint32(toBigInt(args[0]).Int64())
	p.lock.Lock()
	defer p.lock.Unlock()
	err = p.setUint32WithKey(ic.DAO, maxTransactionsPerBlockKey, value)
	if err != nil {
		panic(err)
	}
	p.isValid = false
	return stackitem.NewBool(true)
}

// setMaxBlockSize is Policy contract method and sets maximum block size.
func (p *Policy) setMaxBlockSize(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	ok, err := p.checkValidators(ic)
	if err != nil {
		panic(err)
	}
	if !ok {
		return stackitem.NewBool(false)
	}
	value := uint32(toBigInt(args[0]).Int64())
	if payload.MaxSize <= value {
		return stackitem.NewBool(false)
	}
	p.lock.Lock()
	defer p.lock.Unlock()
	err = p.setUint32WithKey(ic.DAO, maxBlockSizeKey, value)
	if err != nil {
		panic(err)
	}
	p.isValid = false
	return stackitem.NewBool(true)
}

// setFeePerByte is Policy contract method and sets transaction's fee per byte.
func (p *Policy) setFeePerByte(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	ok, err := p.checkValidators(ic)
	if err != nil {
		panic(err)
	}
	if !ok {
		return stackitem.NewBool(false)
	}
	value := toBigInt(args[0]).Int64()
	p.lock.Lock()
	defer p.lock.Unlock()
	err = p.setInt64WithKey(ic.DAO, feePerByteKey, value)
	if err != nil {
		panic(err)
	}
	p.isValid = false
	return stackitem.NewBool(true)
}

// setMaxBlockSystemFee is Policy contract method and sets the maximum system fee per block.
func (p *Policy) setMaxBlockSystemFee(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	ok, err := p.checkValidators(ic)
	if err != nil {
		panic(err)
	}
	if !ok {
		return stackitem.NewBool(false)
	}
	value := toBigInt(args[0]).Int64()
	if value <= minBlockSystemFee {
		return stackitem.NewBool(false)
	}
	p.lock.Lock()
	defer p.lock.Unlock()
	err = p.setInt64WithKey(ic.DAO, maxBlockSystemFeeKey, value)
	if err != nil {
		panic(err)
	}
	p.isValid = false
	return stackitem.NewBool(true)
}

// blockAccount is Policy contract method and adds given account hash to the list
// of blocked accounts.
func (p *Policy) blockAccount(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	ok, err := p.checkValidators(ic)
	if err != nil {
		panic(err)
	}
	if !ok {
		return stackitem.NewBool(false)
	}
	value := toUint160(args[0])
	si := ic.DAO.GetStorageItem(p.ContractID, blockedAccountsKey)
	if si == nil {
		panic("BlockedAccounts uninitialized")
	}
	ba, err := BlockedAccountsFromBytes(si.Value)
	if err != nil {
		panic(err)
	}
	indexToInsert := sort.Search(len(ba), func(i int) bool {
		return !ba[i].Less(value)
	})
	ba = append(ba, value)
	if indexToInsert != len(ba)-1 && ba[indexToInsert].Equals(value) {
		return stackitem.NewBool(false)
	}
	if len(ba) > 1 {
		copy(ba[indexToInsert+1:], ba[indexToInsert:])
		ba[indexToInsert] = value
	}
	err = ic.DAO.PutStorageItem(p.ContractID, blockedAccountsKey, &state.StorageItem{
		Value: ba.Bytes(),
	})
	if err != nil {
		panic(err)
	}
	return stackitem.NewBool(true)
}

// unblockAccount is Policy contract method and removes given account hash from
// the list of blocked accounts.
func (p *Policy) unblockAccount(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	ok, err := p.checkValidators(ic)
	if err != nil {
		panic(err)
	}
	if !ok {
		return stackitem.NewBool(false)
	}
	value := toUint160(args[0])
	si := ic.DAO.GetStorageItem(p.ContractID, blockedAccountsKey)
	if si == nil {
		panic("BlockedAccounts uninitialized")
	}
	ba, err := BlockedAccountsFromBytes(si.Value)
	if err != nil {
		panic(err)
	}
	indexToRemove := sort.Search(len(ba), func(i int) bool {
		return !ba[i].Less(value)
	})
	if indexToRemove == len(ba) || !ba[indexToRemove].Equals(value) {
		return stackitem.NewBool(false)
	}
	ba = append(ba[:indexToRemove], ba[indexToRemove+1:]...)
	err = ic.DAO.PutStorageItem(p.ContractID, blockedAccountsKey, &state.StorageItem{
		Value: ba.Bytes(),
	})
	if err != nil {
		panic(err)
	}
	return stackitem.NewBool(true)
}

func (p *Policy) getUint32WithKey(dao dao.DAO, key []byte) uint32 {
	si := dao.GetStorageItem(p.ContractID, key)
	if si == nil {
		return 0
	}
	return binary.LittleEndian.Uint32(si.Value)
}

func (p *Policy) setUint32WithKey(dao dao.DAO, key []byte, value uint32) error {
	si := dao.GetStorageItem(p.ContractID, key)
	binary.LittleEndian.PutUint32(si.Value, value)
	err := dao.PutStorageItem(p.ContractID, key, si)
	if err != nil {
		return err
	}
	return nil
}

func (p *Policy) getInt64WithKey(dao dao.DAO, key []byte) int64 {
	si := dao.GetStorageItem(p.ContractID, key)
	if si == nil {
		return 0
	}
	return int64(binary.LittleEndian.Uint64(si.Value))
}

func (p *Policy) setInt64WithKey(dao dao.DAO, key []byte, value int64) error {
	si := dao.GetStorageItem(p.ContractID, key)
	binary.LittleEndian.PutUint64(si.Value, uint64(value))
	err := dao.PutStorageItem(p.ContractID, key, si)
	if err != nil {
		return err
	}
	return nil
}

func (p *Policy) checkValidators(ic *interop.Context) (bool, error) {
	prevBlock, err := ic.Chain.GetBlock(ic.Block.PrevHash)
	if err != nil {
		return false, err
	}
	return runtime.CheckHashedWitness(ic, prevBlock.NextConsensus)
}

// CheckPolicy checks whether transaction's script hashes for verifying are
// included into blocked accounts list.
func (p *Policy) CheckPolicy(ic *interop.Context, tx *transaction.Transaction) error {
	ba, err := p.GetBlockedAccountsInternal(ic.DAO)
	if err != nil {
		return fmt.Errorf("unable to get blocked accounts list: %w", err)
	}
	scriptHashes, err := ic.Chain.GetScriptHashesForVerifying(tx)
	if err != nil {
		return fmt.Errorf("unable to get tx script hashes: %w", err)
	}
	for _, acc := range ba {
		for _, hash := range scriptHashes {
			if acc.Equals(hash) {
				return fmt.Errorf("account %s is blocked", hash.StringLE())
			}
		}
	}
	return nil
}
