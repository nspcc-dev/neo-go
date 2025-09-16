package transaction

import (
	"errors"
	"math/big"
	"slices"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
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
		res[i] = stackitem.NewArray([]stackitem.Item{
			stackitem.NewByteArray(s.Account.BytesBE()),
			stackitem.NewBigInteger(big.NewInt(int64(s.Scopes))),
			stackitem.NewArray(contracts),
			stackitem.NewArray(groups),
			stackitem.NewArray(rules),
		})
	}
	return stackitem.NewArray(res)
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
