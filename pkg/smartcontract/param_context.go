package smartcontract

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math/bits"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/pkg/errors"
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
	Value json.RawMessage `json:"value"`
}

type keyValuePair struct {
	Key   rawParameter `json:"key"`
	Value rawParameter `json:"value"`
}

type rawKeyValuePair struct {
	Key   json.RawMessage `json:"key"`
	Value json.RawMessage `json:"value"`
}

// MarshalJSON implements Marshaler interface.
func (p *Parameter) MarshalJSON() ([]byte, error) {
	var (
		resultRawValue json.RawMessage
		resultErr            error
	)
	switch p.Type {
	case BoolType, IntegerType, StringType, Hash256Type, Hash160Type:
		resultRawValue, resultErr = json.Marshal(p.Value)
	case PublicKeyType, ByteArrayType, SignatureType:
		resultRawValue, resultErr = json.Marshal(hex.EncodeToString(p.Value.([]byte)))
	case ArrayType:
		var value = make([]rawParameter, 0)
		for _, parameter := range p.Value.([]Parameter) {
			rawValue, err := json.Marshal(parameter.Value)
			if err != nil {
				return nil, err
			}
			value = append(value, rawParameter{
				Type:  parameter.Type,
				Value: rawValue,
			})
		}
		resultRawValue, resultErr = json.Marshal(value)
	case MapType:
		var value []keyValuePair
		for key, val := range p.Value.(map[Parameter]Parameter) {
			rawKey, err := json.Marshal(key.Value)
			if err != nil {
				return nil, err
			}
			rawValue, err := json.Marshal(val.Value)
			if err != nil {
				return nil, err
			}
			value = append(value, keyValuePair{
				Key: rawParameter{
					Type:  key.Type,
					Value: rawKey,
				},
				Value: rawParameter{
					Type:  val.Type,
					Value: rawValue,
				},
			})
		}
		resultRawValue, resultErr = json.Marshal(value)
	default:
		resultErr = errors.Errorf("Marshaller for type %s not implemented", p.Type)
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
	switch p.Type = r.Type; r.Type {
	case BoolType:
		if err = json.Unmarshal(r.Value, &boolean); err != nil {
			return
		}
		p.Value = boolean
	case ByteArrayType, PublicKeyType:
		if err = json.Unmarshal(r.Value, &s); err != nil {
			return
		}
		if b, err = hex.DecodeString(s); err != nil {
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
			p.Value = i
			return
		}
		// sometimes integer comes as string
		if err = json.Unmarshal(r.Value, &s); err != nil {
			return
		}
		if i, err = strconv.ParseInt(s, 10, 64); err != nil {
			return
		}
		p.Value = i
	case ArrayType:
		// https://github.com/neo-project/neo/blob/3d59ecca5a8deb057bdad94b3028a6d5e25ac088/neo/Network/RPC/RpcServer.cs#L67
		var rs []Parameter
		if err = json.Unmarshal(r.Value, &rs); err != nil {
			return
		}
		p.Value = rs
	case MapType:
		var rawMap []rawKeyValuePair
		if err = json.Unmarshal(r.Value, &rawMap); err != nil {
			return
		}
		rs := make(map[Parameter]Parameter)
		for _, p := range rawMap {
			var key, value Parameter
			if err = json.Unmarshal(p.Key, &key); err != nil {
				return
			}
			if err = json.Unmarshal(p.Value, &value); err != nil {
				return
			}
			rs[key] = value
		}
		p.Value = rs
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
	default:
		return errors.Errorf("Unmarshaller for type %s not implemented", p.Type)
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
			return errors.Errorf("failed to cast %s to []byte", p.Value)
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
			return errors.Errorf("cannot cast param of type %s to type %s", p.Type, dest)
		}
	default:
		return errors.New("cannot define param type")
	}
	return nil
}

func bytesToUint64(b []byte, size int) (uint64, error) {
	var length = size / 8
	if len(b) > length {
		return 0, errors.Errorf("input doesn't fit into %d bits", size)
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
	)
	r = strings.NewReader(in)
	for char, _, err = r.ReadRune(); err == nil && char != utf8.RuneError; char, _, err = r.ReadRune() {
		if char == '\\' && !escaped {
			escaped = true
			continue
		}
		if char == ':' && !escaped && !hadType {
			typStr := buf.String()
			res.Type, err = ParseParamType(typStr)
			if err != nil {
				return nil, err
			}
			// We currently do not support following types:
			if res.Type == ArrayType || res.Type == MapType || res.Type == InteropInterfaceType || res.Type == VoidType {
				return nil, errors.Errorf("Unsupported contract parameter type: %s", res.Type)
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