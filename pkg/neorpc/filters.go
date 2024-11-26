package neorpc

import (
	"errors"
	"fmt"
	"slices"

	"github.com/nspcc-dev/neo-go/pkg/core/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/core/mempoolevent"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/nspcc-dev/neo-go/pkg/vm/vmstate"
)

// MaxNotificationFilterParametersCount is a reasonable filter's parameter limit
// that does not allow attackers to increase node resources usage but that
// also should be enough for real applications.
const MaxNotificationFilterParametersCount = 16

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
	// be filtered by contract hash, by event name and/or by notification
	// parameters. Notification parameter filters will be applied in the order
	// corresponding to a produced notification's parameters. Not more than
	// [MaxNotificationFilterParametersCount] parameters are accepted (see also
	// [NotificationFilter.IsValid]). `Any`-typed parameter with zero value
	// allows any notification parameter. Supported parameter types:
	// - [smartcontract.AnyType]
	// - [smartcontract.BoolType]
	// - [smartcontract.IntegerType]
	// - [smartcontract.ByteArrayType]
	// - [smartcontract.StringType]
	// - [smartcontract.Hash160Type]
	// - [smartcontract.Hash256Type]
	// - [smartcontract.PublicKeyType]
	// - [smartcontract.SignatureType]
	// nil value treated as missing filter.
	NotificationFilter struct {
		Contract        *util.Uint160             `json:"contract,omitempty"`
		Name            *string                   `json:"name,omitempty"`
		Parameters      []smartcontract.Parameter `json:"parameters,omitempty"`
		parametersCache []stackitem.Item
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

// Copy creates a deep copy of the NotificationFilter. It handles nil
// NotificationFilter correctly.
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
	if len(f.Parameters) != 0 {
		res.Parameters = slices.Clone(f.Parameters)
	}
	return res
}

// ParametersAsStackItems returns [stackitem.Item] version of [NotificationFilter.Parameters]
// according to [smartcontract.Parameter.ToStackItem]; Notice that the result is cached
// internally in [NotificationFilter] for efficiency, so once you call this method it will
// not change even if you change any structure fields. If you need to update parameters, use
// [NotificationFilter.Copy]. It mainly should be used by server code. Must not be used
// concurrently.
func (f *NotificationFilter) ParametersAsStackItems() ([]stackitem.Item, error) {
	if len(f.Parameters) == 0 {
		return nil, nil
	}
	if f.parametersCache == nil {
		f.parametersCache = make([]stackitem.Item, 0, len(f.Parameters))
		for i, p := range f.Parameters {
			si, err := p.ToStackItem()
			if err != nil {
				f.parametersCache = nil
				return nil, fmt.Errorf("converting %d parameter to stack item: %w", i, err)
			}
			f.parametersCache = append(f.parametersCache, si)
		}
	}

	return f.parametersCache, nil
}

// IsValid implements SubscriptionFilter interface.
func (f NotificationFilter) IsValid() error {
	if f.Name != nil && len(*f.Name) > runtime.MaxEventNameLen {
		return fmt.Errorf("%w: NotificationFilter name parameter must be less than %d", ErrInvalidSubscriptionFilter, runtime.MaxEventNameLen)
	}
	l := len(f.Parameters)
	noopFilter := l > 0
	if l > 0 {
		if l > MaxNotificationFilterParametersCount {
			return fmt.Errorf("%w: NotificationFilter's parameters number exceeded: %d > %d", ErrInvalidSubscriptionFilter, l, MaxNotificationFilterParametersCount)
		}
		for i, parameter := range f.Parameters {
			switch parameter.Type {
			case smartcontract.BoolType,
				smartcontract.IntegerType,
				smartcontract.ByteArrayType,
				smartcontract.StringType,
				smartcontract.Hash160Type,
				smartcontract.Hash256Type,
				smartcontract.PublicKeyType,
				smartcontract.SignatureType:
				noopFilter = false
			case smartcontract.AnyType:
			default:
				return fmt.Errorf("%w: NotificationFilter type parameter %d is unsupported: %s", ErrInvalidSubscriptionFilter, i, parameter.Type)
			}
			if _, err := parameter.ToStackItem(); err != nil {
				return fmt.Errorf("%w: NotificationFilter %d filter parameter does not correspond to any stack item: %w", ErrInvalidSubscriptionFilter, i, err)
			}
		}
	}
	if noopFilter {
		return fmt.Errorf("%w: NotificationFilter cannot have all parameters of type %s", ErrInvalidSubscriptionFilter, smartcontract.AnyType)
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
