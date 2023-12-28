package neorpc

import (
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/core/mempoolevent"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/vmstate"
)

type (
	// BlockFilter is a wrapper structure for the block event filter. It allows
	// to filter blocks by primary index and/or by block index (allowing blocks
	// since/till the specified index inclusively). nil value treated as missing
	// filter.
	BlockFilter struct {
		Primary *byte   `json:"primary,omitempty"`
		Since   *uint32 `json:"since,omitempty"`
		Till    *uint32 `json:"till,omitempty"`
	}
	// TxFilter is a wrapper structure for the transaction event filter. It
	// allows to filter transactions by senders and/or signers. nil value treated
	// as missing filter.
	TxFilter struct {
		Sender *util.Uint160 `json:"sender,omitempty"`
		Signer *util.Uint160 `json:"signer,omitempty"`
	}
	// NotificationFilter is a wrapper structure representing a filter used for
	// notifications generated during transaction execution. Notifications can
	// be filtered by contract hash and/or by name. nil value treated as missing
	// filter.
	NotificationFilter struct {
		Contract *util.Uint160 `json:"contract,omitempty"`
		Name     *string       `json:"name,omitempty"`
	}
	// ExecutionFilter is a wrapper structure used for transaction and persisting
	// scripts execution events. It allows to choose failing or successful
	// transactions and persisting scripts based on their VM state and/or to
	// choose execution event with the specified container. nil value treated as
	// missing filter.
	ExecutionFilter struct {
		State     *string       `json:"state,omitempty"`
		Container *util.Uint256 `json:"container,omitempty"`
	}
	// NotaryRequestFilter is a wrapper structure used for notary request events.
	// It allows to choose notary request events with the specified request sender,
	// main transaction signer and/or type. nil value treated as missing filter.
	NotaryRequestFilter struct {
		Sender *util.Uint160      `json:"sender,omitempty"`
		Signer *util.Uint160      `json:"signer,omitempty"`
		Type   *mempoolevent.Type `json:"type,omitempty"`
	}
)

// SubscriptionFilter is an interface for all subscription filters.
type SubscriptionFilter interface {
	// IsValid checks whether the filter is valid and returns
	// a specific [ErrInvalidSubscriptionFilter] error if not.
	IsValid() error
}

// ErrInvalidSubscriptionFilter is returned when the subscription filter is invalid.
var ErrInvalidSubscriptionFilter = errors.New("invalid subscription filter")

// Copy creates a deep copy of the BlockFilter. It handles nil BlockFilter correctly.
func (f *BlockFilter) Copy() *BlockFilter {
	if f == nil {
		return nil
	}
	var res = new(BlockFilter)
	if f.Primary != nil {
		res.Primary = new(byte)
		*res.Primary = *f.Primary
	}
	if f.Since != nil {
		res.Since = new(uint32)
		*res.Since = *f.Since
	}
	if f.Till != nil {
		res.Till = new(uint32)
		*res.Till = *f.Till
	}
	return res
}

// IsValid implements SubscriptionFilter interface.
func (f BlockFilter) IsValid() error {
	return nil
}

// Copy creates a deep copy of the TxFilter. It handles nil TxFilter correctly.
func (f *TxFilter) Copy() *TxFilter {
	if f == nil {
		return nil
	}
	var res = new(TxFilter)
	if f.Sender != nil {
		res.Sender = new(util.Uint160)
		*res.Sender = *f.Sender
	}
	if f.Signer != nil {
		res.Signer = new(util.Uint160)
		*res.Signer = *f.Signer
	}
	return res
}

// IsValid implements SubscriptionFilter interface.
func (f TxFilter) IsValid() error {
	return nil
}

// Copy creates a deep copy of the NotificationFilter. It handles nil NotificationFilter correctly.
func (f *NotificationFilter) Copy() *NotificationFilter {
	if f == nil {
		return nil
	}
	var res = new(NotificationFilter)
	if f.Contract != nil {
		res.Contract = new(util.Uint160)
		*res.Contract = *f.Contract
	}
	if f.Name != nil {
		res.Name = new(string)
		*res.Name = *f.Name
	}
	return res
}

// IsValid implements SubscriptionFilter interface.
func (f NotificationFilter) IsValid() error {
	if f.Name != nil && len(*f.Name) > runtime.MaxEventNameLen {
		return fmt.Errorf("%w: NotificationFilter name parameter must be less than %d", ErrInvalidSubscriptionFilter, runtime.MaxEventNameLen)
	}
	return nil
}

// Copy creates a deep copy of the ExecutionFilter. It handles nil ExecutionFilter correctly.
func (f *ExecutionFilter) Copy() *ExecutionFilter {
	if f == nil {
		return nil
	}
	var res = new(ExecutionFilter)
	if f.State != nil {
		res.State = new(string)
		*res.State = *f.State
	}
	if f.Container != nil {
		res.Container = new(util.Uint256)
		*res.Container = *f.Container
	}
	return res
}

// IsValid implements SubscriptionFilter interface.
func (f ExecutionFilter) IsValid() error {
	if f.State != nil {
		if *f.State != vmstate.Halt.String() && *f.State != vmstate.Fault.String() {
			return fmt.Errorf("%w: ExecutionFilter state parameter must be either %s or %s", ErrInvalidSubscriptionFilter, vmstate.Halt, vmstate.Fault)
		}
	}

	return nil
}

// Copy creates a deep copy of the NotaryRequestFilter. It handles nil NotaryRequestFilter correctly.
func (f *NotaryRequestFilter) Copy() *NotaryRequestFilter {
	if f == nil {
		return nil
	}
	var res = new(NotaryRequestFilter)
	if f.Sender != nil {
		res.Sender = new(util.Uint160)
		*res.Sender = *f.Sender
	}
	if f.Signer != nil {
		res.Signer = new(util.Uint160)
		*res.Signer = *f.Signer
	}
	if f.Type != nil {
		res.Type = new(mempoolevent.Type)
		*res.Type = *f.Type
	}
	return res
}

// IsValid implements SubscriptionFilter interface.
func (f NotaryRequestFilter) IsValid() error {
	return nil
}
