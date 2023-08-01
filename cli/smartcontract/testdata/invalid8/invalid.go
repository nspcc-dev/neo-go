package invalid8

import "github.com/nspcc-dev/neo-go/pkg/interop/runtime"

type SomeStruct1 struct {
	Field1 int
}

type SomeStruct2 struct {
	Field2 string
}

func Main() {
	// Inconsistent event params usages (different named types throughout the usages).
	runtime.Notify("SomeEvent", SomeStruct1{Field1: 123})
	runtime.Notify("SomeEvent", SomeStruct2{Field2: "str"})
}
