package transaction

import (
	"errors"
	"fmt"
)

// AssetType represents a NEO asset type.
type AssetType uint8

// Valid asset types.
const (
	CreditFlag     AssetType = 0x40
	DutyFlag       AssetType = 0x80
	GoverningToken AssetType = 0x00
	UtilityToken   AssetType = 0x01
	Currency       AssetType = 0x08
	Share          AssetType = DutyFlag | 0x10
	Invoice        AssetType = DutyFlag | 0x18
	Token          AssetType = CreditFlag | 0x20
)

// String implements Stringer interface.
func (a AssetType) String() string {
	switch a {
	case CreditFlag:
		return "CreditFlag"
	case DutyFlag:
		return "DutyFlag"
	case GoverningToken:
		return "GoverningToken"
	case UtilityToken:
		return "UtilityToken"
	case Currency:
		return "Currency"
	case Share:
		return "Share"
	case Invoice:
		return "Invoice"
	case Token:
		return "Token"
	default:
		return fmt.Sprintf("Unknonwn (%d)", a)

	}
}

// MarshalJSON implements the json marshaller interface.
func (a AssetType) MarshalJSON() ([]byte, error) {
	return []byte(`"` + a.String() + `"`), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (a *AssetType) UnmarshalJSON(data []byte) error {
	l := len(data)
	if l < 2 || data[0] != '"' || data[l-1] != '"' {
		return errors.New("wrong format")
	}
	var err error
	*a, err = AssetTypeFromString(string(data[1 : l-1]))
	return err
}

// AssetTypeFromString converts string into AssetType.
func AssetTypeFromString(s string) (AssetType, error) {
	switch s {
	case "CreditFlag":
		return CreditFlag, nil
	case "DutyFlag":
		return DutyFlag, nil
	case "GoverningToken":
		return GoverningToken, nil
	case "UtilityToken":
		return UtilityToken, nil
	case "Currency":
		return Currency, nil
	case "Share":
		return Share, nil
	case "Invoice":
		return Invoice, nil
	case "Token":
		return Token, nil
	default:
		return 0, errors.New("bad asset type")
	}
}
