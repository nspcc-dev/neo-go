package invalid9

import "github.com/nspcc-dev/neo-go/pkg/interop/runtime"

type Pair struct {
	A int
	B int
}

func Main() {
	// Both calls emit a struct with the same field layout (same SC type, Array),
	// but the first one is named and the second one is anonymous, so their real
	// (Go) types differ.
	runtime.Notify("SomeEvent", Pair{A: 1, B: 2})
	runtime.Notify("SomeEvent", struct {
		A int
		B int
	}{A: 3, B: 4})
}
