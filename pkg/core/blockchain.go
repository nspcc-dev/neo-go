package core

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"sort"
	"sync/atomic"
	"time"

	"github.com/CityOfZion/neo-go/config"
	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/smartcontract"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/CityOfZion/neo-go/pkg/vm"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// tuning parameters
const (
	headerBatchCount = 2000
	version          = "0.0.1"

	// This one comes from C# code and it's different from the constant used
	// when creating an asset with Neo.Asset.Create interop call. It looks
	// like 2000000 is coming from the decrementInterval, but C# code doesn't
	// contain any relationship between the two, so we should follow this
	// behavior.
	registeredAssetLifetime = 2 * 2000000
)

var (
	genAmount         = []int{8, 7, 6, 5, 4, 3, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	decrementInterval = 2000000
	persistInterval   = 1 * time.Second
)

// Blockchain represents the blockchain.
type Blockchain struct {
	config config.ProtocolConfiguration

	// Any object that satisfies the BlockchainStorer interface.
	storage.Store

	// In-memory storage to be persisted into the storage.Store
	memStore *storage.MemoryStore

	// Current index/height of the highest block.
	// Read access should always be called by BlockHeight().
	// Write access should only happen in storeBlock().
	blockHeight uint32

	// Current persisted block count.
	persistedHeight uint32

	// Number of headers stored in the chain file.
	storedHeaderCount uint32

	// All operation on headerList must be called from an
	// headersOp to be routine safe.
	headerList *HeaderHashList

	// Only for operating on the headerList.
	headersOp     chan headersOpFunc
	headersOpDone chan struct{}

	memPool MemPool
}

type headersOpFunc func(headerList *HeaderHashList)

// NewBlockchain return a new blockchain object the will use the
// given Store as its underlying storage.
func NewBlockchain(s storage.Store, cfg config.ProtocolConfiguration) (*Blockchain, error) {
	bc := &Blockchain{
		config:        cfg,
		Store:         s,
		memStore:      storage.NewMemoryStore(),
		headersOp:     make(chan headersOpFunc),
		headersOpDone: make(chan struct{}),
		memPool:       NewMemPool(50000),
	}

	if err := bc.init(); err != nil {
		return nil, err
	}

	return bc, nil
}

func (bc *Blockchain) init() error {
	// If we could not find the version in the Store, we know that there is nothing stored.
	ver, err := storage.Version(bc.Store)
	if err != nil {
		log.Infof("no storage version found! creating genesis block")
		if err = storage.PutVersion(bc.Store, version); err != nil {
			return err
		}
		genesisBlock, err := createGenesisBlock(bc.config)
		if err != nil {
			return err
		}
		bc.headerList = NewHeaderHashList(genesisBlock.Hash())
		return bc.storeBlock(genesisBlock)
	}
	if ver != version {
		return fmt.Errorf("storage version mismatch betweeen %s and %s", version, ver)
	}

	// At this point there was no version found in the storage which
	// implies a creating fresh storage with the version specified
	// and the genesis block as first block.
	log.Infof("restoring blockchain with version: %s", version)

	bHeight, err := storage.CurrentBlockHeight(bc.Store)
	if err != nil {
		return err
	}
	bc.blockHeight = bHeight
	bc.persistedHeight = bHeight

	hashes, err := storage.HeaderHashes(bc.Store)
	if err != nil {
		return err
	}

	bc.headerList = NewHeaderHashList(hashes...)
	bc.storedHeaderCount = uint32(len(hashes))

	currHeaderHeight, currHeaderHash, err := storage.CurrentHeaderHeight(bc.Store)
	if err != nil {
		return err
	}

	// There is a high chance that the Node is stopped before the next
	// batch of 2000 headers was stored. Via the currentHeaders stored we can sync
	// that with stored blocks.
	if currHeaderHeight > bc.storedHeaderCount {
		hash := currHeaderHash
		targetHash := bc.headerList.Get(bc.headerList.Len() - 1)
		headers := make([]*Header, 0)

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
func (bc *Blockchain) Run(ctx context.Context) {
	persistTimer := time.NewTimer(persistInterval)
	defer func() {
		persistTimer.Stop()
		if err := bc.persist(ctx); err != nil {
			log.Warnf("failed to persist: %s", err)
		}
		// never fails
		_ = bc.memStore.Close()
		if err := bc.Store.Close(); err != nil {
			log.Warnf("failed to close db: %s", err)
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case op := <-bc.headersOp:
			op(bc.headerList)
			bc.headersOpDone <- struct{}{}
		case <-persistTimer.C:
			go func() {
				err := bc.persist(ctx)
				if err != nil {
					log.Warnf("failed to persist blockchain: %s", err)
				}
			}()
			persistTimer.Reset(persistInterval)
		}
	}
}

// AddBlock accepts successive block for the Blockchain, verifies it and
// stores internally. Eventually it will be persisted to the backing storage.
func (bc *Blockchain) AddBlock(block *Block) error {
	expectedHeight := bc.BlockHeight() + 1
	if expectedHeight != block.Index {
		return fmt.Errorf("expected block %d, but passed block %d", expectedHeight, block.Index)
	}
	if bc.config.VerifyBlocks {
		err := block.Verify()
		if err == nil {
			err = bc.VerifyBlock(block)
		}
		if err != nil {
			return fmt.Errorf("block %s is invalid: %s", block.Hash().ReverseString(), err)
		}
		if bc.config.VerifyTransactions {
			for _, tx := range block.Transactions {
				err := bc.VerifyTx(tx, block)
				if err != nil {
					return fmt.Errorf("transaction %s failed to verify: %s", tx.Hash().ReverseString(), err)
				}
			}
		}
	}
	headerLen := bc.headerListLen()
	if int(block.Index) == headerLen {
		err := bc.AddHeaders(block.Header())
		if err != nil {
			return err
		}
	}
	return bc.storeBlock(block)
}

// AddHeaders will process the given headers and add them to the
// HeaderHashList.
func (bc *Blockchain) AddHeaders(headers ...*Header) (err error) {
	var (
		start = time.Now()
		batch = bc.memStore.Batch()
	)

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
			if err = bc.memStore.PutBatch(batch); err != nil {
				return
			}
			log.WithFields(log.Fields{
				"headerIndex": headerList.Len() - 1,
				"blockHeight": bc.BlockHeight(),
				"took":        time.Since(start),
			}).Debug("done processing headers")
		}
	}
	<-bc.headersOpDone
	return err
}

// processHeader processes the given header. Note that this is only thread safe
// if executed in headers operation.
func (bc *Blockchain) processHeader(h *Header, batch storage.Batch, headerList *HeaderHashList) error {
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
	h.EncodeBinary(buf.BinWriter)
	if buf.Err != nil {
		return buf.Err
	}

	key := storage.AppendPrefix(storage.DataBlock, h.Hash().BytesReverse())
	batch.Put(key, buf.Bytes())
	batch.Put(storage.SYSCurrentHeader.Bytes(), hashAndIndexToBytes(h.Hash(), h.Index))

	return nil
}

// TODO: storeBlock needs some more love, its implemented as in the original
// project. This for the sake of development speed and understanding of what
// is happening here, quite allot as you can see :). If things are wired together
// and all tests are in place, we can make a more optimized and cleaner implementation.
func (bc *Blockchain) storeBlock(block *Block) error {
	var (
		batch        = bc.memStore.Batch()
		unspentCoins = make(UnspentCoins)
		spentCoins   = make(SpentCoins)
		accounts     = make(Accounts)
		assets       = make(Assets)
		contracts    = make(Contracts)
	)

	if err := storeAsBlock(batch, block, 0); err != nil {
		return err
	}

	storeAsCurrentBlock(batch, block)

	for _, tx := range block.Transactions {
		if err := storeAsTransaction(batch, tx, block.Index); err != nil {
			return err
		}

		unspentCoins[tx.Hash()] = NewUnspentCoinState(len(tx.Outputs))

		// Process TX outputs.
		for _, output := range tx.Outputs {
			account, err := accounts.getAndUpdate(bc.memStore, bc.Store, output.ScriptHash)
			if err != nil {
				return err
			}
			if _, ok := account.Balances[output.AssetID]; ok {
				account.Balances[output.AssetID] += output.Amount
			} else {
				account.Balances[output.AssetID] = output.Amount
			}
		}

		// Process TX inputs that are grouped by previous hash.
		for prevHash, inputs := range tx.GroupInputsByPrevHash() {
			prevTX, prevTXHeight, err := bc.GetTransaction(prevHash)
			if err != nil {
				return fmt.Errorf("could not find previous TX: %s", prevHash)
			}
			for _, input := range inputs {
				unspent, err := unspentCoins.getAndUpdate(bc.memStore, bc.Store, input.PrevHash)
				if err != nil {
					return err
				}
				unspent.states[input.PrevIndex] = CoinStateSpent

				prevTXOutput := prevTX.Outputs[input.PrevIndex]
				account, err := accounts.getAndUpdate(bc.memStore, bc.Store, prevTXOutput.ScriptHash)
				if err != nil {
					return err
				}

				if prevTXOutput.AssetID.Equals(governingTokenTX().Hash()) {
					spentCoin := NewSpentCoinState(input.PrevHash, prevTXHeight)
					spentCoin.items[input.PrevIndex] = block.Index
					spentCoins[input.PrevHash] = spentCoin
				}

				account.Balances[prevTXOutput.AssetID] -= prevTXOutput.Amount
			}
		}

		// Process the underlying type of the TX.
		switch t := tx.Data.(type) {
		case *transaction.RegisterTX:
			assets[tx.Hash()] = &AssetState{
				ID:         tx.Hash(),
				AssetType:  t.AssetType,
				Name:       t.Name,
				Amount:     t.Amount,
				Precision:  t.Precision,
				Owner:      t.Owner,
				Admin:      t.Admin,
				Expiration: bc.BlockHeight() + registeredAssetLifetime,
			}
		case *transaction.IssueTX:
		case *transaction.ClaimTX:
		case *transaction.EnrollmentTX:
		case *transaction.StateTX:
		case *transaction.PublishTX:
			var properties smartcontract.PropertyState
			if t.NeedStorage {
				properties |= smartcontract.HasStorage
			}
			contract := &ContractState{
				Script:      t.Script,
				ParamList:   t.ParamList,
				ReturnType:  t.ReturnType,
				Properties:  properties,
				Name:        t.Name,
				CodeVersion: t.CodeVersion,
				Author:      t.Author,
				Email:       t.Email,
				Description: t.Description,
			}
			contracts[contract.ScriptHash()] = contract

		case *transaction.InvocationTX:
			vm := vm.New(vm.ModeMute)
			vm.SetCheckedHash(tx.VerificationHash().Bytes())
			vm.SetScriptGetter(func(hash util.Uint160) []byte {
				cs := bc.GetContractState(hash)
				if cs == nil {
					return nil
				}

				return cs.Script
			})
			systemInterop := newInteropContext(0x10, bc, block, tx)
			vm.RegisterInteropFuncs(systemInterop.getSystemInteropMap())
			vm.RegisterInteropFuncs(systemInterop.getNeoInteropMap())
			vm.LoadScript(t.Script)
			vm.Run()
			if !vm.HasFailed() {
				_, err := systemInterop.mem.Persist(bc.memStore)
				if err != nil {
					return errors.Wrap(err, "failed to persist invocation results")
				}
			} else {
				log.WithFields(log.Fields{
					"tx":    tx.Hash().ReverseString(),
					"block": block.Index,
				}).Warn("contract invocation failed")
			}
		}
	}

	// Persist all to storage.
	if err := accounts.commit(batch); err != nil {
		return err
	}
	if err := unspentCoins.commit(batch); err != nil {
		return err
	}
	if err := spentCoins.commit(batch); err != nil {
		return err
	}
	if err := assets.commit(batch); err != nil {
		return err
	}
	if err := contracts.commit(batch); err != nil {
		return err
	}
	if err := bc.memStore.PutBatch(batch); err != nil {
		return err
	}

	atomic.StoreUint32(&bc.blockHeight, block.Index)
	return nil
}

// persist flushes current in-memory store contents to the persistent storage.
func (bc *Blockchain) persist(ctx context.Context) error {
	var (
		start     = time.Now()
		persisted = 0
		err       error
	)

	persisted, err = bc.memStore.Persist(bc.Store)
	if err != nil {
		return err
	}
	bHeight, err := storage.CurrentBlockHeight(bc.Store)
	if err != nil {
		return err
	}
	oldHeight := atomic.SwapUint32(&bc.persistedHeight, bHeight)
	diff := bHeight - oldHeight

	if persisted > 0 {
		log.WithFields(log.Fields{
			"persistedBlocks": diff,
			"persistedKeys":   persisted,
			"headerHeight":    bc.HeaderHeight(),
			"blockHeight":     bc.BlockHeight(),
			"persistedHeight": atomic.LoadUint32(&bc.persistedHeight),
			"took":            time.Since(start),
		}).Info("blockchain persist completed")
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
	if tx, ok := bc.memPool.TryGetValue(hash); ok {
		return tx, 0, nil // the height is not actually defined for memPool transaction. Not sure if zero is a good number in this case.
	}
	tx, height, err := getTransactionFromStore(bc.memStore, hash)
	if err != nil {
		tx, height, err = getTransactionFromStore(bc.Store, hash)
	}
	return tx, height, err
}

// getTransactionFromStore returns Transaction and its height by the given hash
// if it exists in the store.
func getTransactionFromStore(s storage.Store, hash util.Uint256) (*transaction.Transaction, uint32, error) {
	key := storage.AppendPrefix(storage.DataTransaction, hash.BytesReverse())
	b, err := s.Get(key)
	if err != nil {
		return nil, 0, err
	}
	r := io.NewBinReaderFromBuf(b)

	var height uint32
	r.ReadLE(&height)

	tx := &transaction.Transaction{}
	tx.DecodeBinary(r)
	if r.Err != nil {
		return nil, 0, r.Err
	}

	return tx, height, nil
}

// GetStorageItem returns an item from storage.
func (bc *Blockchain) GetStorageItem(scripthash util.Uint160, key []byte) *StorageItem {
	sItem := getStorageItemFromStore(bc.memStore, scripthash, key)
	if sItem == nil {
		sItem = getStorageItemFromStore(bc.Store, scripthash, key)
	}
	return sItem
}

// GetStorageItems returns all storage items for a given scripthash.
func (bc *Blockchain) GetStorageItems(hash util.Uint160) (map[string]*StorageItem, error) {
	var siMap = make(map[string]*StorageItem)
	var err error

	saveToMap := func(k, v []byte) {
		if err != nil {
			return
		}
		r := io.NewBinReaderFromBuf(v)
		si := &StorageItem{}
		si.DecodeBinary(r)
		if r.Err != nil {
			err = r.Err
			return
		}

		// Cut prefix and hash.
		siMap[string(k[21:])] = si
	}
	bc.memStore.Seek(storage.AppendPrefix(storage.STStorage, hash.BytesReverse()), saveToMap)
	bc.Store.Seek(storage.AppendPrefix(storage.STStorage, hash.BytesReverse()), saveToMap)
	if err != nil {
		return nil, err
	}
	return siMap, nil
}

// GetBlock returns a Block by the given hash.
func (bc *Blockchain) GetBlock(hash util.Uint256) (*Block, error) {
	block, err := getBlockFromStore(bc.memStore, hash)
	if err != nil {
		block, err = getBlockFromStore(bc.Store, hash)
		if err != nil {
			return nil, err
		}
	}
	if len(block.Transactions) == 0 {
		return nil, fmt.Errorf("only header is available")
	}
	for _, tx := range block.Transactions {
		stx, _, err := bc.GetTransaction(tx.Hash())
		if err != nil {
			return nil, err
		}
		*tx = *stx
	}
	return block, nil
}

// getBlockFromStore returns Block by the given hash if it exists in the store.
func getBlockFromStore(s storage.Store, hash util.Uint256) (*Block, error) {
	key := storage.AppendPrefix(storage.DataBlock, hash.BytesReverse())
	b, err := s.Get(key)
	if err != nil {
		return nil, err
	}
	block, err := NewBlockFromTrimmedBytes(b)
	if err != nil {
		return nil, err
	}
	return block, err
}

// GetHeader returns data block header identified with the given hash value.
func (bc *Blockchain) GetHeader(hash util.Uint256) (*Header, error) {
	header, err := getHeaderFromStore(bc.memStore, hash)
	if err != nil {
		header, err = getHeaderFromStore(bc.Store, hash)
		if err != nil {
			return nil, err
		}
	}
	return header, err
}

// getHeaderFromStore returns Header by the given hash from the store.
func getHeaderFromStore(s storage.Store, hash util.Uint256) (*Header, error) {
	block, err := getBlockFromStore(s, hash)
	if err != nil {
		return nil, err
	}
	return block.Header(), nil
}

// HasTransaction return true if the blockchain contains he given
// transaction hash.
func (bc *Blockchain) HasTransaction(hash util.Uint256) bool {
	return bc.memPool.ContainsKey(hash) ||
		checkTransactionInStore(bc.memStore, hash) ||
		checkTransactionInStore(bc.Store, hash)
}

// checkTransactionInStore returns true if the given store contains the given
// Transaction hash.
func checkTransactionInStore(s storage.Store, hash util.Uint256) bool {
	key := storage.AppendPrefix(storage.DataTransaction, hash.BytesReverse())
	if _, err := s.Get(key); err == nil {
		return true
	}
	return false
}

// HasBlock return true if the blockchain contains the given
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

// GetHeaderHash return the hash from the headerList by its
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

// GetAssetState returns asset state from its assetID
func (bc *Blockchain) GetAssetState(assetID util.Uint256) *AssetState {
	as := getAssetStateFromStore(bc.memStore, assetID)
	if as == nil {
		as = getAssetStateFromStore(bc.Store, assetID)
	}
	return as
}

// getAssetStateFromStore returns given asset state as recorded in the given
// store.
func getAssetStateFromStore(s storage.Store, assetID util.Uint256) *AssetState {
	key := storage.AppendPrefix(storage.STAsset, assetID.Bytes())
	asEncoded, err := s.Get(key)
	if err != nil {
		return nil
	}
	var a AssetState
	r := io.NewBinReaderFromBuf(asEncoded)
	a.DecodeBinary(r)
	if r.Err != nil || a.ID != assetID {
		return nil
	}

	return &a
}

// GetContractState returns contract by its script hash.
func (bc *Blockchain) GetContractState(hash util.Uint160) *ContractState {
	cs := getContractStateFromStore(bc.memStore, hash)
	if cs == nil {
		cs = getContractStateFromStore(bc.Store, hash)
	}
	return cs
}

// getContractStateFromStore returns contract state as recorded in the given
// store by the given script hash.
func getContractStateFromStore(s storage.Store, hash util.Uint160) *ContractState {
	key := storage.AppendPrefix(storage.STContract, hash.Bytes())
	contractBytes, err := s.Get(key)
	if err != nil {
		return nil
	}
	var c ContractState
	r := io.NewBinReaderFromBuf(contractBytes)
	c.DecodeBinary(r)
	if r.Err != nil || c.ScriptHash() != hash {
		return nil
	}

	return &c
}

// GetAccountState returns the account state from its script hash
func (bc *Blockchain) GetAccountState(scriptHash util.Uint160) *AccountState {
	as, err := getAccountStateFromStore(bc.memStore, scriptHash)
	if as == nil {
		if err != storage.ErrKeyNotFound {
			log.Warnf("failed to get account state: %s", err)
		}
		as, err = getAccountStateFromStore(bc.Store, scriptHash)
		if as == nil && err != storage.ErrKeyNotFound {
			log.Warnf("failed to get account state: %s", err)
		}
	}
	return as
}

// GetUnspentCoinState returns unspent coin state for given tx hash.
func (bc *Blockchain) GetUnspentCoinState(hash util.Uint256) *UnspentCoinState {
	ucs, err := getUnspentCoinStateFromStore(bc.memStore, hash)
	if err != nil {
		ucs, _ = getUnspentCoinStateFromStore(bc.Store, hash)
	}
	return ucs
}

// GetConfig returns the config stored in the blockchain
func (bc *Blockchain) GetConfig() config.ProtocolConfiguration {
	return bc.config
}

// References returns a map with input coin reference (prevhash and index) as key
// and transaction output as value from a transaction t.
// @TODO: unfortunately we couldn't attach this method to the Transaction struct in the
// transaction package because of a import cycle problem. Perhaps we should think to re-design
// the code base to avoid this situation.
func (bc *Blockchain) References(t *transaction.Transaction) map[transaction.Input]*transaction.Output {
	references := make(map[transaction.Input]*transaction.Output)

	for prevHash, inputs := range t.GroupInputsByPrevHash() {
		if tx, _, err := bc.GetTransaction(prevHash); err != nil {
			tx = nil
		} else if tx != nil {
			for _, in := range inputs {
				references[*in] = tx.Outputs[in.PrevIndex]
			}
		} else {
			references = nil
		}
	}
	return references
}

// FeePerByte returns network fee divided by the size of the transaction
func (bc *Blockchain) FeePerByte(t *transaction.Transaction) util.Fixed8 {
	return bc.NetworkFee(t).Div(int64(io.GetVarSize(t)))
}

// NetworkFee returns network fee
func (bc *Blockchain) NetworkFee(t *transaction.Transaction) util.Fixed8 {
	inputAmount := util.Fixed8FromInt64(0)
	for _, txOutput := range bc.References(t) {
		if txOutput.AssetID == utilityTokenTX().Hash() {
			inputAmount.Add(txOutput.Amount)
		}
	}

	outputAmount := util.Fixed8FromInt64(0)
	for _, txOutput := range t.Outputs {
		if txOutput.AssetID == utilityTokenTX().Hash() {
			outputAmount.Add(txOutput.Amount)
		}
	}

	return inputAmount.Sub(outputAmount).Sub(bc.SystemFee(t))
}

// SystemFee returns system fee
func (bc *Blockchain) SystemFee(t *transaction.Transaction) util.Fixed8 {
	return bc.GetConfig().SystemFee.TryGetValue(t.Type)
}

// IsLowPriority flags a trnsaction as low priority if the network fee is less than
// LowPriorityThreshold
func (bc *Blockchain) IsLowPriority(t *transaction.Transaction) bool {
	return bc.NetworkFee(t) < util.Fixed8FromFloat(bc.GetConfig().LowPriorityThreshold)
}

// GetMemPool returns the memory pool of the blockchain.
func (bc *Blockchain) GetMemPool() MemPool {
	return bc.memPool
}

// VerifyBlock verifies block against its current state.
func (bc *Blockchain) VerifyBlock(block *Block) error {
	prevHeader, err := bc.GetHeader(block.PrevHash)
	if err != nil {
		return errors.Wrap(err, "unable to get previous header")
	}
	if prevHeader.Index+1 != block.Index {
		return errors.New("previous header index doesn't match")
	}
	if prevHeader.Timestamp >= block.Timestamp {
		return errors.New("block is not newer than the previous one")
	}
	return bc.verifyBlockWitnesses(block, prevHeader)
}

// VerifyTx verifies whether a transaction is bonafide or not. Block parameter
// is used for easy interop access and can be omitted for transactions that are
// not yet added into any block.
// Golang implementation of Verify method in C# (https://github.com/neo-project/neo/blob/master/neo/Network/P2P/Payloads/Transaction.cs#L270).
func (bc *Blockchain) VerifyTx(t *transaction.Transaction, block *Block) error {
	if io.GetVarSize(t) > transaction.MaxTransactionSize {
		return errors.Errorf("invalid transaction size = %d. It shoud be less then MaxTransactionSize = %d", io.GetVarSize(t), transaction.MaxTransactionSize)
	}
	if ok := bc.verifyInputs(t); !ok {
		return errors.New("invalid transaction's inputs")
	}
	if ok := bc.memPool.Verify(t); !ok {
		return errors.New("invalid transaction due to conflicts with the memory pool")
	}
	if IsDoubleSpend(bc.Store, t) {
		return errors.New("invalid transaction caused by double spending")
	}
	if err := bc.verifyOutputs(t); err != nil {
		return errors.Wrap(err, "wrong outputs")
	}
	if err := bc.verifyResults(t); err != nil {
		return err
	}

	for _, a := range t.Attributes {
		if a.Usage == transaction.ECDH02 || a.Usage == transaction.ECDH03 {
			return errors.Errorf("invalid attribute's usage = %s ", a.Usage)
		}
	}

	return bc.verifyTxWitnesses(t, block)
}

func (bc *Blockchain) verifyInputs(t *transaction.Transaction) bool {
	for i := 1; i < len(t.Inputs); i++ {
		for j := 0; j < i; j++ {
			if t.Inputs[i].PrevHash == t.Inputs[j].PrevHash && t.Inputs[i].PrevIndex == t.Inputs[j].PrevIndex {
				return false
			}
		}
	}

	return true
}

func (bc *Blockchain) verifyOutputs(t *transaction.Transaction) error {
	for assetID, outputs := range t.GroupOutputByAssetID() {
		assetState := bc.GetAssetState(assetID)
		if assetState == nil {
			return fmt.Errorf("no asset state for %s", assetID.ReverseString())
		}

		if assetState.Expiration < bc.blockHeight+1 && assetState.AssetType != transaction.GoverningToken && assetState.AssetType != transaction.UtilityToken {
			return fmt.Errorf("asset %s expired", assetID.ReverseString())
		}

		for _, out := range outputs {
			if int64(out.Amount)%int64(math.Pow10(8-int(assetState.Precision))) != 0 {
				return fmt.Errorf("output is not compliant with %s asset precision", assetID.ReverseString())
			}
		}
	}

	return nil
}

func (bc *Blockchain) verifyResults(t *transaction.Transaction) error {
	results := bc.GetTransactionResults(t)
	if results == nil {
		return errors.New("tx has no results")
	}
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
	if len(resultsDestroy) == 1 && resultsDestroy[0].AssetID != utilityTokenTX().Hash() {
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
			if r.AssetID != utilityTokenTX().Hash() {
				return errors.New("miner or claim tx issues non-utility tokens")
			}
		}
		break
	case transaction.IssueType:
		for _, r := range resultsIssue {
			if r.AssetID == utilityTokenTX().Hash() {
				return errors.New("issue tx issues utility tokens")
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
	var tempResults []*transaction.Result
	var results []*transaction.Result
	tempGroupResult := make(map[util.Uint256]util.Fixed8)

	references := bc.References(t)
	if references == nil {
		return nil
	}
	for _, output := range references {
		tempResults = append(tempResults, &transaction.Result{
			AssetID: output.AssetID,
			Amount:  output.Amount,
		})
	}
	for _, output := range t.Outputs {
		tempResults = append(tempResults, &transaction.Result{
			AssetID: output.AssetID,
			Amount:  -output.Amount,
		})
	}
	for _, r := range tempResults {
		if amount, ok := tempGroupResult[r.AssetID]; ok {
			tempGroupResult[r.AssetID] = amount.Add(r.Amount)
		} else {
			tempGroupResult[r.AssetID] = r.Amount
		}
	}

	results = []*transaction.Result{} // this assignment is necessary. (Most of the time amount == 0 and results is the empty slice.)
	for assetID, amount := range tempGroupResult {
		if amount != util.Fixed8(0) {
			results = append(results, &transaction.Result{
				AssetID: assetID,
				Amount:  amount,
			})
		}
	}

	return results
}

// GetScriptHashesForVerifyingClaim returns all ScriptHashes of Claim transaction
// which has a different implementation from generic GetScriptHashesForVerifying.
func (bc *Blockchain) GetScriptHashesForVerifyingClaim(t *transaction.Transaction) ([]util.Uint160, error) {
	// Avoiding duplicates.
	hashmap := make(map[util.Uint160]bool)

	claim := t.Data.(*transaction.ClaimTX)
	clGroups := make(map[util.Uint256][]*transaction.Input)
	for _, in := range claim.Claims {
		clGroups[in.PrevHash] = append(clGroups[in.PrevHash], in)
	}
	for group, inputs := range clGroups {
		refTx, _, err := bc.GetTransaction(group)
		if err != nil {
			return nil, err
		}
		for _, input := range inputs {
			if len(refTx.Outputs) <= int(input.PrevIndex) {
				return nil, fmt.Errorf("wrong PrevIndex reference")
			}
			hashmap[refTx.Outputs[input.PrevIndex].ScriptHash] = true
		}
	}
	if len(hashmap) > 0 {
		hashes := make([]util.Uint160, 0, len(hashmap))
		for k := range hashmap {
			hashes = append(hashes, k)
		}
		return hashes, nil
	}
	return nil, fmt.Errorf("no hashes found")
}

// GetScriptHashesForVerifying returns all the ScriptHashes of a transaction which will be use
// to verify whether the transaction is bonafide or not.
// Golang implementation of GetScriptHashesForVerifying method in C# (https://github.com/neo-project/neo/blob/master/neo/Network/P2P/Payloads/Transaction.cs#L190)
func (bc *Blockchain) GetScriptHashesForVerifying(t *transaction.Transaction) ([]util.Uint160, error) {
	if t.Type == transaction.ClaimType {
		return bc.GetScriptHashesForVerifyingClaim(t)
	}
	references := bc.References(t)
	if references == nil {
		return nil, errors.New("Invalid operation")
	}
	hashes := make(map[util.Uint160]bool)
	for _, i := range t.Inputs {
		h := references[*i].ScriptHash
		if _, ok := hashes[h]; !ok {
			hashes[h] = true
		}
	}
	for _, a := range t.Attributes {
		if a.Usage == transaction.Script {
			h, err := util.Uint160DecodeBytes(a.Data)
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
	// convert hashes to []util.Uint160
	hashesResult := make([]util.Uint160, 0, len(hashes))
	for h := range hashes {
		hashesResult = append(hashesResult, h)
	}

	return hashesResult, nil

}

// verifyHashAgainstScript verifies given hash against the given witness.
func (bc *Blockchain) verifyHashAgainstScript(hash util.Uint160, witness *transaction.Witness, checkedHash util.Uint256, interopCtx *interopContext) error {
	verification := witness.VerificationScript

	if len(verification) == 0 {
		bb := new(bytes.Buffer)
		err := vm.EmitAppCall(bb, hash, false)
		if err != nil {
			return err
		}
		verification = bb.Bytes()
	} else {
		if h := witness.ScriptHash(); hash != h {
			return errors.New("witness hash mismatch")
		}
	}

	vm := vm.New(vm.ModeMute)
	vm.SetCheckedHash(checkedHash.Bytes())
	vm.SetScriptGetter(func(hash util.Uint160) []byte {
		cs := bc.GetContractState(hash)
		if cs == nil {
			return nil
		}
		return cs.Script
	})
	vm.RegisterInteropFuncs(interopCtx.getSystemInteropMap())
	vm.RegisterInteropFuncs(interopCtx.getNeoInteropMap())
	vm.LoadScript(verification)
	vm.LoadScript(witness.InvocationScript)
	vm.Run()
	if vm.HasFailed() {
		return errors.Errorf("vm failed to execute the script")
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
	} else {
		return errors.Errorf("no result returned from the script")
	}
	return nil
}

// verifyTxWitnesses verify the scripts (witnesses) that come with a given
// transaction. It can reorder them by ScriptHash, because that's required to
// match a slice of script hashes from the Blockchain. Block parameter
// is used for easy interop access and can be omitted for transactions that are
// not yet added into any block.
// Golang implementation of VerifyWitnesses method in C# (https://github.com/neo-project/neo/blob/master/neo/SmartContract/Helper.cs#L87).
// Unfortunately the IVerifiable interface could not be implemented because we can't move the References method in blockchain.go to the transaction.go file
func (bc *Blockchain) verifyTxWitnesses(t *transaction.Transaction, block *Block) error {
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
	interopCtx := newInteropContext(0, bc, block, t)
	for i := 0; i < len(hashes); i++ {
		err := bc.verifyHashAgainstScript(hashes[i], witnesses[i], t.VerificationHash(), interopCtx)
		if err != nil {
			numStr := fmt.Sprintf("witness #%d", i)
			return errors.Wrap(err, numStr)
		}
	}

	return nil
}

// verifyBlockWitnesses is a block-specific implementation of VerifyWitnesses logic.
func (bc *Blockchain) verifyBlockWitnesses(block *Block, prevHeader *Header) error {
	var hash util.Uint160
	if prevHeader == nil && block.PrevHash.Equals(util.Uint256{}) {
		hash = block.Script.ScriptHash()
	} else {
		hash = prevHeader.NextConsensus
	}
	interopCtx := newInteropContext(0, bc, nil, nil)
	return bc.verifyHashAgainstScript(hash, block.Script, block.VerificationHash(), interopCtx)
}

func hashAndIndexToBytes(h util.Uint256, index uint32) []byte {
	buf := io.NewBufBinWriter()
	buf.WriteLE(h.BytesReverse())
	buf.WriteLE(index)
	return buf.Bytes()
}

func (bc *Blockchain) secondsPerBlock() int {
	return bc.config.SecondsPerBlock
}
