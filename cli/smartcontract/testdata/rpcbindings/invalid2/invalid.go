package invalid2

import "github.com/nspcc-dev/neo-go/pkg/interop/runtime"

func Main() {
	runtime.Notify("SomeEvent", "p1", "p2")
}
