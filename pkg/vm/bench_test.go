package vm

import (
	"encoding/base64"
	"strconv"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/require"
)

func benchScript(t *testing.B, script []byte) {
	for n := 0; n < t.N; n++ {
		t.StopTimer()
		vm := load(script)
		t.StartTimer()
		err := vm.Run()
		t.StopTimer()
		require.NoError(t, err)
		t.StartTimer()
	}
}

// Shared as is by @ixje once upon a time (compiled from Python).
func BenchmarkScriptFibonacci(t *testing.B) {
	var script = []byte{87, 5, 0, 16, 112, 17, 113, 105, 104, 18, 192, 114, 16, 115, 34, 28, 104, 105, 158, 116, 106, 108, 75,
		217, 48, 38, 5, 139, 34, 5, 207, 34, 3, 114, 105, 112, 108, 113, 107, 17, 158, 115, 107, 12, 2, 94, 1,
		219, 33, 181, 36, 222, 106, 64}
	benchScript(t, script)
}

func BenchmarkScriptNestedRefCount(t *testing.B) {
	b64script := "whBNEcARTRHAVgEB/gGdYBFNEU0SwFMSwFhKJPNFUUVFRQ=="
	script, err := base64.StdEncoding.DecodeString(b64script)
	require.NoError(t, err)
	benchScript(t, script)
}

func BenchmarkScriptPushPop(t *testing.B) {
	for _, i := range []int{4, 16, 128, 1024} {
		t.Run(strconv.Itoa(i), func(t *testing.B) {
			var script = make([]byte, i*2)
			for p := 0; p < i; p++ {
				script[p] = byte(opcode.PUSH1)
				script[i+p] = byte(opcode.DROP)
			}
			benchScript(t, script)
		})
	}
}
