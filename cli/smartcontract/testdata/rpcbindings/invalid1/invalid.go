package invalid1

import "github.com/nspcc-dev/neo-go/pkg/interop/runtime"

func Main() {
	runtime.Notify("Non declared event")
}
