package mempool_test

import (
	"testing"
	"time"

	"github.com/CityOfZion/neo-go/pkg/mempool"
	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction"
	"github.com/stretchr/testify/assert"
)

func TestMempoolExists(t *testing.T) {
	cfg := mempool.Config{
		MaxNumOfTX: 100,
		FreeTX:     20,
		MinTXFee:   0,
		MaxTXSize:  10000,
		TXTTL:      10 * time.Minute,
		SigLimit:   20,
	}
	mem := mempool.New(cfg)

	trans := transaction.NewContract(0)

	assert.Equal(t, false, mem.Exists(trans.Hash))

	err := mem.AddTransaction(trans)
	assert.Equal(t, nil, err)

	assert.Equal(t, true, mem.Exists(trans.Hash))
}
func TestMempoolFullPool(t *testing.T) {

	maxTx := uint64(100)
	cfg := mempool.Config{
		MaxNumOfTX: maxTx,
		FreeTX:     20,
		MinTXFee:   0,
		MaxTXSize:  10000,
		TXTTL:      10 * time.Minute,
		SigLimit:   20,
	}
	mem := mempool.New(cfg)

	for i := uint64(1); i <= maxTx; i++ {
		trans := transaction.NewContract(0)
		attr := &transaction.Attribute{
			Usage: transaction.Remark,
			Data:  []byte{byte(i)},
		}
		trans.AddAttribute(attr)
		err := mem.AddTransaction(trans)
		assert.Equal(t, nil, err)
	}
	trans := transaction.NewContract(0)
	err := mem.AddTransaction(trans)
	assert.NotEqual(t, nil, err)

	assert.Equal(t, mempool.ErrMemPoolFull, err)
}
func TestMempoolLargeTX(t *testing.T) {

	maxTxSize := uint64(100)
	cfg := mempool.Config{
		MaxNumOfTX: 100,
		FreeTX:     20,
		MinTXFee:   0,
		MaxTXSize:  maxTxSize,
		TXTTL:      10 * time.Minute,
		SigLimit:   20,
	}
	mem := mempool.New(cfg)

	trans := transaction.NewContract(0)
	for i := uint64(1); i <= 100; i++ { // 100 attributes will be over 100 bytes
		attr := &transaction.Attribute{
			Usage: transaction.Remark,
			Data:  []byte{byte(i)},
		}
		trans.AddAttribute(attr)
	}

	err := mem.AddTransaction(trans)
	assert.NotEqual(t, nil, err)
	assert.Equal(t, mempool.ErrTXTooBig, err)
}
func TestMempoolTooManyWitness(t *testing.T) {

	maxWitness := uint8(3)
	cfg := mempool.Config{
		MaxNumOfTX: 100,
		FreeTX:     20,
		MinTXFee:   0,
		MaxTXSize:  10000,
		TXTTL:      10 * time.Minute,
		SigLimit:   maxWitness,
	}
	mem := mempool.New(cfg)

	trans := transaction.NewContract(0)
	for i := uint8(1); i <= maxWitness; i++ { // 100 attributes will be over 100 bytes
		wit := &transaction.Witness{
			InvocationScript:   []byte{byte(i)},
			VerificationScript: []byte{byte(i)},
		}
		trans.AddWitness(wit)
	}

	trans.AddWitness(&transaction.Witness{
		InvocationScript:   []byte{},
		VerificationScript: []byte{},
	})

	err := mem.AddTransaction(trans)
	assert.NotEqual(t, nil, err)
	assert.Equal(t, mempool.ErrTXTooManyWitnesses, err)
}
func TestMempoolDuplicate(t *testing.T) {

	cfg := mempool.Config{
		MaxNumOfTX: 100,
		FreeTX:     20,
		MinTXFee:   0,
		MaxTXSize:  10000,
		TXTTL:      10 * time.Minute,
		SigLimit:   1,
	}
	mem := mempool.New(cfg)

	trans := transaction.NewContract(0)

	err := mem.AddTransaction(trans)
	assert.Equal(t, nil, err)

	err = mem.AddTransaction(trans)
	assert.NotEqual(t, nil, err)
	assert.Equal(t, mempool.ErrDuplicateTX, err)
}
func TestMempoolReturnAll(t *testing.T) {

	cfg := mempool.Config{
		MaxNumOfTX: 100,
		FreeTX:     20,
		MinTXFee:   0,
		MaxTXSize:  10000,
		TXTTL:      10 * time.Minute,
		SigLimit:   1,
	}
	mem := mempool.New(cfg)

	numTx := uint64(10)

	for i := uint64(1); i <= numTx; i++ {
		trans := transaction.NewContract(0)
		attr := &transaction.Attribute{
			Usage: transaction.Remark,
			Data:  []byte{byte(i)},
		}
		trans.AddAttribute(attr)
		err := mem.AddTransaction(trans)
		assert.Equal(t, nil, err)
	}

	AllTrans, err := mem.ReturnAllTransactions()
	assert.Equal(t, nil, err)

	assert.Equal(t, numTx, uint64(len(AllTrans)))

}
func TestMempoolRemove(t *testing.T) {

	cfg := mempool.Config{
		MaxNumOfTX: 100,
		FreeTX:     20,
		MinTXFee:   0,
		MaxTXSize:  10000,
		TXTTL:      3 * time.Minute,
		SigLimit:   1,
	}
	mem := mempool.New(cfg)

	// Remove a transaction when mempool is empty
	trans := transaction.NewContract(0)
	hash, _ := trans.ID()
	err := mem.RemoveTransaction(hash)
	assert.Equal(t, mempool.ErrMempoolEmpty, err)

	// Add tx1 into mempool
	err = mem.AddTransaction(trans)
	assert.Equal(t, nil, err)

	diffTrans := transaction.NewContract(0) // TX2

	diffTrans.AddAttribute(
		&transaction.Attribute{
			Usage: transaction.Remark,
			Data:  []byte{},
		})

	diffHash, _ := diffTrans.ID()

	// Try removing TX2, when only TX1 is in mempool
	err = mem.RemoveTransaction(diffHash)
	assert.Equal(t, nil, err)
	assert.Equal(t, uint64(1), mem.Size())
	// Remove hash that is in mempool
	err = mem.RemoveTransaction(hash)
	assert.Equal(t, nil, err)
	assert.Equal(t, uint64(0), mem.Size())

}
