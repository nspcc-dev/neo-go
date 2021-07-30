package transaction

import (
	"errors"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// The maximum number of AllowedContracts or AllowedGroups.
const maxSubitems = 16

// Signer implements a Transaction signer.
type Signer struct {
	Account          util.Uint160      `json:"account"`
	Scopes           WitnessScope      `json:"scopes"`
	AllowedContracts []util.Uint160    `json:"allowedcontracts,omitempty"`
	AllowedGroups    []*keys.PublicKey `json:"allowedgroups,omitempty"`
}

// EncodeBinary implements Serializable interface.
func (c *Signer) EncodeBinary(bw io.BinaryWriter) {
	bw.WriteBytes(c.Account[:])
	bw.WriteB(byte(c.Scopes))
	if c.Scopes&CustomContracts != 0 {
		bw.WriteArray(c.AllowedContracts)
	}
	if c.Scopes&CustomGroups != 0 {
		bw.WriteArray(c.AllowedGroups)
	}
}

// DecodeBinary implements Serializable interface.
func (c *Signer) DecodeBinary(br *io.BinReader) {
	br.ReadBytes(c.Account[:])
	c.Scopes = WitnessScope(br.ReadB())
	if c.Scopes & ^(Global|CalledByEntry|CustomContracts|CustomGroups|None) != 0 {
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
}
