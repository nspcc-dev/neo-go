package native

import (
	"fmt"
	"math"

	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/nspcc-dev/neo-go/pkg/vm/vmstate"
)

// Ledger provides an interface to blocks/transactions storage for smart
// contracts. It's not a part of the proper chain's state, so it's just a
// proxy between regular Blockchain/DAO interface and smart contracts.
type Ledger struct {
	interop.ContractMD
}

const ledgerContractID = -4

// newLedger creates a new Ledger native contract.
func newLedger() *Ledger {
	var l = &Ledger{
		ContractMD: *interop.NewContractMD(nativenames.Ledger, ledgerContractID),
	}
	defer l.UpdateHash()

	desc := newDescriptor("currentHash", smartcontract.Hash256Type)
	md := newMethodAndPrice(l.currentHash, 1<<15, callflag.ReadStates)
	l.AddMethod(md, desc)

	desc = newDescriptor("currentIndex", smartcontract.IntegerType)
	md = newMethodAndPrice(l.currentIndex, 1<<15, callflag.ReadStates)
	l.AddMethod(md, desc)

	desc = newDescriptor("getBlock", smartcontract.ArrayType,
		manifest.NewParameter("indexOrHash", smartcontract.ByteArrayType))
	md = newMethodAndPrice(l.getBlock, 1<<15, callflag.ReadStates)
	l.AddMethod(md, desc)

	desc = newDescriptor("getTransaction", smartcontract.ArrayType,
		manifest.NewParameter("hash", smartcontract.Hash256Type))
	md = newMethodAndPrice(l.getTransaction, 1<<15, callflag.ReadStates)
	l.AddMethod(md, desc)

	desc = newDescriptor("getTransactionHeight", smartcontract.IntegerType,
		manifest.NewParameter("hash", smartcontract.Hash256Type))
	md = newMethodAndPrice(l.getTransactionHeight, 1<<15, callflag.ReadStates)
	l.AddMethod(md, desc)

	desc = newDescriptor("getTransactionFromBlock", smartcontract.ArrayType,
		manifest.NewParameter("blockIndexOrHash", smartcontract.ByteArrayType),
		manifest.NewParameter("txIndex", smartcontract.IntegerType))
	md = newMethodAndPrice(l.getTransactionFromBlock, 1<<16, callflag.ReadStates)
	l.AddMethod(md, desc)

	desc = newDescriptor("getTransactionSigners", smartcontract.ArrayType,
		manifest.NewParameter("hash", smartcontract.Hash256Type))
	md = newMethodAndPrice(l.getTransactionSigners, 1<<15, callflag.ReadStates)
	l.AddMethod(md, desc)

	desc = newDescriptor("getTransactionVMState", smartcontract.IntegerType,
		manifest.NewParameter("hash", smartcontract.Hash256Type))
	md = newMethodAndPrice(l.getTransactionVMState, 1<<15, callflag.ReadStates)
	l.AddMethod(md, desc)

	return l
}

// Metadata implements the Contract interface.
func (l *Ledger) Metadata() *interop.ContractMD {
	return &l.ContractMD
}

// Initialize implements the Contract interface.
func (l *Ledger) Initialize(ic *interop.Context) error {
	return nil
}

// InitializeCache implements the Contract interface.
func (l *Ledger) InitializeCache(blockHeight uint32, d *dao.Simple) error {
	return nil
}

// OnPersist implements the Contract interface.
func (l *Ledger) OnPersist(ic *interop.Context) error {
	// Actual block/tx processing is done in Blockchain.storeBlock().
	// Even though C# node add them to storage here, they're not
	// accessible to smart contracts (see isTraceableBlock()), thus
	// the end effect is the same.
	return nil
}

// PostPersist implements the Contract interface.
func (l *Ledger) PostPersist(ic *interop.Context) error {
	return nil // Actual block/tx processing is done in Blockchain.storeBlock().
}

// currentHash implements currentHash SC method.
func (l *Ledger) currentHash(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	return stackitem.Make(ic.CurrentBlockHash().BytesBE())
}

// currentIndex implements currentIndex SC method.
func (l *Ledger) currentIndex(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	return stackitem.Make(ic.BlockHeight())
}

// getBlock implements getBlock SC method.
func (l *Ledger) getBlock(ic *interop.Context, params []stackitem.Item) stackitem.Item {
	hash := getBlockHashFromItem(ic, params[0])
	block, err := ic.GetBlock(hash)
	if err != nil || !isTraceableBlock(ic, block.Index) {
		return stackitem.Null{}
	}
	return block.ToStackItem()
}

// getTransaction returns transaction to the SC.
func (l *Ledger) getTransaction(ic *interop.Context, params []stackitem.Item) stackitem.Item {
	tx, h, err := getTransactionAndHeight(ic.DAO, params[0])
	if err != nil || !isTraceableBlock(ic, h) {
		return stackitem.Null{}
	}
	return tx.ToStackItem()
}

// getTransactionHeight returns transaction height to the SC.
func (l *Ledger) getTransactionHeight(ic *interop.Context, params []stackitem.Item) stackitem.Item {
	_, h, err := getTransactionAndHeight(ic.DAO, params[0])
	if err != nil || !isTraceableBlock(ic, h) {
		return stackitem.Make(-1)
	}
	return stackitem.Make(h)
}

// getTransactionFromBlock returns a transaction with the given index from the
// block with the height or hash specified.
func (l *Ledger) getTransactionFromBlock(ic *interop.Context, params []stackitem.Item) stackitem.Item {
	hash := getBlockHashFromItem(ic, params[0])
	index := toUint32(params[1])
	block, err := ic.GetBlock(hash)
	if err != nil || !isTraceableBlock(ic, block.Index) {
		return stackitem.Null{}
	}
	if index >= uint32(len(block.Transactions)) {
		panic("wrong transaction index")
	}
	return block.Transactions[index].ToStackItem()
}

// getTransactionSigners returns transaction signers to the SC.
func (l *Ledger) getTransactionSigners(ic *interop.Context, params []stackitem.Item) stackitem.Item {
	tx, h, err := getTransactionAndHeight(ic.DAO, params[0])
	if err != nil || !isTraceableBlock(ic, h) {
		return stackitem.Null{}
	}
	return transaction.SignersToStackItem(tx.Signers)
}

// getTransactionVMState returns VM state got after transaction invocation.
func (l *Ledger) getTransactionVMState(ic *interop.Context, params []stackitem.Item) stackitem.Item {
	hash, err := getUint256FromItem(params[0])
	if err != nil {
		panic(err)
	}
	h, _, aer, err := ic.DAO.GetTxExecResult(hash)
	if err != nil || !isTraceableBlock(ic, h) {
		return stackitem.Make(vmstate.None)
	}
	return stackitem.Make(aer.VMState)
}

// isTraceableBlock defines whether we're able to give information about
// the block with the index specified.
func isTraceableBlock(ic *interop.Context, index uint32) bool {
	height := ic.BlockHeight()
	MaxTraceableBlocks := ic.Chain.GetConfig().MaxTraceableBlocks
	return index <= height && index+MaxTraceableBlocks > height
}

// getBlockHashFromItem converts the given stackitem.Item to a block hash using the given
// Ledger if needed. Interop functions accept both block numbers and
// block hashes as parameters, thus this function is needed. It's supposed to
// be called within VM context, so it panics if anything goes wrong.
func getBlockHashFromItem(ic *interop.Context, item stackitem.Item) util.Uint256 {
	bigindex, err := item.TryInteger()
	if err == nil && bigindex.IsUint64() {
		index := bigindex.Uint64()
		if index > math.MaxUint32 {
			panic("bad block index")
		}
		if uint32(index) > ic.BlockHeight() {
			panic(fmt.Errorf("no block with index %d", index))
		}
		return ic.Chain.GetHeaderHash(uint32(index))
	}
	hash, err := getUint256FromItem(item)
	if err != nil {
		panic(err)
	}
	return hash
}

func getUint256FromItem(item stackitem.Item) (util.Uint256, error) {
	hashbytes, err := item.TryBytes()
	if err != nil {
		return util.Uint256{}, fmt.Errorf("failed to get hash bytes: %w", err)
	}
	hash, err := util.Uint256DecodeBytesBE(hashbytes)
	if err != nil {
		return util.Uint256{}, fmt.Errorf("failed to decode hash: %w", err)
	}
	return hash, nil
}

// getTransactionAndHeight returns a transaction and its height if it's present
// on the chain. It panics if anything goes wrong.
func getTransactionAndHeight(d *dao.Simple, item stackitem.Item) (*transaction.Transaction, uint32, error) {
	hash, err := getUint256FromItem(item)
	if err != nil {
		panic(err)
	}
	return d.GetTransaction(hash)
}
