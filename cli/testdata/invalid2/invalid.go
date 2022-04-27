// invalid is an example of a contract which doesn't pass event check.
package invalid2

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/runtime"
)

// Notify1 emits a correctly typed event.
func Notify1() bool {
	runtime.Notify("Event", interop.Hash160{1, 2, 3})

	return true
}

// Notify2 emits an invalid event (extra parameter).
func Notify2() bool {
	runtime.Notify("Event", interop.Hash160{1, 2, 3}, "extra parameter")

	return true
}
