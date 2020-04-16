package transaction

import (
	"strings"

	"github.com/pkg/errors"
)

// TXType is the type of a transaction.
type TXType uint8

// Constants for all valid transaction types.
const (
	MinerType      TXType = 0x00
	IssueType      TXType = 0x01
	ClaimType      TXType = 0x02
	EnrollmentType TXType = 0x20
	RegisterType   TXType = 0x40
	ContractType   TXType = 0x80
	StateType      TXType = 0x90
	InvocationType TXType = 0xd1
)

// String implements the stringer interface.
func (t TXType) String() string {
	switch t {
	case MinerType:
		return "MinerTransaction"
	case IssueType:
		return "IssueTransaction"
	case ClaimType:
		return "ClaimTransaction"
	case EnrollmentType:
		return "EnrollmentTransaction"
	case RegisterType:
		return "RegisterTransaction"
	case ContractType:
		return "ContractTransaction"
	case StateType:
		return "StateTransaction"
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
	case "MinerTransaction":
		return MinerType, nil
	case "IssueTransaction":
		return IssueType, nil
	case "ClaimTransaction":
		return ClaimType, nil
	case "EnrollmentTransaction":
		return EnrollmentType, nil
	case "RegisterTransaction":
		return RegisterType, nil
	case "ContractTransaction":
		return ContractType, nil
	case "StateTransaction":
		return StateType, nil
	case "InvocationTransaction":
		return InvocationType, nil
	default:
		return 0, errors.New("unknown state")
	}
}
