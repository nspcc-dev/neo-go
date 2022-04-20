package mempoolevent

import (
	"encoding/json"
	"errors"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
)

// Type represents mempool event type.
type Type byte

const (
	// TransactionAdded marks transaction addition mempool event.
	TransactionAdded Type = 0x01
	// TransactionRemoved marks transaction removal mempool event.
	TransactionRemoved Type = 0x02
)

// Event represents one of mempool events: transaction was added or removed from the mempool.
type Event struct {
	Type Type
	Tx   *transaction.Transaction
	Data interface{}
}

// String is a Stringer implementation.
func (e Type) String() string {
	switch e {
	case TransactionAdded:
		return "added"
	case TransactionRemoved:
		return "removed"
	default:
		return "unknown"
	}
}

// GetEventTypeFromString converts the input string into the Type if it's possible.
func GetEventTypeFromString(s string) (Type, error) {
	switch s {
	case "added":
		return TransactionAdded, nil
	case "removed":
		return TransactionRemoved, nil
	default:
		return 0, errors.New("invalid event type name")
	}
}

// MarshalJSON implements the json.Marshaler interface.
func (e Type) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.String())
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (e *Type) UnmarshalJSON(b []byte) error {
	var s string

	err := json.Unmarshal(b, &s)
	if err != nil {
		return err
	}
	id, err := GetEventTypeFromString(s)
	if err != nil {
		return err
	}
	*e = id
	return nil
}
