package notify

import "github.com/nspcc-dev/neo-go/pkg/interop/runtime"

// Value is the constant we use.
const Value = 42

// EmitEvent emits some event.
func EmitEvent() {
	emitPrivate()
}

func emitPrivate() {
	runtime.Notify("Event")
}
