package rpc

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

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
)

const (
	defaultT paramType = iota
	stringT
	numberT
	arrayT
)

func (p Param) String() string {
	return fmt.Sprintf("%v", p.Value)
}

// GetString returns string value of the parameter.
func (p Param) GetString() string { return p.Value.(string) }

// GetInt returns int value of te parameter.
func (p Param) GetInt() int { return p.Value.(int) }

// GetUint256 returns Uint256 value of the parameter.
func (p Param) GetUint256() (util.Uint256, error) {
	s, ok := p.Value.(string)
	if !ok {
		return util.Uint256{}, errors.New("must be a string")
	}

	return util.Uint256DecodeReverseString(s)
}

// GetBytesHex returns []byte value of the parameter if
// it is a hex-encoded string.
func (p Param) GetBytesHex() ([]byte, error) {
	s, ok := p.Value.(string)
	if !ok {
		return nil, errors.New("must be a string")
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

	var ps []Param
	if err := json.Unmarshal(data, &ps); err == nil {
		p.Type = arrayT
		p.Value = ps

		return nil
	}

	return errors.New("unknown type")
}
