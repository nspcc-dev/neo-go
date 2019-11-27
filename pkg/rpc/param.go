package rpc

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/CityOfZion/neo-go/pkg/crypto"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/pkg/errors"
)

type (
	// Param represents a param either passed to
	// the server or to send to a server using
	// the client.
	Param struct {
		Type  paramType
		Value interface{}
	}

	paramType int
	// FuncParam represents a function argument parameter used in the
	// invokefunction RPC method.
	FuncParam struct {
		Type  StackParamType `json:"type"`
		Value Param          `json:"value"`
	}
)

const (
	defaultT paramType = iota
	stringT
	numberT
	arrayT
	funcParamT
)

func (p Param) String() string {
	return fmt.Sprintf("%v", p.Value)
}

// GetString returns string value of the parameter.
func (p Param) GetString() (string, error) {
	str, ok := p.Value.(string)
	if !ok {
		return "", errors.New("not a string")
	}
	return str, nil
}

// GetInt returns int value of te parameter.
func (p Param) GetInt() (int, error) {
	i, ok := p.Value.(int)
	if !ok {
		return 0, errors.New("not an integer")
	}
	return i, nil
}

// GetArray returns a slice of Params stored in the parameter.
func (p Param) GetArray() ([]Param, error) {
	a, ok := p.Value.([]Param)
	if !ok {
		return nil, errors.New("not an array")
	}
	return a, nil
}

// GetUint256 returns Uint256 value of the parameter.
func (p Param) GetUint256() (util.Uint256, error) {
	s, err := p.GetString()
	if err != nil {
		return util.Uint256{}, err
	}

	return util.Uint256DecodeReverseString(s)
}

// GetUint160FromHex returns Uint160 value of the parameter encoded in hex.
func (p Param) GetUint160FromHex() (util.Uint160, error) {
	s, err := p.GetString()
	if err != nil {
		return util.Uint160{}, err
	}

	scriptHashLE, err := util.Uint160DecodeStringBE(s)
	if err != nil {
		return util.Uint160{}, err
	}
	return util.Uint160DecodeBytesBE(scriptHashLE.BytesLE())
}

// GetUint160FromAddress returns Uint160 value of the parameter that was
// supplied as an address.
func (p Param) GetUint160FromAddress() (util.Uint160, error) {
	s, err := p.GetString()
	if err != nil {
		return util.Uint160{}, err
	}

	return crypto.Uint160DecodeAddress(s)
}

// GetFuncParam returns current parameter as a function call parameter.
func (p Param) GetFuncParam() (FuncParam, error) {
	fp, ok := p.Value.(FuncParam)
	if !ok {
		return FuncParam{}, errors.New("not a function parameter")
	}
	return fp, nil
}

// GetBytesHex returns []byte value of the parameter if
// it is a hex-encoded string.
func (p Param) GetBytesHex() ([]byte, error) {
	s, err := p.GetString()
	if err != nil {
		return nil, err
	}

	return hex.DecodeString(s)
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (p *Param) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		p.Type = stringT
		p.Value = s

		return nil
	}

	var num float64
	if err := json.Unmarshal(data, &num); err == nil {
		p.Type = numberT
		p.Value = int(num)

		return nil
	}

	r := bytes.NewReader(data)
	jd := json.NewDecoder(r)
	jd.DisallowUnknownFields()
	var fp FuncParam
	if err := jd.Decode(&fp); err == nil {
		p.Type = funcParamT
		p.Value = fp

		return nil
	}

	var ps []Param
	if err := json.Unmarshal(data, &ps); err == nil {
		p.Type = arrayT
		p.Value = ps

		return nil
	}

	return errors.New("unknown type")
}
