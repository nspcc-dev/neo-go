package core

import (
	"fmt"
	"math"
	"math/big"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/mempool"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// Tuning parameters.
const (
	headerBatchCount = 2000
	version          = "0.1.0"

	// This one comes from C# code and it's different from the constant used
	// when creating an asset with Neo.Asset.Create interop call. It looks
	// like 2000000 is coming from the decrementInterval, but C# code doesn't
	// contain any relationship between the two, so we should follow this
	// behavior.
	registeredAssetLifetime = 2 * 2000000

	defaultMemPoolSize = 50000
)

var (
	// ErrAlreadyExists is returned when trying to add some already existing
	// transaction into the pool (not specifying whether it exists in the
	// chain or mempool).
	ErrAlreadyExists = errors.New("already exists")
	// ErrOOM is returned when adding transaction to the memory pool because
	// it reached its full capacity.
	ErrOOM = errors.New("no space left in the memory pool")
	// ErrPolicy is returned on attempt to add transaction that doesn't
	// comply with node's configured policy into the mempool.
	ErrPolicy = errors.New("not allowed by policy")
	// ErrInvalidBlockIndex is returned when trying to add block with index
	// other than expected height of the blockchain.
	ErrInvalidBlockIndex error = errors.New("invalid block index")
)
var (
	genAmount         = []int{8, 7, 6, 5, 4, 3, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	decrementInterval = 2000000
	persistInterval   = 1 * time.Second
)

// Blockchain represents the blockchain.
type Blockchain struct {
	config config.ProtocolConfiguration

	// The only way chain state changes is by adding blocks, so we can't
	// allow concurrent block additions. It differs from the next lock in
	// that it's only for AddBlock method itself, the chain state is
	// protected by the lock below, but holding it during all of AddBlock
	// is too expensive (because the state only changes when persisting
	// change cache).
	addLock sync.Mutex

	// This lock ensures blockchain immutability for operations that need
	// that while performing their tasks. It's mostly used as a read lock
	// with the only writer being the block addition logic.
	lock sync.RWMutex

	// Data access object for CRUD operations around storage.
	dao *dao.Simple

	// Current index/height of the highest block.
	// Read access should always be called by BlockHeight().
	// Write access should only happen in storeBlock().
	blockHeight uint32

	// Current top Block wrapped in an atomic.Value for safe access.
	topBlock atomic.Value

	// Current persisted block count.
	persistedHeight uint32

	// Number of headers stored in the chain file.
	storedHeaderCount uint32

	generationAmount  []int
	decrementInterval int

	// All operations on headerList must be called from an
	// headersOp to be routine safe.
	headerList *HeaderHashList

	// Only for operating on the headerList.
	headersOp     chan headersOpFunc
	headersOpDone chan struct{}

	// Stop synchronization mechanisms.
	stopCh      chan struct{}
	runToExitCh chan struct{}

	memPool mempool.Pool

	// This lock protects concurrent access to keyCache.
	keyCacheLock sync.RWMutex
	// cache for block verification keys.
	keyCache map[util.Uint160]map[string]*keys.PublicKey

	log *zap.Logger

	lastBatch *storage.MemBatch

	contracts native.Contracts
}

type headersOpFunc func(headerList *HeaderHashList)

// NewBlockchain returns a new blockchain object the will use the
// given Store as its underlying storage.
func NewBlockchain(s storage.Store, cfg config.ProtocolConfiguration, log *zap.Logger) (*Blockchain, error) {
	if log == nil {
		return nil, errors.New("empty logger")
	}

	if cfg.MemPoolSize <= 0 {
		cfg.MemPoolSize = defaultMemPoolSize
		log.Info("mempool size is not set or wrong, setting default value", zap.Int("MemPoolSize", cfg.MemPoolSize))
	}
	if cfg.MaxTransactionsPerBlock <= 0 {
		cfg.MaxTransactionsPerBlock = 0
		log.Info("MaxTransactionsPerBlock is not set or wrong, setting default value (unlimited)", zap.Int("MaxTransactionsPerBlock", cfg.MaxTransactionsPerBlock))
	}
	if cfg.MaxFreeTransactionsPerBlock <= 0 {
		cfg.MaxFreeTransactionsPerBlock = 0
		log.Info("MaxFreeTransactionsPerBlock is not set or wrong, setting default value (unlimited)", zap.Int("MaxFreeTransactionsPerBlock", cfg.MaxFreeTransactionsPerBlock))
	}
	if cfg.MaxFreeTransactionSize <= 0 {
		cfg.MaxFreeTransactionSize = 0
		log.Info("MaxFreeTransactionSize is not set or wrong, setting default value (unlimited)", zap.Int("MaxFreeTransactionSize", cfg.MaxFreeTransactionSize))
	}
	if cfg.FeePerExtraByte <= 0 {
		cfg.FeePerExtraByte = 0
		log.Info("FeePerExtraByte is not set or wrong, setting default value", zap.Float64("FeePerExtraByte", cfg.FeePerExtraByte))
	}
	bc := &Blockchain{
		config:        cfg,
		dao:           dao.NewSimple(s),
		headersOp:     make(chan headersOpFunc),
		headersOpDone: make(chan struct{}),
		stopCh:        make(chan struct{}),
		runToExitCh:   make(chan struct{}),
		memPool:       mempool.NewMemPool(cfg.MemPoolSize),
		keyCache:      make(map[util.Uint160]map[string]*keys.PublicKey),
		log:           log,

		generationAmount:  genAmount,
		decrementInterval: decrementInterval,

		contracts: *native.NewContracts(),
	}

	if err := bc.init(); err != nil {
		return nil, err
	}

	return bc, nil
}

func (bc *Blockchain) init() error {
	// If we could not find the version in the Store, we know that there is nothing stored.
	ver, err := bc.dao.GetVersion()
	if err != nil {
		bc.log.Info("no storage version found! creating genesis block")
		if err = bc.dao.PutVersion(version); err != nil {
			return err
		}
		genesisBlock, err := createGenesisBlock(bc.config)
		if err != nil {
			return err
		}
		bc.headerList = NewHeaderHashList(genesisBlock.Hash())
		err = bc.dao.PutCurrentHeader(hashAndIndexToBytes(genesisBlock.Hash(), genesisBlock.Index))
		if err != nil {
			return err
		}
		return bc.storeBlock(genesisBlock)
	}
	if ver != version {
		return fmt.Errorf("storage version mismatch betweeen %s and %s", version, ver)
	}

	// At this point there was no version found in the storage which
	// implies a creating fresh storage with the version specified
	// and the genesis block as first block.
	bc.log.Info("restoring blockchain", zap.String("version", version))

	bHeight, err := bc.dao.GetCurrentBlockHeight()
	if err != nil {
		return err
	}
	bc.blockHeight = bHeight
	bc.persistedHeight = bHeight

	hashes, err := bc.dao.GetHeaderHashes()
	if err != nil {
		return err
	}

	bc.headerList = NewHeaderHashList(hashes...)
	bc.storedHeaderCount = uint32(len(hashes))

	currHeaderHeight, currHeaderHash, err := bc.dao.GetCurrentHeaderHeight()
	if err != nil {
		return err
	}
	if bc.storedHeaderCount == 0 && currHeaderHeight == 0 {
		bc.headerList.Add(currHeaderHash)
	}

	// There is a high chance that the Node is stopped before the next
	// batch of 2000 headers was stored. Via the currentHeaders stored we can sync
	// that with stored blocks.
	if currHeaderHeight >= bc.storedHeaderCount {
		hash := currHeaderHash
		var targetHash util.Uint256
		if bc.headerList.Len() > 0 {
			targetHash = bc.headerList.Get(bc.headerList.Len() - 1)
		} else {
			genesisBlock, err := createGenesisBlock(bc.config)
			if err != nil {
				return err
			}
			targetHash = genesisBlock.Hash()
			bc.headerList.Add(targetHash)
		}
		headers := make([]*block.Header, 0)

		for hash != targetHash {
			header, err := bc.GetHeader(hash)
			if err != nil {
				return fmt.Errorf("could not get header %s: %s", hash, err)
			}
			headers = append(headers, header)
			hash = header.PrevHash
		}
		headerSliceReverse(headers)
		for _, h := range headers {
			if !h.Verify() {
				return fmt.Errorf("bad header %d/%s in the storage", h.Index, h.Hash())
			}
			bc.headerList.Add(h.Hash())
		}
	}

	return nil
}

// Run runs chain loop.
func (bc *Blockchain) Run() {
	persistTimer := time.NewTimer(persistInterval)
	defer func() {
		persistTimer.Stop()
		if err := bc.persist(); err != nil {
			bc.log.Warn("failed to persist", zap.Error(err))
		}
		if err := bc.dao.Store.Close(); err != nil {
			bc.log.Warn("failed to close db", zap.Error(err))
		}
		close(bc.runToExitCh)
	}()
	for {
		select {
		case <-bc.stopCh:
			return
		case op := <-bc.headersOp:
			op(bc.headerList)
			bc.headersOpDone <- struct{}{}
		case <-persistTimer.C:
			go func() {
				err := bc.persist()
				if err != nil {
					bc.log.Warn("failed to persist blockchain", zap.Error(err))
				}
				persistTimer.Reset(persistInterval)
			}()
		}
	}
}

// Close stops Blockchain's internal loop, syncs changes to persistent storage
// and closes it. The Blockchain is no longer functional after the call to Close.
func (bc *Blockchain) Close() {
	close(bc.stopCh)
	<-bc.runToExitCh
}

// AddBlock accepts successive block for the Blockchain, verifies it and
// stores internally. Eventually it will be persisted to the backing storage.
func (bc *Blockchain) AddBlock(block *block.Block) error {
	bc.addLock.Lock()
	defer bc.addLock.Unlock()

	expectedHeight := bc.BlockHeight() + 1
	if expectedHeight != block.Index {
		return ErrInvalidBlockIndex
	}

	headerLen := bc.headerListLen()
	if int(block.Index) == headerLen {
		err := bc.addHeaders(bc.config.VerifyBlocks, block.Header())
		if err != nil {
			return err
		}
	}
	if bc.config.VerifyBlocks {
		err := block.Verify()
		if err != nil {
			return fmt.Errorf("block %s is invalid: %s", block.Hash().StringLE(), err)
		}
		if bc.config.VerifyTransactions {
			for _, tx := range block.Transactions {
				err := bc.VerifyTx(tx, block)
				if err != nil {
					return fmt.Errorf("transaction %s failed to verify: %s", tx.Hash().StringLE(), err)
				}
			}
		}
	}
	return bc.storeBlock(block)
}

// AddHeaders processes the given headers and add them to the
// HeaderHashList. It expects headers to be sorted by index.
func (bc *Blockchain) AddHeaders(headers ...*block.Header) error {
	return bc.addHeaders(bc.config.VerifyBlocks, headers...)
}

// addHeaders is an internal implementation of AddHeaders (`verify` parameter
// tells it to verify or not verify given headers).
func (bc *Blockchain) addHeaders(verify bool, headers ...*block.Header) (err error) {
	var (
		start = time.Now()
		batch = bc.dao.Store.Batch()
	)

	if len(headers) > 0 {
		var i int
		curHeight := bc.HeaderHeight()
		for i = range headers {
			if headers[i].Index > curHeight {
				break
			}
		}
		headers = headers[i:]
	}

	if len(headers) == 0 {
		return nil
	} else if verify {
		// Verify that the chain of the headers is consistent.
		var lastHeader *block.Header
		if lastHeader, err = bc.GetHeader(headers[0].PrevHash); err != nil {
			return fmt.Errorf("previous header was not found: %v", err)
		}
		for _, h := range headers {
			if err = bc.verifyHeader(h, lastHeader); err != nil {
				return
			}
			lastHeader = h
		}
	}

	bc.headersOp <- func(headerList *HeaderHashList) {
		oldlen := headerList.Len()
		for _, h := range headers {
			if int(h.Index-1) >= headerList.Len() {
				err = fmt.Errorf(
					"height of received header %d is higher then the current header %d",
					h.Index, headerList.Len(),
				)
				return
			}
			if int(h.Index) < headerList.Len() {
				continue
			}
			if !h.Verify() {
				err = fmt.Errorf("header %v is invalid", h)
				return
			}
			if err = bc.processHeader(h, batch, headerList); err != nil {
				return
			}
		}

		if oldlen != headerList.Len() {
			updateHeaderHeightMetric(headerList.Len() - 1)
			if err = bc.dao.Store.PutBatch(batch); err != nil {
				return
			}
			bc.log.Debug("done processing headers",
				zap.Int("headerIndex", headerList.Len()-1),
				zap.Uint32("blockHeight", bc.BlockHeight()),
				zap.Duration("took", time.Since(start)))
		}
	}
	<-bc.headersOpDone
	return err
}

// processHeader processes the given header. Note that this is only thread safe
// if executed in headers operation.
func (bc *Blockchain) processHeader(h *block.Header, batch storage.Batch, headerList *HeaderHashList) error {
	headerList.Add(h.Hash())

	buf := io.NewBufBinWriter()
	for int(h.Index)-headerBatchCount >= int(bc.storedHeaderCount) {
		if err := headerList.Write(buf.BinWriter, int(bc.storedHeaderCount), headerBatchCount); err != nil {
			return err
		}
		key := storage.AppendPrefixInt(storage.IXHeaderHashList, int(bc.storedHeaderCount))
		batch.Put(key, buf.Bytes())
		bc.storedHeaderCount += headerBatchCount
		buf.Reset()
	}

	buf.Reset()
	buf.BinWriter.WriteU32LE(0) // sys fee is yet to be calculated
	h.EncodeBinary(buf.BinWriter)
	if buf.Err != nil {
		return buf.Err
	}

	key := storage.AppendPrefix(storage.DataBlock, h.Hash().BytesLE())
	batch.Put(key, buf.Bytes())
	batch.Put(storage.SYSCurrentHeader.Bytes(), hashAndIndexToBytes(h.Hash(), h.Index))

	return nil
}

// getSystemFeeAmount returns sum of all system fees for blocks up to h.
// and 0 if no such block exists.
func (bc *Blockchain) getSystemFeeAmount(h util.Uint256) uint32 {
	_, sf, _ := bc.dao.GetBlock(h)
	return sf
}

// TODO: storeBlock needs some more love, its implemented as in the original
// project. This for the sake of development speed and understanding of what
// is happening here, quite allot as you can see :). If things are wired together
// and all tests are in place, we can make a more optimized and cleaner implementation.
func (bc *Blockchain) storeBlock(block *block.Block) error {
	cache := dao.NewCached(bc.dao)
	fee := bc.getSystemFeeAmount(block.PrevHash)
	for _, tx := range block.Transactions {
		fee += uint32(bc.SystemFee(tx).IntegralValue())
	}
	if err := cache.StoreAsBlock(block, fee); err != nil {
		return err
	}

	if err := cache.StoreAsCurrentBlock(block); err != nil {
		return err
	}

	for _, tx := range block.Transactions {
		if err := cache.StoreAsTransaction(tx, block.Index); err != nil {
			return err
		}

		if err := cache.PutUnspentCoinState(tx.Hash(), state.NewUnspentCoin(block.Index, tx)); err != nil {
			return err
		}

		// Process TX outputs.
		if err := processOutputs(tx, cache); err != nil {
			return err
		}

		// Process TX inputs that are grouped by previous hash.
		for _, inputs := range transaction.GroupInputsByPrevHash(tx.Inputs) {
			prevHash := inputs[0].PrevHash
			unspent, err := cache.GetUnspentCoinState(prevHash)
			if err != nil {
				return err
			}
			for _, input := range inputs {
				if len(unspent.States) <= int(input.PrevIndex) {
					return fmt.Errorf("bad input: %s/%d", input.PrevHash.StringLE(), input.PrevIndex)
				}
				if unspent.States[input.PrevIndex].State&state.CoinSpent != 0 {
					return fmt.Errorf("double spend: %s/%d", input.PrevHash.StringLE(), input.PrevIndex)
				}
				unspent.States[input.PrevIndex].State |= state.CoinSpent
				unspent.States[input.PrevIndex].SpendHeight = block.Index
				prevTXOutput := &unspent.States[input.PrevIndex].Output
				account, err := cache.GetAccountStateOrNew(prevTXOutput.ScriptHash)
				if err != nil {
					return err
				}

				if prevTXOutput.AssetID.Equals(GoverningTokenID()) {
					err = account.Unclaimed.Put(&state.UnclaimedBalance{
						Tx:    input.PrevHash,
						Index: input.PrevIndex,
						Start: unspent.Height,
						End:   block.Index,
						Value: prevTXOutput.Amount,
					})
					if err != nil {
						return err
					}
					if err = processTXWithValidatorsSubtract(prevTXOutput, account, cache); err != nil {
						return err
					}
				}

				balancesLen := len(account.Balances[prevTXOutput.AssetID])
				if balancesLen <= 1 {
					delete(account.Balances, prevTXOutput.AssetID)
				} else {
					var index = -1
					for i, balance := range account.Balances[prevTXOutput.AssetID] {
						if balance.Tx.Equals(input.PrevHash) && balance.Index == input.PrevIndex {
							index = i
							break
						}
					}
					if index >= 0 {
						last := balancesLen - 1
						if last > index {
							account.Balances[prevTXOutput.AssetID][index] = account.Balances[prevTXOutput.AssetID][last]
						}
						account.Balances[prevTXOutput.AssetID] = account.Balances[prevTXOutput.AssetID][:last]
					}
				}
				if err = cache.PutAccountState(account); err != nil {
					return err
				}
			}
			if err = cache.PutUnspentCoinState(prevHash, unspent); err != nil {
				return err
			}
		}

		// Process the underlying type of the TX.
		switch t := tx.Data.(type) {
		case *transaction.RegisterTX:
			err := cache.PutAssetState(&state.Asset{
				ID:         tx.Hash(),
				AssetType:  t.AssetType,
				Name:       t.Name,
				Amount:     t.Amount,
				Precision:  t.Precision,
				Owner:      t.Owner,
				Admin:      t.Admin,
				Expiration: bc.BlockHeight() + registeredAssetLifetime,
			})
			if err != nil {
				return err
			}
		case *transaction.IssueTX:
			for _, res := range bc.GetTransactionResults(tx) {
				if res.Amount < 0 {
					asset, err := cache.GetAssetState(res.AssetID)
					if asset == nil || err != nil {
						return fmt.Errorf("issue failed: no asset %s or error %s", res.AssetID, err)
					}
					asset.Available -= res.Amount
					if err := cache.PutAssetState(asset); err != nil {
						return err
					}
				}
			}
		case *transaction.ClaimTX:
			// Remove claimed NEO from spent coins making it unavalaible for
			// additional claims.
			for _, input := range t.Claims {
				scs, err := cache.GetUnspentCoinState(input.PrevHash)
				if err == nil {
					if len(scs.States) <= int(input.PrevIndex) {
						err = errors.New("invalid claim index")
					} else if scs.States[input.PrevIndex].State&state.CoinClaimed != 0 {
						err = errors.New("double claim")
					}
				}
				if err != nil {
					// We can't really do anything about it
					// as it's a transaction in a signed block.
					bc.log.Warn("FALSE OR DOUBLE CLAIM",
						zap.String("PrevHash", input.PrevHash.StringLE()),
						zap.Uint16("PrevIndex", input.PrevIndex),
						zap.String("tx", tx.Hash().StringLE()),
						zap.Uint32("block", block.Index),
					)
					// "Strict" mode.
					if bc.config.VerifyTransactions {
						return err
					}
					break
				}

				acc, err := cache.GetAccountState(scs.States[input.PrevIndex].ScriptHash)
				if err != nil {
					return err
				}

				scs.States[input.PrevIndex].State |= state.CoinClaimed
				if err = cache.PutUnspentCoinState(input.PrevHash, scs); err != nil {
					return err
				}

				changed := acc.Unclaimed.Remove(input.PrevHash, input.PrevIndex)
				if !changed {
					bc.log.Warn("no spent coin in the account",
						zap.String("tx", tx.Hash().StringLE()),
						zap.String("input", input.PrevHash.StringLE()),
						zap.String("account", acc.ScriptHash.String()))
				} else if err := cache.PutAccountState(acc); err != nil {
					return err
				}
			}
		case *transaction.EnrollmentTX:
			if err := processEnrollmentTX(cache, t); err != nil {
				return err
			}
		case *transaction.StateTX:
			if err := bc.processStateTX(cache, tx, t); err != nil {
				return err
			}
		case *transaction.InvocationTX:
			systemInterop := bc.newInteropContext(trigger.Application, cache, block, tx)
			v := SpawnVM(systemInterop)
			v.LoadScript(t.Script)
			v.SetPriceGetter(getPrice)
			if bc.config.FreeGasLimit > 0 {
				v.SetGasLimit(bc.config.FreeGasLimit + t.Gas)
			}

			err := v.Run()
			if !v.HasFailed() {
				_, err := systemInterop.DAO.Persist()
				if err != nil {
					return errors.Wrap(err, "failed to persist invocation results")
				}
				for _, note := range systemInterop.Notifications {
					arr, ok := note.Item.Value().([]vm.StackItem)
					if !ok || len(arr) != 4 {
						continue
					}
					op, ok := arr[0].Value().([]byte)
					if !ok || string(op) != "transfer" {
						continue
					}
					from, ok := arr[1].Value().([]byte)
					if !ok {
						continue
					}
					to, ok := arr[2].Value().([]byte)
					if !ok {
						continue
					}
					amount, ok := arr[3].Value().(*big.Int)
					if !ok {
						bs, ok := arr[3].Value().([]byte)
						if !ok {
							continue
						}
						amount = emit.BytesToInt(bs)
					}
					bc.processNEP5Transfer(cache, tx, block, note.ScriptHash, from, to, amount.Int64())
				}
			} else {
				bc.log.Warn("contract invocation failed",
					zap.String("tx", tx.Hash().StringLE()),
					zap.Uint32("block", block.Index),
					zap.Error(err))
			}
			aer := &state.AppExecResult{
				TxHash:      tx.Hash(),
				Trigger:     trigger.Application,
				VMState:     v.State(),
				GasConsumed: v.GasConsumed(),
				Stack:       v.Estack().ToContractParameters(),
				Events:      systemInterop.Notifications,
			}
			err = cache.PutAppExecResult(aer)
			if err != nil {
				return errors.Wrap(err, "failed to Store notifications")
			}
		}
	}
	bc.lock.Lock()
	defer bc.lock.Unlock()

	if bc.config.SaveStorageBatch {
		bc.lastBatch = cache.DAO.GetBatch()
	}

	for i := range bc.contracts.Contracts {
		systemInterop := bc.newInteropContext(trigger.Application, cache, block, nil)
		if err := bc.contracts.Contracts[i].OnPersist(systemInterop); err != nil {
			return err
		}
	}

	_, err := cache.Persist()
	if err != nil {
		return err
	}
	bc.topBlock.Store(block)
	atomic.StoreUint32(&bc.blockHeight, block.Index)
	updateBlockHeightMetric(block.Index)
	bc.memPool.RemoveStale(bc.isTxStillRelevant)
	return nil
}

func parseUint160(addr []byte) util.Uint160 {
	if u, err := util.Uint160DecodeBytesBE(addr); err == nil {
		return u
	}
	return util.Uint160{}
}

func (bc *Blockchain) processNEP5Transfer(cache *dao.Cached, tx *transaction.Transaction, b *block.Block, sc util.Uint160, from, to []byte, amount int64) {
	toAddr := parseUint160(to)
	fromAddr := parseUint160(from)
	transfer := &state.NEP5Transfer{
		Asset:     sc,
		From:      fromAddr,
		To:        toAddr,
		Block:     b.Index,
		Timestamp: b.Timestamp,
		Tx:        tx.Hash(),
	}
	if !fromAddr.Equals(util.Uint160{}) {
		balances, err := cache.GetNEP5Balances(fromAddr)
		if err != nil {
			return
		}
		bs := balances.Trackers[sc]
		bs.Balance -= amount
		bs.LastUpdatedBlock = b.Index
		balances.Trackers[sc] = bs

		transfer.Amount = -amount
		isBig, err := cache.AppendNEP5Transfer(fromAddr, balances.NextTransferBatch, transfer)
		if err != nil {
			return
		}
		if isBig {
			balances.NextTransferBatch++
		}
		if err := cache.PutNEP5Balances(fromAddr, balances); err != nil {
			return
		}
	}
	if !toAddr.Equals(util.Uint160{}) {
		balances, err := cache.GetNEP5Balances(toAddr)
		if err != nil {
			return
		}
		bs := balances.Trackers[sc]
		bs.Balance += amount
		bs.LastUpdatedBlock = b.Index
		balances.Trackers[sc] = bs

		transfer.Amount = amount
		isBig, err := cache.AppendNEP5Transfer(toAddr, balances.NextTransferBatch, transfer)
		if err != nil {
			return
		}
		if isBig {
			balances.NextTransferBatch++
		}
		if err := cache.PutNEP5Balances(toAddr, balances); err != nil {
			return
		}
	}
}

// GetNEP5TransferLog returns NEP5 transfer log for the acc.
func (bc *Blockchain) GetNEP5TransferLog(acc util.Uint160) *state.NEP5TransferLog {
	balances, err := bc.dao.GetNEP5Balances(acc)
	if err != nil {
		return nil
	}
	result := new(state.NEP5TransferLog)
	for i := uint32(0); i <= balances.NextTransferBatch; i++ {
		lg, err := bc.dao.GetNEP5TransferLog(acc, i)
		if err != nil {
			return nil
		}
		result.Raw = append(result.Raw, lg.Raw...)
	}
	return result
}

// GetNEP5Balances returns NEP5 balances for the acc.
func (bc *Blockchain) GetNEP5Balances(acc util.Uint160) *state.NEP5Balances {
	bs, err := bc.dao.GetNEP5Balances(acc)
	if err != nil {
		return nil
	}
	return bs
}

// LastBatch returns last persisted storage batch.
func (bc *Blockchain) LastBatch() *storage.MemBatch {
	return bc.lastBatch
}

// processOutputs processes transaction outputs.
func processOutputs(tx *transaction.Transaction, dao *dao.Cached) error {
	for index, output := range tx.Outputs {
		account, err := dao.GetAccountStateOrNew(output.ScriptHash)
		if err != nil {
			return err
		}
		account.Balances[output.AssetID] = append(account.Balances[output.AssetID], state.UnspentBalance{
			Tx:    tx.Hash(),
			Index: uint16(index),
			Value: output.Amount,
		})
		if err = dao.PutAccountState(account); err != nil {
			return err
		}
		if err = processTXWithValidatorsAdd(&output, account, dao); err != nil {
			return err
		}
	}
	return nil
}

func processTXWithValidatorsAdd(output *transaction.Output, account *state.Account, dao *dao.Cached) error {
	if output.AssetID.Equals(GoverningTokenID()) && len(account.Votes) > 0 {
		return modAccountVotes(account, dao, output.Amount)
	}
	return nil
}

func processTXWithValidatorsSubtract(output *transaction.Output, account *state.Account, dao *dao.Cached) error {
	if output.AssetID.Equals(GoverningTokenID()) && len(account.Votes) > 0 {
		return modAccountVotes(account, dao, -output.Amount)
	}
	return nil
}

// modAccountVotes adds given value to given account voted validators.
func modAccountVotes(account *state.Account, dao *dao.Cached, value util.Fixed8) error {
	if err := native.ModifyAccountVotes(account, dao, value); err != nil {
		return err
	}
	if len(account.Votes) > 0 {
		vc, err := dao.GetValidatorsCount()
		if err != nil {
			return err
		}
		vc[len(account.Votes)-1] += value
		err = dao.PutValidatorsCount(vc)
		if err != nil {
			return err
		}
	}
	return nil
}

func processValidatorStateDescriptor(descriptor *transaction.StateDescriptor, dao *dao.Cached) error {
	publicKey := &keys.PublicKey{}
	err := publicKey.DecodeBytes(descriptor.Key)
	if err != nil {
		return err
	}
	validatorState, err := dao.GetValidatorStateOrNew(publicKey)
	if err != nil {
		return err
	}
	if descriptor.Field == "Registered" {
		if len(descriptor.Value) == 1 {
			validatorState.Registered = descriptor.Value[0] != 0
			return dao.PutValidatorState(validatorState)
		}
		return errors.New("bad descriptor value")
	}
	return nil
}

func (bc *Blockchain) processAccountStateDescriptor(descriptor *transaction.StateDescriptor, t *transaction.Transaction, dao *dao.Cached) error {
	hash, err := util.Uint160DecodeBytesBE(descriptor.Key)
	if err != nil {
		return err
	}

	if descriptor.Field == "Votes" {
		votes := keys.PublicKeys{}
		if err := votes.DecodeBytes(descriptor.Value); err != nil {
			return err
		}
		ic := bc.newInteropContext(trigger.Application, dao, nil, t)
		return bc.contracts.NEO.VoteInternal(ic, hash, votes)
	}
	return nil
}

// persist flushes current in-memory Store contents to the persistent storage.
func (bc *Blockchain) persist() error {
	var (
		start     = time.Now()
		persisted int
		err       error
	)

	persisted, err = bc.dao.Persist()
	if err != nil {
		return err
	}
	if persisted > 0 {
		bHeight, err := bc.dao.GetCurrentBlockHeight()
		if err != nil {
			return err
		}
		oldHeight := atomic.SwapUint32(&bc.persistedHeight, bHeight)
		diff := bHeight - oldHeight

		storedHeaderHeight, _, err := bc.dao.GetCurrentHeaderHeight()
		if err != nil {
			return err
		}
		bc.log.Info("blockchain persist completed",
			zap.Uint32("persistedBlocks", diff),
			zap.Int("persistedKeys", persisted),
			zap.Uint32("headerHeight", storedHeaderHeight),
			zap.Uint32("blockHeight", bHeight),
			zap.Duration("took", time.Since(start)))

		// update monitoring metrics.
		updatePersistedHeightMetric(bHeight)
	}

	return nil
}

func (bc *Blockchain) headerListLen() (n int) {
	bc.headersOp <- func(headerList *HeaderHashList) {
		n = headerList.Len()
	}
	<-bc.headersOpDone
	return
}

// GetTransaction returns a TX and its height by the given hash.
func (bc *Blockchain) GetTransaction(hash util.Uint256) (*transaction.Transaction, uint32, error) {
	if tx, _, ok := bc.memPool.TryGetValue(hash); ok {
		return tx, 0, nil // the height is not actually defined for memPool transaction. Not sure if zero is a good number in this case.
	}
	return bc.dao.GetTransaction(hash)
}

// GetAppExecResult returns application execution result by the given
// tx hash.
func (bc *Blockchain) GetAppExecResult(hash util.Uint256) (*state.AppExecResult, error) {
	return bc.dao.GetAppExecResult(hash)
}

// GetStorageItem returns an item from storage.
func (bc *Blockchain) GetStorageItem(scripthash util.Uint160, key []byte) *state.StorageItem {
	return bc.dao.GetStorageItem(scripthash, key)
}

// GetStorageItems returns all storage items for a given scripthash.
func (bc *Blockchain) GetStorageItems(hash util.Uint160) (map[string]*state.StorageItem, error) {
	return bc.dao.GetStorageItems(hash)
}

// GetBlock returns a Block by the given hash.
func (bc *Blockchain) GetBlock(hash util.Uint256) (*block.Block, error) {
	topBlock := bc.topBlock.Load()
	if topBlock != nil {
		if tb, ok := topBlock.(*block.Block); ok && tb.Hash().Equals(hash) {
			return tb, nil
		}
	}

	block, _, err := bc.dao.GetBlock(hash)
	if err != nil {
		return nil, err
	}
	if len(block.Transactions) == 0 {
		return nil, fmt.Errorf("only header is available")
	}
	for _, tx := range block.Transactions {
		stx, _, err := bc.dao.GetTransaction(tx.Hash())
		if err != nil {
			return nil, err
		}
		*tx = *stx
	}
	return block, nil
}

// GetHeader returns data block header identified with the given hash value.
func (bc *Blockchain) GetHeader(hash util.Uint256) (*block.Header, error) {
	topBlock := bc.topBlock.Load()
	if topBlock != nil {
		if tb, ok := topBlock.(*block.Block); ok && tb.Hash().Equals(hash) {
			return tb.Header(), nil
		}
	}
	block, _, err := bc.dao.GetBlock(hash)
	if err != nil {
		return nil, err
	}
	return block.Header(), nil
}

// HasTransaction returns true if the blockchain contains he given
// transaction hash.
func (bc *Blockchain) HasTransaction(hash util.Uint256) bool {
	return bc.memPool.ContainsKey(hash) || bc.dao.HasTransaction(hash)
}

// HasBlock returns true if the blockchain contains the given
// block hash.
func (bc *Blockchain) HasBlock(hash util.Uint256) bool {
	if header, err := bc.GetHeader(hash); err == nil {
		return header.Index <= bc.BlockHeight()
	}
	return false
}

// CurrentBlockHash returns the highest processed block hash.
func (bc *Blockchain) CurrentBlockHash() (hash util.Uint256) {
	bc.headersOp <- func(headerList *HeaderHashList) {
		hash = headerList.Get(int(bc.BlockHeight()))
	}
	<-bc.headersOpDone
	return
}

// CurrentHeaderHash returns the hash of the latest known header.
func (bc *Blockchain) CurrentHeaderHash() (hash util.Uint256) {
	bc.headersOp <- func(headerList *HeaderHashList) {
		hash = headerList.Last()
	}
	<-bc.headersOpDone
	return
}

// GetHeaderHash returns the hash from the headerList by its
// height/index.
func (bc *Blockchain) GetHeaderHash(i int) (hash util.Uint256) {
	bc.headersOp <- func(headerList *HeaderHashList) {
		hash = headerList.Get(i)
	}
	<-bc.headersOpDone
	return
}

// BlockHeight returns the height/index of the highest block.
func (bc *Blockchain) BlockHeight() uint32 {
	return atomic.LoadUint32(&bc.blockHeight)
}

// HeaderHeight returns the index/height of the highest header.
func (bc *Blockchain) HeaderHeight() uint32 {
	return uint32(bc.headerListLen() - 1)
}

// GetAssetState returns asset state from its assetID.
func (bc *Blockchain) GetAssetState(assetID util.Uint256) *state.Asset {
	asset, err := bc.dao.GetAssetState(assetID)
	if asset == nil && err != storage.ErrKeyNotFound {
		bc.log.Warn("failed to get asset state",
			zap.Stringer("asset", assetID),
			zap.Error(err))
	}
	return asset
}

// GetContractState returns contract by its script hash.
func (bc *Blockchain) GetContractState(hash util.Uint160) *state.Contract {
	contract, err := bc.dao.GetContractState(hash)
	if contract == nil && err != storage.ErrKeyNotFound {
		bc.log.Warn("failed to get contract state", zap.Error(err))
	}
	return contract
}

// GetAccountState returns the account state from its script hash.
func (bc *Blockchain) GetAccountState(scriptHash util.Uint160) *state.Account {
	as, err := bc.dao.GetAccountState(scriptHash)
	if as == nil && err != storage.ErrKeyNotFound {
		bc.log.Warn("failed to get account state", zap.Error(err))
	}
	return as
}

// GetUnspentCoinState returns unspent coin state for given tx hash.
func (bc *Blockchain) GetUnspentCoinState(hash util.Uint256) *state.UnspentCoin {
	ucs, err := bc.dao.GetUnspentCoinState(hash)
	if ucs == nil && err != storage.ErrKeyNotFound {
		bc.log.Warn("failed to get unspent coin state", zap.Error(err))
	}
	return ucs
}

// GetConfig returns the config stored in the blockchain.
func (bc *Blockchain) GetConfig() config.ProtocolConfiguration {
	return bc.config
}

// CalculateClaimable calculates the amount of GAS which can be claimed for a transaction with value.
// First return value is GAS generated between startHeight and endHeight.
// Second return value is GAS returned from accumulated SystemFees between startHeight and endHeight.
func (bc *Blockchain) CalculateClaimable(value util.Fixed8, startHeight, endHeight uint32) (util.Fixed8, util.Fixed8, error) {
	var amount util.Fixed8
	di := uint32(bc.decrementInterval)

	ustart := startHeight / di
	if genSize := uint32(len(bc.generationAmount)); ustart < genSize {
		uend := endHeight / di
		iend := endHeight % di
		if uend >= genSize {
			uend = genSize - 1
			iend = di
		} else if iend == 0 {
			uend--
			iend = di
		}

		istart := startHeight % di
		for ustart < uend {
			amount += util.Fixed8(di-istart) * util.Fixed8(bc.generationAmount[ustart])
			ustart++
			istart = 0
		}

		amount += util.Fixed8(iend-istart) * util.Fixed8(bc.generationAmount[ustart])
	}

	if startHeight == 0 {
		startHeight++
	}
	h := bc.GetHeaderHash(int(startHeight - 1))
	feeStart := bc.getSystemFeeAmount(h)
	h = bc.GetHeaderHash(int(endHeight - 1))
	feeEnd := bc.getSystemFeeAmount(h)

	sysFeeTotal := util.Fixed8(feeEnd - feeStart)
	ratio := value / 100000000
	return amount * ratio, sysFeeTotal * ratio, nil
}

// References maps transaction's inputs into a slice of InOuts, effectively
// joining each Input with the corresponding Output.
// @TODO: unfortunately we couldn't attach this method to the Transaction struct in the
// transaction package because of a import cycle problem. Perhaps we should think to re-design
// the code base to avoid this situation.
func (bc *Blockchain) References(t *transaction.Transaction) ([]transaction.InOut, error) {
	return bc.references(t.Inputs)
}

// references is an internal implementation of References that operates directly
// on a slice of Input.
func (bc *Blockchain) references(ins []transaction.Input) ([]transaction.InOut, error) {
	references := make([]transaction.InOut, 0, len(ins))

	for _, inputs := range transaction.GroupInputsByPrevHash(ins) {
		prevHash := inputs[0].PrevHash
		unspent, err := bc.dao.GetUnspentCoinState(prevHash)
		if err != nil {
			return nil, errors.New("bad input reference")
		}
		for _, in := range inputs {
			if int(in.PrevIndex) > len(unspent.States)-1 {
				return nil, errors.New("bad input reference")
			}
			references = append(references, transaction.InOut{In: *in, Out: unspent.States[in.PrevIndex].Output})
		}
	}
	return references, nil
}

// FeePerByte returns network fee divided by the size of the transaction.
func (bc *Blockchain) FeePerByte(t *transaction.Transaction) util.Fixed8 {
	return bc.NetworkFee(t).Div(int64(io.GetVarSize(t)))
}

// NetworkFee returns network fee.
func (bc *Blockchain) NetworkFee(t *transaction.Transaction) util.Fixed8 {
	// https://github.com/neo-project/neo/blob/master-2.x/neo/Network/P2P/Payloads/ClaimTransaction.cs#L16
	if t.Type == transaction.ClaimType || t.Type == transaction.MinerType {
		return 0
	}

	inputAmount := util.Fixed8FromInt64(0)
	refs, err := bc.References(t)
	if err != nil {
		return inputAmount
	}
	for i := range refs {
		if refs[i].Out.AssetID == UtilityTokenID() {
			inputAmount = inputAmount.Add(refs[i].Out.Amount)
		}
	}

	outputAmount := util.Fixed8FromInt64(0)
	for _, txOutput := range t.Outputs {
		if txOutput.AssetID == UtilityTokenID() {
			outputAmount = outputAmount.Add(txOutput.Amount)
		}
	}

	return inputAmount.Sub(outputAmount).Sub(bc.SystemFee(t))
}

// SystemFee returns system fee.
func (bc *Blockchain) SystemFee(t *transaction.Transaction) util.Fixed8 {
	if t.Type == transaction.InvocationType {
		inv := t.Data.(*transaction.InvocationTX)
		if inv.Version >= 1 {
			return inv.Gas
		}
	}
	return bc.GetConfig().SystemFee.TryGetValue(t.Type)
}

// IsLowPriority checks given fee for being less than configured
// LowPriorityThreshold.
func (bc *Blockchain) IsLowPriority(fee util.Fixed8) bool {
	return fee < util.Fixed8FromFloat(bc.GetConfig().LowPriorityThreshold)
}

// GetMemPool returns the memory pool of the blockchain.
func (bc *Blockchain) GetMemPool() *mempool.Pool {
	return &bc.memPool
}

// ApplyPolicyToTxSet applies configured policies to given transaction set. It
// expects slice to be ordered by fee and returns a subslice of it.
func (bc *Blockchain) ApplyPolicyToTxSet(txes []mempool.TxWithFee) []mempool.TxWithFee {
	if bc.config.MaxTransactionsPerBlock != 0 && len(txes) > bc.config.MaxTransactionsPerBlock {
		txes = txes[:bc.config.MaxTransactionsPerBlock]
	}
	maxFree := bc.config.MaxFreeTransactionsPerBlock
	if maxFree != 0 {
		lowStart := sort.Search(len(txes), func(i int) bool {
			return bc.IsLowPriority(txes[i].Fee)
		})
		if lowStart+maxFree < len(txes) {
			txes = txes[:lowStart+maxFree]
		}
	}
	return txes
}

func (bc *Blockchain) verifyHeader(currHeader, prevHeader *block.Header) error {
	if prevHeader.Hash() != currHeader.PrevHash {
		return errors.New("previous header hash doesn't match")
	}
	if prevHeader.Index+1 != currHeader.Index {
		return errors.New("previous header index doesn't match")
	}
	if prevHeader.Timestamp >= currHeader.Timestamp {
		return errors.New("block is not newer than the previous one")
	}
	return bc.verifyHeaderWitnesses(currHeader, prevHeader)
}

// verifyTx verifies whether a transaction is bonafide or not.
func (bc *Blockchain) verifyTx(t *transaction.Transaction, block *block.Block) error {
	height := bc.BlockHeight()
	if t.ValidUntilBlock <= height || t.ValidUntilBlock > height+transaction.MaxValidUntilBlockIncrement {
		return errors.Errorf("transaction has expired. ValidUntilBlock = %d, current height = %d", t.ValidUntilBlock, height)
	}
	if io.GetVarSize(t) > transaction.MaxTransactionSize {
		return errors.Errorf("invalid transaction size = %d. It shoud be less then MaxTransactionSize = %d", io.GetVarSize(t), transaction.MaxTransactionSize)
	}
	if transaction.HaveDuplicateInputs(t.Inputs) {
		return errors.New("invalid transaction's inputs")
	}
	if block == nil {
		if ok := bc.memPool.Verify(t); !ok {
			return errors.New("invalid transaction due to conflicts with the memory pool")
		}
	}
	if bc.dao.IsDoubleSpend(t) {
		return errors.New("invalid transaction caused by double spending")
	}
	if err := bc.verifyOutputs(t); err != nil {
		return errors.Wrap(err, "wrong outputs")
	}
	refs, err := bc.References(t)
	if err != nil {
		return err
	}
	results := refsAndOutsToResults(refs, t.Outputs)
	if err := bc.verifyResults(t, results); err != nil {
		return err
	}

	for _, a := range t.Attributes {
		if a.Usage == transaction.ECDH02 || a.Usage == transaction.ECDH03 {
			return errors.Errorf("invalid attribute's usage = %s ", a.Usage)
		}
	}

	switch t.Type {
	case transaction.ClaimType:
		claim := t.Data.(*transaction.ClaimTX)
		if transaction.HaveDuplicateInputs(claim.Claims) {
			return errors.New("duplicate claims")
		}
		if bc.dao.IsDoubleClaim(claim) {
			return errors.New("double claim")
		}
		if err := bc.verifyClaims(t, results); err != nil {
			return err
		}
	case transaction.InvocationType:
		inv := t.Data.(*transaction.InvocationTX)
		if inv.Gas.FractionalValue() != 0 {
			return errors.New("invocation gas can only be integer")
		}
	case transaction.StateType:
		stx := t.Data.(*transaction.StateTX)
		for _, desc := range stx.Descriptors {
			switch desc.Type {
			case transaction.Account:
				if desc.Field != "Votes" {
					return errors.New("bad field in account descriptor")
				}
				votes := keys.PublicKeys{}
				err := votes.DecodeBytes(desc.Value)
				if err != nil {
					return err
				}
				if len(votes) > state.MaxValidatorsVoted {
					return errors.New("voting candidate limit exceeded")
				}
				hash, err := util.Uint160DecodeBytesBE(desc.Key)
				if err != nil {
					return err
				}
				account, err := bc.dao.GetAccountStateOrNew(hash)
				if err != nil {
					return err
				}
				if account.IsFrozen {
					return errors.New("account is frozen")
				}
				if votes.Len() > 0 {
					balance := account.GetBalanceValues()[GoverningTokenID()]
					if balance == 0 {
						return errors.New("no governing tokens available to vote")
					}
					validators, err := bc.GetEnrollments()
					if err != nil {
						return err
					}
					for _, k := range votes {
						var isRegistered bool
						for i := range validators {
							if k.Equal(validators[i].PublicKey) {
								isRegistered = true
								break
							}
						}
						if !isRegistered {
							return errors.New("vote for unregistered validator")
						}
					}
				}
			case transaction.Validator:
				if desc.Field != "Registered" {
					return errors.New("bad field in validator descriptor")
				}
			default:
				return errors.New("bad descriptor type")
			}
		}
	}

	return bc.verifyTxWitnesses(t, block)
}

func (bc *Blockchain) verifyClaims(tx *transaction.Transaction, results []*transaction.Result) (err error) {
	t := tx.Data.(*transaction.ClaimTX)
	var result *transaction.Result
	for i := range results {
		if results[i].AssetID == UtilityTokenID() {
			result = results[i]
			break
		}
	}

	if result == nil || result.Amount.GreaterThan(0) {
		return errors.New("invalid output in claim tx")
	}

	bonus, err := bc.calculateBonus(t.Claims)
	if err == nil && bonus != -result.Amount {
		return fmt.Errorf("wrong bonus calculated in claim tx: %s != %s",
			bonus.String(), (-result.Amount).String())
	}

	return err
}

func (bc *Blockchain) calculateBonus(claims []transaction.Input) (util.Fixed8, error) {
	unclaimed := []*spentCoin{}
	inputs := transaction.GroupInputsByPrevHash(claims)

	for _, group := range inputs {
		h := group[0].PrevHash
		unspent, err := bc.dao.GetUnspentCoinState(h)
		if err != nil {
			return 0, err
		}

		for _, c := range group {
			if len(unspent.States) <= int(c.PrevIndex) {
				return 0, fmt.Errorf("can't find spent coins for %s (%d)", c.PrevHash.StringLE(), c.PrevIndex)
			}
			if unspent.States[c.PrevIndex].State&state.CoinSpent == 0 {
				return 0, fmt.Errorf("not spent yet: %s/%d", c.PrevHash.StringLE(), c.PrevIndex)
			}
			if unspent.States[c.PrevIndex].State&state.CoinClaimed != 0 {
				return 0, fmt.Errorf("already claimed: %s/%d", c.PrevHash.StringLE(), c.PrevIndex)
			}
			unclaimed = append(unclaimed, &spentCoin{
				Output:      &unspent.States[c.PrevIndex].Output,
				StartHeight: unspent.Height,
				EndHeight:   unspent.States[c.PrevIndex].SpendHeight,
			})
		}
	}

	return bc.calculateBonusInternal(unclaimed)
}

func (bc *Blockchain) calculateBonusInternal(scs []*spentCoin) (util.Fixed8, error) {
	var claimed util.Fixed8
	for _, sc := range scs {
		gen, sys, err := bc.CalculateClaimable(sc.Output.Amount, sc.StartHeight, sc.EndHeight)
		if err != nil {
			return 0, err
		}
		claimed += gen + sys
	}

	return claimed, nil
}

// isTxStillRelevant is a callback for mempool transaction filtering after the
// new block addition. It returns false for transactions already present in the
// chain (added by the new block), transactions using some inputs that are
// already used (double spends) and does witness reverification for non-standard
// contracts. It operates under the assumption that full transaction verification
// was already done so we don't need to check basic things like size, input/output
// correctness, etc.
func (bc *Blockchain) isTxStillRelevant(t *transaction.Transaction) bool {
	var recheckWitness bool

	if bc.dao.HasTransaction(t.Hash()) {
		return false
	}
	if bc.dao.IsDoubleSpend(t) {
		return false
	}
	if t.Type == transaction.ClaimType {
		claim := t.Data.(*transaction.ClaimTX)
		if bc.dao.IsDoubleClaim(claim) {
			return false
		}
	}
	for i := range t.Scripts {
		if !vm.IsStandardContract(t.Scripts[i].VerificationScript) {
			recheckWitness = true
			break
		}
	}
	if recheckWitness {
		return bc.verifyTxWitnesses(t, nil) == nil
	}
	return true

}

// VerifyTx verifies whether a transaction is bonafide or not. Block parameter
// is used for easy interop access and can be omitted for transactions that are
// not yet added into any block.
// Golang implementation of Verify method in C# (https://github.com/neo-project/neo/blob/master/neo/Network/P2P/Payloads/Transaction.cs#L270).
func (bc *Blockchain) VerifyTx(t *transaction.Transaction, block *block.Block) error {
	bc.lock.RLock()
	defer bc.lock.RUnlock()
	return bc.verifyTx(t, block)
}

// PoolTx verifies and tries to add given transaction into the mempool.
func (bc *Blockchain) PoolTx(t *transaction.Transaction) error {
	bc.lock.RLock()
	defer bc.lock.RUnlock()

	if bc.HasTransaction(t.Hash()) {
		return ErrAlreadyExists
	}
	if err := bc.verifyTx(t, nil); err != nil {
		return err
	}
	// Policying.
	if t.Type != transaction.ClaimType {
		txSize := io.GetVarSize(t)
		maxFree := bc.config.MaxFreeTransactionSize
		if maxFree != 0 && txSize > maxFree {
			netFee := bc.NetworkFee(t)
			if bc.IsLowPriority(netFee) ||
				netFee < util.Fixed8FromFloat(bc.config.FeePerExtraByte)*util.Fixed8(txSize-maxFree) {
				return ErrPolicy
			}
		}
	}
	if err := bc.memPool.Add(t, bc); err != nil {
		switch err {
		case mempool.ErrOOM:
			return ErrOOM
		case mempool.ErrConflict:
			return ErrAlreadyExists
		default:
			return err
		}
	}
	return nil
}

func (bc *Blockchain) verifyOutputs(t *transaction.Transaction) error {
	for assetID, outputs := range t.GroupOutputByAssetID() {
		assetState := bc.GetAssetState(assetID)
		if assetState == nil {
			return fmt.Errorf("no asset state for %s", assetID.StringLE())
		}

		if assetState.Expiration < bc.blockHeight+1 && assetState.AssetType != transaction.GoverningToken && assetState.AssetType != transaction.UtilityToken {
			return fmt.Errorf("asset %s expired", assetID.StringLE())
		}

		for _, out := range outputs {
			if int64(out.Amount)%int64(math.Pow10(8-int(assetState.Precision))) != 0 {
				return fmt.Errorf("output is not compliant with %s asset precision", assetID.StringLE())
			}
		}
	}

	return nil
}

func (bc *Blockchain) verifyResults(t *transaction.Transaction, results []*transaction.Result) error {
	var resultsDestroy []*transaction.Result
	var resultsIssue []*transaction.Result
	for _, re := range results {
		if re.Amount.GreaterThan(util.Fixed8(0)) {
			resultsDestroy = append(resultsDestroy, re)
		}

		if re.Amount.LessThan(util.Fixed8(0)) {
			resultsIssue = append(resultsIssue, re)
		}
	}
	if len(resultsDestroy) > 1 {
		return errors.New("tx has more than 1 destroy output")
	}
	if len(resultsDestroy) == 1 && resultsDestroy[0].AssetID != UtilityTokenID() {
		return errors.New("tx destroys non-utility token")
	}
	sysfee := bc.SystemFee(t)
	if sysfee.GreaterThan(util.Fixed8(0)) {
		if len(resultsDestroy) == 0 {
			return fmt.Errorf("system requires to pay %s fee, but tx pays nothing", sysfee.String())
		}
		if resultsDestroy[0].Amount.LessThan(sysfee) {
			return fmt.Errorf("system requires to pay %s fee, but tx pays %s only", sysfee.String(), resultsDestroy[0].Amount.String())
		}
	}

	switch t.Type {
	case transaction.MinerType, transaction.ClaimType:
		for _, r := range resultsIssue {
			if r.AssetID != UtilityTokenID() {
				return errors.New("miner or claim tx issues non-utility tokens")
			}
		}
		break
	case transaction.IssueType:
		for _, r := range resultsIssue {
			if r.AssetID == UtilityTokenID() {
				return errors.New("issue tx issues utility tokens")
			}
			asset, err := bc.dao.GetAssetState(r.AssetID)
			if asset == nil || err != nil {
				return errors.New("invalid asset in issue tx")
			}
			if asset.Available < r.Amount {
				return errors.New("trying to issue more than available")
			}
		}
		break
	default:
		if len(resultsIssue) > 0 {
			return errors.New("non issue/miner/claim tx issues tokens")
		}
		break
	}

	return nil
}

// GetTransactionResults returns the transaction results aggregate by assetID.
// Golang of GetTransationResults method in C# (https://github.com/neo-project/neo/blob/master/neo/Network/P2P/Payloads/Transaction.cs#L207)
func (bc *Blockchain) GetTransactionResults(t *transaction.Transaction) []*transaction.Result {
	references, err := bc.References(t)
	if err != nil {
		return nil
	}
	return refsAndOutsToResults(references, t.Outputs)
}

// mapReferencesToResults returns cumulative results of transaction based in its
// references and outputs.
func refsAndOutsToResults(references []transaction.InOut, outputs []transaction.Output) []*transaction.Result {
	var results []*transaction.Result
	tempResult := make(map[util.Uint256]util.Fixed8)

	for _, inout := range references {
		c := tempResult[inout.Out.AssetID]
		tempResult[inout.Out.AssetID] = c.Add(inout.Out.Amount)
	}
	for _, output := range outputs {
		c := tempResult[output.AssetID]
		tempResult[output.AssetID] = c.Sub(output.Amount)
	}

	results = []*transaction.Result{} // this assignment is necessary. (Most of the time amount == 0 and results is the empty slice.)
	for assetID, amount := range tempResult {
		if amount != util.Fixed8(0) {
			results = append(results, &transaction.Result{
				AssetID: assetID,
				Amount:  amount,
			})
		}
	}

	return results
}

//GetStandByValidators returns validators from the configuration.
func (bc *Blockchain) GetStandByValidators() (keys.PublicKeys, error) {
	return getValidators(bc.config)
}

// GetValidators returns validators.
// Golang implementation of GetValidators method in C# (https://github.com/neo-project/neo/blob/c64748ecbac3baeb8045b16af0d518398a6ced24/neo/Persistence/Snapshot.cs#L182)
func (bc *Blockchain) GetValidators(txes ...*transaction.Transaction) ([]*keys.PublicKey, error) {
	cache := dao.NewCached(bc.dao)
	if len(txes) > 0 {
		for _, tx := range txes {
			// iterate through outputs
			for index, output := range tx.Outputs {
				accountState, err := cache.GetAccountStateOrNew(output.ScriptHash)
				if err != nil {
					return nil, err
				}
				accountState.Balances[output.AssetID] = append(accountState.Balances[output.AssetID], state.UnspentBalance{
					Tx:    tx.Hash(),
					Index: uint16(index),
					Value: output.Amount,
				})
				if err := cache.PutAccountState(accountState); err != nil {
					return nil, err
				}
				if err = processTXWithValidatorsAdd(&output, accountState, cache); err != nil {
					return nil, err
				}
			}

			// group inputs by the same previous hash and iterate through inputs
			group := make(map[util.Uint256][]*transaction.Input)
			for i := range tx.Inputs {
				hash := tx.Inputs[i].PrevHash
				group[hash] = append(group[hash], &tx.Inputs[i])
			}

			for hash, inputs := range group {
				unspent, err := cache.GetUnspentCoinState(hash)
				if err != nil {
					return nil, err
				}
				// process inputs
				for _, input := range inputs {
					prevOutput := &unspent.States[input.PrevIndex].Output
					accountState, err := cache.GetAccountStateOrNew(prevOutput.ScriptHash)
					if err != nil {
						return nil, err
					}

					// process account state votes: if there are any -> validators will be updated.
					if err = processTXWithValidatorsSubtract(prevOutput, accountState, cache); err != nil {
						return nil, err
					}
					delete(accountState.Balances, prevOutput.AssetID)
					if err = cache.PutAccountState(accountState); err != nil {
						return nil, err
					}
				}
			}

			switch t := tx.Data.(type) {
			case *transaction.EnrollmentTX:
				if err := processEnrollmentTX(cache, t); err != nil {
					return nil, err
				}
			case *transaction.StateTX:
				if err := bc.processStateTX(cache, tx, t); err != nil {
					return nil, err
				}
			}
		}
	}

	return bc.contracts.NEO.GetValidatorsInternal(bc, cache)
}

// GetEnrollments returns all registered validators and non-registered SB validators
func (bc *Blockchain) GetEnrollments() ([]*state.Validator, error) {
	validators := bc.dao.GetValidators()
	standByValidators, err := bc.GetStandByValidators()
	if err != nil {
		return nil, err
	}
	uniqueSBValidators := standByValidators.Unique()

	var result []*state.Validator
	for _, validator := range validators {
		if validator.Registered {
			result = append(result, validator)
		}
	}
	for _, sBValidator := range uniqueSBValidators {
		isAdded := false
		for _, v := range result {
			if v.PublicKey == sBValidator {
				isAdded = true
				break
			}
		}
		if !isAdded {
			result = append(result, &state.Validator{
				PublicKey:  sBValidator,
				Registered: false,
				Votes:      0,
			})
		}
	}
	return result, nil
}

func (bc *Blockchain) processStateTX(dao *dao.Cached, t *transaction.Transaction, tx *transaction.StateTX) error {
	for _, desc := range tx.Descriptors {
		switch desc.Type {
		case transaction.Account:
			if err := bc.processAccountStateDescriptor(desc, t, dao); err != nil {
				return err
			}
		case transaction.Validator:
			if err := processValidatorStateDescriptor(desc, dao); err != nil {
				return err
			}
		}
	}
	return nil
}

func processEnrollmentTX(dao *dao.Cached, tx *transaction.EnrollmentTX) error {
	validatorState, err := dao.GetValidatorStateOrNew(&tx.PublicKey)
	if err != nil {
		return err
	}
	validatorState.Registered = true
	return dao.PutValidatorState(validatorState)
}

// GetScriptHashesForVerifying returns all the ScriptHashes of a transaction which will be use
// to verify whether the transaction is bonafide or not.
// Golang implementation of GetScriptHashesForVerifying method in C# (https://github.com/neo-project/neo/blob/master/neo/Network/P2P/Payloads/Transaction.cs#L190)
func (bc *Blockchain) GetScriptHashesForVerifying(t *transaction.Transaction) ([]util.Uint160, error) {
	references, err := bc.References(t)
	if err != nil {
		return nil, err
	}
	hashes := make(map[util.Uint160]bool)
	for i := range references {
		hashes[references[i].Out.ScriptHash] = true
	}
	for _, a := range t.Attributes {
		if a.Usage == transaction.Script {
			h, err := util.Uint160DecodeBytesBE(a.Data)
			if err != nil {
				return nil, err
			}
			if _, ok := hashes[h]; !ok {
				hashes[h] = true
			}
		}
	}

	for a, outputs := range t.GroupOutputByAssetID() {
		as := bc.GetAssetState(a)
		if as == nil {
			return nil, errors.New("Invalid operation")
		}
		if as.AssetType&transaction.DutyFlag != 0 {
			for _, o := range outputs {
				h := o.ScriptHash
				if _, ok := hashes[h]; !ok {
					hashes[h] = true
				}
			}
		}
	}
	hashes[t.Sender] = true
	switch t.Type {
	case transaction.ClaimType:
		claim := t.Data.(*transaction.ClaimTX)
		refs, err := bc.references(claim.Claims)
		if err != nil {
			return nil, err
		}
		for i := range refs {
			hashes[refs[i].Out.ScriptHash] = true
		}
	case transaction.EnrollmentType:
		etx := t.Data.(*transaction.EnrollmentTX)
		hashes[etx.PublicKey.GetScriptHash()] = true
	case transaction.IssueType:
		for _, res := range refsAndOutsToResults(references, t.Outputs) {
			if res.Amount < 0 {
				asset, err := bc.dao.GetAssetState(res.AssetID)
				if asset == nil || err != nil {
					return nil, errors.New("invalid asset in issue tx")
				}
				hashes[asset.Issuer] = true
			}
		}
	case transaction.RegisterType:
		reg := t.Data.(*transaction.RegisterTX)
		hashes[reg.Owner.GetScriptHash()] = true
	case transaction.StateType:
		stx := t.Data.(*transaction.StateTX)
		for _, desc := range stx.Descriptors {
			switch desc.Type {
			case transaction.Account:
				if desc.Field != "Votes" {
					return nil, errors.New("bad account state descriptor")
				}
				hash, err := util.Uint160DecodeBytesBE(desc.Key)
				if err != nil {
					return nil, err
				}
				hashes[hash] = true
			case transaction.Validator:
				if desc.Field != "Registered" {
					return nil, errors.New("bad validator state descriptor")
				}
				key := &keys.PublicKey{}
				err := key.DecodeBytes(desc.Key)
				if err != nil {
					return nil, err
				}
				hashes[key.GetScriptHash()] = true
			}
		}
	}
	// convert hashes to []util.Uint160
	hashesResult := make([]util.Uint160, 0, len(hashes))
	for h := range hashes {
		hashesResult = append(hashesResult, h)
	}

	return hashesResult, nil

}

// GetTestVM returns a VM and a Store setup for a test run of some sort of code.
func (bc *Blockchain) GetTestVM() *vm.VM {
	systemInterop := bc.newInteropContext(trigger.Application, bc.dao, nil, nil)
	vm := SpawnVM(systemInterop)
	vm.SetPriceGetter(getPrice)
	return vm
}

// ScriptFromWitness returns verification script for provided witness.
// If hash is not equal to the witness script hash, error is returned.
func ScriptFromWitness(hash util.Uint160, witness *transaction.Witness) ([]byte, error) {
	verification := witness.VerificationScript

	if len(verification) == 0 {
		bb := io.NewBufBinWriter()
		emit.AppCall(bb.BinWriter, hash, false)
		verification = bb.Bytes()
	} else if h := witness.ScriptHash(); hash != h {
		return nil, errors.New("witness hash mismatch")
	}

	return verification, nil
}

// verifyHashAgainstScript verifies given hash against the given witness.
func (bc *Blockchain) verifyHashAgainstScript(hash util.Uint160, witness *transaction.Witness, interopCtx *interop.Context, useKeys bool) error {
	verification, err := ScriptFromWitness(hash, witness)
	if err != nil {
		return err
	}

	vm := SpawnVM(interopCtx)
	vm.LoadScript(verification)
	vm.LoadScript(witness.InvocationScript)
	if useKeys {
		bc.keyCacheLock.RLock()
		if bc.keyCache[hash] != nil {
			vm.SetPublicKeys(bc.keyCache[hash])
		}
		bc.keyCacheLock.RUnlock()
	}
	err = vm.Run()
	if vm.HasFailed() {
		return errors.Errorf("vm failed to execute the script with error: %s", err)
	}
	resEl := vm.Estack().Pop()
	if resEl != nil {
		res, err := resEl.TryBool()
		if err != nil {
			return err
		}
		if !res {
			return errors.Errorf("signature check failed")
		}
		if useKeys {
			bc.keyCacheLock.RLock()
			_, ok := bc.keyCache[hash]
			bc.keyCacheLock.RUnlock()
			if !ok {
				bc.keyCacheLock.Lock()
				bc.keyCache[hash] = vm.GetPublicKeys()
				bc.keyCacheLock.Unlock()
			}
		}
	} else {
		return errors.Errorf("no result returned from the script")
	}
	return nil
}

// verifyTxWitnesses verifies the scripts (witnesses) that come with a given
// transaction. It can reorder them by ScriptHash, because that's required to
// match a slice of script hashes from the Blockchain. Block parameter
// is used for easy interop access and can be omitted for transactions that are
// not yet added into any block.
// Golang implementation of VerifyWitnesses method in C# (https://github.com/neo-project/neo/blob/master/neo/SmartContract/Helper.cs#L87).
// Unfortunately the IVerifiable interface could not be implemented because we can't move the References method in blockchain.go to the transaction.go file.
func (bc *Blockchain) verifyTxWitnesses(t *transaction.Transaction, block *block.Block) error {
	hashes, err := bc.GetScriptHashesForVerifying(t)
	if err != nil {
		return err
	}

	witnesses := t.Scripts
	if len(hashes) != len(witnesses) {
		return errors.Errorf("expected len(hashes) == len(witnesses). got: %d != %d", len(hashes), len(witnesses))
	}
	sort.Slice(hashes, func(i, j int) bool { return hashes[i].Less(hashes[j]) })
	sort.Slice(witnesses, func(i, j int) bool { return witnesses[i].ScriptHash().Less(witnesses[j].ScriptHash()) })
	interopCtx := bc.newInteropContext(trigger.Verification, bc.dao, block, t)
	for i := 0; i < len(hashes); i++ {
		err := bc.verifyHashAgainstScript(hashes[i], &witnesses[i], interopCtx, false)
		if err != nil {
			numStr := fmt.Sprintf("witness #%d", i)
			return errors.Wrap(err, numStr)
		}
	}

	return nil
}

// verifyHeaderWitnesses is a block-specific implementation of VerifyWitnesses logic.
func (bc *Blockchain) verifyHeaderWitnesses(currHeader, prevHeader *block.Header) error {
	var hash util.Uint160
	if prevHeader == nil && currHeader.PrevHash.Equals(util.Uint256{}) {
		hash = currHeader.Script.ScriptHash()
	} else {
		hash = prevHeader.NextConsensus
	}
	interopCtx := bc.newInteropContext(trigger.Verification, bc.dao, nil, nil)
	interopCtx.Container = currHeader
	return bc.verifyHashAgainstScript(hash, &currHeader.Script, interopCtx, true)
}

func hashAndIndexToBytes(h util.Uint256, index uint32) []byte {
	buf := io.NewBufBinWriter()
	buf.WriteBytes(h.BytesLE())
	buf.WriteU32LE(index)
	return buf.Bytes()
}

func (bc *Blockchain) secondsPerBlock() int {
	return bc.config.SecondsPerBlock
}

func (bc *Blockchain) newInteropContext(trigger trigger.Type, d dao.DAO, block *block.Block, tx *transaction.Transaction) *interop.Context {
	ic := interop.NewContext(trigger, bc, d, bc.contracts.Contracts, block, tx, bc.log)
	switch {
	case tx != nil:
		ic.Container = tx
	case block != nil:
		ic.Container = block
	}
	return ic
}
