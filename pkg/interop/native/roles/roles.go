package roles

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
)

// Hash represents RoleManagement contract hash.
const Hash = "\x02\x8b\x00\x50\x70\xb6\x0d\xf1\xc8\xe2\x09\x78\x7b\x49\xce\xbb\x71\x14\x7b\x59"

// Role represents node role.
type Role byte

// Various node roles.
const (
	StateValidator Role = 4
	Oracle         Role = 8
	P2PNotary      Role = 128
)

// GetDesignatedByRole represents `getDesignatedByRole` method of RoleManagement native contract.
func GetDesignatedByRole(r Role, height uint32) []interop.PublicKey {
	return contract.Call(interop.Hash160(Hash), "getDesignatedByRole",
		contract.ReadStates, r, height).([]interop.PublicKey)
}

// DesignateAsRole represents `designateAsRole` method of RoleManagement native contract.
func DesignateAsRole(r Role, pubs []interop.PublicKey) {
	contract.Call(interop.Hash160(Hash), "designateAsRole",
		contract.States, r, pubs)
}
