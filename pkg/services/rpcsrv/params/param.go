package params

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

	"github.com/google/uuid"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/neorpc"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

type (
	// Param represents a param either passed to
	// the server or to be sent to a server using
	// the client.
	Param struct {
		json.RawMessage
		cache any
	}

	// FuncParam represents a function argument parameter used in the
	// invokefunction RPC method.
	FuncParam struct {
		Type  smartcontract.ParamType `json:"type"`
		Value Param                   `json:"value"`
	}

	// FuncParamKV represents a pair of function argument parameters
	// a slice of which is stored in FuncParam of [smartcontract.MapType] type.
	FuncParamKV struct {
		Key   FuncParam `json:"key"`
		Value FuncParam `json:"value"`
	}
)

var (
	jsonNullBytes       = []byte("null")
	jsonFalseBytes      = []byte("false")
	jsonTrueBytes       = []byte("true")
	errMissingParameter = errors.New("parameter is missing")
	errNullParameter    = errors.New("parameter is null")
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

func (p *Param) fillIntCache() (any, error) {
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

// GetFuncParamPair returns a pair of function call parameters.
func (p *Param) GetFuncParamPair() (FuncParamKV, error) {
	if p == nil {
		return FuncParamKV{}, errMissingParameter
	}
	// This one doesn't need to be cached, it's used only once.
	fpp := FuncParamKV{}
	err := json.Unmarshal(p.RawMessage, &fpp)
	if err != nil {
		return FuncParamKV{}, err
	}

	return fpp, nil
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

// GetSignerWithWitness returns a neorpc.SignerWithWitness value of the parameter.
func (p *Param) GetSignerWithWitness() (neorpc.SignerWithWitness, error) {
	// This one doesn't need to be cached, it's used only once.
	c := neorpc.SignerWithWitness{}
	err := json.Unmarshal(p.RawMessage, &c)
	if err != nil {
		return neorpc.SignerWithWitness{}, fmt.Errorf("not a signer: %w", err)
	}
	return c, nil
}

// GetSignersWithWitnesses returns a slice of SignerWithWitness with CalledByEntry
// scope from an array of Uint160 or an array of serialized transaction.Signer stored
// in the parameter.
func (p *Param) GetSignersWithWitnesses() ([]transaction.Signer, []transaction.Witness, error) {
	hashes, err := p.GetArray()
	if err != nil {
		return nil, nil, err
	}
	if len(hashes) > transaction.MaxAttributes {
		return nil, nil, errors.New("too many signers")
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

// GetUUID returns UUID from parameter.
func (p *Param) GetUUID() (uuid.UUID, error) {
	s, err := p.GetString()
	if err != nil {
		return uuid.UUID{}, err
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("not a valid UUID: %w", err)
	}
	return id, nil
}

// GetFakeTx returns [transaction.Transaction] from parameter. The resulting
// transaction has overridden hash and size fields (if specified).
func (p *Param) GetFakeTx() (*transaction.Transaction, error) {
	if p == nil {
		return nil, errMissingParameter
	}
	if p.IsNull() {
		return nil, errNullParameter
	}
	var tx = new(transaction.Transaction)
	err := tx.UnmarshalJSONUnsafe(p.RawMessage) // this field is accessed only once, hence doesn't need to be cached.
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal fake transaction: %w", err)
	}

	return tx, nil
}

// GetFakeHeader returns [block.Header] from parameter. The resulting header
// always has StateRoot filled irrespectively of StateRootEnabled setting
// (if provided).
func (p *Param) GetFakeHeader() (*block.Header, error) {
	if p == nil {
		return nil, errMissingParameter
	}
	if p.IsNull() {
		return nil, errNullParameter
	}
	var b = new(block.Header)
	err := b.UnmarshalJSON(p.RawMessage) // this field is accessed only once, hence doesn't need to be cached.
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal fake header: %w", err)
	}

	return b, nil
}
