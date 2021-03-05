package native

import (
	"fmt"
	"math"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Ledger provides an interface to blocks/transactions storage for smart
// contracts. It's not a part of the proper chain's state, so it's just a
// proxy between regular Blockchain/DAO interface and smart contracts.
type Ledger struct {
	interop.ContractMD
}

const (
	ledgerContractID = -4

	prefixBlockHash    = 9
	prefixCurrentBlock = 12
	prefixBlock        = 5
	prefixTransaction  = 11
)

// newLedger creates new Ledger native contract.
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

	return l
}

// Metadata implements Contract interface.
func (l *Ledger) Metadata() *interop.ContractMD {
	return &l.ContractMD
}

// Initialize implements Contract interface.
func (l *Ledger) Initialize(ic *interop.Context) error {
	return nil
}

// OnPersist implements Contract interface.
func (l *Ledger) OnPersist(ic *interop.Context) error {
	// Actual block/tx processing is done in Blockchain.storeBlock().
	// Even though C# node add them to storage here, they're not
	// accessible to smart contracts (see isTraceableBlock()), thus
	// the end effect is the same.
	return nil
}

// PostPersist implements Contract interface.
func (l *Ledger) PostPersist(ic *interop.Context) error {
	return nil // Actual block/tx processing is done in Blockchain.storeBlock().
}

// currentHash implements currentHash SC method.
func (l *Ledger) currentHash(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	return stackitem.Make(ic.Chain.CurrentBlockHash().BytesBE())
}

// currentIndex implements currentIndex SC method.
func (l *Ledger) currentIndex(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	return stackitem.Make(ic.Chain.BlockHeight())
}

// getBlock implements getBlock SC method.
func (l *Ledger) getBlock(ic *interop.Context, params []stackitem.Item) stackitem.Item {
	hash := getBlockHashFromItem(ic.Chain, params[0])
	block, err := ic.Chain.GetBlock(hash)
	if err != nil || !isTraceableBlock(ic.Chain, block.Index) {
		return stackitem.Null{}
	}
	return BlockToStackItem(block)
}

// getTransaction returns transaction to the SC.
func (l *Ledger) getTransaction(ic *interop.Context, params []stackitem.Item) stackitem.Item {
	tx, h, err := getTransactionAndHeight(ic.DAO, params[0])
	if err != nil || !isTraceableBlock(ic.Chain, h) {
		return stackitem.Null{}
	}
	return TransactionToStackItem(tx)
}

// getTransactionHeight returns transaction height to the SC.
func (l *Ledger) getTransactionHeight(ic *interop.Context, params []stackitem.Item) stackitem.Item {
	_, h, err := getTransactionAndHeight(ic.DAO, params[0])
	if err != nil || !isTraceableBlock(ic.Chain, h) {
		return stackitem.Make(-1)
	}
	return stackitem.Make(h)
}

// getTransactionFromBlock returns transaction with the given index from the
// block with height or hash specified.
func (l *Ledger) getTransactionFromBlock(ic *interop.Context, params []stackitem.Item) stackitem.Item {
	hash := getBlockHashFromItem(ic.Chain, params[0])
	index := toUint32(params[1])
	block, err := ic.Chain.GetBlock(hash)
	if err != nil || !isTraceableBlock(ic.Chain, block.Index) {
		return stackitem.Null{}
	}
	if index >= uint32(len(block.Transactions)) {
		panic("wrong transaction index")
	}
	return TransactionToStackItem(block.Transactions[index])
}

// isTraceableBlock defines whether we're able to give information about
// the block with index specified.
func isTraceableBlock(bc blockchainer.Blockchainer, index uint32) bool {
	height := bc.BlockHeight()
	MaxTraceableBlocks := bc.GetConfig().MaxTraceableBlocks
	return index <= height && index+MaxTraceableBlocks > height
}

// getBlockHashFromItem converts given stackitem.Item to block hash using given
// Blockchainer if needed. Interop functions accept both block numbers and
// block hashes as parameters, thus this function is needed. It's supposed to
// be called within VM context, so it panics if anything goes wrong.
func getBlockHashFromItem(bc blockchainer.Blockchainer, item stackitem.Item) util.Uint256 {
	bigindex, err := item.TryInteger()
	if err == nil && bigindex.IsInt64() {
		index := bigindex.Int64()
		if index < 0 || index > math.MaxUint32 {
			panic("bad block index")
		}
		if uint32(index) > bc.BlockHeight() {
			panic(fmt.Errorf("no block with index %d", index))
		}
		return bc.GetHeaderHash(int(index))
	}
	bytes, err := item.TryBytes()
	if err != nil {
		panic(err)
	}
	hash, err := util.Uint256DecodeBytesBE(bytes)
	if err != nil {
		panic(err)
	}
	return hash
}

// getTransactionAndHeight returns transaction and its height if it's present
// on the chain. It panics if anything goes wrong.
func getTransactionAndHeight(cd *dao.Cached, item stackitem.Item) (*transaction.Transaction, uint32, error) {
	hashbytes, err := item.TryBytes()
	if err != nil {
		panic(err)
	}
	hash, err := util.Uint256DecodeBytesBE(hashbytes)
	if err != nil {
		panic(err)
	}
	return cd.GetTransaction(hash)
}

// BlockToStackItem converts block.Block to stackitem.Item
func BlockToStackItem(b *block.Block) stackitem.Item {
	return stackitem.NewArray([]stackitem.Item{
		stackitem.NewByteArray(b.Hash().BytesBE()),
		stackitem.NewBigInteger(big.NewInt(int64(b.Version))),
		stackitem.NewByteArray(b.PrevHash.BytesBE()),
		stackitem.NewByteArray(b.MerkleRoot.BytesBE()),
		stackitem.NewBigInteger(big.NewInt(int64(b.Timestamp))),
		stackitem.NewBigInteger(big.NewInt(int64(b.Index))),
		stackitem.NewByteArray(b.NextConsensus.BytesBE()),
		stackitem.NewBigInteger(big.NewInt(int64(len(b.Transactions)))),
	})
}

// TransactionToStackItem converts transaction.Transaction to stackitem.Item
func TransactionToStackItem(t *transaction.Transaction) stackitem.Item {
	return stackitem.NewArray([]stackitem.Item{
		stackitem.NewByteArray(t.Hash().BytesBE()),
		stackitem.NewBigInteger(big.NewInt(int64(t.Version))),
		stackitem.NewBigInteger(big.NewInt(int64(t.Nonce))),
		stackitem.NewByteArray(t.Sender().BytesBE()),
		stackitem.NewBigInteger(big.NewInt(int64(t.SystemFee))),
		stackitem.NewBigInteger(big.NewInt(int64(t.NetworkFee))),
		stackitem.NewBigInteger(big.NewInt(int64(t.ValidUntilBlock))),
		stackitem.NewByteArray(t.Script),
	})
}
