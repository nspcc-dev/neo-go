package invalid9

import "github.com/nspcc-dev/neo-go/pkg/interop/runtime"

type NamedStruct struct {
	SomeInt int
}

func Main() NamedStruct {
	runtime.Notify("SomeEvent", []interface{}{123})
	return NamedStruct{SomeInt: 123}
}
