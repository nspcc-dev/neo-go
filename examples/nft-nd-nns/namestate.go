package nns

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/runtime"
)

// NameState represents domain name state.
type NameState struct {
	Owner      interop.Hash160
	Name       string
	Expiration int
	Admin      interop.Hash160
}

// ensureNotExpired panics if domain name is expired.
func (n NameState) ensureNotExpired() {
	if runtime.GetTime() >= n.Expiration {
		panic("name has expired")
	}
}

// checkAdmin panics if script container is not signed by the domain name admin.
func (n NameState) checkAdmin() {
	if runtime.CheckWitness(n.Owner) {
		return
	}
	if n.Admin == nil || !runtime.CheckWitness(n.Admin) {
		panic("not witnessed by admin")
	}
}
