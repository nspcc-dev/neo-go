package invalid6

import "github.com/nspcc-dev/neo-go/pkg/interop/runtime"

type SomeStruct struct {
	Field int
	// RPC binding generator will convert this field into exported, which matches
	// exactly the existing Field.
	field int
}

func Main() {
	runtime.Notify("SomeEvent", SomeStruct{Field: 123, field: 123})
}
