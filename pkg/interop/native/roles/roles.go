/*
Package roles provides interface to RoleManagement native contract.
Role management contract is used by committee to designate some nodes as
providing some service on the network.
*/
package roles

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
)

// Hash represents RoleManagement contract hash.
const Hash = "\xe2\x95\xe3\x91\x54\x4c\x17\x8a\xd9\x4f\x03\xec\x4d\xcd\xff\x78\x53\x4e\xcf\x49"

// Role represents node role.
type Role byte

// Various node roles.
const (
	StateValidator Role = 4
	Oracle         Role = 8
	// P2PNotary is an extension of Neo protocol available on specifically configured NeoGo networks.
	P2PNotary Role = 128
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
