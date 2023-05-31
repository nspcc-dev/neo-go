// Package verify contains RPC wrappers for verify contract.
package verify

import (
	"errors"
	"fmt"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Hash contains contract hash.
var Hash = util.Uint160{0x33, 0x22, 0x11, 0x0, 0xff, 0xee, 0xdd, 0xcc, 0xbb, 0xaa, 0x99, 0x88, 0x77, 0x66, 0x55, 0x44, 0x33, 0x22, 0x11, 0x0}

// HelloWorldEvent represents "Hello world!" event emitted by the contract.
type HelloWorldEvent struct {
	Args []any
}

// Actor is used by Contract to call state-changing methods.
type Actor interface {
	MakeCall(contract util.Uint160, method string, params ...any) (*transaction.Transaction, error)
	MakeRun(script []byte) (*transaction.Transaction, error)
	MakeUnsignedCall(contract util.Uint160, method string, attrs []transaction.Attribute, params ...any) (*transaction.Transaction, error)
	MakeUnsignedRun(script []byte, attrs []transaction.Attribute) (*transaction.Transaction, error)
	SendCall(contract util.Uint160, method string, params ...any) (util.Uint256, uint32, error)
	SendRun(script []byte) (util.Uint256, uint32, error)
}

// Contract implements all contract methods.
type Contract struct {
	actor Actor
}

// New creates an instance of Contract using Hash and the given Actor.
func New(actor Actor) *Contract {
	return &Contract{actor}
}

func scriptForVerify() ([]byte, error) {
	return smartcontract.CreateCallWithAssertScript(Hash, "verify")
}

// Verify creates a transaction invoking `verify` method of the contract.
// This transaction is signed and immediately sent to the network.
// The values returned are its hash, ValidUntilBlock value and error if any.
func (c *Contract) Verify() (util.Uint256, uint32, error) {
	script, err := scriptForVerify()
	if err != nil {
		return util.Uint256{}, 0, err
	}
	return c.actor.SendRun(script)
}

// VerifyTransaction creates a transaction invoking `verify` method of the contract.
// This transaction is signed, but not sent to the network, instead it's
// returned to the caller.
func (c *Contract) VerifyTransaction() (*transaction.Transaction, error) {
	script, err := scriptForVerify()
	if err != nil {
		return nil, err
	}
	return c.actor.MakeRun(script)
}

// VerifyUnsigned creates a transaction invoking `verify` method of the contract.
// This transaction is not signed, it's simply returned to the caller.
// Any fields of it that do not affect fees can be changed (ValidUntilBlock,
// Nonce), fee values (NetworkFee, SystemFee) can be increased as well.
func (c *Contract) VerifyUnsigned() (*transaction.Transaction, error) {
	script, err := scriptForVerify()
	if err != nil {
		return nil, err
	}
	return c.actor.MakeUnsignedRun(script, nil)
}

// HelloWorldEventsFromApplicationLog retrieves a set of all emitted events
// with "Hello world!" name from the provided [result.ApplicationLog].
func HelloWorldEventsFromApplicationLog(log *result.ApplicationLog) ([]*HelloWorldEvent, error) {
	if log == nil {
		return nil, errors.New("nil application log")
	}

	var res []*HelloWorldEvent
	for i, ex := range log.Executions {
		for j, e := range ex.Events {
			if e.Name != "Hello world!" {
				continue
			}
			event := new(HelloWorldEvent)
			err := event.FromStackItem(e.Item)
			if err != nil {
				return nil, fmt.Errorf("failed to deserialize HelloWorldEvent from stackitem (execution #%d, event #%d): %w", i, j, err)
			}
			res = append(res, event)
		}
	}

	return res, nil
}

// FromStackItem converts provided [stackitem.Array] to HelloWorldEvent or
// returns an error if it's not possible to do to so.
func (e *HelloWorldEvent) FromStackItem(item *stackitem.Array) error {
	if item == nil {
		return errors.New("nil item")
	}
	arr, ok := item.Value().([]stackitem.Item)
	if !ok {
		return errors.New("not an array")
	}
	if len(arr) != 1 {
		return errors.New("wrong number of structure elements")
	}

	var (
		index = -1
		err error
	)
	index++
	e.Args, err = func (item stackitem.Item) ([]any, error) {
		arr, ok := item.Value().([]stackitem.Item)
		if !ok {
			return nil, errors.New("not an array")
		}
		res := make([]any, len(arr))
		for i := range res {
			res[i], err = arr[i].Value(), error(nil)
			if err != nil {
				return nil, fmt.Errorf("item %d: %w", i, err)
			}
		}
		return res, nil
	} (arr[index])
	if err != nil {
		return fmt.Errorf("field Args: %w", err)
	}

	return nil
}
