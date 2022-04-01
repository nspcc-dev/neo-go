package smartcontract

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"math/bits"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Parameter represents a smart contract parameter.
type Parameter struct {
	// Type of the parameter.
	Type ParamType `json:"type"`
	// The actual value of the parameter.
	Value interface{} `json:"value"`
}

// ParameterPair represents key-value pair, a slice of which is stored in
// MapType Parameter.
type ParameterPair struct {
	Key   Parameter `json:"key"`
	Value Parameter `json:"value"`
}

// NewParameter returns a Parameter with proper initialized Value
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

// MarshalJSON implements Marshaler interface.
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

// UnmarshalJSON implements Unmarshaler interface.
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

// Params is an array of Parameter (TODO: drop it?).
type Params []Parameter

// TryParseArray converts an array of Parameter into an array of more appropriate things.
func (p Params) TryParseArray(vals ...interface{}) error {
	var (
		err error
		i   int
		par Parameter
	)
	if len(p) != len(vals) {
		return errors.New("receiver array doesn't fit the Params length")
	}
	for i, par = range p {
		if err = par.TryParse(vals[i]); err != nil {
			return err
		}
	}
	return nil
}

// TryParse converts one Parameter into something more appropriate.
func (p Parameter) TryParse(dest interface{}) error {
	var (
		err  error
		ok   bool
		data []byte
	)
	switch p.Type {
	case ByteArrayType:
		if data, ok = p.Value.([]byte); !ok {
			return fmt.Errorf("failed to cast %s to []byte", p.Value)
		}
		switch dest := dest.(type) {
		case *util.Uint160:
			if *dest, err = util.Uint160DecodeBytesBE(data); err != nil {
				return err
			}
			return nil
		case *[]byte:
			*dest = data
			return nil
		case *util.Uint256:
			if *dest, err = util.Uint256DecodeBytesLE(data); err != nil {
				return err
			}
			return nil
		case **big.Int:
			*dest = bigint.FromBytes(data)
			return nil
		case *int64, *int32, *int16, *int8, *int, *uint64, *uint32, *uint16, *uint8, *uint:
			var size int
			switch dest.(type) {
			case *int64, *uint64:
				size = 64
			case *int32, *uint32:
				size = 32
			case *int16, *uint16:
				size = 16
			case *int8, *uint8:
				size = 8
			case *int, *uint:
				size = bits.UintSize
			}

			i, err := bytesToUint64(data, size)
			if err != nil {
				return err
			}

			switch dest := dest.(type) {
			case *int64:
				*dest = int64(i)
			case *int32:
				*dest = int32(i)
			case *int16:
				*dest = int16(i)
			case *int8:
				*dest = int8(i)
			case *int:
				*dest = int(i)
			case *uint64:
				*dest = i
			case *uint32:
				*dest = uint32(i)
			case *uint16:
				*dest = uint16(i)
			case *uint8:
				*dest = uint8(i)
			case *uint:
				*dest = uint(i)
			}
		case *string:
			*dest = string(data)
			return nil
		default:
			return fmt.Errorf("cannot cast param of type %s to type %s", p.Type, dest)
		}
	default:
		return errors.New("cannot define param type")
	}
	return nil
}

func bytesToUint64(b []byte, size int) (uint64, error) {
	var length = size / 8
	if len(b) > length {
		return 0, fmt.Errorf("input doesn't fit into %d bits", size)
	}
	if len(b) < length {
		data := make([]byte, length)
		copy(data, b)
		return binary.LittleEndian.Uint64(data), nil
	}
	return binary.LittleEndian.Uint64(b), nil
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
				return nil, fmt.Errorf("unsupported parameter type %s", res.Type)
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

// ExpandParameterToEmitable converts parameter to a type which can be handled as
// an array item by emit.Array. It correlates with the way RPC server handles
// FuncParams for invoke* calls inside the request.ExpandArrayIntoScript function.
func ExpandParameterToEmitable(param Parameter) (interface{}, error) {
	var err error
	switch t := param.Type; t {
	case PublicKeyType:
		return param.Value.(*keys.PublicKey).Bytes(), nil
	case ArrayType:
		arr := param.Value.([]Parameter)
		res := make([]interface{}, len(arr))
		for i := range arr {
			res[i], err = ExpandParameterToEmitable(arr[i])
			if err != nil {
				return nil, err
			}
		}
		return res, nil
	case MapType, InteropInterfaceType, UnknownType, AnyType, VoidType:
		return nil, fmt.Errorf("unsupported parameter type: %s", t.String())
	default:
		return param.Value, nil
	}
}
