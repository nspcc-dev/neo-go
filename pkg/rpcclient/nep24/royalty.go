/*
Package nep24 provides RPC wrappers for NEP-24 contracts.

All methods are safe (read-only) and encapsulated in the RoyaltyReader structure,
designed for managing NFT royalties and retrieving royalty information.
Refer to the nep11 package for basic NFT functionalities, while nep24 handles
royalty-related operations.
*/
package nep24

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/neptoken"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// RoyaltyRecipient contains information about the recipient and the royalty amount.
type RoyaltyRecipient struct {
	Address util.Uint160
	Amount  *big.Int
}

// RoyaltiesTransferredEvent represents a RoyaltiesTransferred event as defined in
// the NEP-24 standard.
type RoyaltiesTransferredEvent struct {
	RoyaltyToken     util.Uint160
	RoyaltyRecipient util.Uint160
	Buyer            util.Uint160
	TokenID          []byte
	Amount           *big.Int
}

// RoyaltyReader represents safe (read-only) methods of NEP-24 token. It can be
// used to query data about royalties.
type RoyaltyReader struct {
	invoker neptoken.Invoker
	hash    util.Uint160
}

// NewRoyaltyReader returns a new RoyaltyReader instance.
func NewRoyaltyReader(invoker neptoken.Invoker, hash util.Uint160) *RoyaltyReader {
	return &RoyaltyReader{
		invoker: invoker,
		hash:    hash,
	}
}

// RoyaltyInfo returns the royalty information for the given tokenID, royaltyToken,
// and salePrice.
func (c *RoyaltyReader) RoyaltyInfo(tokenID []byte, royaltyToken util.Uint160, salePrice *big.Int) ([]RoyaltyRecipient, error) {
	res, err := c.invoker.Call(c.hash, "royaltyInfo", tokenID, royaltyToken, salePrice)
	if err != nil {
		return nil, err
	}
	if len(res.Stack) != 1 {
		return nil, errors.New("invalid response: expected a single item on the stack")
	}
	rootItem, ok := res.Stack[0].Value().([]stackitem.Item)
	if !ok {
		return nil, errors.New("invalid response: expected an array of royalties")
	}

	var royalties []RoyaltyRecipient
	for _, item := range rootItem {
		royalty, ok := item.Value().([]stackitem.Item)
		if !ok {
			return nil, fmt.Errorf("invalid royalty structure: expected array of 2 items, got %d", len(royalty))
		}
		var recipient RoyaltyRecipient
		err = recipient.FromStackItem(royalty)
		if err != nil {
			return nil, fmt.Errorf("failed to decode royalty detail: %w", err)
		}
		royalties = append(royalties, recipient)
	}

	return royalties, nil
}

// FromStackItem converts a stack item into a RoyaltyRecipient struct.
func (r *RoyaltyRecipient) FromStackItem(item []stackitem.Item) error {
	if len(item) != 2 {
		return fmt.Errorf("invalid royalty structure: expected 2 items, got %d", len(item))
	}

	recipientBytes, err := item[0].TryBytes()
	if err != nil {
		return fmt.Errorf("failed to decode recipient address: %w", err)
	}

	recipient, err := util.Uint160DecodeBytesBE(recipientBytes)
	if err != nil {
		return fmt.Errorf("invalid recipient address: %w", err)
	}

	amountBigInt, err := item[1].TryInteger()
	if err != nil {
		return fmt.Errorf("failed to decode royalty amount: %w", err)
	}
	if amountBigInt.Sign() < 0 {
		return errors.New("negative royalty amount")
	}
	amount := big.NewInt(0).Set(amountBigInt)
	r.Amount = amount
	r.Address = recipient
	return nil
}

// RoyaltiesTransferredEventsFromApplicationLog retrieves all emitted
// RoyaltiesTransferredEvents from the provided [result.ApplicationLog].
func RoyaltiesTransferredEventsFromApplicationLog(log *result.ApplicationLog) ([]*RoyaltiesTransferredEvent, error) {
	if log == nil {
		return nil, errors.New("nil application log")
	}
	var res []*RoyaltiesTransferredEvent
	for i, ex := range log.Executions {
		for j, e := range ex.Events {
			if e.Name != "RoyaltiesTransferred" {
				continue
			}
			event := new(RoyaltiesTransferredEvent)
			err := event.FromStackItem(e.Item)
			if err != nil {
				return nil, fmt.Errorf("failed to decode event from stackitem (event #%d, execution #%d): %w", j, i, err)
			}
			res = append(res, event)
		}
	}
	return res, nil
}

// FromStackItem converts a stack item into a RoyaltiesTransferredEvent struct.
func (e *RoyaltiesTransferredEvent) FromStackItem(item *stackitem.Array) error {
	if item == nil {
		return errors.New("nil item")
	}
	arr, ok := item.Value().([]stackitem.Item)
	if !ok || len(arr) != 5 {
		return errors.New("invalid event structure: expected array of 5 items")
	}

	b, err := arr[0].TryBytes()
	if err != nil {
		return fmt.Errorf("failed to decode RoyaltyToken: %w", err)
	}
	e.RoyaltyToken, err = util.Uint160DecodeBytesBE(b)
	if err != nil {
		return fmt.Errorf("invalid RoyaltyToken: %w", err)
	}

	b, err = arr[1].TryBytes()
	if err != nil {
		return fmt.Errorf("failed to decode RoyaltyRecipient: %w", err)
	}
	e.RoyaltyRecipient, err = util.Uint160DecodeBytesBE(b)
	if err != nil {
		return fmt.Errorf("invalid RoyaltyRecipient: %w", err)
	}

	b, err = arr[2].TryBytes()
	if err != nil {
		return fmt.Errorf("failed to decode Buyer: %w", err)
	}
	e.Buyer, err = util.Uint160DecodeBytesBE(b)
	if err != nil {
		return fmt.Errorf("invalid Buyer: %w", err)
	}

	e.TokenID, err = arr[3].TryBytes()
	if err != nil {
		return fmt.Errorf("failed to decode TokenID: %w", err)
	}

	e.Amount, err = arr[4].TryInteger()
	if err != nil {
		return fmt.Errorf("failed to decode Amount: %w", err)
	}

	return nil
}
