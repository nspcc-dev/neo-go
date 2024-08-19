// Package nep11 provides RPC wrappers for NEP-11 contracts, including support for NEP-24 NFT royalties.
package nep11

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/unwrap"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// RoyaltyInfoDetail contains information about the recipient and the royalty amount.
type RoyaltyInfoDetail struct {
	RoyaltyRecipient util.Uint160
	RoyaltyAmount    *big.Int
}

// RoyaltiesTransferredEvent represents a RoyaltiesTransferred event as defined in the NEP-24 standard.
type RoyaltiesTransferredEvent struct {
	RoyaltyToken     util.Uint160
	RoyaltyRecipient util.Uint160
	Buyer            util.Uint160
	TokenID          []byte
	Amount           *big.Int
}

// RoyaltyReader is an interface for contracts implementing NEP-24 royalties.
type RoyaltyReader struct {
	BaseReader
}

// RoyaltyWriter is an interface for state-changing methods related to NEP-24 royalties.
type RoyaltyWriter struct {
	BaseWriter
}

// Royalty is a full reader and writer interface for NEP-24 royalties.
type Royalty struct {
	RoyaltyReader
	RoyaltyWriter
}

// NewRoyaltyReader creates an instance of RoyaltyReader for a contract with the given hash using the given invoker.
func NewRoyaltyReader(invoker Invoker, hash util.Uint160) *RoyaltyReader {
	return &RoyaltyReader{*NewBaseReader(invoker, hash)}
}

// NewRoyalty creates an instance of Royalty for a contract with the given hash using the given actor.
func NewRoyalty(actor Actor, hash util.Uint160) *Royalty {
	return &Royalty{*NewRoyaltyReader(actor, hash), RoyaltyWriter{BaseWriter{hash, actor}}}
}

// RoyaltyInfo retrieves the royalty information for a given token ID, including the recipient(s) and amount(s).
func (r *RoyaltyReader) RoyaltyInfo(tokenID []byte, royaltyToken util.Uint160, salePrice *big.Int) ([]RoyaltyInfoDetail, error) {
	items, err := unwrap.Array(r.invoker.Call(r.hash, "RoyaltyInfo", tokenID, royaltyToken, salePrice))
	if err != nil {
		return nil, err
	}

	royaltyDetail, err := itemToRoyaltyInfoDetail(items)
	if err != nil {
		return nil, fmt.Errorf("failed to decode royalty detail: %w", err)
	}

	return []RoyaltyInfoDetail{*royaltyDetail}, nil
}

// itemToRoyaltyInfoDetail converts an array of stack items into a RoyaltyInfoDetail struct.
func itemToRoyaltyInfoDetail(items []stackitem.Item) (*RoyaltyInfoDetail, error) {
	if len(items) != 2 {
		return nil, fmt.Errorf("invalid structure: expected 2 items, got %d", len(items))
	}

	recipientBytes, err := items[0].TryBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to decode RoyaltyRecipient: %w", err)
	}

	// Validate recipient byte size (should be 20 bytes for Uint160)
	if len(recipientBytes) != 20 {
		return nil, fmt.Errorf("invalid RoyaltyRecipient: expected byte size of 20, got %d", len(recipientBytes))
	}

	recipient, err := util.Uint160DecodeBytesBE(recipientBytes)
	if err != nil {
		return nil, fmt.Errorf("invalid RoyaltyRecipient: %w", err)
	}

	amountBigInt, err := items[1].TryInteger()
	if err != nil {
		return nil, fmt.Errorf("failed to decode RoyaltyAmount: %w", err)
	}
	amount := big.NewInt(0).Set(amountBigInt)

	return &RoyaltyInfoDetail{
		RoyaltyRecipient: recipient,
		RoyaltyAmount:    amount,
	}, nil
}

// RoyaltiesTransferredEventsFromApplicationLog retrieves all emitted RoyaltiesTransferredEvents from the provided [result.ApplicationLog].
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
	if _, ok := arr[4].Value().(*big.Int); !ok {
		return fmt.Errorf("invalid type for Amount: expected Integer, got %T", arr[4].Value())
	}
	e.Amount, err = arr[4].TryInteger()
	if err != nil {
		return fmt.Errorf("failed to decode Amount: %w", err)
	}

	return nil
}
