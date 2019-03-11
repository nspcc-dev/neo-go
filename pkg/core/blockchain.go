package core

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"sync/atomic"
	"time"

	"github.com/CityOfZion/neo-go/config"
	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/util"
	log "github.com/sirupsen/logrus"
)

// tuning parameters
const (
	headerBatchCount = 2000
	version          = "0.0.1"
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

	// Current index/height of the highest block.
	// Read access should always be called by BlockHeight().
	// Write access should only happen in persist().
	blockHeight uint32

	// Number of headers stored in the chain file.
	storedHeaderCount uint32

	blockCache *Cache

	// All operation on headerList must be called from an
	// headersOp to be routine safe.
	headerList *HeaderHashList

	// Only for operating on the headerList.
	headersOp     chan headersOpFunc
	headersOpDone chan struct{}

	// Whether we will verify received blocks.
	verifyBlocks bool

	memPool MemPool
}

type headersOpFunc func(headerList *HeaderHashList)

// NewBlockchain return a new blockchain object the will use the
// given Store as its underlying storage.
func NewBlockchain(ctx context.Context, s storage.Store, cfg config.ProtocolConfiguration) (*Blockchain, error) {
	bc := &Blockchain{
		config:        cfg,
		Store:         s,
		headersOp:     make(chan headersOpFunc),
		headersOpDone: make(chan struct{}),
		blockCache:    NewCache(),
		verifyBlocks:  false,
		memPool:       NewMemPool(50000),
	}

	go bc.run(ctx)
	if err := bc.init(); err != nil {
		return nil, err
	}

	return bc, nil
}

// GetBlockchainLevelDB returns blockchain based on configuration
func NewBlockchainLevelDB(ctx context.Context, cfg config.Config) (*Blockchain, error) {
	store, err := storage.NewLevelDBStore(
		ctx,
		cfg.ApplicationConfiguration.DataDirectoryPath,
		nil,
	)
	if err != nil {
		return nil, err
	}

	return NewBlockchain(ctx, store, cfg.ProtocolConfiguration)
}

func (bc *Blockchain) init() error {
	genesisBlock, err := createGenesisBlock(bc.config)
	if err != nil {
		return err
	}
	bc.headerList = NewHeaderHashList(genesisBlock.Hash())

	// If we could not find the version in the Store, we know that there is nothing stored.
	ver, err := storage.Version(bc.Store)
	if err != nil {
		log.Infof("no storage version found! creating genesis block")
		if err = storage.PutVersion(bc.Store, version); err != nil {
			return err
		}
		return bc.persistBlock(genesisBlock)
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

	hashes, err := storage.HeaderHashes(bc.Store)
	if err != nil {
		return err
	}

	for _, hash := range hashes {
		if !genesisBlock.Hash().Equals(hash) {
			bc.headerList.Add(hash)
			bc.storedHeaderCount++
		}
	}

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
		if err := bc.AddHeaders(headers...); err != nil {
			return err
		}
	}

	return nil
}

func (bc *Blockchain) run(ctx context.Context) {
	persistTimer := time.NewTimer(persistInterval)
	defer persistTimer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case op := <-bc.headersOp:
			op(bc.headerList)
			bc.headersOpDone <- struct{}{}
		case <-persistTimer.C:
			go bc.persist(ctx)
			persistTimer.Reset(persistInterval)
		}
	}
}

// AddBlock processes the given block and will add it to the cache so it
// can be persisted.
func (bc *Blockchain) AddBlock(block *Block) error {
	if !bc.blockCache.Has(block.Hash()) {
		bc.blockCache.Add(block.Hash(), block)
	}

	headerLen := bc.headerListLen()
	if int(block.Index-1) >= headerLen {
		return nil
	}
	if int(block.Index) == headerLen {
		if bc.verifyBlocks && !block.Verify(false) {
			return fmt.Errorf("block %s is invalid", block.Hash())
		}
		return bc.AddHeaders(block.Header())
	}
	return nil
}

// AddHeaders will process the given headers and add them to the
// HeaderHashList.
func (bc *Blockchain) AddHeaders(headers ...*Header) (err error) {
	var (
		start = time.Now()
		batch = bc.Batch()
	)

	bc.headersOp <- func(headerList *HeaderHashList) {
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

		if batch.Len() > 0 {
			if err = bc.PutBatch(batch); err != nil {
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

	buf := new(bytes.Buffer)
	for int(h.Index)-headerBatchCount >= int(bc.storedHeaderCount) {
		if err := headerList.Write(buf, int(bc.storedHeaderCount), headerBatchCount); err != nil {
			return err
		}
		key := storage.AppendPrefixInt(storage.IXHeaderHashList, int(bc.storedHeaderCount))
		batch.Put(key, buf.Bytes())
		bc.storedHeaderCount += headerBatchCount
		buf.Reset()
	}

	buf.Reset()
	if err := h.EncodeBinary(buf); err != nil {
		return err
	}

	key := storage.AppendPrefix(storage.DataBlock, h.Hash().BytesReverse())
	batch.Put(key, buf.Bytes())
	batch.Put(storage.SYSCurrentHeader.Bytes(), hashAndIndexToBytes(h.Hash(), h.Index))

	return nil
}

// TODO: persistBlock needs some more love, its implemented as in the original
// project. This for the sake of development speed and understanding of what
// is happening here, quite allot as you can see :). If things are wired together
// and all tests are in place, we can make a more optimized and cleaner implementation.
func (bc *Blockchain) persistBlock(block *Block) error {
	var (
		batch        = bc.Batch()
		unspentCoins = make(UnspentCoins)
		spentCoins   = make(SpentCoins)
		accounts     = make(Accounts)
		assets       = make(Assets)
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
			account, err := accounts.getAndUpdate(bc.Store, output.ScriptHash)
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
				unspent, err := unspentCoins.getAndUpdate(bc.Store, input.PrevHash)
				if err != nil {
					return err
				}
				unspent.states[input.PrevIndex] = CoinStateSpent

				prevTXOutput := prevTX.Outputs[input.PrevIndex]
				account, err := accounts.getAndUpdate(bc.Store, prevTXOutput.ScriptHash)
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
				ID:        tx.Hash(),
				AssetType: t.AssetType,
				Name:      t.Name,
				Amount:    t.Amount,
				Precision: t.Precision,
				Owner:     t.Owner,
				Admin:     t.Admin,
			}
		case *transaction.IssueTX:
		case *transaction.ClaimTX:
		case *transaction.EnrollmentTX:
		case *transaction.StateTX:
		case *transaction.PublishTX:
			contract := &ContractState{
				Script:      t.Script,
				ParamList:   t.ParamList,
				ReturnType:  t.ReturnType,
				HasStorage:  t.NeedStorage,
				Name:        t.Name,
				CodeVersion: t.CodeVersion,
				Author:      t.Author,
				Email:       t.Email,
				Description: t.Description,
			}

			fmt.Printf("%+v", contract)

		case *transaction.InvocationTX:
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
	if err := bc.PutBatch(batch); err != nil {
		return err
	}

	atomic.StoreUint32(&bc.blockHeight, block.Index)
	return nil
}

func (bc *Blockchain) persist(ctx context.Context) (err error) {
	var (
		start     = time.Now()
		persisted = 0
		lenCache  = bc.blockCache.Len()
	)

	if lenCache == 0 {
		return nil
	}

	bc.headersOp <- func(headerList *HeaderHashList) {
		for i := 0; i < lenCache; i++ {
			if uint32(headerList.Len()) <= bc.BlockHeight() {
				return
			}
			hash := headerList.Get(int(bc.BlockHeight() + 1))
			if block, ok := bc.blockCache.GetBlock(hash); ok {
				if err = bc.persistBlock(block); err != nil {
					log.Warnf("failed to persist blocks: %s", err)
					return
				}
				bc.blockCache.Delete(hash)
				persisted++
			}
		}
	}

	select {
	case <-ctx.Done():
		return
	case <-bc.headersOpDone:
		//
	}

	if persisted > 0 {
		log.WithFields(log.Fields{
			"persisted":    persisted,
			"headerHeight": bc.HeaderHeight(),
			"blockHeight":  bc.BlockHeight(),
			"took":         time.Since(start),
		}).Info("blockchain persist completed")
	}

	return
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

	key := storage.AppendPrefix(storage.DataTransaction, hash.BytesReverse())
	b, err := bc.Get(key)
	if err != nil {
		return nil, 0, err
	}
	r := bytes.NewReader(b)

	var height uint32
	if err := binary.Read(r, binary.LittleEndian, &height); err != nil {
		return nil, 0, err
	}

	tx := &transaction.Transaction{}
	if err := tx.DecodeBinary(r); err != nil {
		return nil, 0, err
	}
	return tx, height, nil
}

// GetBlock returns a Block by the given hash.
func (bc *Blockchain) GetBlock(hash util.Uint256) (*Block, error) {
	key := storage.AppendPrefix(storage.DataBlock, hash.BytesReverse())
	b, err := bc.Get(key)
	if err != nil {
		return nil, err
	}
	block, err := NewBlockFromTrimmedBytes(b)
	if err != nil {
		return nil, err
	}
	// TODO: persist TX first before we can handle this logic.
	// if len(block.Transactions) == 0 {
	// 	return nil, fmt.Errorf("block has no TX")
	// }
	return block, nil
}

func (bc *Blockchain) GetHeader(hash util.Uint256) (*Header, error) {
	b, err := bc.Get(storage.AppendPrefix(storage.DataBlock, hash.BytesReverse()))
	if err != nil {
		return nil, err
	}
	block, err := NewBlockFromTrimmedBytes(b)
	if err != nil {
		return nil, err
	}
	return block.Header(), nil
}

// HasTransaction return true if the blockchain contains he given
// transaction hash.
func (bc *Blockchain) HasTransaction(hash util.Uint256) bool {
	if bc.memPool.ContainsKey(hash) {
		return true
	}

	key := storage.AppendPrefix(storage.DataTransaction, hash.BytesReverse())
	if _, err := bc.Get(key); err == nil {
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
	var as *AssetState
	bc.Store.Seek(storage.STAsset.Bytes(), func(k, v []byte) {
		var a AssetState
		if err := a.DecodeBinary(bytes.NewReader(v)); err == nil && a.ID == assetID {
			as = &a
		}
	})

	return as
}

// GetAccountState returns the account state from its script hash
func (bc *Blockchain) GetAccountState(scriptHash util.Uint160) *AccountState {
	var as *AccountState
	bc.Store.Seek(storage.STAccount.Bytes(), func(k, v []byte) {
		var a AccountState
		if err := a.DecodeBinary(bytes.NewReader(v)); err == nil && a.ScriptHash == scriptHash {
			as = &a
		}
	})

	return as
}

// GetConfig returns the config stored in the blockchain
func (bc *Blockchain) GetConfig() config.ProtocolConfiguration {
	return bc.config
}

// References returns a map with input prevHash as key (util.Uint256)
// and transaction output as value from a transaction t.
// @TODO: unfortunately we couldn't attach this method to the Transaction struct in the
// transaction package because of a import cycle problem. Perhaps we should think to re-design
// the code base to avoid this situation.
func (bc *Blockchain) References(t *transaction.Transaction) map[util.Uint256]*transaction.Output {
	references := make(map[util.Uint256]*transaction.Output)

	for prevHash, inputs := range t.GroupInputsByPrevHash() {
		if tx, _, err := bc.GetTransaction(prevHash); err != nil {
			tx = nil
		} else if tx != nil {
			for _, in := range inputs {
				references[in.PrevHash] = tx.Outputs[in.PrevIndex]
			}
		} else {
			references = nil
		}
	}
	return references
}

// FeePerByte returns network fee divided by the size of the transaction
func (bc *Blockchain) FeePerByte(t *transaction.Transaction) util.Fixed8 {
	return bc.NetworkFee(t).Div(int64(t.Size()))
}

// NetworkFee returns network fee
func (bc *Blockchain) NetworkFee(t *transaction.Transaction) util.Fixed8 {
	inputAmount := util.NewFixed8(0)
	for _, txOutput := range bc.References(t) {
		if txOutput.AssetID == utilityTokenTX().Hash() {
			inputAmount.Add(txOutput.Amount)
		}
	}

	outputAmount := util.NewFixed8(0)
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
	return bc.NetworkFee(t) < util.NewFixed8FromFloat(bc.GetConfig().LowPriorityThreshold)
}

// GetMemPool returns the memory pool of the blockchain.
func (bc *Blockchain) GetMemPool() MemPool {
	return bc.memPool
}

// Verify verifies whether a transaction is bonafide or not.
// Golang implementation of Verify method in C# (https://github.com/neo-project/neo/blob/master/neo/Network/P2P/Payloads/Transaction.cs#L270).
func (bc *Blockchain) Verify(t *transaction.Transaction) bool {
	if t.Size() > transaction.MaxTransactionSize {
		return false
	}
	if ok := bc.verifyInputs(t); !ok {
		return false
	}
	if ok := bc.memPool.Verify(t); !ok {
		return false
	}
	if IsDoubleSpend(bc.Store, t) {
		return false
	}
	if ok := bc.verifyOutputs(t); !ok {
		return false
	}
	if ok := bc.verifyResults(t); !ok {
		return false
	}

	for _, a := range t.Attributes {
		if a.Usage == transaction.ECDH02 || a.Usage == transaction.ECDH03 {
			return false
		}
	}

	return bc.VerifyWitnesses(t)
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

func (bc *Blockchain) verifyOutputs(t *transaction.Transaction) bool {
	for assetID, outputs := range t.GroupOutputByAssetID() {
		assetState := bc.GetAssetState(assetID)
		if assetState == nil {
			return false
		}

		if assetState.Expiration < bc.blockHeight+1 && assetState.AssetType != transaction.GoverningToken && assetState.AssetType != transaction.UtilityToken {
			return false
		}

		for _, out := range outputs {
			if int64(out.Amount)%int64(math.Pow10(8-int(assetState.Precision))) != 0 {
				return false
			}
		}
	}

	return true
}

func (bc *Blockchain) verifyResults(t *transaction.Transaction) bool {
	if results := bc.GetTransationResults(t); results == nil {
		return false
	} else {
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
			return false
		}
		if len(resultsDestroy) == 1 && resultsDestroy[0].AssetID != utilityTokenTX().Hash() {
			return false
		}
		if bc.SystemFee(t).GreaterThan(util.Fixed8(0)) && (len(resultsDestroy) == 0 || resultsDestroy[0].Amount.LessThan(bc.SystemFee(t))) {
			return false
		}

		switch t.Type {
		case transaction.MinerType, transaction.ClaimType:
			for _, r := range resultsIssue {
				if r.AssetID != utilityTokenTX().Hash() {
					return false
				}
			}
			break
		case transaction.IssueType:
			for _, r := range resultsIssue {
				if r.AssetID == utilityTokenTX().Hash() {
					return false
				}
			}
			break
		default:
			if len(resultsIssue) > 0 {
				return false
			}
			break
		}

	}

	return true
}

// GetTransationResults returns the transaction results aggregate by assetID.
// Golang of GetTransationResults method in C# (https://github.com/neo-project/neo/blob/master/neo/Network/P2P/Payloads/Transaction.cs#L207)
func (bc *Blockchain) GetTransationResults(t *transaction.Transaction) []*transaction.Result {
	var tempResults []*transaction.Result
	var results []*transaction.Result
	tempGroupResult := make(map[util.Uint256]util.Fixed8)

	if references := bc.References(t); references == nil {
		return nil
	} else {
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

		for assetID, amount := range tempGroupResult {
			results = []*transaction.Result{} // this assignment is necessary. (Most of the time amount == 0 and results is the empty slice.)
			if amount != util.Fixed8(0) {
				results = append(results, &transaction.Result{
					AssetID: assetID,
					Amount:  amount,
				})
			}
		}

		return results

	}

}

// GetScriptHashesForVerifying returns all the ScriptHashes of a transaction which will be use
// to verify whether the transaction is bonafide or not.
// Golang implementation of GetScriptHashesForVerifying method in C# (https://github.com/neo-project/neo/blob/master/neo/Network/P2P/Payloads/Transaction.cs#L190)
func (bc *Blockchain) GetScriptHashesForVerifying(t *transaction.Transaction) ([]util.Uint160, error) {
	references := bc.References(t)
	if references == nil {
		return nil, errors.New("Invalid operation")
	}
	hashes := make(map[util.Uint160]bool)
	for _, i := range t.Inputs {
		h := references[i.PrevHash].ScriptHash
		if _, ok := hashes[h]; !ok {
			hashes[h] = true
		}
	}
	for _, a := range t.Attributes {
		if a.Usage == transaction.Script {
			h, err := util.Uint160FromScript(a.Data)
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
		if as.AssetType == transaction.DutyFlag {
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

// VerifyWitnesses verify the scripts (witnesses) that come with a transactions.
// Golang implementation of VerifyWitnesses method in C# (https://github.com/neo-project/neo/blob/master/neo/SmartContract/Helper.cs#L87).
// Unfortunately the IVerifiable interface could not be implemented because we can't move the References method in blockchain.go to the transaction.go file
func (bc *Blockchain) VerifyWitnesses(t *transaction.Transaction) bool {
	hashes, err := bc.GetScriptHashesForVerifying(t)
	if err != nil {
		return false
	}

	witnesses := t.Scripts
	if len(hashes) != len(witnesses) {
		return false
	}
	for i := 0; i < len(hashes); i++ {
		verification := witnesses[i].VerificationScript

		if len(verification) == 0 {
			/*TODO: replicate following C# code:
			using (ScriptBuilder sb = new ScriptBuilder())
			{
				sb.EmitAppCall(hashes[i].ToArray());
				verification = sb.ToArray();
			}
			*/

		} else {
			if h, err := witnesses[i].ScriptHash(); err != nil || hashes[i] != h {
				return false
			}
		}

		/*TODO: replicate following C# code:
		using (ApplicationEngine engine = new ApplicationEngine(TriggerType.Verification, verifiable, snapshot, Fixed8.Zero))
		{
			engine.LoadScript(verification);
			engine.LoadScript(verifiable.Witnesses[i].InvocationScript);
			if (!engine.Execute()) return false;
			if (engine.ResultStack.Count != 1 || !engine.ResultStack.Pop().GetBoolean()) return false;
		}*/
	}

	return true
}

func hashAndIndexToBytes(h util.Uint256, index uint32) []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, index)
	return append(h.BytesReverse(), buf...)
}

func (bc *Blockchain) secondsPerBlock() int {
	return bc.config.SecondsPerBlock
}
