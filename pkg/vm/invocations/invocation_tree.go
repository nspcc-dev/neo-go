package invocations

import (
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// Tree represents a tree with script hashes; when traversing it,
// you can see how contracts called each other.
type Tree struct {
	Current util.Uint160 `json:"hash"`
	Calls   []*Tree      `json:"call,omitempty"`
}
