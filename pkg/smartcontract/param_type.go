package smartcontract

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// ParamType represents the Type of the smart contract parameter.
type ParamType int

// A list of supported smart contract parameter types.
const (
	UnknownType          ParamType = -1
	AnyType              ParamType = 0x00
	BoolType             ParamType = 0x10
	IntegerType          ParamType = 0x11
	ByteArrayType        ParamType = 0x12
	StringType           ParamType = 0x13
	Hash160Type          ParamType = 0x14
	Hash256Type          ParamType = 0x15
	PublicKeyType        ParamType = 0x16
	SignatureType        ParamType = 0x17
	ArrayType            ParamType = 0x20
	MapType              ParamType = 0x22
	InteropInterfaceType ParamType = 0x30
	VoidType             ParamType = 0xff
)

// validParamTypes contains a map of known ParamTypes
var validParamTypes = map[ParamType]bool{
	UnknownType:          true,
	AnyType:              true,
	BoolType:             true,
	IntegerType:          true,
	ByteArrayType:        true,
	StringType:           true,
	Hash160Type:          true,
	Hash256Type:          true,
	PublicKeyType:        true,
	SignatureType:        true,
	ArrayType:            true,
	MapType:              true,
	InteropInterfaceType: true,
	VoidType:             true,
}

// String implements the stringer interface.
func (pt ParamType) String() string {
	switch pt {
	case SignatureType:
		return "Signature"
	case BoolType:
		return "Boolean"
	case IntegerType:
		return "Integer"
	case Hash160Type:
		return "Hash160"
	case Hash256Type:
		return "Hash256"
	case ByteArrayType:
		return "ByteArray"
	case PublicKeyType:
		return "PublicKey"
	case StringType:
		return "String"
	case ArrayType:
		return "Array"
	case MapType:
		return "Map"
	case InteropInterfaceType:
		return "InteropInterface"
	case VoidType:
		return "Void"
	case AnyType:
		return "Any"
	default:
		return ""
	}
}

// MarshalJSON implements the json.Marshaler interface.
func (pt ParamType) MarshalJSON() ([]byte, error) {
	return []byte(`"` + pt.String() + `"`), nil
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (pt *ParamType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	p, err := ParseParamType(s)
	if err != nil {
		return err
	}

	*pt = p
	return nil
}

// MarshalYAML implements the YAML Marshaler interface.
func (pt ParamType) MarshalYAML() (interface{}, error) {
	return pt.String(), nil
}

// UnmarshalYAML implements the YAML Unmarshaler interface.
func (pt *ParamType) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var name string

	err := unmarshal(&name)
	if err != nil {
		return err
	}
	*pt, err = ParseParamType(name)
	return err
}

// EncodeBinary implements io.Serializable interface.
func (pt ParamType) EncodeBinary(w *io.BinWriter) {
	w.WriteB(byte(pt))
}

// DecodeBinary implements io.Serializable interface.
func (pt *ParamType) DecodeBinary(r *io.BinReader) {
	*pt = ParamType(r.ReadB())
}

// ParseParamType is a user-friendly string to ParamType converter, it's
// case-insensitive and makes the following conversions:
//     signature -> SignatureType
//     bool, boolean -> BoolType
//     int, integer -> IntegerType
//     hash160 -> Hash160Type
//     hash256 -> Hash256Type
//     bytes, bytearray -> ByteArrayType
//     key, publickey -> PublicKeyType
//     string -> StringType
//     array, struct -> ArrayType
//     map -> MapType
//     interopinterface -> InteropInterfaceType
//     void -> VoidType
// anything else generates an error.
func ParseParamType(typ string) (ParamType, error) {
	switch strings.ToLower(typ) {
	case "signature":
		return SignatureType, nil
	case "bool", "boolean":
		return BoolType, nil
	case "int", "integer":
		return IntegerType, nil
	case "hash160":
		return Hash160Type, nil
	case "hash256":
		return Hash256Type, nil
	case "bytes", "bytearray", "bytestring":
		return ByteArrayType, nil
	case "key", "publickey":
		return PublicKeyType, nil
	case "string":
		return StringType, nil
	case "array", "struct":
		return ArrayType, nil
	case "map":
		return MapType, nil
	case "interopinterface":
		return InteropInterfaceType, nil
	case "void":
		return VoidType, nil
	case "any":
		return AnyType, nil
	default:
		return UnknownType, fmt.Errorf("bad parameter type: %s", typ)
	}
}

// adjustValToType is a value type-checker and converter.
func adjustValToType(typ ParamType, val string) (interface{}, error) {
	switch typ {
	case SignatureType:
		b, err := hex.DecodeString(val)
		if err != nil {
			return nil, err
		}
		if len(b) != 64 {
			return nil, errors.New("not a signature")
		}
		return b, nil
	case BoolType:
		switch val {
		case "true":
			return true, nil
		case "false":
			return false, nil
		default:
			return nil, errors.New("invalid boolean value")
		}
	case IntegerType:
		return strconv.ParseInt(val, 10, 64)
	case Hash160Type:
		u, err := address.StringToUint160(val)
		if err == nil {
			return u, nil
		}
		u, err = util.Uint160DecodeStringLE(val)
		if err != nil {
			return nil, err
		}
		return u, nil
	case Hash256Type:
		u, err := util.Uint256DecodeStringLE(val)
		if err != nil {
			return nil, err
		}
		return u, nil
	case ByteArrayType:
		b, err := hex.DecodeString(val)
		if err != nil {
			return nil, err
		}
		return b, nil
	case PublicKeyType:
		pub, err := keys.NewPublicKeyFromString(val)
		if err != nil {
			return nil, err
		}
		return pub.Bytes(), nil
	case StringType:
		return val, nil
	default:
		return nil, errors.New("unsupported parameter type")
	}
}

// inferParamType tries to infer the value type from its contents. It returns
// IntegerType for anything that looks like decimal integer (can be converted
// with strconv.Atoi), BoolType for true and false values, Hash160Type for
// addresses and hex strings encoding 20 bytes long values, PublicKeyType for
// valid hex-encoded public keys, Hash256Type for hex-encoded 32 bytes values,
// SignatureType for hex-encoded 64 bytes values, ByteArrayType for any other
// valid hex-encoded values and StringType for anything else.
func inferParamType(val string) ParamType {
	var err error

	_, err = strconv.Atoi(val)
	if err == nil {
		return IntegerType
	}

	if val == "true" || val == "false" {
		return BoolType
	}

	_, err = address.StringToUint160(val)
	if err == nil {
		return Hash160Type
	}

	_, err = keys.NewPublicKeyFromString(val)
	if err == nil {
		return PublicKeyType
	}

	unhexed, err := hex.DecodeString(val)
	if err == nil {
		switch len(unhexed) {
		case 20:
			return Hash160Type
		case 32:
			return Hash256Type
		case 64:
			return SignatureType
		default:
			return ByteArrayType
		}
	}
	// Anything can be a string.
	return StringType
}

// ConvertToParamType converts provided value to parameter type if it's a valid type.
func ConvertToParamType(val int) (ParamType, error) {
	if validParamTypes[ParamType(val)] {
		return ParamType(val), nil
	}
	return UnknownType, errors.New("unknown parameter type")
}
