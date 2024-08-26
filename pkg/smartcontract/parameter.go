package smartcontract

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Parameter represents a smart contract parameter.
type Parameter struct {
	// Type of the parameter.
	Type ParamType `json:"type"`
	// The actual value of the parameter.
	Value any `json:"value"`
}

// Convertible is something that can be converted to Parameter.
type Convertible interface {
	ToSCParameter() (Parameter, error)
}

// ParameterPair represents a key-value pair, a slice of which is stored in
// MapType Parameter.
type ParameterPair struct {
	Key   Parameter `json:"key"`
	Value Parameter `json:"value"`
}

// NewParameter returns a Parameter with a proper initialized Value
// of the given ParamType.
func NewParameter(t ParamType) Parameter {
	return Parameter{
		Type:  t,
		Value: nil,
	}
}

type rawParameter struct {
	Type  ParamType       `json:"type"`
	Value json.RawMessage `json:"value,omitempty"`
}

// MarshalJSON implements the Marshaler interface.
func (p Parameter) MarshalJSON() ([]byte, error) {
	var (
		resultRawValue json.RawMessage
		resultErr      error
	)
	if p.Value == nil {
		if _, ok := validParamTypes[p.Type]; ok && p.Type != UnknownType {
			return json.Marshal(rawParameter{Type: p.Type})
		}
		return nil, fmt.Errorf("can't marshal %s", p.Type)
	}
	switch p.Type {
	case BoolType, StringType, Hash160Type, Hash256Type:
		resultRawValue, resultErr = json.Marshal(p.Value)
	case IntegerType:
		val, ok := p.Value.(*big.Int)
		if !ok {
			resultErr = errors.New("invalid integer value")
			break
		}
		resultRawValue = json.RawMessage(`"` + val.String() + `"`)
	case PublicKeyType, ByteArrayType, SignatureType:
		if p.Type == PublicKeyType {
			resultRawValue, resultErr = json.Marshal(hex.EncodeToString(p.Value.([]byte)))
		} else {
			resultRawValue, resultErr = json.Marshal(base64.StdEncoding.EncodeToString(p.Value.([]byte)))
		}
	case ArrayType:
		var value = p.Value.([]Parameter)
		if value == nil {
			resultRawValue, resultErr = json.Marshal([]Parameter{})
		} else {
			resultRawValue, resultErr = json.Marshal(value)
		}
	case MapType:
		ppair := p.Value.([]ParameterPair)
		resultRawValue, resultErr = json.Marshal(ppair)
	case InteropInterfaceType, AnyType:
		resultRawValue = nil
	default:
		resultErr = fmt.Errorf("can't marshal %s", p.Type)
	}
	if resultErr != nil {
		return nil, resultErr
	}
	return json.Marshal(rawParameter{
		Type:  p.Type,
		Value: resultRawValue,
	})
}

// UnmarshalJSON implements the Unmarshaler interface.
func (p *Parameter) UnmarshalJSON(data []byte) (err error) {
	var (
		r       rawParameter
		i       int64
		s       string
		b       []byte
		boolean bool
	)
	if err = json.Unmarshal(data, &r); err != nil {
		return
	}
	p.Type = r.Type
	p.Value = nil
	if len(r.Value) == 0 || bytes.Equal(r.Value, []byte("null")) {
		return
	}
	switch r.Type {
	case BoolType:
		if err = json.Unmarshal(r.Value, &boolean); err != nil {
			return
		}
		p.Value = boolean
	case ByteArrayType, PublicKeyType, SignatureType:
		if err = json.Unmarshal(r.Value, &s); err != nil {
			return
		}
		if r.Type == PublicKeyType {
			b, err = hex.DecodeString(s)
		} else {
			b, err = base64.StdEncoding.DecodeString(s)
		}
		if err != nil {
			return
		}
		p.Value = b
	case StringType:
		if err = json.Unmarshal(r.Value, &s); err != nil {
			return
		}
		p.Value = s
	case IntegerType:
		if err = json.Unmarshal(r.Value, &i); err == nil {
			p.Value = big.NewInt(i)
			return
		}
		// sometimes integer comes as string
		if jErr := json.Unmarshal(r.Value, &s); jErr != nil {
			return jErr
		}
		bi, ok := new(big.Int).SetString(s, 10)
		if !ok {
			// In this case previous err should mean string contains non-digit characters.
			return err
		}
		err = stackitem.CheckIntegerSize(bi)
		if err == nil {
			p.Value = bi
		}
	case ArrayType:
		// https://github.com/neo-project/neo/blob/3d59ecca5a8deb057bdad94b3028a6d5e25ac088/neo/Network/RPC/RpcServer.cs#L67
		var rs []Parameter
		if err = json.Unmarshal(r.Value, &rs); err != nil {
			return
		}
		p.Value = rs
	case MapType:
		var ppair []ParameterPair
		if err = json.Unmarshal(r.Value, &ppair); err != nil {
			return
		}
		p.Value = ppair
	case Hash160Type:
		var h util.Uint160
		if err = json.Unmarshal(r.Value, &h); err != nil {
			return
		}
		p.Value = h
	case Hash256Type:
		var h util.Uint256
		if err = json.Unmarshal(r.Value, &h); err != nil {
			return
		}
		p.Value = h
	case InteropInterfaceType, AnyType:
		// stub, ignore value, it can only be null
		p.Value = nil
	default:
		return fmt.Errorf("can't unmarshal %s", p.Type)
	}
	return
}

// NewParameterFromString returns a new Parameter initialized from the given
// string in neo-go-specific format. It is intended to be used in user-facing
// interfaces and has some heuristics in it to simplify parameter passing. The exact
// syntax is documented in the cli documentation. [errors.ErrUnsupported] will be
// returned in case of unsupported parameter types.
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
		typStr  string
	)
	r = strings.NewReader(in)
	for char, _, err = r.ReadRune(); err == nil && char != utf8.RuneError; char, _, err = r.ReadRune() {
		if char == '\\' && !escaped {
			escaped = true
			continue
		}
		if char == ':' && !escaped && !hadType {
			typStr = buf.String()
			res.Type, err = ParseParamType(typStr)
			if err != nil {
				return nil, err
			}
			// We currently do not support following types:
			if res.Type == ArrayType || res.Type == MapType || res.Type == InteropInterfaceType || res.Type == VoidType {
				return nil, fmt.Errorf("%w: type %s", errors.ErrUnsupported, res.Type)
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
	if res.Type == ByteArrayType && typStr == fileBytesParamType {
		res.Value, err = os.ReadFile(val)
		if err != nil {
			return nil, fmt.Errorf("failed to read '%s' parameter from file '%s': %w", fileBytesParamType, val, err)
		}
		return res, nil
	}
	res.Value, err = adjustValToType(res.Type, val)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// NewParameterFromValue infers Parameter type from the value given and adjusts
// the value if needed. It does not copy the value if it can avoid doing so. All
// regular integers, util.*, keys.PublicKey*, string and bool types are supported,
// slice of byte slices is accepted and converted as well. [errors.ErrUnsupported]
// will be returned for types that can't be used now.
func NewParameterFromValue(value any) (Parameter, error) {
	var result = Parameter{
		Value: value,
	}

	switch v := value.(type) {
	case []byte:
		result.Type = ByteArrayType
	case string:
		result.Type = StringType
	case bool:
		result.Type = BoolType
	case *big.Int:
		result.Type = IntegerType
	case int8:
		result.Type = IntegerType
		result.Value = big.NewInt(int64(v))
	case byte:
		result.Type = IntegerType
		result.Value = big.NewInt(int64(v))
	case int16:
		result.Type = IntegerType
		result.Value = big.NewInt(int64(v))
	case uint16:
		result.Type = IntegerType
		result.Value = big.NewInt(int64(v))
	case int32:
		result.Type = IntegerType
		result.Value = big.NewInt(int64(v))
	case uint32:
		result.Type = IntegerType
		result.Value = big.NewInt(int64(v))
	case int:
		result.Type = IntegerType
		result.Value = big.NewInt(int64(v))
	case uint:
		result.Type = IntegerType
		result.Value = new(big.Int).SetUint64(uint64(v))
	case int64:
		result.Type = IntegerType
		result.Value = big.NewInt(v)
	case uint64:
		result.Type = IntegerType
		result.Value = new(big.Int).SetUint64(v)
	case *Parameter:
		result = *v
	case Parameter:
		result = v
	case Convertible:
		var err error
		result, err = v.ToSCParameter()
		if err != nil {
			return result, fmt.Errorf("failed to convert smartcontract.Convertible (%T) to Parameter: %w", v, err)
		}
	case util.Uint160:
		result.Type = Hash160Type
	case util.Uint256:
		result.Type = Hash256Type
	case *util.Uint160:
		if v != nil {
			return NewParameterFromValue(*v)
		}
		result.Type = AnyType
		result.Value = nil
	case *util.Uint256:
		if v != nil {
			return NewParameterFromValue(*v)
		}
		result.Type = AnyType
		result.Value = nil
	case keys.PublicKey:
		return NewParameterFromValue(&v)
	case *keys.PublicKey:
		result.Type = PublicKeyType
		result.Value = v.Bytes()
	case [][]byte:
		arr := make([]Parameter, 0, len(v))
		for i := range v {
			// We know the type exactly, so error is not possible.
			elem, _ := NewParameterFromValue(v[i])
			arr = append(arr, elem)
		}
		result.Type = ArrayType
		result.Value = arr
	case []Parameter:
		result.Type = ArrayType
		result.Value = slices.Clone(v)
	case []*keys.PublicKey:
		return NewParameterFromValue(keys.PublicKeys(v))
	case keys.PublicKeys:
		arr := make([]Parameter, 0, len(v))
		for i := range v {
			// We know the type exactly, so error is not possible.
			elem, _ := NewParameterFromValue(v[i])
			arr = append(arr, elem)
		}
		result.Type = ArrayType
		result.Value = arr
	case []any:
		arr, err := NewParametersFromValues(v...)
		if err != nil {
			return result, err
		}
		result.Type = ArrayType
		result.Value = arr
	case nil:
		result.Type = AnyType
	default:
		return result, fmt.Errorf("%w: %T type", errors.ErrUnsupported, value)
	}

	return result, nil
}

// NewParametersFromValues is similar to NewParameterFromValue except that it
// works with multiple values and returns a simple slice of Parameter.
func NewParametersFromValues(values ...any) ([]Parameter, error) {
	res := make([]Parameter, 0, len(values))
	for i := range values {
		elem, err := NewParameterFromValue(values[i])
		if err != nil {
			return nil, err
		}
		res = append(res, elem)
	}
	return res, nil
}

// ExpandParameterToEmitable converts a parameter to a type which can be handled as
// an array item by emit.Array. It correlates with the way an RPC server handles
// FuncParams for invoke* calls inside the request.ExpandArrayIntoScript function.
// [errors.ErrUnsupported] is returned for unsupported types.
func ExpandParameterToEmitable(param Parameter) (any, error) {
	var err error
	switch t := param.Type; t {
	case ArrayType:
		arr := param.Value.([]Parameter)
		res := make([]any, len(arr))
		for i := range arr {
			res[i], err = ExpandParameterToEmitable(arr[i])
			if err != nil {
				return nil, err
			}
		}
		return res, nil
	case MapType, InteropInterfaceType, UnknownType, VoidType:
		return nil, fmt.Errorf("%w: %s type", errors.ErrUnsupported, t.String())
	default:
		return param.Value, nil
	}
}

// ToStackItem converts smartcontract parameter to stackitem.Item.
func (p *Parameter) ToStackItem() (stackitem.Item, error) {
	e, err := ExpandParameterToEmitable(*p)
	if err != nil {
		return nil, err
	}
	return stackitem.Make(e), nil
}
