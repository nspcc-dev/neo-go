package vm

import (
	"encoding/base64"
	"strconv"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/vmstate"
	"github.com/stretchr/testify/require"
)

func benchScript(t *testing.B, script []byte) {
	for range t.N {
		t.StopTimer()
		vm := load(script)
		t.StartTimer()
		err := vm.Run()
		t.StopTimer()
		require.NoError(t, err)
		require.Equal(t, vmstate.Halt, vm.State())
		t.StartTimer()
	}
}

func benchBase64Script(t *testing.B, base64Script string) {
	b, err := base64.StdEncoding.DecodeString(base64Script)
	require.NoError(t, err)
	benchScript(t, b)
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
			for p := range i {
				script[p] = byte(opcode.PUSH1)
				script[i+p] = byte(opcode.DROP)
			}
			benchScript(t, script)
		})
	}
}

func BenchmarkIsSignatureContract(t *testing.B) {
	b64script := "DCED2eixa9myLTNF1tTN4xvhw+HRYVMuPQzOy5Xs4utYM25BVuezJw=="
	script, err := base64.StdEncoding.DecodeString(b64script)
	require.NoError(t, err)
	for range t.N {
		_ = IsSignatureContract(script)
	}
}

// BenchmarkNeoIssue2528Compat is a port of benchmark for the case described in
// https://github.com/neo-project/neo/issues/2528.
func BenchmarkNeoIssue2528Compat(t *testing.B) {
	// L01: INITSLOT 1, 0
	// L02: NEWARRAY0
	// L03: DUP
	// L04: DUP
	// L05: PUSHINT16 2043
	// L06: STLOC 0
	// L07: PUSH1
	// L08: PACK
	// L09: LDLOC 0
	// L10: DEC
	// L11: STLOC 0
	// L12: LDLOC 0
	// L13: JMPIF_L L07
	// L14: PUSH1
	// L15: PACK
	// L16: APPEND
	// L17: PUSHINT32 38000
	// L18: STLOC 0
	// L19: PUSH0
	// L20: PICKITEM
	// L21: LDLOC 0
	// L22: DEC
	// L23: STLOC 0
	// L24: LDLOC 0
	// L25: JMPIF_L L19
	// L26: DROP
	benchBase64Script(t, "VwEAwkpKAfsHdwARwG8AnXcAbwAl9////xHAzwJwlAAAdwAQzm8AnXcAbwAl9////0U=")
}

// BenchmarkNeoVMIssue418Compat is a port of benchmark for the case described in
// https://github.com/neo-project/neo-vm/issues/418.
func BenchmarkNeoVMIssue418Compat(t *testing.B) {
	// L00: NEWARRAY0
	// L01: PUSH0
	// L02: PICK
	// L03: PUSH1
	// L04: PACK
	// L05: PUSH1
	// L06: PICK
	// L07: PUSH1
	// L08: PACK
	// L09: INITSSLOT 1
	// L10: PUSHINT16 510
	// L11: DEC
	// L12: STSFLD0
	// L13: PUSH1
	// L14: PICK
	// L15: PUSH1
	// L16: PICK
	// L17: PUSH2
	// L18: PACK
	// L19: REVERSE3
	// L20: PUSH2
	// L21: PACK
	// L22: LDSFLD0
	// L23: DUP
	// L24: JMPIF L11
	// L25: DROP
	// L26: ROT
	// L27: DROP
	benchBase64Script(t, "whBNEcARTRHAVgEB/gGdYBFNEU0SwFMSwFhKJPNFUUU=")
}

// BenchmarkNeoIssue2723Compat is a port of benchmark for the case described in
// https://github.com/neo-project/neo/issues/2723.
func BenchmarkNeoIssue2723Compat(t *testing.B) {
	// L00: INITSSLOT 1
	// L01: PUSHINT32 130000
	// L02: STSFLD 0
	// L03: PUSHINT32 1048576
	// L04: NEWBUFFER
	// L05: DROP
	// L06: LDSFLD 0
	// L07: DEC
	// L08: DUP
	// L09: STSFLD 0
	// L10: JMPIF L03
	benchBase64Script(t, "VgEC0PsBAGcAAgAAEACIRV8AnUpnACTz")
	// TODO
}

// BenchmarkPoCNewBufferCompat is a port of benchmark introduced in
// https://github.com/neo-project/neo/pull/3512.
func BenchmarkPoCNewBufferCompat(t *testing.B) {
	// INITSLOT 0100
	// PUSHINT32 23000000
	// STLOC 00
	// PUSHINT32 1048576
	// NEWBUFFER
	// DROP
	// LDLOC 00
	// DEC
	// STLOC 00
	// LDLOC 00
	// JMPIF_L f2ffffff
	// CLEAR
	// RET
	benchBase64Script(t, "VwEAAsDzXgF3AAIAABAAiEVvAJ13AG8AJfL///9JQA==")
	// TODO:
	// BenchmarkPoCNewBufferCompat
	//    bench_test.go:20:
	//        	Error Trace:	/home/anna/Documents/GitProjects/nspcc-dev/neo-go/pkg/vm/bench_test.go:20
	//        	            				/home/anna/Documents/GitProjects/nspcc-dev/neo-go/pkg/vm/bench_test.go:29
	//        	            				/home/anna/Documents/GitProjects/nspcc-dev/neo-go/pkg/vm/bench_test.go:168
	//        	Error:      	Received unexpected error:
	//        	            	at instruction 15 (NEWBUFFER): invalid size
	//        	Test:       	BenchmarkPoCNewBufferCompat
}

// BenchmarkPoCCatCompat is a port of benchmark introduced in
// https://github.com/neo-project/neo/pull/3512.
func BenchmarkPoCCatCompat(t *testing.B) {
	// INITSLOT 0100
	// PUSHINT32 1048575
	// NEWBUFFER
	// PUSH1
	// NEWBUFFER
	// PUSHINT32 133333337
	// STLOC 00
	// OVER
	// OVER
	// CAT
	// DROP
	// LDLOC 00
	// DEC
	// STLOC 00
	// LDLOC 00
	// JMPIF_L f5ffffff
	// CLEAR
	// RET
	benchBase64Script(t, "VwEAAv//DwCIEYgCWYHyB3cAS0uLRW8AnXcAbwAl9f///0lA")
	// TODO:
	// BenchmarkPoCCatCompat
	//    bench_test.go:20:
	//        	Error Trace:	/home/anna/Documents/GitProjects/nspcc-dev/neo-go/pkg/vm/bench_test.go:20
	//        	            				/home/anna/Documents/GitProjects/nspcc-dev/neo-go/pkg/vm/bench_test.go:29
	//        	            				/home/anna/Documents/GitProjects/nspcc-dev/neo-go/pkg/vm/bench_test.go:192
	//        	Error:      	Received unexpected error:
	//        	            	at instruction 8 (NEWBUFFER): invalid size
	//        	Test:       	BenchmarkPoCCatCompat
}

// BenchmarkPoCLeftCompat is a port of benchmark introduced in
// https://github.com/neo-project/neo/pull/3512.
func BenchmarkPoCLeftCompat(t *testing.B) {
	// INITSLOT 0100
	// PUSHINT32 1048576
	// NEWBUFFER
	// PUSHINT32 133333337
	// STLOC 00
	// DUP
	// PUSHINT32 1048576
	// LEFT
	// DROP
	// LDLOC 00
	// DEC
	// STLOC 00
	// LDLOC 00
	// JMPIF_L f1ffffff
	// CLEAR
	// RET
	benchBase64Script(t, "VwEAAgAAEACIAlmB8gd3AEoCAAAQAI1FbwCddwBvACXx////SUA=")
	// TODO:
	// BenchmarkPoCLeftCompat
	//    bench_test.go:20:
	//        	Error Trace:	/home/anna/Documents/GitProjects/nspcc-dev/neo-go/pkg/vm/bench_test.go:20
	//        	            				/home/anna/Documents/GitProjects/nspcc-dev/neo-go/pkg/vm/bench_test.go:29
	//        	            				/home/anna/Documents/GitProjects/nspcc-dev/neo-go/pkg/vm/bench_test.go:214
	//        	Error:      	Received unexpected error:
	//        	            	at instruction 8 (NEWBUFFER): invalid size
	//        	Test:       	BenchmarkPoCLeftCompat
}

// BenchmarkPoCRightCompat is a port of benchmark introduced in
// https://github.com/neo-project/neo/pull/3512.
func BenchmarkPoCRightCompat(t *testing.B) {
	// INITSLOT 0100
	// PUSHINT32 1048576
	// NEWBUFFER
	// PUSHINT32 133333337
	// STLOC 00
	// DUP
	// PUSHINT32 1048576
	// RIGHT
	// DROP
	// LDLOC 00
	// DEC
	// STLOC 00
	// LDLOC 00
	// JMPIF_L f1ffffff
	// CLEAR
	// RET
	benchBase64Script(t, "VwEAAgAAEACIAlmB8gd3AEoCAAAQAI5FbwCddwBvACXx////SUA=")
	// TODO:
	// BenchmarkPoCRightCompat
	//    bench_test.go:20:
	//        	Error Trace:	/home/anna/Documents/GitProjects/nspcc-dev/neo-go/pkg/vm/bench_test.go:20
	//        	            				/home/anna/Documents/GitProjects/nspcc-dev/neo-go/pkg/vm/bench_test.go:29
	//        	            				/home/anna/Documents/GitProjects/nspcc-dev/neo-go/pkg/vm/bench_test.go:236
	//        	Error:      	Received unexpected error:
	//        	            	at instruction 8 (NEWBUFFER): invalid size
	//        	Test:       	BenchmarkPoCRightCompat
}

// BenchmarkPoCReverseNCompat is a port of benchmark introduced in
// https://github.com/neo-project/neo/pull/3512.
func BenchmarkPoCReverseNCompat(t *testing.B) {
	// INITSLOT 0100
	// PUSHINT16 2040
	// STLOC 00
	// PUSHDATA1 aaabbbbbbbbbcccccccdddddddeeeeeeefffffff
	// LDLOC 00
	// DEC
	// STLOC 00
	// LDLOC 00
	// JMPIF_L cfffffff
	// PUSHINT32 23000000
	// STLOC 00
	// PUSHINT16 2040
	// REVERSEN
	// LDLOC 00
	// DEC
	// STLOC 00
	// LDLOC 00
	// JMPIF_L f5ffffff
	// CLEAR
	// RET
	benchBase64Script(t, "VwEAAfgHdwAMKGFhYWJiYmJiYmJiYmNjY2NjY2NkZGRkZGRkZWVlZWVlZWZmZmZmZmZvAJ13AG8AJc////8CwPNeAXcAAfgHVW8AnXcAbwAl9f///0lA")
}

// BenchmarkPoCSubstrCompat is a port of benchmark introduced in
// https://github.com/neo-project/neo/pull/3512.
func BenchmarkPoCSubstrCompat(t *testing.B) {
	// INITSLOT 0100
	// PUSHINT32 1048576
	// NEWBUFFER
	// PUSHINT32 133333337
	// STLOC 00
	// DUP
	// PUSH0
	// PUSHINT32 1048576
	// SUBSTR
	// DROP
	// LDLOC 00
	// DEC
	// STLOC 00
	// LDLOC 00
	// JMPIF_L f0ffffff
	// CLEAR
	// RET
	benchBase64Script(t, "VwEAAgAAEACIAlmB8gd3AEoQAgAAEACMRW8AnXcAbwAl8P///0lA")
	// TODO:
	//     bench_test.go:20:
	//        	Error Trace:	/home/anna/Documents/GitProjects/nspcc-dev/neo-go/pkg/vm/bench_test.go:20
	//        	            				/home/anna/Documents/GitProjects/nspcc-dev/neo-go/pkg/vm/bench_test.go:29
	//        	            				/home/anna/Documents/GitProjects/nspcc-dev/neo-go/pkg/vm/bench_test.go:285
	//        	Error:      	Received unexpected error:
	//        	            	at instruction 8 (NEWBUFFER): invalid size
	//        	Test:       	BenchmarkPoCSubstrCompat
}

// BenchmarkPoCNewArray is a port of benchmark introduced in
// https://github.com/neo-project/neo/pull/3512.
func BenchmarkPoCNewArray(t *testing.B) {
	// INITSLOT 0100
	// PUSHINT32 1333333337
	// STLOC 00
	// PUSHINT16 2040
	// NEWARRAY
	// DROP
	// LDLOC 00
	// DEC
	// STLOC 00
	// LDLOC 00
	// JMPIF_L f4ffffff
	// RET
	benchBase64Script(t, "VwEAAlkNeU93AAH4B8NFbwCddwBvACX0////QA==")
}

// BenchmarkPoCNewStructCompat is a port of benchmark introduced in
// https://github.com/neo-project/neo/pull/3512.
func BenchmarkPoCNewStructCompat(t *testing.B) {
	// INITSLOT 0100
	// PUSHINT32 1333333337
	// STLOC 00
	// PUSHINT16 2040
	// NEWSTRUCT
	// DROP
	// LDLOC 00
	// DEC
	// STLOC 00
	// LDLOC 00
	// JMPIF_L f4ffffff
	// RET
	benchBase64Script(t, "VwEAAlkNeU93AAH4B8ZFbwCddwBvACX0////QA==")
}

// BenchmarkPoCRollCompat is a port of benchmark introduced in
// https://github.com/neo-project/neo/pull/3512.
func BenchmarkPoCRollCompat(t *testing.B) {
	// INITSLOT 0100
	// PUSHINT16 2040
	// STLOC 00
	// PUSHDATA1 aaabbbbbbbbbcccccccdddddddeeeeeeefffffff
	// LDLOC 00
	// DEC
	// STLOC 00
	// LDLOC 00
	// JMPIF_L cfffffff
	// PUSHINT32 23000000
	// STLOC 00
	// PUSHINT16 2039
	// ROLL
	// LDLOC 00
	// DEC
	// STLOC 00
	// LDLOC 00
	// JMPIF_L f5ffffff
	// CLEAR
	// RET
	benchBase64Script(t, "VwEAAfgHdwAMKGFhYWJiYmJiYmJiYmNjY2NjY2NkZGRkZGRkZWVlZWVlZWZmZmZmZmZvAJ13AG8AJc////8CwPNeAXcAAfcHUm8AnXcAbwAl9f///0lA")
}

// BenchmarkPoCXDropCompat is a port of benchmark introduced in
// https://github.com/neo-project/neo/pull/3512.
func BenchmarkPoCXDropCompat(t *testing.B) {
	// INITSLOT 0100
	// PUSHINT16 2040
	// STLOC 00
	// PUSHDATA1 aaabbbbbbbbbcccccccdddddddeeeeeeefffffff
	// LDLOC 00
	// DEC
	// STLOC 00
	// LDLOC 00
	// JMPIF_L cfffffff
	// PUSHINT32 23000000
	// STLOC 00
	// PUSHINT16 2039
	// XDROP
	// DUP
	// LDLOC 00
	// DEC
	// STLOC 00
	// LDLOC 00
	// JMPIF_L f4ffffff
	// CLEAR
	// RET
	benchBase64Script(t, "VwEAAfgHdwAMKGFhYWJiYmJiYmJiYmNjY2NjY2NkZGRkZGRkZWVlZWVlZWZmZmZmZmZvAJ13AG8AJc////8CwPNeAXcAAfcHSEpvAJ13AG8AJfT///9JQA==")
}

// BenchmarkPoCMemCpyBenchmark is a port of benchmark introduced in
// https://github.com/neo-project/neo/pull/3512.
func BenchmarkPoCMemCpyBenchmark(t *testing.B) {
	// INITSLOT 0100
	// PUSHINT32 1048576
	// NEWBUFFER
	// PUSHINT32 1048576
	// NEWBUFFER
	// PUSHINT32 133333337
	// STLOC 00
	// OVER
	// PUSH0
	// PUSH2
	// PICK
	// PUSH0
	// PUSHINT32 1048576
	// MEMCPY
	// LDLOC 00
	// DEC
	// STLOC 00
	// LDLOC 00
	// JMPIF_L eeffffff
	// CLEAR
	// RET
	benchBase64Script(t, "VwEAAgAAEACIAgAAEACIAlmB8gd3AEsQEk0QAgAAEACJbwCddwBvACXu////SUA=")
	// TODO:
	//     bench_test.go:20:
	//        	Error Trace:	/home/anna/Documents/GitProjects/nspcc-dev/neo-go/pkg/vm/bench_test.go:20
	//        	            				/home/anna/Documents/GitProjects/nspcc-dev/neo-go/pkg/vm/bench_test.go:29
	//        	            				/home/anna/Documents/GitProjects/nspcc-dev/neo-go/pkg/vm/bench_test.go:401
	//        	Error:      	Received unexpected error:
	//        	            	at instruction 8 (NEWBUFFER): invalid size
	//        	Test:       	BenchmarkPoCMemCpyBenchmark
}

// BenchmarkPoCUnpackCompat is a port of benchmark introduced in
// https://github.com/neo-project/neo/pull/3512.
func BenchmarkPoCUnpackCompat(t *testing.B) {
	// INITSLOT 0200
	// PUSHINT16 1010
	// NEWARRAY
	// STLOC 01
	// PUSHINT32 1333333337
	// STLOC 00
	// LDLOC 01
	// UNPACK
	// CLEAR
	// LDLOC 00
	// DEC
	// STLOC 00
	// LDLOC 00
	// JMPIF_L f5ffffff
	// RET
	benchBase64Script(t, "VwIAAfIDw3cBAlkNeU93AG8BwUlvAJ13AG8AJfX///9A")
}

// BenchmarkPoCGetScriptContainerCompat is a port of benchmark introduced in
// https://github.com/neo-project/neo/pull/3512.
func BenchmarkPoCGetScriptContainerCompat(t *testing.B) {
	// SYSCALL System.Runtime.GetScriptContainer
	// DROP
	// JMP fa
	benchBase64Script(t, "QS1RCDBFIvo=")
}
