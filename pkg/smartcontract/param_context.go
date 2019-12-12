package smartcontract

import (
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/CityOfZion/neo-go/pkg/crypto"
	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// ParamType represents the Type of the contract parameter.
type ParamType byte

// A list of supported smart contract parameter types.
const (
	SignatureType ParamType = iota
	BoolType
	IntegerType
	Hash160Type
	Hash256Type
	ByteArrayType
	PublicKeyType
	StringType
	ArrayType
)

// PropertyState represents contract properties (flags).
type PropertyState byte

// List of supported properties.
const (
	HasStorage PropertyState = 1 << iota
	HasDynamicInvoke
	IsPayable
	NoProperties = 0
)

// Parameter represents a smart contract parameter.
type Parameter struct {
	// Type of the parameter.
	Type ParamType `json:"type"`
	// The actual value of the parameter.
	Value interface{} `json:"value"`
}

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
	default:
		return ""
	}
}

// MarshalJSON implements the json.Marshaler interface.
func (pt ParamType) MarshalJSON() ([]byte, error) {
	return []byte(`"` + pt.String() + `"`), nil
}

// EncodeBinary implements io.Serializable interface.
func (pt ParamType) EncodeBinary(w *io.BinWriter) {
	w.WriteByte(byte(pt))
}

// DecodeBinary implements io.Serializable interface.
func (pt *ParamType) DecodeBinary(r *io.BinReader) {
	*pt = ParamType(r.ReadByte())
}

// NewParameter returns a Parameter with proper initialized Value
// of the given ParamType.
func NewParameter(t ParamType) Parameter {
	return Parameter{
		Type:  t,
		Value: nil,
	}
}

// parseParamType is a user-friendly string to ParamType converter, it's
// case-insensitive and makes the following conversions:
//     signature -> SignatureType
//     bool -> BoolType
//     int -> IntegerType
//     hash160 -> Hash160Type
//     hash256 -> Hash256Type
//     bytes -> ByteArrayType
//     key -> PublicKeyType
//     string -> StringType
// anything else generates an error.
func parseParamType(typ string) (ParamType, error) {
	switch strings.ToLower(typ) {
	case "signature":
		return SignatureType, nil
	case "bool":
		return BoolType, nil
	case "int":
		return IntegerType, nil
	case "hash160":
		return Hash160Type, nil
	case "hash256":
		return Hash256Type, nil
	case "bytes":
		return ByteArrayType, nil
	case "key":
		return PublicKeyType, nil
	case "string":
		return StringType, nil
	default:
		// We deliberately don't support array here.
		return 0, errors.New("wrong or unsupported parameter type")
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
		return val, nil
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
		return strconv.Atoi(val)
	case Hash160Type:
		u, err := crypto.Uint160DecodeAddress(val)
		if err == nil {
			return hex.EncodeToString(u.BytesBE()), nil
		}
		b, err := hex.DecodeString(val)
		if err != nil {
			return nil, err
		}
		if len(b) != 20 {
			return nil, errors.New("not a hash160")
		}
		return val, nil
	case Hash256Type:
		b, err := hex.DecodeString(val)
		if err != nil {
			return nil, err
		}
		if len(b) != 32 {
			return nil, errors.New("not a hash256")
		}
		return val, nil
	case ByteArrayType:
		_, err := hex.DecodeString(val)
		if err != nil {
			return nil, err
		}
		return val, nil
	case PublicKeyType:
		_, err := keys.NewPublicKeyFromString(val)
		if err != nil {
			return nil, err
		}
		return val, nil
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

	_, err = crypto.Uint160DecodeAddress(val)
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

// NewParameterFromString returns a new Parameter initialized from the given
// string in neo-go-specific format. It is intended to be used in user-facing
// interfaces and has some heuristics in it to simplify parameter passing. Exact
// syntax is documented in the cli documentation.
func NewParameterFromString(in string) (*Parameter, error) {
	var (
		char    rune
		val     string
		err     error
		r       *strings.Reader
		buf     strings.Builder
		escaped bool
		hadType bool
		res     = &Parameter{}
	)
	r = strings.NewReader(in)
	for char, _, err = r.ReadRune(); err == nil && char != utf8.RuneError; char, _, err = r.ReadRune() {
		if char == '\\' && !escaped {
			escaped = true
			continue
		}
		if char == ':' && !escaped && !hadType {
			typStr := buf.String()
			res.Type, err = parseParamType(typStr)
			if err != nil {
				return nil, err
			}
			buf.Reset()
			hadType = true
			continue
		}
		escaped = false
		// We don't care about length and it never fails.
		_, _ = buf.WriteRune(char)
	}
	if char == utf8.RuneError {
		return nil, errors.New("bad UTF-8 string")
	}
	// The only other error `ReadRune` returns is io.EOF, which is fine and
	// expected, so we don't check err here.

	val = buf.String()
	if !hadType {
		res.Type = inferParamType(val)
	}
	res.Value, err = adjustValToType(res.Type, val)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// ContextItem represents a transaction context item.
type ContextItem struct {
	Script     util.Uint160
	Parameters []Parameter
	Signatures []Signature
}

// Signature represents a transaction signature.
type Signature struct {
	Data      []byte
	PublicKey []byte
}

// ParameterContext holds the parameter context.
type ParameterContext struct{}
