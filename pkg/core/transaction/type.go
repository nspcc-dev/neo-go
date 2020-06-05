package transaction

import (
	"strings"

	"github.com/pkg/errors"
)

// TXType is the type of a transaction.
type TXType uint8

// Constants for all valid transaction types.
const (
	IssueType      TXType = 0x01
	RegisterType   TXType = 0x40
	InvocationType TXType = 0xd1
)

// String implements the stringer interface.
func (t TXType) String() string {
	switch t {
	case IssueType:
		return "IssueTransaction"
	case RegisterType:
		return "RegisterTransaction"
	case InvocationType:
		return "InvocationTransaction"
	default:
		return "UnknownTransaction"
	}
}

// MarshalJSON implements the json marshaller interface.
func (t TXType) MarshalJSON() ([]byte, error) {
	return []byte(`"` + t.String() + `"`), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (t *TXType) UnmarshalJSON(data []byte) error {
	l := len(data)
	if l < 2 || data[0] != '"' || data[l-1] != '"' {
		return errors.New("wrong format")
	}
	var err error
	*t, err = TXTypeFromString(string(data[1 : l-1]))
	return err
}

// TXTypeFromString searches for TXType by string name.
func TXTypeFromString(jsonString string) (TXType, error) {
	switch jsonString = strings.TrimSpace(jsonString); jsonString {
	case "IssueTransaction":
		return IssueType, nil
	case "RegisterTransaction":
		return RegisterType, nil
	case "InvocationTransaction":
		return InvocationType, nil
	default:
		return 0, errors.New("unknown state")
	}
}
