package core

import (
	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/io"
)

// BlockChainState represents Blockchain state structure with mempool.
type BlockChainState struct {
	store        *storage.MemCachedStore
	unspentCoins UnspentCoins
	spentCoins   SpentCoins
	accounts     Accounts
	assets       Assets
	contracts    Contracts
	validators   Validators
}

// NewBlockChainState creates blockchain state with it's memchached store.
func NewBlockChainState(store *storage.MemCachedStore) *BlockChainState {
	tmpStore := storage.NewMemCachedStore(store)
	return &BlockChainState{
		store:        tmpStore,
		unspentCoins: make(UnspentCoins),
		spentCoins:   make(SpentCoins),
		accounts:     make(Accounts),
		assets:       make(Assets),
		contracts:    make(Contracts),
		validators:   make(Validators),
	}
}

// commit commits all the data in current state into storage.
func (state *BlockChainState) commit() error {
	if err := state.accounts.commit(state.store); err != nil {
		return err
	}
	if err := state.unspentCoins.commit(state.store); err != nil {
		return err
	}
	if err := state.spentCoins.commit(state.store); err != nil {
		return err
	}
	if err := state.assets.commit(state.store); err != nil {
		return err
	}
	if err := state.contracts.commit(state.store); err != nil {
		return err
	}
	if err := state.validators.commit(state.store); err != nil {
		return err
	}
	if _, err := state.store.Persist(); err != nil {
		return err
	}
	return nil
}

// storeAsBlock stores the given block as DataBlock.
func (state *BlockChainState) storeAsBlock(block *Block, sysFee uint32) error {
	var (
		key = storage.AppendPrefix(storage.DataBlock, block.Hash().BytesReverse())
		buf = io.NewBufBinWriter()
	)
	// sysFee needs to be handled somehow
	//	buf.WriteLE(sysFee)
	b, err := block.Trim()
	if err != nil {
		return err
	}
	buf.WriteBytes(b)
	if buf.Err != nil {
		return buf.Err
	}
	return state.store.Put(key, buf.Bytes())
}

// storeAsCurrentBlock stores the given block witch prefix SYSCurrentBlock.
func (state *BlockChainState) storeAsCurrentBlock(block *Block) error {
	buf := io.NewBufBinWriter()
	buf.WriteBytes(block.Hash().BytesReverse())
	buf.WriteLE(block.Index)
	return state.store.Put(storage.SYSCurrentBlock.Bytes(), buf.Bytes())
}

// storeAsTransaction stores the given TX as DataTransaction.
func (state *BlockChainState) storeAsTransaction(tx *transaction.Transaction, index uint32) error {
	key := storage.AppendPrefix(storage.DataTransaction, tx.Hash().BytesReverse())
	buf := io.NewBufBinWriter()
	buf.WriteLE(index)
	tx.EncodeBinary(buf.BinWriter)
	if buf.Err != nil {
		return buf.Err
	}
	return state.store.Put(key, buf.Bytes())
}
