package funccall

import (
	"github.com/nspcc-dev/neo-go/pkg/compiler/testdata/globalvar/nested1"
	"github.com/nspcc-dev/neo-go/pkg/compiler/testdata/globalvar/nested2"
	alias "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/globalvar/nested3"
)

// F should be called from the main package to check usage analyzer against
// nested constructions handling.
func F() int {
	return nested1.F(nested2.Argument + alias.Argument)
}

// GetAge calls method on the global struct.
func GetAge() int {
	return alias.Anna.GetAge()
}
