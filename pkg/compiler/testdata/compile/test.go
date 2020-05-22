package compile

import "github.com/nspcc-dev/neo-go/pkg/interop/runtime"

func Main() {
	runtime.Notify("Hello world!")
}
