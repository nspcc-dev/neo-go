package nested1

import (
	"github.com/nspcc-dev/neo-go/pkg/compiler/testdata/globalvar/nested2"
	alias "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/globalvar/nested3"
)

// Unused shouldn't produce any code if unused.
var Unused = 11

// A should produce call to f and should not be DROPped if C is used. It uses
// aliased package var as an argument to check analizator.
var A = f(alias.Argument)

// B should produce call to f and be DROPped if unused. It uses foreign package var as an argument
// to check analizator.
var B = f(nested2.Argument)

// C shouldn't produce any code if unused. It uses
var C = A + nested2.A + nested2.Unique

func f(i int) int {
	return i
}

// F is used for nested calls check.
func F(i int) int {
	return i
}
