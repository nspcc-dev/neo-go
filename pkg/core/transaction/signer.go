package transaction

import (
	"crypto/elliptic"
	"errors"
	"fmt"
	"math/big"
	"slices"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// The maximum number of AllowedContracts or AllowedGroups.
const maxSubitems = 16

// Signer implements a Transaction signer.
type Signer struct {
	Account          util.Uint160      `json:"account"`
	Scopes           WitnessScope      `json:"scopes"`
	AllowedContracts []util.Uint160    `json:"allowedcontracts,omitzero"`
	AllowedGroups    []*keys.PublicKey `json:"allowedgroups,omitzero"`
	Rules            []WitnessRule     `json:"rules,omitzero"`
}

// Ensure required interfaces are implemented for proper RPC bindings generation.
var (
	_ = stackitem.Convertible(&Signer{})
	_ = smartcontract.Convertible(&Signer{})
)

// EncodeBinary implements the Serializable interface.
func (c *Signer) EncodeBinary(bw *io.BinWriter) {
	bw.WriteBytes(c.Account[:])
	bw.WriteB(byte(c.Scopes))
	if c.Scopes&CustomContracts != 0 {
		bw.WriteArray(c.AllowedContracts)
	}
	if c.Scopes&CustomGroups != 0 {
		bw.WriteArray(c.AllowedGroups)
	}
	if c.Scopes&Rules != 0 {
		bw.WriteArray(c.Rules)
	}
}

// DecodeBinary implements the Serializable interface.
func (c *Signer) DecodeBinary(br *io.BinReader) {
	br.ReadBytes(c.Account[:])
	c.Scopes = WitnessScope(br.ReadB())
	if c.Scopes & ^(Global|CalledByEntry|CustomContracts|CustomGroups|Rules|None) != 0 {
		br.Err = errors.New("unknown witness scope")
		return
	}
	if c.Scopes&Global != 0 && c.Scopes != Global {
		br.Err = errors.New("global scope can not be combined with other scopes")
		return
	}
	if c.Scopes&CustomContracts != 0 {
		br.ReadArray(&c.AllowedContracts, maxSubitems)
	}
	if c.Scopes&CustomGroups != 0 {
		br.ReadArray(&c.AllowedGroups, maxSubitems)
	}
	if c.Scopes&Rules != 0 {
		br.ReadArray(&c.Rules, maxSubitems)
	}
}

// SignersToStackItem converts transaction.Signers to stackitem.Item.
func SignersToStackItem(signers []Signer) stackitem.Item {
	res := make([]stackitem.Item, len(signers))
	for i, s := range signers {
		item, err := s.ToStackItem()
		if err != nil {
			panic(fmt.Errorf("bug: unexpected error: %w", err))
		}
		res[i] = item
	}
	return stackitem.NewArray(res)
}

// ToStackItem creates [stackitem.Item] representing [Signer]. It never
// returns an error. It implements [stackitem.Convertible] interface.
func (s *Signer) ToStackItem() (stackitem.Item, error) {
	contracts := make([]stackitem.Item, len(s.AllowedContracts))
	for j, c := range s.AllowedContracts {
		contracts[j] = stackitem.NewByteArray(c.BytesBE())
	}
	groups := make([]stackitem.Item, len(s.AllowedGroups))
	for j, g := range s.AllowedGroups {
		groups[j] = stackitem.NewByteArray(g.Bytes())
	}
	rules := make([]stackitem.Item, len(s.Rules))
	for j, r := range s.Rules {
		rules[j] = r.ToStackItem()
	}
	return stackitem.NewArray([]stackitem.Item{
		stackitem.NewByteArray(s.Account.BytesBE()),
		stackitem.NewBigInteger(big.NewInt(int64(s.Scopes))),
		stackitem.NewArray(contracts),
		stackitem.NewArray(groups),
		stackitem.NewArray(rules),
	}), nil
}

// FromStackItem retrieves fields of [Signer] from the given
// [stackitem.Item] or returns an error if it's not possible to do so. It
// implements [stackitem.Convertible] interface.
func (s *Signer) FromStackItem(item stackitem.Item) error {
	arr, ok := item.Value().([]stackitem.Item)
	if !ok {
		return errors.New("not an array")
	}
	if len(arr) != 5 {
		return errors.New("wrong number of structure elements")
	}
	account, err := stackitem.ToUint160(arr[0])
	if err != nil {
		return fmt.Errorf("field Account: %w", err)
	}
	scopes, err := stackitem.ToUint8(arr[1])
	if err != nil {
		return fmt.Errorf("field Scopes: %w", err)
	}
	contractsArr, ok := arr[2].Value().([]stackitem.Item)
	if !ok {
		return errors.New("field AllowedContracts: not an array")
	}
	if len(contractsArr) > maxSubitems {
		return errors.New("field AllowedContracts: too many elements")
	}
	contracts := make([]util.Uint160, len(contractsArr))
	for i := range contractsArr {
		contracts[i], err = stackitem.ToUint160(contractsArr[i])
		if err != nil {
			return fmt.Errorf("field AllowedContracts: element %d: %w", i, err)
		}
	}
	groupsArr, ok := arr[3].Value().([]stackitem.Item)
	if !ok {
		return errors.New("field AllowedGroups: not an array")
	}
	if len(groupsArr) > maxSubitems {
		return errors.New("field AllowedGroups: too many elements")
	}
	groups := make([]*keys.PublicKey, len(groupsArr))
	for i := range groupsArr {
		b, err := groupsArr[i].TryBytes()
		if err != nil {
			return fmt.Errorf("field AllowedGroups: element %d: %w", i, err)
		}
		groups[i], err = keys.NewPublicKeyFromBytes(b, elliptic.P256())
		if err != nil {
			return fmt.Errorf("field AllowedGroups: element %d: %w", i, err)
		}
	}
	rulesArr, ok := arr[4].Value().([]stackitem.Item)
	if !ok {
		return errors.New("field Rules: not an array")
	}
	if len(rulesArr) > maxSubitems {
		return errors.New("field Rules: too many elements")
	}
	rules := make([]WitnessRule, len(rulesArr))
	for i := range rulesArr {
		if err := rules[i].FromStackItem(rulesArr[i]); err != nil {
			return fmt.Errorf("field Rules: element %d: %w", i, err)
		}
	}

	s.Account = account
	s.Scopes = WitnessScope(scopes)
	s.AllowedContracts = contracts
	s.AllowedGroups = groups
	s.Rules = rules
	return nil
}

// ToSCParameter creates [smartcontract.Parameter] representing [Signer]. It
// implements [smartcontract.Convertible] interface so that [Signer] could be
// used with invokers.
func (s *Signer) ToSCParameter() (smartcontract.Parameter, error) {
	contracts := make([]smartcontract.Parameter, len(s.AllowedContracts))
	for i, c := range s.AllowedContracts {
		contracts[i] = smartcontract.Parameter{Type: smartcontract.Hash160Type, Value: c}
	}
	groups := make([]smartcontract.Parameter, len(s.AllowedGroups))
	for i, g := range s.AllowedGroups {
		groups[i] = smartcontract.Parameter{Type: smartcontract.PublicKeyType, Value: g.Bytes()}
	}
	rules := make([]smartcontract.Parameter, len(s.Rules))
	for i := range s.Rules {
		prm, err := s.Rules[i].ToSCParameter()
		if err != nil {
			return smartcontract.Parameter{}, fmt.Errorf("field Rules: element %d: %w", i, err)
		}
		rules[i] = prm
	}
	return smartcontract.Parameter{Type: smartcontract.ArrayType, Value: []smartcontract.Parameter{
		{Type: smartcontract.Hash160Type, Value: s.Account},
		{Type: smartcontract.IntegerType, Value: big.NewInt(int64(s.Scopes))},
		{Type: smartcontract.ArrayType, Value: contracts},
		{Type: smartcontract.ArrayType, Value: groups},
		{Type: smartcontract.ArrayType, Value: rules},
	}}, nil
}

// Copy creates a deep copy of the Signer.
func (c *Signer) Copy() *Signer {
	if c == nil {
		return nil
	}
	cp := *c
	cp.AllowedContracts = slices.Clone(c.AllowedContracts)
	cp.AllowedGroups = keys.PublicKeys(c.AllowedGroups).Copy()
	if c.Rules != nil {
		cp.Rules = make([]WitnessRule, len(c.Rules))
		for i, rule := range c.Rules {
			cp.Rules[i] = *rule.Copy()
		}
	}

	return &cp
}
