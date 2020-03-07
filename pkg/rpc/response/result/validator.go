package result

import (
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// Validator used for the representation of
// state.Validator on the RPC Server.
type Validator struct {
	PublicKey keys.PublicKey `json:"publickey"`
	Votes     util.Fixed8    `json:"votes"`
	Active    bool           `json:"active"`
}
