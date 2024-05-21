/*
Package roles provides an interface to RoleManagement native contract.
Role management contract is used by committee to designate some nodes as
providing some service on the network.
*/
package roles

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/interop/neogointernal"
)

// Hash represents RoleManagement contract hash.
const Hash = "\xe2\x95\xe3\x91\x54\x4c\x17\x8a\xd9\x4f\x03\xec\x4d\xcd\xff\x78\x53\x4e\xcf\x49"

// Role represents a node role.
type Role byte

// Various node roles.
const (
	StateValidator Role = 4
	Oracle         Role = 8
	NeoFSAlphabet  Role = 16
	P2PNotary      Role = 32
)

// GetDesignatedByRole represents `getDesignatedByRole` method of RoleManagement native contract.
func GetDesignatedByRole(r Role, height uint32) []interop.PublicKey {
	return neogointernal.CallWithToken(Hash, "getDesignatedByRole",
		int(contract.ReadStates), r, height).([]interop.PublicKey)
}

// DesignateAsRole represents `designateAsRole` method of RoleManagement native contract.
func DesignateAsRole(r Role, pubs []interop.PublicKey) {
	neogointernal.CallWithTokenNoRet(Hash, "designateAsRole",
		int(contract.States|contract.AllowNotify), r, pubs)
}
