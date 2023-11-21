package pkg1

import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/importcycle/pkg2"

var A int

func init() {
	pkg2.A = 1
}
