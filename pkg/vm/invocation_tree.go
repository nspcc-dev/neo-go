package vm

import (
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// InvocationTree represents a tree with script hashes, traversing it
// you can see how contracts called each other.
type InvocationTree struct {
	Current util.Uint160
	Calls   []*InvocationTree
}
