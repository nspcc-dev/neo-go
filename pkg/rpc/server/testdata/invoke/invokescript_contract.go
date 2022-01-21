package invoke

import "github.com/nspcc-dev/neo-go/pkg/interop/runtime"

// This contract is used to test `invokescript` and `invokefunction` RPC-calls
func Main() int {
	// h1 and h2 are just random uint160 hashes
	h1 := []byte{1, 12, 3, 14, 5, 6, 12, 13, 2, 14, 15, 13, 3, 14, 7, 9, 0, 0, 0, 0}
	if !runtime.CheckWitness(h1) {
		return 1
	}
	h2 := []byte{13, 15, 3, 2, 9, 0, 2, 1, 3, 7, 3, 4, 5, 2, 1, 0, 14, 6, 12, 9}
	if !runtime.CheckWitness(h2) {
		return 2
	}
	return 3
}
