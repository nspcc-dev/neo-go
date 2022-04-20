package request

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

type (
	// Param represents a param either passed to
	// the server or to be sent to a server using
	// the client.
	Param struct {
		json.RawMessage
		cache interface{}
	}

	// FuncParam represents a function argument parameter used in the
	// invokefunction RPC method.
	FuncParam struct {
		Type  smartcontract.ParamType `json:"type"`
		Value Param                   `json:"value"`
	}
	// BlockFilter is a wrapper structure for the block event filter. The only
	// allowed filter is primary index.
	BlockFilter struct {
		Primary int `json:"primary"`
	}
	// TxFilter is a wrapper structure for the transaction event filter. It
	// allows to filter transactions by senders and signers.
	TxFilter struct {
		Sender *util.Uint160 `json:"sender,omitempty"`
		Signer *util.Uint160 `json:"signer,omitempty"`
	}
	// NotificationFilter is a wrapper structure representing a filter used for
	// notifications generated during transaction execution. Notifications can
	// be filtered by contract hash and by name.
	NotificationFilter struct {
		Contract *util.Uint160 `json:"contract,omitempty"`
		Name     *string       `json:"name,omitempty"`
	}
	// ExecutionFilter is a wrapper structure used for transaction execution
	// events. It allows to choose failing or successful transactions based
	// on their VM state.
	ExecutionFilter struct {
		State string `json:"state"`
	}
	// SignerWithWitness represents transaction's signer with the corresponding witness.
	SignerWithWitness struct {
		transaction.Signer
		transaction.Witness
	}
)

var (
	jsonNullBytes       = []byte("null")
	jsonFalseBytes      = []byte("false")
	jsonTrueBytes       = []byte("true")
	errMissingParameter = errors.New("parameter is missing")
	errNotAString       = errors.New("not a string")
	errNotAnInt         = errors.New("not an integer")
	errNotABool         = errors.New("not a boolean")
	errNotAnArray       = errors.New("not an array")
)

func (p Param) String() string {
	str, _ := p.GetString()
	return str
}

// GetStringStrict returns a string value of the parameter.
func (p *Param) GetStringStrict() (string, error) {
	if p == nil {
		return "", errMissingParameter
	}
	if p.IsNull() {
		return "", errNotAString
	}
	if p.cache == nil {
		var s string
		err := json.Unmarshal(p.RawMessage, &s)
		if err != nil {
			return "", errNotAString
		}
		p.cache = s
	}
	if s, ok := p.cache.(string); ok {
		return s, nil
	}
	return "", errNotAString
}

// GetString returns a string value of the parameter or tries to cast the parameter to a string value.
func (p *Param) GetString() (string, error) {
	if p == nil {
		return "", errMissingParameter
	}
	if p.IsNull() {
		return "", errNotAString
	}
	if p.cache == nil {
		var s string
		err := json.Unmarshal(p.RawMessage, &s)
		if err == nil {
			p.cache = s
		} else {
			var i int64
			err = json.Unmarshal(p.RawMessage, &i)
			if err == nil {
				p.cache = i
			} else {
				var b bool
				err = json.Unmarshal(p.RawMessage, &b)
				if err == nil {
					p.cache = b
				} else {
					return "", errNotAString
				}
			}
		}
	}
	switch t := p.cache.(type) {
	case string:
		return t, nil
	case int64:
		return strconv.FormatInt(t, 10), nil
	case bool:
		if t {
			return "true", nil
		}
		return "false", nil
	default:
		return "", errNotAString
	}
}

// GetBooleanStrict returns boolean value of the parameter.
func (p *Param) GetBooleanStrict() (bool, error) {
	if p == nil {
		return false, errMissingParameter
	}
	if bytes.Equal(p.RawMessage, jsonTrueBytes) {
		p.cache = true
		return true, nil
	}
	if bytes.Equal(p.RawMessage, jsonFalseBytes) {
		p.cache = false
		return false, nil
	}
	return false, errNotABool
}

// GetBoolean returns a boolean value of the parameter or tries to cast the parameter to a bool value.
func (p *Param) GetBoolean() (bool, error) {
	if p == nil {
		return false, errMissingParameter
	}
	if p.IsNull() {
		return false, errNotABool
	}
	var b bool
	if p.cache == nil {
		err := json.Unmarshal(p.RawMessage, &b)
		if err == nil {
			p.cache = b
		} else {
			var s string
			err = json.Unmarshal(p.RawMessage, &s)
			if err == nil {
				p.cache = s
			} else {
				var i int64
				err = json.Unmarshal(p.RawMessage, &i)
				if err == nil {
					p.cache = i
				} else {
					return false, errNotABool
				}
			}
		}
	}
	switch t := p.cache.(type) {
	case bool:
		return t, nil
	case string:
		return t != "", nil
	case int64:
		return t != 0, nil
	default:
		return false, errNotABool
	}
}

// GetIntStrict returns an int value of the parameter if the parameter is an integer.
func (p *Param) GetIntStrict() (int, error) {
	if p == nil {
		return 0, errMissingParameter
	}
	if p.IsNull() {
		return 0, errNotAnInt
	}
	value, err := p.fillIntCache()
	if err != nil {
		return 0, err
	}
	if i, ok := value.(int64); ok && i == int64(int(i)) {
		return int(i), nil
	}
	return 0, errNotAnInt
}

func (p *Param) fillIntCache() (interface{}, error) {
	if p.cache != nil {
		return p.cache, nil
	}

	// We could also try unmarshalling to uint64, but JSON reliably supports numbers
	// up to 53 bits in size.
	var i int64
	err := json.Unmarshal(p.RawMessage, &i)
	if err == nil {
		p.cache = i
		return i, nil
	}

	var s string
	err = json.Unmarshal(p.RawMessage, &s)
	if err == nil {
		p.cache = s
		return s, nil
	}

	var b bool
	err = json.Unmarshal(p.RawMessage, &b)
	if err == nil {
		p.cache = b
		return b, nil
	}
	return nil, errNotAnInt
}

// GetInt returns an int value of the parameter or tries to cast the parameter to an int value.
func (p *Param) GetInt() (int, error) {
	if p == nil {
		return 0, errMissingParameter
	}
	if p.IsNull() {
		return 0, errNotAnInt
	}
	value, err := p.fillIntCache()
	if err != nil {
		return 0, err
	}
	switch t := value.(type) {
	case int64:
		if t == int64(int(t)) {
			return int(t), nil
		}
		return 0, errNotAnInt
	case string:
		return strconv.Atoi(t)
	case bool:
		if t {
			return 1, nil
		}
		return 0, nil
	default:
		panic("unreachable")
	}
}

// GetBigInt returns a big-integer value of the parameter.
func (p *Param) GetBigInt() (*big.Int, error) {
	if p == nil {
		return nil, errMissingParameter
	}
	if p.IsNull() {
		return nil, errNotAnInt
	}
	value, err := p.fillIntCache()
	if err != nil {
		return nil, err
	}
	switch t := value.(type) {
	case int64:
		return big.NewInt(t), nil
	case string:
		bi, ok := new(big.Int).SetString(t, 10)
		if !ok {
			return nil, errNotAnInt
		}
		return bi, nil
	case bool:
		if t {
			return big.NewInt(1), nil
		}
		return new(big.Int), nil
	default:
		panic("unreachable")
	}
}

// GetArray returns a slice of Params stored in the parameter.
func (p *Param) GetArray() ([]Param, error) {
	if p == nil {
		return nil, errMissingParameter
	}
	if p.IsNull() {
		return nil, errNotAnArray
	}
	if p.cache == nil {
		a := []Param{}
		err := json.Unmarshal(p.RawMessage, &a)
		if err != nil {
			return nil, errNotAnArray
		}
		p.cache = a
	}
	if a, ok := p.cache.([]Param); ok {
		return a, nil
	}
	return nil, errNotAnArray
}

// GetUint256 returns a Uint256 value of the parameter.
func (p *Param) GetUint256() (util.Uint256, error) {
	s, err := p.GetString()
	if err != nil {
		return util.Uint256{}, err
	}

	return util.Uint256DecodeStringLE(strings.TrimPrefix(s, "0x"))
}

// GetUint160FromHex returns a Uint160 value of the parameter encoded in hex.
func (p *Param) GetUint160FromHex() (util.Uint160, error) {
	s, err := p.GetString()
	if err != nil {
		return util.Uint160{}, err
	}

	return util.Uint160DecodeStringLE(strings.TrimPrefix(s, "0x"))
}

// GetUint160FromAddress returns a Uint160 value of the parameter that was
// supplied as an address.
func (p *Param) GetUint160FromAddress() (util.Uint160, error) {
	s, err := p.GetString()
	if err != nil {
		return util.Uint160{}, err
	}

	return address.StringToUint160(s)
}

// GetUint160FromAddressOrHex returns a Uint160 value of the parameter that was
// supplied either as raw hex or as an address.
func (p *Param) GetUint160FromAddressOrHex() (util.Uint160, error) {
	u, err := p.GetUint160FromHex()
	if err == nil {
		return u, err
	}
	return p.GetUint160FromAddress()
}

// GetFuncParam returns the current parameter as a function call parameter.
func (p *Param) GetFuncParam() (FuncParam, error) {
	if p == nil {
		return FuncParam{}, errMissingParameter
	}
	// This one doesn't need to be cached, it's used only once.
	fp := FuncParam{}
	err := json.Unmarshal(p.RawMessage, &fp)
	return fp, err
}

// GetBytesHex returns a []byte value of the parameter if
// it is a hex-encoded string.
func (p *Param) GetBytesHex() ([]byte, error) {
	s, err := p.GetString()
	if err != nil {
		return nil, err
	}

	return hex.DecodeString(s)
}

// GetBytesBase64 returns a []byte value of the parameter if
// it is a base64-encoded string.
func (p *Param) GetBytesBase64() ([]byte, error) {
	s, err := p.GetString()
	if err != nil {
		return nil, err
	}

	return base64.StdEncoding.DecodeString(s)
}

// GetSignerWithWitness returns a SignerWithWitness value of the parameter.
func (p *Param) GetSignerWithWitness() (SignerWithWitness, error) {
	// This one doesn't need to be cached, it's used only once.
	aux := new(signerWithWitnessAux)
	err := json.Unmarshal(p.RawMessage, aux)
	if err != nil {
		return SignerWithWitness{}, fmt.Errorf("not a signer: %w", err)
	}
	acc, err := util.Uint160DecodeStringLE(strings.TrimPrefix(aux.Account, "0x"))
	if err != nil {
		acc, err = address.StringToUint160(aux.Account)
	}
	if err != nil {
		return SignerWithWitness{}, fmt.Errorf("not a signer: %w", err)
	}
	c := SignerWithWitness{
		Signer: transaction.Signer{
			Account:          acc,
			Scopes:           aux.Scopes,
			AllowedContracts: aux.AllowedContracts,
			AllowedGroups:    aux.AllowedGroups,
			Rules:            aux.Rules,
		},
		Witness: transaction.Witness{
			InvocationScript:   aux.InvocationScript,
			VerificationScript: aux.VerificationScript,
		},
	}
	return c, nil
}

// GetSignersWithWitnesses returns a slice of SignerWithWitness with CalledByEntry
// scope from an array of Uint160 or an array of serialized transaction.Signer stored
// in the parameter.
func (p Param) GetSignersWithWitnesses() ([]transaction.Signer, []transaction.Witness, error) {
	hashes, err := p.GetArray()
	if err != nil {
		return nil, nil, err
	}
	signers := make([]transaction.Signer, len(hashes))
	witnesses := make([]transaction.Witness, len(hashes))
	// try to extract hashes first
	for i, h := range hashes {
		var u util.Uint160
		u, err = h.GetUint160FromHex()
		if err != nil {
			break
		}
		signers[i] = transaction.Signer{
			Account: u,
			Scopes:  transaction.CalledByEntry,
		}
	}
	if err != nil {
		for i, h := range hashes {
			signerWithWitness, err := h.GetSignerWithWitness()
			if err != nil {
				return nil, nil, err
			}
			signers[i] = signerWithWitness.Signer
			witnesses[i] = signerWithWitness.Witness
		}
	}
	return signers, witnesses, nil
}

// IsNull returns whether the parameter represents JSON nil value.
func (p *Param) IsNull() bool {
	return bytes.Equal(p.RawMessage, jsonNullBytes)
}

// signerWithWitnessAux is an auxiliary struct for JSON marshalling. We need it because of
// DisallowUnknownFields JSON marshaller setting.
type signerWithWitnessAux struct {
	Account            string                    `json:"account"`
	Scopes             transaction.WitnessScope  `json:"scopes"`
	AllowedContracts   []util.Uint160            `json:"allowedcontracts,omitempty"`
	AllowedGroups      []*keys.PublicKey         `json:"allowedgroups,omitempty"`
	Rules              []transaction.WitnessRule `json:"rules,omitempty"`
	InvocationScript   []byte                    `json:"invocation,omitempty"`
	VerificationScript []byte                    `json:"verification,omitempty"`
}

// MarshalJSON implements the json.Marshaler interface.
func (s *SignerWithWitness) MarshalJSON() ([]byte, error) {
	signer := &signerWithWitnessAux{
		Account:            s.Account.StringLE(),
		Scopes:             s.Scopes,
		AllowedContracts:   s.AllowedContracts,
		AllowedGroups:      s.AllowedGroups,
		Rules:              s.Rules,
		InvocationScript:   s.InvocationScript,
		VerificationScript: s.VerificationScript,
	}
	return json.Marshal(signer)
}
