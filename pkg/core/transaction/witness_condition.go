package transaction

import (
	"encoding/json"
	"errors"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

//go:generate stringer -type=WitnessConditionType -linecomment

// WitnessConditionType encodes a type of witness condition.
type WitnessConditionType byte

const (
	// WitnessBoolean is a generic boolean condition.
	WitnessBoolean WitnessConditionType = 0x00 // Boolean
	// WitnessNot reverses another condition.
	WitnessNot WitnessConditionType = 0x01 // Not
	// WitnessAnd means that all conditions must be met.
	WitnessAnd WitnessConditionType = 0x02 // And
	// WitnessOr means that any of conditions must be met.
	WitnessOr WitnessConditionType = 0x03 // Or
	// WitnessScriptHash matches executing contract's script hash.
	WitnessScriptHash WitnessConditionType = 0x18 // ScriptHash
	// WitnessGroup matches executing contract's group key.
	WitnessGroup WitnessConditionType = 0x19 // Group
	// WitnessCalledByEntry matches when current script is an entry script or is called by an entry script.
	WitnessCalledByEntry WitnessConditionType = 0x20 // CalledByEntry
	// WitnessCalledByContract matches when current script is called by the specified contract.
	WitnessCalledByContract WitnessConditionType = 0x28 // CalledByContract
	// WitnessCalledByGroup matches when current script is called by contract belonging to the specified group.
	WitnessCalledByGroup WitnessConditionType = 0x29 // CalledByGroup

	// MaxConditionNesting limits the maximum allowed level of condition nesting.
	MaxConditionNesting = 2
)

// WitnessCondition is a condition of WitnessRule.
type WitnessCondition interface {
	// Type returns a type of this condition.
	Type() WitnessConditionType
	// Match checks whether this condition matches current context.
	Match(MatchContext) (bool, error)
	// EncodeBinary allows to serialize condition to its binary
	// representation (including type data).
	EncodeBinary(*io.BinWriter)
	// DecodeBinarySpecific decodes type-specific binary data from the given
	// reader (not including type data).
	DecodeBinarySpecific(*io.BinReader, int)

	json.Marshaler
}

// MatchContext is a set of methods from execution engine needed to perform the
// witness check.
type MatchContext interface {
	GetCallingScriptHash() util.Uint160
	GetCurrentScriptHash() util.Uint160
	GetEntryScriptHash() util.Uint160
	CallingScriptHasGroup(*keys.PublicKey) (bool, error)
	CurrentScriptHasGroup(*keys.PublicKey) (bool, error)
}

type (
	// ConditionBoolean is a boolean condition type.
	ConditionBoolean bool
	// ConditionNot inverses the meaning of contained condition.
	ConditionNot struct {
		Condition WitnessCondition
	}
	// ConditionAnd is a set of conditions required to match.
	ConditionAnd []WitnessCondition
	// ConditionOr is a set of conditions one of which is required to match.
	ConditionOr []WitnessCondition
	// ConditionScriptHash is a condition matching executing script hash.
	ConditionScriptHash util.Uint160
	// ConditionGroup is a condition matching executing script group.
	ConditionGroup keys.PublicKey
	// ConditionCalledByEntry is a condition matching entry script or one directly called by it.
	ConditionCalledByEntry struct{}
	// ConditionCalledByContract is a condition matching calling script hash.
	ConditionCalledByContract util.Uint160
	// ConditionCalledByGroup is a condition matching calling script group.
	ConditionCalledByGroup keys.PublicKey
)

// conditionAux is used for JSON marshaling/unmarshaling.
type conditionAux struct {
	Expression  json.RawMessage   `json:"expression,omitempty"` // Can be either boolean or conditionAux.
	Expressions []json.RawMessage `json:"expressions,omitempty"`
	Group       *keys.PublicKey   `json:"group,omitempty"`
	Hash        *util.Uint160     `json:"hash,omitempty"`
	Type        string            `json:"type"`
}

// Type implements the WitnessCondition interface and returns condition type.
func (c *ConditionBoolean) Type() WitnessConditionType {
	return WitnessBoolean
}

// Match implements the WitnessCondition interface checking whether this condition
// matches given context.
func (c *ConditionBoolean) Match(_ MatchContext) (bool, error) {
	return bool(*c), nil
}

// EncodeBinary implements the WitnessCondition interface allowing to serialize condition.
func (c *ConditionBoolean) EncodeBinary(w *io.BinWriter) {
	w.WriteB(byte(c.Type()))
	w.WriteBool(bool(*c))
}

// DecodeBinarySpecific implements the WitnessCondition interface allowing to
// deserialize condition-specific data.
func (c *ConditionBoolean) DecodeBinarySpecific(r *io.BinReader, maxDepth int) {
	*c = ConditionBoolean(r.ReadBool())
}

// MarshalJSON implements the json.Marshaler interface.
func (c *ConditionBoolean) MarshalJSON() ([]byte, error) {
	boolJSON, _ := json.Marshal(bool(*c)) // Simple boolean can't fail.
	aux := conditionAux{
		Type:       c.Type().String(),
		Expression: json.RawMessage(boolJSON),
	}
	return json.Marshal(aux)
}

// Type implements the WitnessCondition interface and returns condition type.
func (c *ConditionNot) Type() WitnessConditionType {
	return WitnessNot
}

// Match implements the WitnessCondition interface checking whether this condition
// matches given context.
func (c *ConditionNot) Match(ctx MatchContext) (bool, error) {
	res, err := c.Condition.Match(ctx)
	return ((err == nil) && !res), err
}

// EncodeBinary implements the WitnessCondition interface allowing to serialize condition.
func (c *ConditionNot) EncodeBinary(w *io.BinWriter) {
	w.WriteB(byte(c.Type()))
	c.Condition.EncodeBinary(w)
}

// DecodeBinarySpecific implements the WitnessCondition interface allowing to
// deserialize condition-specific data.
func (c *ConditionNot) DecodeBinarySpecific(r *io.BinReader, maxDepth int) {
	c.Condition = decodeBinaryCondition(r, maxDepth-1)
}

// MarshalJSON implements the json.Marshaler interface.
func (c *ConditionNot) MarshalJSON() ([]byte, error) {
	condJSON, err := json.Marshal(c.Condition)
	if err != nil {
		return nil, err
	}
	aux := conditionAux{
		Type:       c.Type().String(),
		Expression: json.RawMessage(condJSON),
	}
	return json.Marshal(aux)
}

// Type implements the WitnessCondition interface and returns condition type.
func (c *ConditionAnd) Type() WitnessConditionType {
	return WitnessAnd
}

// Match implements the WitnessCondition interface checking whether this condition
// matches given context.
func (c *ConditionAnd) Match(ctx MatchContext) (bool, error) {
	for _, cond := range *c {
		res, err := cond.Match(ctx)
		if err != nil {
			return false, err
		}
		if !res {
			return false, nil
		}
	}
	return true, nil
}

// EncodeBinary implements the WitnessCondition interface allowing to serialize condition.
func (c *ConditionAnd) EncodeBinary(w *io.BinWriter) {
	w.WriteB(byte(c.Type()))
	w.WriteArray([]WitnessCondition(*c))
}

func readArrayOfConditions(r *io.BinReader, maxDepth int) []WitnessCondition {
	l := r.ReadVarUint()
	if l == 0 {
		r.Err = errors.New("empty array of conditions")
		return nil
	}
	if l > maxSubitems {
		r.Err = errors.New("too many elements")
		return nil
	}
	a := make([]WitnessCondition, l)
	for i := 0; i < int(l); i++ {
		a[i] = decodeBinaryCondition(r, maxDepth-1)
	}
	if r.Err != nil {
		return nil
	}
	return a
}

// DecodeBinarySpecific implements the WitnessCondition interface allowing to
// deserialize condition-specific data.
func (c *ConditionAnd) DecodeBinarySpecific(r *io.BinReader, maxDepth int) {
	a := readArrayOfConditions(r, maxDepth)
	if r.Err == nil {
		*c = a
	}
}

func arrayToJSON(c WitnessCondition, a []WitnessCondition) ([]byte, error) {
	exprs := make([]json.RawMessage, len(a))
	for i := 0; i < len(a); i++ {
		b, err := a[i].MarshalJSON()
		if err != nil {
			return nil, err
		}
		exprs[i] = json.RawMessage(b)
	}
	aux := conditionAux{
		Type:        c.Type().String(),
		Expressions: exprs,
	}
	return json.Marshal(aux)
}

// MarshalJSON implements the json.Marshaler interface.
func (c *ConditionAnd) MarshalJSON() ([]byte, error) {
	return arrayToJSON(c, []WitnessCondition(*c))
}

// Type implements the WitnessCondition interface and returns condition type.
func (c *ConditionOr) Type() WitnessConditionType {
	return WitnessOr
}

// Match implements the WitnessCondition interface checking whether this condition
// matches given context.
func (c *ConditionOr) Match(ctx MatchContext) (bool, error) {
	for _, cond := range *c {
		res, err := cond.Match(ctx)
		if err != nil {
			return false, err
		}
		if res {
			return true, nil
		}
	}
	return false, nil
}

// EncodeBinary implements the WitnessCondition interface allowing to serialize condition.
func (c *ConditionOr) EncodeBinary(w *io.BinWriter) {
	w.WriteB(byte(c.Type()))
	w.WriteArray([]WitnessCondition(*c))
}

// DecodeBinarySpecific implements the WitnessCondition interface allowing to
// deserialize condition-specific data.
func (c *ConditionOr) DecodeBinarySpecific(r *io.BinReader, maxDepth int) {
	a := readArrayOfConditions(r, maxDepth)
	if r.Err == nil {
		*c = a
	}
}

// MarshalJSON implements the json.Marshaler interface.
func (c *ConditionOr) MarshalJSON() ([]byte, error) {
	return arrayToJSON(c, []WitnessCondition(*c))
}

// Type implements the WitnessCondition interface and returns condition type.
func (c *ConditionScriptHash) Type() WitnessConditionType {
	return WitnessScriptHash
}

// Match implements the WitnessCondition interface checking whether this condition
// matches given context.
func (c *ConditionScriptHash) Match(ctx MatchContext) (bool, error) {
	return util.Uint160(*c).Equals(ctx.GetCurrentScriptHash()), nil
}

// EncodeBinary implements the WitnessCondition interface allowing to serialize condition.
func (c *ConditionScriptHash) EncodeBinary(w *io.BinWriter) {
	w.WriteB(byte(c.Type()))
	w.WriteBytes(c[:])
}

// DecodeBinarySpecific implements the WitnessCondition interface allowing to
// deserialize condition-specific data.
func (c *ConditionScriptHash) DecodeBinarySpecific(r *io.BinReader, _ int) {
	r.ReadBytes(c[:])
}

// MarshalJSON implements the json.Marshaler interface.
func (c *ConditionScriptHash) MarshalJSON() ([]byte, error) {
	aux := conditionAux{
		Type: c.Type().String(),
		Hash: (*util.Uint160)(c),
	}
	return json.Marshal(aux)
}

// Type implements the WitnessCondition interface and returns condition type.
func (c *ConditionGroup) Type() WitnessConditionType {
	return WitnessGroup
}

// Match implements the WitnessCondition interface checking whether this condition
// matches given context.
func (c *ConditionGroup) Match(ctx MatchContext) (bool, error) {
	return ctx.CurrentScriptHasGroup((*keys.PublicKey)(c))
}

// EncodeBinary implements the WitnessCondition interface allowing to serialize condition.
func (c *ConditionGroup) EncodeBinary(w *io.BinWriter) {
	w.WriteB(byte(c.Type()))
	(*keys.PublicKey)(c).EncodeBinary(w)
}

// DecodeBinarySpecific implements the WitnessCondition interface allowing to
// deserialize condition-specific data.
func (c *ConditionGroup) DecodeBinarySpecific(r *io.BinReader, _ int) {
	(*keys.PublicKey)(c).DecodeBinary(r)
}

// MarshalJSON implements the json.Marshaler interface.
func (c *ConditionGroup) MarshalJSON() ([]byte, error) {
	aux := conditionAux{
		Type:  c.Type().String(),
		Group: (*keys.PublicKey)(c),
	}
	return json.Marshal(aux)
}

// Type implements the WitnessCondition interface and returns condition type.
func (c ConditionCalledByEntry) Type() WitnessConditionType {
	return WitnessCalledByEntry
}

// Match implements the WitnessCondition interface checking whether this condition
// matches given context.
func (c ConditionCalledByEntry) Match(ctx MatchContext) (bool, error) {
	entry := ctx.GetEntryScriptHash()
	return entry.Equals(ctx.GetCallingScriptHash()) || entry.Equals(ctx.GetCurrentScriptHash()), nil
}

// EncodeBinary implements the WitnessCondition interface allowing to serialize condition.
func (c ConditionCalledByEntry) EncodeBinary(w *io.BinWriter) {
	w.WriteB(byte(c.Type()))
}

// DecodeBinarySpecific implements the WitnessCondition interface allowing to
// deserialize condition-specific data.
func (c ConditionCalledByEntry) DecodeBinarySpecific(_ *io.BinReader, _ int) {
}

// MarshalJSON implements the json.Marshaler interface.
func (c ConditionCalledByEntry) MarshalJSON() ([]byte, error) {
	aux := conditionAux{
		Type: c.Type().String(),
	}
	return json.Marshal(aux)
}

// Type implements the WitnessCondition interface and returns condition type.
func (c *ConditionCalledByContract) Type() WitnessConditionType {
	return WitnessCalledByContract
}

// Match implements the WitnessCondition interface checking whether this condition
// matches given context.
func (c *ConditionCalledByContract) Match(ctx MatchContext) (bool, error) {
	return util.Uint160(*c).Equals(ctx.GetCallingScriptHash()), nil
}

// EncodeBinary implements the WitnessCondition interface allowing to serialize condition.
func (c *ConditionCalledByContract) EncodeBinary(w *io.BinWriter) {
	w.WriteB(byte(c.Type()))
	w.WriteBytes(c[:])
}

// DecodeBinarySpecific implements the WitnessCondition interface allowing to
// deserialize condition-specific data.
func (c *ConditionCalledByContract) DecodeBinarySpecific(r *io.BinReader, _ int) {
	r.ReadBytes(c[:])
}

// MarshalJSON implements the json.Marshaler interface.
func (c *ConditionCalledByContract) MarshalJSON() ([]byte, error) {
	aux := conditionAux{
		Type: c.Type().String(),
		Hash: (*util.Uint160)(c),
	}
	return json.Marshal(aux)
}

// Type implements the WitnessCondition interface and returns condition type.
func (c *ConditionCalledByGroup) Type() WitnessConditionType {
	return WitnessCalledByGroup
}

// Match implements the WitnessCondition interface checking whether this condition
// matches given context.
func (c *ConditionCalledByGroup) Match(ctx MatchContext) (bool, error) {
	return ctx.CallingScriptHasGroup((*keys.PublicKey)(c))
}

// EncodeBinary implements the WitnessCondition interface allowing to serialize condition.
func (c *ConditionCalledByGroup) EncodeBinary(w *io.BinWriter) {
	w.WriteB(byte(c.Type()))
	(*keys.PublicKey)(c).EncodeBinary(w)
}

// DecodeBinarySpecific implements the WitnessCondition interface allowing to
// deserialize condition-specific data.
func (c *ConditionCalledByGroup) DecodeBinarySpecific(r *io.BinReader, _ int) {
	(*keys.PublicKey)(c).DecodeBinary(r)
}

// MarshalJSON implements the json.Marshaler interface.
func (c *ConditionCalledByGroup) MarshalJSON() ([]byte, error) {
	aux := conditionAux{
		Type:  c.Type().String(),
		Group: (*keys.PublicKey)(c),
	}
	return json.Marshal(aux)
}

// DecodeBinaryCondition decodes and returns condition from the given binary stream.
func DecodeBinaryCondition(r *io.BinReader) WitnessCondition {
	return decodeBinaryCondition(r, MaxConditionNesting)
}

func decodeBinaryCondition(r *io.BinReader, maxDepth int) WitnessCondition {
	if maxDepth <= 0 {
		r.Err = errors.New("too many nesting levels")
		return nil
	}
	t := WitnessConditionType(r.ReadB())
	if r.Err != nil {
		return nil
	}
	var res WitnessCondition
	switch t {
	case WitnessBoolean:
		var v ConditionBoolean
		res = &v
	case WitnessNot:
		res = &ConditionNot{}
	case WitnessAnd:
		res = &ConditionAnd{}
	case WitnessOr:
		res = &ConditionOr{}
	case WitnessScriptHash:
		res = &ConditionScriptHash{}
	case WitnessGroup:
		res = &ConditionGroup{}
	case WitnessCalledByEntry:
		res = ConditionCalledByEntry{}
	case WitnessCalledByContract:
		res = &ConditionCalledByContract{}
	case WitnessCalledByGroup:
		res = &ConditionCalledByGroup{}
	default:
		r.Err = errors.New("invalid condition type")
		return nil
	}
	res.DecodeBinarySpecific(r, maxDepth)
	if r.Err != nil {
		return nil
	}
	return res
}

func unmarshalArrayOfConditionJSONs(arr []json.RawMessage, maxDepth int) ([]WitnessCondition, error) {
	l := len(arr)
	if l == 0 {
		return nil, errors.New("empty array of conditions")
	}
	if l >= maxSubitems {
		return nil, errors.New("too many elements")
	}
	res := make([]WitnessCondition, l)
	for i := range arr {
		v, err := unmarshalConditionJSON(arr[i], maxDepth-1)
		if err != nil {
			return nil, err
		}
		res[i] = v
	}
	return res, nil
}

// UnmarshalConditionJSON unmarshalls condition from the given JSON data.
func UnmarshalConditionJSON(data []byte) (WitnessCondition, error) {
	return unmarshalConditionJSON(data, MaxConditionNesting)
}

func unmarshalConditionJSON(data []byte, maxDepth int) (WitnessCondition, error) {
	if maxDepth <= 0 {
		return nil, errors.New("too many nesting levels")
	}
	aux := &conditionAux{}
	err := json.Unmarshal(data, aux)
	if err != nil {
		return nil, err
	}
	var res WitnessCondition
	switch aux.Type {
	case WitnessBoolean.String():
		var v bool
		err = json.Unmarshal(aux.Expression, &v)
		if err != nil {
			return nil, err
		}
		res = (*ConditionBoolean)(&v)
	case WitnessNot.String():
		v, err := unmarshalConditionJSON(aux.Expression, maxDepth-1)
		if err != nil {
			return nil, err
		}
		res = &ConditionNot{Condition: v}
	case WitnessAnd.String():
		v, err := unmarshalArrayOfConditionJSONs(aux.Expressions, maxDepth)
		if err != nil {
			return nil, err
		}
		res = (*ConditionAnd)(&v)
	case WitnessOr.String():
		v, err := unmarshalArrayOfConditionJSONs(aux.Expressions, maxDepth)
		if err != nil {
			return nil, err
		}
		res = (*ConditionOr)(&v)
	case WitnessScriptHash.String():
		if aux.Hash == nil {
			return nil, errors.New("no hash specified")
		}
		res = (*ConditionScriptHash)(aux.Hash)
	case WitnessGroup.String():
		if aux.Group == nil {
			return nil, errors.New("no group specified")
		}
		res = (*ConditionGroup)(aux.Group)
	case WitnessCalledByEntry.String():
		res = ConditionCalledByEntry{}
	case WitnessCalledByContract.String():
		if aux.Hash == nil {
			return nil, errors.New("no hash specified")
		}
		res = (*ConditionCalledByContract)(aux.Hash)
	case WitnessCalledByGroup.String():
		if aux.Group == nil {
			return nil, errors.New("no group specified")
		}
		res = (*ConditionCalledByGroup)(aux.Group)
	default:
		return nil, errors.New("invalid condition type")
	}
	return res, nil
}
