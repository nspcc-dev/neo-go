package invalid4

import "github.com/nspcc-dev/neo-go/pkg/interop/runtime"

func Main() {
	// Inconsistent event params usages (different field layout throughout the usages).
	runtime.Notify("SomeEvent", struct{ Field1 int }{Field1: 123})
	runtime.Notify("SomeEvent", struct{ Field2 string }{Field2: "str"})
}
