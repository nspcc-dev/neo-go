package testdata

import "github.com/nspcc-dev/neo-go/pkg/interop"

func Verify() bool {
	return true
}

func OnNEP17Payment(from interop.Hash160, amount int, data interface{}) {
}
