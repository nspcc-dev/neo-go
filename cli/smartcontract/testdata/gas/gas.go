// Package gastoken contains RPC wrappers for GasToken contract.
package gastoken

import (
	"errors"
	"fmt"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/nep17"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"math/big"
)

// Hash contains contract hash.
var Hash = util.Uint160{0xcf, 0x76, 0xe2, 0x8b, 0xd0, 0x6, 0x2c, 0x4a, 0x47, 0x8e, 0xe3, 0x55, 0x61, 0x1, 0x13, 0x19, 0xf3, 0xcf, 0xa4, 0xd2}



// TransferEvent represents event emitted by the contract.
type TransferEvent struct {
	From util.Uint160
	To util.Uint160
	Amount *big.Int
}
// Invoker is used by ContractReader to call various safe methods.
type Invoker interface {
	nep17.Invoker
}

// Actor is used by Contract to call state-changing methods.
type Actor interface {
	Invoker

	nep17.Actor
}

// ContractReader implements safe contract methods.
type ContractReader struct {
	nep17.TokenReader
	invoker Invoker
}

// Contract implements all contract methods.
type Contract struct {
	ContractReader
	nep17.TokenWriter
	actor Actor
}

// NewReader creates an instance of ContractReader using Hash and the given Invoker.
func NewReader(invoker Invoker) *ContractReader {
	return &ContractReader{*nep17.NewReader(invoker, Hash), invoker}
}

// New creates an instance of Contract using Hash and the given Actor.
func New(actor Actor) *Contract {
	var nep17t = nep17.New(actor, Hash)
	return &Contract{ContractReader{nep17t.TokenReader, actor}, nep17t.TokenWriter, actor}
}


// TransferEventFromApplicationLog retrieves TransferEvent from the
// provided ApplicationLog located at the specified index in the events list
// of the specified execution.
func TransferEventFromApplicationLog(log *result.ApplicationLog, executionIdx, eventIdx int) (*TransferEvent, error) {
	if log == nil {
		return nil, errors.New("nil application log")
	}
	if len(log.Executions) < executionIdx+1 {
		return nil, fmt.Errorf("missing execution result: expected %d, got %d", executionIdx+1, len(log.Executions))
	}
	ex := log.Executions[executionIdx]
	if len(ex.Events) < eventIdx+1 {
		return nil, fmt.Errorf("missing event: expected %d, got %d", eventIdx+1, len(ex.Events))
	}
	e := ex.Events[eventIdx].Item

	res := new(TransferEvent)
	err := res.FromStackItem(e)
	return res, err
}

// FromStackItem converts provided stackitem.Array to TransferEvent and
// returns an error if so.
func (e *TransferEvent) FromStackItem(item *stackitem.Array) error {
	if item == nil {
		return errors.New("nil item")
	}
	arr, ok := item.Value().([]stackitem.Item)
	if !ok {
		return errors.New("not an array")
	}
	if len(arr) != 3 {
		return errors.New("wrong number of structure elements")
	}

	var (
		index = -1
		err error
	)
	index++
	e.From, err = func (item stackitem.Item) (util.Uint160, error) {
		b, err := item.TryBytes()
		if err != nil {
			return util.Uint160{}, err
		}
		u, err := util.Uint160DecodeBytesBE(b)
		if err != nil {
			return util.Uint160{}, err
		}
		return u, nil
	} (arr[index])
	if err != nil {
		return fmt.Errorf("field From: %w", err)
	}
	
	index++
	e.To, err = func (item stackitem.Item) (util.Uint160, error) {
		b, err := item.TryBytes()
		if err != nil {
			return util.Uint160{}, err
		}
		u, err := util.Uint160DecodeBytesBE(b)
		if err != nil {
			return util.Uint160{}, err
		}
		return u, nil
	} (arr[index])
	if err != nil {
		return fmt.Errorf("field To: %w", err)
	}
	
	index++
	e.Amount, err = arr[index].TryInteger()
	if err != nil {
		return fmt.Errorf("field Amount: %w", err)
	}
	
	return nil
}
