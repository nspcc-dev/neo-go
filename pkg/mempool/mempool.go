package mempool

import (
	"errors"
	"fmt"
	"sync"

	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

var (
	ErrMemPoolFull        = errors.New("mempool is currently full")
	ErrMempoolEmpty       = errors.New("There are no TXs in the mempool")
	ErrTXTooBig           = errors.New("TX has exceed the maximum threshold")
	ErrTXTooManyWitnesses = errors.New("Too many witness scripts")
	ErrFeeTooLow          = errors.New("Fee for transaction too low")
	ErrDuplicateTX        = errors.New("TX Already in pool")
)

type Mempool struct {
	mtx  sync.RWMutex
	pool map[util.Uint256]*TX

	cfg Config
}

func New(cfg Config) *Mempool {
	mem := &Mempool{
		sync.RWMutex{},
		make(map[util.Uint256]*TX, 200),
		cfg,
	}

	return mem
}
func (m *Mempool) AddTransaction(trans transaction.Transactioner) error {

	hash, err := trans.ID()
	if err != nil {
		return err
	}

	// check if tx already in pool
	if m.Exists(hash) {
		return ErrDuplicateTX
	}

	m.mtx.Lock()
	defer m.mtx.Unlock()

	if m.cfg.MaxNumOfTX == uint64(len(m.pool)) {
		return ErrMemPoolFull
	}

	// TODO:Check for double spend from blockchain itself

	// create tx descriptor
	tx := Descriptor(trans)

	// check TX size
	if tx.Size > m.cfg.MaxTXSize {
		return ErrTXTooBig
	}

	// check witness length
	if tx.NumWitness > m.cfg.SigLimit {
		return ErrTXTooManyWitnesses
	}

	// TODO: check witness data is good -- Add method to take the Witness and tx return true or false.(blockchain)

	//check fee is over minimum cnfigured
	if tx.Fee < m.cfg.MinTXFee {
		return ErrFeeTooLow
	}

	// Add into pool
	m.pool[hash] = tx

	return nil
}

// RemoveTransaction will remove a transaction from the nodes mempool
func (m *Mempool) RemoveTransaction(hash util.Uint256) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	if len(m.pool) == 0 {
		return ErrMempoolEmpty
	}
	// deletes regardless of whether key is there or not. So do not check for existence before delete.
	// Use Exists() for this.
	delete(m.pool, hash)

	return nil
}

// Size returns the size of the mempool
func (m *Mempool) Size() uint64 {
	m.mtx.RLock()
	len := uint64(len(m.pool))
	m.mtx.RUnlock()

	return len
}

// ReturnAllTransactions will return all transactions in the
// mempool, will be mostly used by the RPC server
func (m *Mempool) ReturnAllTransactions() ([]transaction.Transactioner, error) {
	transactions := make([]transaction.Transactioner, 0)

	m.mtx.RLock()
	defer m.mtx.RUnlock()
	if len(m.pool) == 0 {
		return nil, ErrMempoolEmpty
	}

	for _, t := range m.pool {

		if t.ParentTX == nil {
			fmt.Println(t, "NILNIL")
		}
		transactions = append(transactions, *t.ParentTX)
		fmt.Println(transactions)
	}

	return transactions, nil

}

// Exists check whether the transaction exists in the mempool
func (m *Mempool) Exists(hash util.Uint256) bool {
	m.mtx.RLock()
	_, ok := m.pool[hash]
	m.mtx.RUnlock()

	return ok
}
