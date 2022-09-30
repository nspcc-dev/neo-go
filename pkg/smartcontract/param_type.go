package smartcontract

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
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

// fileBytesParamType is a string representation of `filebytes` parameter type used in cli.
const fileBytesParamType string = "filebytes"

// validParamTypes contains a map of known ParamTypes.
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

// UnmarshalJSON implements the json.Unmarshaler interface.
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

// EncodeBinary implements the io.Serializable interface.
func (pt ParamType) EncodeBinary(w *io.BinWriter) {
	w.WriteB(byte(pt))
}

// DecodeBinary implements the io.Serializable interface.
func (pt *ParamType) DecodeBinary(r *io.BinReader) {
	*pt = ParamType(r.ReadB())
}

// EncodeDefaultValue writes a script to push the default parameter value onto
// the evaluation stack into the given writer. It's mostly useful for constructing
// dummy invocation scripts when parameter types are known, but they can't be
// filled in. A best effort approach is used, it can't be perfect since for many
// types the exact values can be arbitrarily long, but it tries to do something
// reasonable in each case. For signatures, strings, arrays and "any" type a 64-byte
// zero-filled value is used, hash160 and hash256 use appropriately sized values,
// public key is represented by 33-byte value while 32 bytes are used for integer
// and a simple push+convert is used for boolean. Other types produce no code at all.
func (pt ParamType) EncodeDefaultValue(w *io.BinWriter) {
	var b [64]byte

	switch pt {
	case AnyType, SignatureType, StringType, ByteArrayType:
		emit.Bytes(w, b[:])
	case BoolType:
		emit.Bool(w, true)
	case IntegerType:
		emit.Instruction(w, opcode.PUSHINT256, b[:32])
	case Hash160Type:
		emit.Bytes(w, b[:20])
	case Hash256Type:
		emit.Bytes(w, b[:32])
	case PublicKeyType:
		emit.Bytes(w, b[:33])
	case ArrayType, MapType, InteropInterfaceType, VoidType:
	}
}

// ParseParamType is a user-friendly string to ParamType converter, it's
// case-insensitive and makes the following conversions:
//
//	signature -> SignatureType
//	bool, boolean -> BoolType
//	int, integer -> IntegerType
//	hash160 -> Hash160Type
//	hash256 -> Hash256Type
//	bytes, bytearray, filebytes -> ByteArrayType
//	key, publickey -> PublicKeyType
//	string -> StringType
//	array, struct -> ArrayType
//	map -> MapType
//	interopinterface -> InteropInterfaceType
//	void -> VoidType
//
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
	case "bytes", "bytearray", "bytestring", fileBytesParamType:
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
		bi, ok := new(big.Int).SetString(val, 10)
		if !ok || stackitem.CheckIntegerSize(bi) != nil {
			return nil, errors.New("invalid integer value")
		}
		return bi, nil
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
		return hex.DecodeString(val)
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
// IntegerType for anything that looks like a decimal integer (can be converted
// with strconv.Atoi), BoolType for true and false values, Hash160Type for
// addresses and hex strings encoding 20 bytes long values, PublicKeyType for
// valid hex-encoded public keys, Hash256Type for hex-encoded 32 bytes values,
// SignatureType for hex-encoded 64 bytes values, ByteArrayType for any other
// valid hex-encoded values and StringType for anything else.
func inferParamType(val string) ParamType {
	var err error

	bi, ok := new(big.Int).SetString(val, 10)
	if ok && stackitem.CheckIntegerSize(bi) == nil {
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

// ConvertToParamType converts the provided value to the parameter type if it's a valid type.
func ConvertToParamType(val int) (ParamType, error) {
	if validParamTypes[ParamType(val)] {
		return ParamType(val), nil
	}
	return UnknownType, errors.New("unknown parameter type")
}

// ConvertToStackitemType converts ParamType to corresponding Stackitem.Type.
func (pt ParamType) ConvertToStackitemType() stackitem.Type {
	switch pt {
	case SignatureType:
		return stackitem.ByteArrayT
	case BoolType:
		return stackitem.BooleanT
	case IntegerType:
		return stackitem.IntegerT
	case Hash160Type:
		return stackitem.ByteArrayT
	case Hash256Type:
		return stackitem.ByteArrayT
	case ByteArrayType:
		return stackitem.ByteArrayT
	case PublicKeyType:
		return stackitem.ByteArrayT
	case StringType:
		// Do not use BufferT to match System.Runtime.Notify conversion rules.
		return stackitem.ByteArrayT
	case ArrayType:
		return stackitem.ArrayT
	case MapType:
		return stackitem.MapT
	case InteropInterfaceType:
		return stackitem.InteropT
	case VoidType:
		return stackitem.AnyT
	case AnyType:
		return stackitem.AnyT
	default:
		panic(fmt.Sprintf("unknown param type %d", pt))
	}
}
