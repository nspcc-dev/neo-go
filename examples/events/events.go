package events

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/runtime"
)

// NotifySomeBytes emits notification with ByteArray.
func NotifySomeBytes(arg []byte) {
	runtime.Notify("SomeBytes", arg)
}

// NotifySomeInteger emits notification with Integer.
func NotifySomeInteger(arg int) {
	runtime.Notify("SomeInteger", arg)
}

// NotifySomeString emits notification with String.
func NotifySomeString(arg string) {
	runtime.Notify("SomeString", arg)
}

// NotifySomeMap emits notification with Map.
func NotifySomeMap(arg map[string]int) {
	runtime.Notify("SomeMap", arg)
}

// NotifySomeCrazyMap emits notification with complicated Map.
func NotifySomeCrazyMap(arg map[int][]map[string][]interop.Hash160) {
	runtime.Notify("SomeCrazyMap", arg)
}

// NotifySomeArray emits notification with Array.
func NotifySomeArray(arg []int) {
	runtime.Notify("SomeArray", arg)
}
