package request

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
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
		Type  smartcontract.ParamType `json:"type"`
		Value Param                   `json:"value"`
	}
	// BlockFilter is a wrapper structure for block event filter. The only
	// allowed filter is primary index.
	BlockFilter struct {
		Primary int `json:"primary"`
	}
	// TxFilter is a wrapper structure for transaction event filter. It
	// allows to filter transactions by senders and signers.
	TxFilter struct {
		Sender *util.Uint160 `json:"sender,omitempty"`
		Signer *util.Uint160 `json:"signer,omitempty"`
	}
	// NotificationFilter is a wrapper structure representing filter used for
	// notifications generated during transaction execution. Notifications can
	// only be filtered by contract hash.
	NotificationFilter struct {
		Contract util.Uint160 `json:"contract"`
	}
	// ExecutionFilter is a wrapper structure used for transaction execution
	// events. It allows to choose failing or successful transactions based
	// on their VM state.
	ExecutionFilter struct {
		State string `json:"state"`
	}
)

// These are parameter types accepted by RPC server.
const (
	defaultT paramType = iota
	StringT
	NumberT
	ArrayT
	FuncParamT
	BlockFilterT
	TxFilterT
	NotificationFilterT
	ExecutionFilterT
	Signer
)

var errMissingParameter = errors.New("parameter is missing")

func (p Param) String() string {
	return fmt.Sprintf("%v", p.Value)
}

// GetString returns string value of the parameter.
func (p *Param) GetString() (string, error) {
	if p == nil {
		return "", errMissingParameter
	}
	str, ok := p.Value.(string)
	if !ok {
		return "", errors.New("not a string")
	}
	return str, nil
}

// GetBoolean returns boolean value of the parameter.
func (p *Param) GetBoolean() bool {
	if p == nil {
		return false
	}
	switch p.Type {
	case NumberT:
		return p.Value != 0
	case StringT:
		return p.Value != ""
	default:
		return true
	}
}

// GetInt returns int value of te parameter.
func (p *Param) GetInt() (int, error) {
	if p == nil {
		return 0, errMissingParameter
	}
	i, ok := p.Value.(int)
	if ok {
		return i, nil
	} else if s, ok := p.Value.(string); ok {
		return strconv.Atoi(s)
	}
	return 0, errors.New("not an integer")
}

// GetArray returns a slice of Params stored in the parameter.
func (p *Param) GetArray() ([]Param, error) {
	if p == nil {
		return nil, errMissingParameter
	}
	a, ok := p.Value.([]Param)
	if !ok {
		return nil, errors.New("not an array")
	}
	return a, nil
}

// GetUint256 returns Uint256 value of the parameter.
func (p *Param) GetUint256() (util.Uint256, error) {
	s, err := p.GetString()
	if err != nil {
		return util.Uint256{}, err
	}

	return util.Uint256DecodeStringLE(s)
}

// GetUint160FromHex returns Uint160 value of the parameter encoded in hex.
func (p *Param) GetUint160FromHex() (util.Uint160, error) {
	s, err := p.GetString()
	if err != nil {
		return util.Uint160{}, err
	}
	if len(s) == 2*util.Uint160Size+2 && s[0] == '0' && s[1] == 'x' {
		s = s[2:]
	}

	return util.Uint160DecodeStringLE(s)
}

// GetUint160FromAddress returns Uint160 value of the parameter that was
// supplied as an address.
func (p *Param) GetUint160FromAddress() (util.Uint160, error) {
	s, err := p.GetString()
	if err != nil {
		return util.Uint160{}, err
	}

	return address.StringToUint160(s)
}

// GetUint160FromAddressOrHex returns Uint160 value of the parameter that was
// supplied either as raw hex or as an address.
func (p *Param) GetUint160FromAddressOrHex() (util.Uint160, error) {
	u, err := p.GetUint160FromHex()
	if err == nil {
		return u, err
	}
	return p.GetUint160FromAddress()
}

// GetFuncParam returns current parameter as a function call parameter.
func (p *Param) GetFuncParam() (FuncParam, error) {
	if p == nil {
		return FuncParam{}, errMissingParameter
	}
	fp, ok := p.Value.(FuncParam)
	if !ok {
		return FuncParam{}, errors.New("not a function parameter")
	}
	return fp, nil
}

// GetBytesHex returns []byte value of the parameter if
// it is a hex-encoded string.
func (p *Param) GetBytesHex() ([]byte, error) {
	s, err := p.GetString()
	if err != nil {
		return nil, err
	}

	return hex.DecodeString(s)
}

// GetBytesBase64 returns []byte value of the parameter if
// it is a base64-encoded string.
func (p *Param) GetBytesBase64() ([]byte, error) {
	s, err := p.GetString()
	if err != nil {
		return nil, err
	}

	return base64.StdEncoding.DecodeString(s)
}

// GetSigner returns transaction.Signer value of the parameter.
func (p Param) GetSigner() (transaction.Signer, error) {
	c, ok := p.Value.(transaction.Signer)
	if !ok {
		return transaction.Signer{}, errors.New("not a signer")
	}
	return c, nil
}

// GetSigners returns a slice of transaction.Signer with global scope from
// array of Uint160 or array of serialized transaction.Signer stored in the
// parameter.
func (p Param) GetSigners() ([]transaction.Signer, error) {
	hashes, err := p.GetArray()
	if err != nil {
		return nil, err
	}
	signers := make([]transaction.Signer, len(hashes))
	// try to extract hashes first
	for i, h := range hashes {
		var u util.Uint160
		u, err = h.GetUint160FromHex()
		if err != nil {
			break
		}
		signers[i] = transaction.Signer{
			Account: u,
			Scopes:  transaction.Global,
		}
	}
	if err != nil {
		for i, h := range hashes {
			signers[i], err = h.GetSigner()
			if err != nil {
				return nil, err
			}
		}
	}
	return signers, nil
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (p *Param) UnmarshalJSON(data []byte) error {
	var s string
	var num float64
	// To unmarshal correctly we need to pass pointers into the decoder.
	var attempts = [...]Param{
		{NumberT, &num},
		{StringT, &s},
		{FuncParamT, &FuncParam{}},
		{BlockFilterT, &BlockFilter{}},
		{TxFilterT, &TxFilter{}},
		{NotificationFilterT, &NotificationFilter{}},
		{ExecutionFilterT, &ExecutionFilter{}},
		{Signer, &transaction.Signer{}},
		{ArrayT, &[]Param{}},
	}

	if bytes.Equal(data, []byte("null")) {
		p.Type = defaultT
		return nil
	}

	for _, cur := range attempts {
		r := bytes.NewReader(data)
		jd := json.NewDecoder(r)
		jd.DisallowUnknownFields()
		if err := jd.Decode(cur.Value); err == nil {
			p.Type = cur.Type
			// But we need to store actual values, not pointers.
			switch val := cur.Value.(type) {
			case *float64:
				p.Value = int(*val)
			case *string:
				p.Value = *val
			case *FuncParam:
				p.Value = *val
			case *BlockFilter:
				p.Value = *val
			case *TxFilter:
				p.Value = *val
			case *NotificationFilter:
				p.Value = *val
			case *ExecutionFilter:
				if (*val).State == "HALT" || (*val).State == "FAULT" {
					p.Value = *val
				} else {
					continue
				}
			case *transaction.Signer:
				p.Value = *val
			case *[]Param:
				p.Value = *val
			}
			return nil
		}
	}

	return errors.New("unknown type")
}
