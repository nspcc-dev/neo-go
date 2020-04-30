package transaction

import (
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// The maximum number of AllowedContracts or AllowedGroups
const maxSubitems = 16

// Cosigner implements a Transaction cosigner.
type Cosigner struct {
	Account          util.Uint160      `json:"account"`
	Scopes           WitnessScope      `json:"scopes"`
	AllowedContracts []util.Uint160    `json:"allowedContracts,omitempty"`
	AllowedGroups    []*keys.PublicKey `json:"allowedGroups,omitempty"`
}

// EncodeBinary implements Serializable interface.
func (c *Cosigner) EncodeBinary(bw *io.BinWriter) {
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
func (c *Cosigner) DecodeBinary(br *io.BinReader) {
	br.ReadBytes(c.Account[:])
	c.Scopes = WitnessScope(br.ReadB())
	if c.Scopes&CustomContracts != 0 {
		br.ReadArray(&c.AllowedContracts, maxSubitems)
	}
	if c.Scopes&CustomGroups != 0 {
		br.ReadArray(&c.AllowedGroups, maxSubitems)
	}
}
