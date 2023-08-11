package structs

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/runtime"
)

func Main() {
	runtime.Notify("! complicated name %$#", "str1")
}

func CrazyMap() {
	runtime.Notify("SomeMap", map[int][]map[string][]interop.Hash160{})
}

func Struct() {
	runtime.Notify("SomeStruct", struct {
		I int
		B bool
	}{I: 123, B: true})
}

func Array() {
	runtime.Notify("SomeArray", [][]int{})
}

// UnexportedField emits notification with unexported field that must be converted
// to exported in the resulting RPC binding.
func UnexportedField() {
	runtime.Notify("SomeUnexportedField", struct {
		i int
	}{i: 123})
}
