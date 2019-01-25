package rpc

import (
	"encoding/hex"
	"encoding/json"
	"strconv"

	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/pkg/errors"
)

type StackParamType int

const (
	Unknown          StackParamType = -1
	Signature        StackParamType = 0x00
	Boolean          StackParamType = 0x01
	Integer          StackParamType = 0x02
	Hash160          StackParamType = 0x03
	Hash256          StackParamType = 0x04
	ByteArray        StackParamType = 0x05
	PublicKey        StackParamType = 0x06
	String           StackParamType = 0x07
	Array            StackParamType = 0x10
	InteropInterface StackParamType = 0xf0
	Void             StackParamType = 0xff
)

func (t StackParamType) String() string {
	switch t {
	case Signature:
		return "Signature"
	case Boolean:
		return "Boolean"
	case Integer:
		return "Integer"
	case Hash160:
		return "Hash160"
	case Hash256:
		return "Hash256"
	case ByteArray:
		return "ByteArray"
	case PublicKey:
		return "PublicKey"
	case String:
		return "String"
	case Array:
		return "Array"
	case InteropInterface:
		return "InteropInterface"
	case Void:
		return "Void"
	default:
		return "Unknown"
	}
}

func StackParamTypeFromString(s string) (StackParamType, error) {
	switch s {
	case "Signature":
		return Signature, nil
	case "Boolean":
		return Boolean, nil
	case "Integer":
		return Integer, nil
	case "Hash160":
		return Hash160, nil
	case "Hash256":
		return Hash256, nil
	case "ByteArray":
		return ByteArray, nil
	case "PublicKey":
		return PublicKey, nil
	case "String":
		return String, nil
	case "Array":
		return Array, nil
	case "InteropInterface":
		return InteropInterface, nil
	case "Void":
		return Void, nil
	default:
		return Unknown, errors.Errorf("unknown stack parameter type: %s", s)
	}
}

func (p *StackParamType) UnmarshalJSON(data []byte) (err error) {
	var (
		s = string(data)
		l = len(s)
	)
	if l < 2 || s[0] != '"' || s[l-1] != '"' {
		*p = Unknown
		return errors.Errorf("invalid type: %s", s)
	}
	*p, err = StackParamTypeFromString(s[1 : l-1])
	return
}

// StackParam respresent a stack parameter.
type StackParam struct {
	Type  StackParamType `json:"type"`
	Value interface{}    `json:"value"`
}

type rawStackParam struct {
	Type  StackParamType  `json:"type"`
	Value json.RawMessage `json:"value"`
}

// UnmarshalJSON implements Unmarshaler interface
func (p *StackParam) UnmarshalJSON(data []byte) (err error) {
	var (
		r rawStackParam
		i int64
		s string
		b []byte
	)

	if err = json.Unmarshal(data, &r); err != nil {
		return
	}

	switch p.Type = r.Type; r.Type {
	case ByteArray:
		if err = json.Unmarshal(r.Value, &s); err != nil {
			return
		}
		if b, err = hex.DecodeString(s); err != nil {
			return
		}
		p.Value = b
	case String:
		if err = json.Unmarshal(r.Value, &s); err != nil {
			return
		}
		p.Value = s
	case Integer:
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
	case Array:
		//https://github.com/neo-project/neo/blob/3d59ecca5a8deb057bdad94b3028a6d5e25ac088/neo/Network/RPC/RpcServer.cs#L67
		var rs []StackParam
		if err = json.Unmarshal(r.Value, &rs); err != nil {
			return
		}
		p.Value = rs
	case Hash160:
		var h util.Uint160
		if err = json.Unmarshal(r.Value, &h); err != nil {
			return
		}
		p.Value = h
	case Hash256:
		var h util.Uint256
		if err = json.Unmarshal(r.Value, &h); err != nil {
			return
		}
		p.Value = h
	default:
		return errors.New("not implemented")
	}
	return
}

func (p *StackParam) TryParse(dest interface{}) error {
	var (
		err  error
		ok   bool
		data []byte
	)
	switch p.Type {
	case ByteArray:
		if data, ok = p.Value.([]byte); !ok {
			return errors.Errorf("failed to cast %s to []byte", p.Value)
		}
		switch dest := dest.(type) {
		case *util.Uint160:
			if *dest, err = util.Uint160DecodeBytes(data); err != nil {
				return err
			}
			return nil
		case *[]byte:
			dest = &data
			return nil
		default:
			return errors.Errorf("cannot cast stackparam of type %s to type %s", p.Type, dest)
		}
	default:
		return errors.New("cannot define stackparam type")
	}
	return nil
}
