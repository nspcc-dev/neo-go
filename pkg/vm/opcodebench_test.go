package vm

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"strconv"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

const (
	maxScriptLen     = 2048 //math.MaxInt
	maxArrayDeep     = MaxStackSize
	maxArraySize     = MaxStackSize
	maxMapSize       = stackitem.MaxDeserialized
	maxByteArraySize = stackitem.MaxSize
	maxStructSize    = max(stackitem.MaxDeserialized, stackitem.MaxClonableNumOfItems)
	maxInteropSize   = stackitem.MaxSize
)

func benchOpcodeInt(t *testing.B, f func() *VM, fail bool) {
	for t.Loop() {
		t.StopTimer()
		v := f()
		t.StartTimer()
		err := v.Step()
		t.StopTimer()
		if fail {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
		t.StartTimer()
	}
}

func benchOpcode(t *testing.B, f func() *VM) {
	benchOpcodeInt(t, f, false)
}

func benchOpcodeFail(t *testing.B, f func() *VM) {
	benchOpcodeInt(t, f, true)
}

func opVM(op opcode.Opcode) func() *VM {
	return opParamVM(op, nil)
}

func opParamVM(op opcode.Opcode, param []byte) func() *VM {
	return opParamPushVM(op, param)
}

func opParamPushVM(op opcode.Opcode, param []byte, items ...any) func() *VM {
	return opParamSlotsPushVM(op, param, 0, 0, 0, items...)
}

func opParamSlotsPushVM(op opcode.Opcode, param []byte, sslot int, slotloc int, slotarg int, items ...any) func() *VM {
	return opParamSlotsPushReadOnlyVM(op, param, sslot, slotloc, slotarg, true, items...)
}

func opParamSlotsPushReadOnlyVM(op opcode.Opcode, param []byte, sslot int, slotloc int, slotarg int, isReadOnly bool, items ...any) func() *VM {
	return func() *VM {
		script := []byte{byte(op)}
		script = append(script, param...)
		v := load(script)
		v.SyscallHandler = func(_ *VM, _ uint32) error {
			return nil
		}
		if sslot != 0 {
			v.Context().sc.static.init(sslot, &v.refs)
		}
		if slotloc != 0 && slotarg != 0 {
			v.Context().local.init(slotloc, &v.refs)
			v.Context().arguments.init(slotarg, &v.refs)
		}
		for i := range items {
			item, ok := items[i].(stackitem.Item)
			if ok && isReadOnly {
				item = stackitem.DeepCopy(item, true)
			} else {
				item = stackitem.Make(items[i])
			}
			v.estack.PushVal(item)
		}
		return v
	}
}

func exceptParamPushVM(op opcode.Opcode, param []byte, ilen int, elen int, exception bool, items ...any) func() *VM {
	return func() *VM {
		regVMF := opParamPushVM(op, param, items...)
		v := regVMF()
		if ilen != 0 {
			eCtx := newExceptionHandlingContext(1, 2)
			v.Context().tryStack.PushVal(eCtx)
			for range ilen {
				v.call(v.Context(), 0)
			}
		} else if elen != 0 {
			for range elen {
				eCtx := newExceptionHandlingContext(1, 2)
				v.Context().tryStack.PushVal(eCtx)
			}
		}
		if exception {
			v.uncaughtException = &stackitem.Null{}
		}
		return v
	}
}

func zeroSlice(l int) []byte {
	return make([]byte, l)
}

func ffSlice(l int) []byte {
	var s = make([]byte, l)
	for i := range s {
		s[i] = 0xff
	}
	return s
}

func maxNumber() []byte {
	s := ffSlice(32)
	s[0] = 0
	return s
}

func bigNumber() []byte {
	s := maxNumber()
	s[31] = 0
	return s
}

func bigNegNumber() []byte {
	s := ffSlice(32)
	s[0] = 80
	return s
}

func arrayOfOnes(size int) []stackitem.Item {
	var elems = make([]stackitem.Item, size)
	for i := range elems {
		elems[i] = stackitem.Make(1)
	}
	return elems
}

func arrayOfIfaces(size int) []any {
	var elems = make([]any, size)
	for i := range elems {
		elems[i] = 1
	}
	return elems
}

func bigMap() *stackitem.Map {
	var m = stackitem.NewMap()
	for i := range 1024 {
		m.Add(stackitem.Make(i), stackitem.Make(i))
	}
	return m
}

func maxBytes() []byte {
	return zeroSlice(1024 * 1024)
}

func maxBuf() *stackitem.Buffer {
	return stackitem.NewBuffer(maxBytes())
}

func BenchmarkOpcodes(t *testing.B) {
	t.Run("NOP", func(t *testing.B) { benchOpcode(t, opVM(opcode.NOP)) })
	t.Run("PUSHINT8", func(t *testing.B) {
		t.Run("00", func(t *testing.B) { benchOpcode(t, opParamVM(opcode.PUSHINT8, []byte{0})) })
		t.Run("FF", func(t *testing.B) { benchOpcode(t, opParamVM(opcode.PUSHINT8, []byte{0xff})) })
	})
	t.Run("PUSHINT16", func(t *testing.B) {
		t.Run("0000", func(t *testing.B) { benchOpcode(t, opParamVM(opcode.PUSHINT16, []byte{0, 0})) })
		t.Run("FFFF", func(t *testing.B) { benchOpcode(t, opParamVM(opcode.PUSHINT16, []byte{0xff, 0xff})) })
	})
	t.Run("PUSHINT32", func(t *testing.B) {
		t.Run("0000...", func(t *testing.B) { benchOpcode(t, opParamVM(opcode.PUSHINT32, zeroSlice(4))) })
		t.Run("FFFF...", func(t *testing.B) { benchOpcode(t, opParamVM(opcode.PUSHINT32, ffSlice(4))) })
	})
	t.Run("PUSHINT64", func(t *testing.B) {
		t.Run("0000...", func(t *testing.B) { benchOpcode(t, opParamVM(opcode.PUSHINT64, zeroSlice(8))) })
		t.Run("FFFF...", func(t *testing.B) { benchOpcode(t, opParamVM(opcode.PUSHINT64, ffSlice(8))) })
	})
	t.Run("PUSHINT128", func(t *testing.B) {
		t.Run("0000...", func(t *testing.B) { benchOpcode(t, opParamVM(opcode.PUSHINT128, zeroSlice(16))) })
		t.Run("FFFF...", func(t *testing.B) { benchOpcode(t, opParamVM(opcode.PUSHINT128, ffSlice(16))) })
	})
	t.Run("PUSHINT256", func(t *testing.B) {
		t.Run("0000...", func(t *testing.B) { benchOpcode(t, opParamVM(opcode.PUSHINT256, zeroSlice(32))) })
		t.Run("FFFF...", func(t *testing.B) { benchOpcode(t, opParamVM(opcode.PUSHINT256, ffSlice(32))) })
	})
	t.Run("PUSHA", func(t *testing.B) {
		t.Run("small script", func(t *testing.B) { benchOpcode(t, opParamVM(opcode.PUSHA, zeroSlice(4))) })
		t.Run("big script", func(t *testing.B) { benchOpcode(t, opParamVM(opcode.PUSHA, zeroSlice(4+65536))) })
	})
	t.Run("PUSHNULL", func(t *testing.B) { benchOpcode(t, opVM(opcode.PUSHNULL)) })
	t.Run("PUSHDATA1", func(t *testing.B) {
		var oneSlice = []byte{1, 0}
		var maxSlice = zeroSlice(255)
		maxSlice = append([]byte{255}, maxSlice...)

		t.Run("1", func(t *testing.B) { benchOpcode(t, opParamVM(opcode.PUSHDATA1, oneSlice)) })
		t.Run("255", func(t *testing.B) { benchOpcode(t, opParamVM(opcode.PUSHDATA1, maxSlice)) })
	})
	t.Run("PUSHDATA2", func(t *testing.B) {
		const minLen = 256
		const maxLen = 65535
		var minSlice = zeroSlice(minLen + 2)
		var maxSlice = zeroSlice(maxLen + 2)

		binary.LittleEndian.PutUint16(minSlice, minLen)
		binary.LittleEndian.PutUint16(maxSlice, maxLen)

		t.Run("256", func(t *testing.B) { benchOpcode(t, opParamVM(opcode.PUSHDATA2, minSlice)) })
		t.Run("65535", func(t *testing.B) { benchOpcode(t, opParamVM(opcode.PUSHDATA2, maxSlice)) })
	})
	t.Run("PUSHDATA4", func(t *testing.B) {
		const minLen = 65536
		const maxLen = 1024 * 1024
		var minSlice = zeroSlice(minLen + 4)
		var maxSlice = zeroSlice(maxLen + 4)

		binary.LittleEndian.PutUint32(minSlice, minLen)
		binary.LittleEndian.PutUint32(maxSlice, maxLen)

		t.Run("64K", func(t *testing.B) { benchOpcode(t, opParamVM(opcode.PUSHDATA4, minSlice)) })
		t.Run("1M", func(t *testing.B) { benchOpcode(t, opParamVM(opcode.PUSHDATA4, maxSlice)) })
	})
	t.Run("PUSHM1", func(t *testing.B) { benchOpcode(t, opVM(opcode.PUSHM1)) })
	t.Run("PUSH0", func(t *testing.B) { benchOpcode(t, opVM(opcode.PUSH0)) })

	t.Run("JMP", func(t *testing.B) { benchOpcode(t, opParamVM(opcode.JMP, zeroSlice(1))) })
	t.Run("JMP_L", func(t *testing.B) { benchOpcode(t, opParamVM(opcode.JMPL, zeroSlice(4))) })

	var jmpifs = []struct {
		op   opcode.Opcode
		size int
	}{
		{opcode.JMPIF, 1},
		{opcode.JMPIFL, 4},
		{opcode.JMPIFNOT, 1},
		{opcode.JMPIFNOTL, 4},
	}
	for _, jmpif := range jmpifs {
		t.Run(jmpif.op.String(), func(t *testing.B) {
			t.Run("false", func(t *testing.B) { benchOpcode(t, opParamPushVM(jmpif.op, zeroSlice(jmpif.size), false)) })
			t.Run("true", func(t *testing.B) { benchOpcode(t, opParamPushVM(jmpif.op, zeroSlice(jmpif.size), true)) })
		})
	}

	var jmpcmps = []struct {
		op   opcode.Opcode
		size int
	}{
		{opcode.JMPEQ, 1},
		{opcode.JMPEQL, 4},
		{opcode.JMPNE, 1},
		{opcode.JMPNEL, 4},
		{opcode.JMPGT, 1},
		{opcode.JMPGTL, 4},
		{opcode.JMPGE, 1},
		{opcode.JMPGEL, 4},
		{opcode.JMPLT, 1},
		{opcode.JMPLTL, 4},
		{opcode.JMPLE, 1},
		{opcode.JMPLEL, 4},
	}
	for _, jmpcmp := range jmpcmps {
		t.Run(jmpcmp.op.String(), func(t *testing.B) {
			t.Run("false", func(t *testing.B) {
				t.Run("small", func(t *testing.B) { benchOpcode(t, opParamPushVM(jmpcmp.op, zeroSlice(jmpcmp.size), 1, 0)) })
				t.Run("big", func(t *testing.B) {
					benchOpcode(t, opParamPushVM(jmpcmp.op, zeroSlice(jmpcmp.size), maxNumber(), bigNumber()))
				})
			})
			t.Run("true", func(t *testing.B) {
				t.Run("small", func(t *testing.B) { benchOpcode(t, opParamPushVM(jmpcmp.op, zeroSlice(jmpcmp.size), 1, 1)) })
				t.Run("big", func(t *testing.B) {
					benchOpcode(t, opParamPushVM(jmpcmp.op, zeroSlice(jmpcmp.size), maxNumber(), maxNumber()))
				})
			})
		})
	}

	t.Run("CALL", func(t *testing.B) { benchOpcode(t, opParamVM(opcode.CALL, zeroSlice(1))) })
	t.Run("CALL_L", func(t *testing.B) { benchOpcode(t, opParamVM(opcode.CALLL, zeroSlice(4))) })
	t.Run("CALLA", func(t *testing.B) {
		t.Run("small script", func(t *testing.B) {
			p := stackitem.NewPointer(0, []byte{byte(opcode.CALLA)})
			benchOpcode(t, opParamPushVM(opcode.CALLA, nil, p))
		})
		t.Run("big script", func(t *testing.B) {
			prog := zeroSlice(65536)
			prog[0] = byte(opcode.CALLA)
			p := stackitem.NewPointer(0, prog)
			benchOpcode(t, opParamPushVM(opcode.CALLA, zeroSlice(65535), p))
		})
	})

	t.Run("ABORT", func(t *testing.B) { benchOpcodeFail(t, opVM(opcode.ABORT)) })
	t.Run("ASSERT", func(t *testing.B) {
		t.Run("true", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.ASSERT, nil, true)) })
		t.Run("false", func(t *testing.B) { benchOpcodeFail(t, opParamPushVM(opcode.ASSERT, nil, false)) })
	})

	t.Run("THROW", func(t *testing.B) {
		t.Run("0/1", func(t *testing.B) {
			benchOpcode(t, exceptParamPushVM(opcode.THROW, []byte{0, 0, 0, 0}, 0, 1, false, 1))
		})
		t.Run("0/16", func(t *testing.B) {
			benchOpcode(t, exceptParamPushVM(opcode.THROW, []byte{0, 0, 0, 0}, 0, 16, false, 1))
		})
		t.Run("255/0", func(t *testing.B) {
			benchOpcode(t, exceptParamPushVM(opcode.THROW, []byte{0, 0, 0, 0}, 255, 0, false, 1))
		})
		t.Run("1023/0", func(t *testing.B) {
			benchOpcode(t, exceptParamPushVM(opcode.THROW, []byte{0, 0, 0, 0}, 1023, 0, false, 1))
		})
	})
	t.Run("TRY", func(t *testing.B) { benchOpcode(t, opParamVM(opcode.TRY, []byte{1, 2, 0, 0})) })
	t.Run("TRY_L", func(t *testing.B) { benchOpcode(t, opParamVM(opcode.TRYL, []byte{1, 0, 0, 0, 2, 0, 0, 0})) })
	t.Run("ENDTRY", func(t *testing.B) {
		t.Run("1", func(t *testing.B) { benchOpcode(t, exceptParamPushVM(opcode.ENDTRY, []byte{0, 0, 0, 0}, 0, 1, false)) })
		t.Run("16", func(t *testing.B) { benchOpcode(t, exceptParamPushVM(opcode.ENDTRY, []byte{0, 0, 0, 0}, 0, 16, false)) })
	})
	t.Run("ENDTRY_L", func(t *testing.B) {
		t.Run("1", func(t *testing.B) { benchOpcode(t, exceptParamPushVM(opcode.ENDTRYL, []byte{0, 0, 0, 0}, 0, 1, false)) })
		t.Run("16", func(t *testing.B) {
			benchOpcode(t, exceptParamPushVM(opcode.ENDTRYL, []byte{0, 0, 0, 0}, 0, 16, false))
		})
	})
	t.Run("ENDFINALLY", func(t *testing.B) {
		t.Run("0/1", func(t *testing.B) {
			benchOpcode(t, exceptParamPushVM(opcode.ENDFINALLY, []byte{0, 0, 0, 0}, 0, 1, true))
		})
		t.Run("0/16", func(t *testing.B) {
			benchOpcode(t, exceptParamPushVM(opcode.ENDFINALLY, []byte{0, 0, 0, 0}, 0, 16, true))
		})
		t.Run("255/0", func(t *testing.B) {
			benchOpcode(t, exceptParamPushVM(opcode.ENDFINALLY, []byte{0, 0, 0, 0}, 255, 0, true))
		})
		t.Run("1023/0", func(t *testing.B) {
			benchOpcode(t, exceptParamPushVM(opcode.ENDFINALLY, []byte{0, 0, 0, 0}, 1023, 0, true))
		})
	})

	t.Run("RET", func(t *testing.B) { benchOpcode(t, opVM(opcode.RET)) })
	t.Run("SYSCALL", func(t *testing.B) { benchOpcode(t, opParamVM(opcode.SYSCALL, zeroSlice(4))) })

	t.Run("DEPTH", func(t *testing.B) {
		t.Run("0", func(t *testing.B) { benchOpcode(t, opVM(opcode.DEPTH)) })
		t.Run("1024", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.DEPTH, nil, arrayOfIfaces(1024)...)) })
	})
	t.Run("DROP", func(t *testing.B) {
		t.Run("1", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.DROP, nil, 1)) })
		t.Run("1024", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.DEPTH, nil, arrayOfIfaces(1024)...)) })
	})
	t.Run("NIP", func(t *testing.B) {
		t.Run("2", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.NIP, nil, 1, 1)) })
		t.Run("1024", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.NIP, nil, arrayOfIfaces(1024)...)) })
	})
	t.Run("XDROP", func(t *testing.B) {
		t.Run("0/1", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.XDROP, nil, 0, 0)) })
		var items = arrayOfIfaces(1025)
		items[1024] = 0
		t.Run("0/1024", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.XDROP, nil, items...)) })
		items[1024] = 1023
		t.Run("1024/1024", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.XDROP, nil, items...)) })
		items = arrayOfIfaces(2048)
		items[2047] = 2046
		t.Run("2047/2048", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.XDROP, nil, items...)) })
	})
	t.Run("CLEAR", func(t *testing.B) {
		t.Run("0", func(t *testing.B) { benchOpcode(t, opVM(opcode.CLEAR)) })
		t.Run("1024", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.CLEAR, nil, arrayOfIfaces(1024)...)) })
		t.Run("2048", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.CLEAR, nil, arrayOfIfaces(2048)...)) })
	})
	var copiers = []struct {
		op  opcode.Opcode
		pos int
		l   int
	}{
		{opcode.DUP, 0, 1},
		{opcode.OVER, 1, 2},
		{opcode.PICK, 2, 3},
		{opcode.PICK, 1024, 1025},
		{opcode.TUCK, 1, 2},
	}
	for _, cp := range copiers {
		var name = cp.op.String()
		if cp.op == opcode.PICK {
			name += "/" + strconv.Itoa(cp.pos)
		}
		var getitems = func(element any) []any {
			l := cp.l
			pos := cp.pos
			if cp.op == opcode.PICK {
				pos++
				l++
			}
			var items = make([]any, l)
			for i := range items {
				items[i] = 0
			}
			items[pos] = element
			if cp.op == opcode.PICK {
				items[0] = cp.pos
			}
			return items
		}
		t.Run(name, func(t *testing.B) {
			t.Run("null", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.DUP, nil, getitems(stackitem.Null{})...)) })
			t.Run("boolean", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.DUP, nil, getitems(true)...)) })
			t.Run("integer", func(t *testing.B) {
				t.Run("small", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.DUP, nil, getitems(1)...)) })
				t.Run("big", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.DUP, nil, getitems(bigNumber())...)) })
			})
			t.Run("bytearray", func(t *testing.B) {
				t.Run("small", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.DUP, nil, getitems("01234567")...)) })
				t.Run("big", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.DUP, nil, getitems(zeroSlice(65536))...)) })
			})
			t.Run("buffer", func(t *testing.B) {
				t.Run("small", func(t *testing.B) {
					benchOpcode(t, opParamPushVM(opcode.DUP, nil, getitems(stackitem.NewBuffer(zeroSlice(1)))...))
				})
				t.Run("big", func(t *testing.B) {
					benchOpcode(t, opParamPushVM(opcode.DUP, nil, getitems(stackitem.NewBuffer(zeroSlice(65536)))...))
				})
			})
			t.Run("struct", func(t *testing.B) {
				var items = make([]stackitem.Item, 1)
				t.Run("small", func(t *testing.B) {
					benchOpcode(t, opParamPushVM(opcode.DUP, nil, getitems(stackitem.NewStruct(items))...))
				})
				// Stack overflow.
				if cp.op == opcode.PICK && cp.pos == 1024 {
					return
				}
				items = make([]stackitem.Item, 1024)
				t.Run("big", func(t *testing.B) {
					benchOpcode(t, opParamPushVM(opcode.DUP, nil, getitems(stackitem.NewStruct(items))...))
				})
			})
			p := stackitem.NewPointer(0, zeroSlice(1024))
			t.Run("pointer", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.DUP, nil, getitems(p)...)) })
		})
	}

	var swappers = []struct {
		op  opcode.Opcode
		num int
	}{
		{opcode.SWAP, 2},
		{opcode.ROT, 3},
		{opcode.ROLL, 4},
		{opcode.ROLL, 1024},
		{opcode.REVERSE3, 3},
		{opcode.REVERSE4, 4},
		{opcode.REVERSEN, 5},
		{opcode.REVERSEN, 1024},
	}
	for _, sw := range swappers {
		var name = sw.op.String()
		if sw.op == opcode.ROLL || sw.op == opcode.REVERSEN {
			name += "/" + strconv.Itoa(sw.num)
		}
		var getitems = func(element any) []any {
			l := sw.num
			if sw.op == opcode.ROLL || sw.op == opcode.REVERSEN {
				l++
			}
			var items = make([]any, l)
			for i := range items {
				items[i] = element
			}
			if sw.op == opcode.ROLL || sw.op == opcode.REVERSEN {
				items[len(items)-1] = sw.num - 1
			}
			return items
		}
		t.Run(name, func(t *testing.B) {
			t.Run("null", func(t *testing.B) {
				benchOpcode(t, opParamPushVM(sw.op, nil, getitems(stackitem.Null{})...))
			})
			t.Run("integer", func(t *testing.B) { benchOpcode(t, opParamPushVM(sw.op, nil, getitems(0)...)) })
			t.Run("big bytes", func(t *testing.B) {
				benchOpcode(t, opParamPushVM(sw.op, nil, getitems(zeroSlice(65536))...))
			})
		})
	}

	t.Run("INITSSLOT", func(t *testing.B) {
		t.Run("1", func(t *testing.B) { benchOpcode(t, opParamVM(opcode.INITSSLOT, []byte{1})) })
		t.Run("255", func(t *testing.B) { benchOpcode(t, opParamVM(opcode.INITSSLOT, []byte{255})) })
	})
	t.Run("INITSLOT", func(t *testing.B) {
		t.Run("1/1", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.INITSLOT, []byte{1, 1}, 0)) })
		t.Run("1/255", func(t *testing.B) {
			benchOpcode(t, opParamPushVM(opcode.INITSLOT, []byte{1, 255}, arrayOfIfaces(255)...))
		})
		t.Run("255/1", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.INITSLOT, []byte{255, 1}, 0)) })
		t.Run("255/255", func(t *testing.B) {
			benchOpcode(t, opParamPushVM(opcode.INITSLOT, []byte{255, 255}, arrayOfIfaces(255)...))
		})
	})
	t.Run("LDSFLD0", func(t *testing.B) { benchOpcode(t, opParamSlotsPushVM(opcode.LDSFLD0, nil, 1, 0, 0)) })
	t.Run("LDSFLD254", func(t *testing.B) { benchOpcode(t, opParamSlotsPushVM(opcode.LDSFLD, []byte{254}, 255, 0, 0)) })
	t.Run("STSFLD0", func(t *testing.B) { benchOpcode(t, opParamSlotsPushVM(opcode.STSFLD0, nil, 1, 0, 0, 0)) })
	t.Run("STSFLD254", func(t *testing.B) { benchOpcode(t, opParamSlotsPushVM(opcode.STSFLD, []byte{254}, 255, 0, 0, 0)) })
	t.Run("LDLOC0", func(t *testing.B) { benchOpcode(t, opParamSlotsPushVM(opcode.LDLOC0, nil, 0, 1, 1)) })
	t.Run("LDLOC254", func(t *testing.B) { benchOpcode(t, opParamSlotsPushVM(opcode.LDLOC, []byte{254}, 0, 255, 255)) })
	t.Run("STLOC0", func(t *testing.B) { benchOpcode(t, opParamSlotsPushVM(opcode.STLOC0, nil, 0, 1, 1, 0)) })
	t.Run("STLOC254", func(t *testing.B) { benchOpcode(t, opParamSlotsPushVM(opcode.STLOC, []byte{254}, 0, 255, 255, 0)) })
	t.Run("LDARG0", func(t *testing.B) { benchOpcode(t, opParamSlotsPushVM(opcode.LDARG0, nil, 0, 1, 1)) })
	t.Run("LDARG254", func(t *testing.B) { benchOpcode(t, opParamSlotsPushVM(opcode.LDARG, []byte{254}, 0, 255, 255)) })
	t.Run("STARG0", func(t *testing.B) { benchOpcode(t, opParamSlotsPushVM(opcode.STARG0, nil, 0, 1, 1, 0)) })
	t.Run("STARG254", func(t *testing.B) { benchOpcode(t, opParamSlotsPushVM(opcode.STARG, []byte{254}, 0, 255, 255, 0)) })

	t.Run("NEWBUFFER", func(t *testing.B) {
		t.Run("1", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.NEWBUFFER, nil, 1)) })
		t.Run("255", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.NEWBUFFER, nil, 255)) })
		t.Run("64K", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.NEWBUFFER, nil, 65536)) })
		t.Run("1M", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.NEWBUFFER, nil, 1024*1024)) })
	})
	t.Run("MEMCPY", func(t *testing.B) {
		buf1 := maxBuf()
		buf2 := maxBuf()

		t.Run("1", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.MEMCPY, nil, buf1, 0, buf2, 0, 1)) })
		t.Run("255", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.MEMCPY, nil, buf1, 0, buf2, 0, 255)) })
		t.Run("64K", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.MEMCPY, nil, buf1, 0, buf2, 0, 65536)) })
		t.Run("1M", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.MEMCPY, nil, buf1, 0, buf2, 0, 1024*1024)) })
	})
	t.Run("CAT", func(t *testing.B) {
		t.Run("1+1", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.CAT, nil, zeroSlice(1), zeroSlice(1))) })
		t.Run("256+256", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.CAT, nil, zeroSlice(255), zeroSlice(255))) })
		t.Run("64K+64K", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.CAT, nil, zeroSlice(65536), zeroSlice(65536))) })
		t.Run("512K+512K", func(t *testing.B) {
			benchOpcode(t, opParamPushVM(opcode.CAT, nil, zeroSlice(512*1024), zeroSlice(512*1024)))
		})
	})
	t.Run("SUBSTR", func(t *testing.B) {
		buf := stackitem.NewBuffer(zeroSlice(1024 * 1024))

		t.Run("1", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.SUBSTR, nil, buf, 0, 1)) })
		t.Run("256", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.SUBSTR, nil, buf, 0, 256)) })
		t.Run("64K", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.SUBSTR, nil, buf, 0, 65536)) })
		t.Run("1M", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.SUBSTR, nil, buf, 0, 1024*1024)) })
	})
	t.Run("LEFT", func(t *testing.B) {
		buf := stackitem.NewBuffer(zeroSlice(1024 * 1024))

		t.Run("1", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.LEFT, nil, buf, 1)) })
		t.Run("256", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.LEFT, nil, buf, 256)) })
		t.Run("64K", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.LEFT, nil, buf, 65536)) })
		t.Run("1M", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.LEFT, nil, buf, 1024*1024)) })
	})
	t.Run("RIGHT", func(t *testing.B) {
		buf := stackitem.NewBuffer(zeroSlice(1024 * 1024))

		t.Run("1", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.RIGHT, nil, buf, 1)) })
		t.Run("256", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.RIGHT, nil, buf, 256)) })
		t.Run("64K", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.RIGHT, nil, buf, 65536)) })
		t.Run("1M", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.RIGHT, nil, buf, 1024*1024)) })
	})
	unaries := []opcode.Opcode{opcode.INVERT, opcode.SIGN, opcode.ABS, opcode.NEGATE, opcode.INC, opcode.DEC, opcode.NOT, opcode.NZ}
	for _, op := range unaries {
		t.Run(op.String(), func(t *testing.B) {
			t.Run("1", func(t *testing.B) { benchOpcode(t, opParamPushVM(op, nil, 1)) })
			t.Run("0", func(t *testing.B) { benchOpcode(t, opParamPushVM(op, nil, 0)) })
			t.Run("big", func(t *testing.B) { benchOpcode(t, opParamPushVM(op, nil, bigNumber())) })
			t.Run("big negative", func(t *testing.B) { benchOpcode(t, opParamPushVM(op, nil, bigNegNumber())) })
		})
	}
	binaries := []opcode.Opcode{opcode.AND, opcode.OR, opcode.XOR, opcode.ADD, opcode.SUB,
		opcode.BOOLAND, opcode.BOOLOR, opcode.NUMEQUAL, opcode.NUMNOTEQUAL,
		opcode.LT, opcode.LE, opcode.GT, opcode.GE, opcode.MIN, opcode.MAX}
	for _, op := range binaries {
		t.Run(op.String(), func(t *testing.B) {
			t.Run("0+0", func(t *testing.B) { benchOpcode(t, opParamPushVM(op, nil, 0, 0)) })
			t.Run("1+1", func(t *testing.B) { benchOpcode(t, opParamPushVM(op, nil, 1, 1)) })
			t.Run("1/big", func(t *testing.B) { benchOpcode(t, opParamPushVM(op, nil, 1, bigNumber())) })
			t.Run("big/big", func(t *testing.B) { benchOpcode(t, opParamPushVM(op, nil, bigNumber(), bigNumber())) })
			t.Run("big/bigneg", func(t *testing.B) { benchOpcode(t, opParamPushVM(op, nil, bigNumber(), bigNegNumber())) })
		})
	}
	equals := []opcode.Opcode{opcode.EQUAL, opcode.NOTEQUAL}
	for _, op := range equals {
		t.Run(op.String(), func(t *testing.B) {
			t.Run("bools", func(t *testing.B) { benchOpcode(t, opParamPushVM(op, nil, true, false)) })
			t.Run("small integers", func(t *testing.B) { benchOpcode(t, opParamPushVM(op, nil, 1, 0)) })
			t.Run("big integers", func(t *testing.B) { benchOpcode(t, opParamPushVM(op, nil, bigNumber(), bigNegNumber())) })
			t.Run("255B", func(t *testing.B) { benchOpcode(t, opParamPushVM(op, nil, zeroSlice(255), zeroSlice(255))) })
			t.Run("64KEQ", func(t *testing.B) { benchOpcode(t, opParamPushVM(op, nil, zeroSlice(65535), zeroSlice(65535))) })
			t.Run("64KNEQ", func(t *testing.B) { benchOpcode(t, opParamPushVM(op, nil, zeroSlice(65535), ffSlice(65535))) })
		})
	}
	t.Run(opcode.MUL.String(), func(t *testing.B) {
		t.Run("1+0", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.MUL, nil, 1, 0)) })
		t.Run("100/big", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.MUL, nil, 100, bigNumber())) })
		t.Run("16ff*16ff", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.MUL, nil, ffSlice(16), ffSlice(16))) })
	})
	t.Run(opcode.DIV.String(), func(t *testing.B) {
		t.Run("0/1", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.DIV, nil, 0, 1)) })
		t.Run("big/100", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.DIV, nil, bigNumber(), 100)) })
		t.Run("bigneg/big", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.DIV, nil, bigNumber(), bigNegNumber())) })
	})
	t.Run(opcode.MOD.String(), func(t *testing.B) {
		t.Run("1+1", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.MOD, nil, 1, 1)) })
		t.Run("big/100", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.MOD, nil, bigNumber(), 100)) })
		t.Run("big/bigneg", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.MOD, nil, bigNumber(), bigNegNumber())) })
	})
	t.Run(opcode.SHL.String(), func(t *testing.B) {
		t.Run("1/1", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.SHL, nil, 1, 1)) })
		t.Run("1/254", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.SHL, nil, 1, 254)) })
		t.Run("big/7", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.SHL, nil, bigNumber(), 7)) })
		t.Run("bigneg/7", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.SHL, nil, bigNegNumber(), 7)) })
		t.Run("16ff/15", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.SHL, nil, ffSlice(16), 15)) })
	})
	t.Run(opcode.SHR.String(), func(t *testing.B) {
		t.Run("1/1", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.SHR, nil, 1, 1)) })
		t.Run("1/254", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.SHR, nil, 1, 254)) })
		t.Run("big/254", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.SHR, nil, bigNumber(), 254)) })
		t.Run("bigneg/7", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.SHR, nil, bigNegNumber(), 7)) })
		t.Run("16ff/15", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.SHR, nil, ffSlice(16), 15)) })
	})
	t.Run(opcode.WITHIN.String(), func(t *testing.B) {
		t.Run("0/1/2", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.WITHIN, nil, 1, 0, 2)) })
		t.Run("bigNeg/1/big", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.WITHIN, nil, 1, bigNegNumber(), bigNumber())) })
		t.Run("bigNeg/big/max", func(t *testing.B) {
			benchOpcode(t, opParamPushVM(opcode.WITHIN, nil, bigNumber(), bigNegNumber(), maxNumber()))
		})
	})
	var newrefs = []opcode.Opcode{opcode.NEWARRAY0, opcode.NEWSTRUCT0, opcode.NEWMAP}
	for _, op := range newrefs {
		t.Run(op.String(), func(t *testing.B) { benchOpcode(t, opVM(op)) })
	}
	var newcountedrefs = []opcode.Opcode{opcode.NEWARRAY, opcode.NEWSTRUCT}
	for _, op := range newcountedrefs {
		var nums = []int{1, 255, 1024}
		t.Run(op.String(), func(t *testing.B) {
			for _, n := range nums {
				t.Run(strconv.Itoa(n), func(t *testing.B) { benchOpcode(t, opParamPushVM(op, nil, n)) })
			}
		})
	}
	t.Run("NEWARRAYT", func(t *testing.B) {
		var nums = []int{1, 255, 1024}
		var types = []stackitem.Type{stackitem.AnyT, stackitem.PointerT, stackitem.BooleanT,
			stackitem.IntegerT, stackitem.ByteArrayT, stackitem.BufferT, stackitem.ArrayT,
			stackitem.StructT, stackitem.MapT, stackitem.InteropT}
		for _, n := range nums {
			for _, typ := range types {
				t.Run(typ.String()+"/"+strconv.Itoa(n), func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.NEWARRAYT, []byte{byte(typ)}, n)) })
			}
		}
	})
	t.Run("PACK", func(t *testing.B) {
		var nums = []int{1, 255, 1024}
		for _, n := range nums {
			t.Run(strconv.Itoa(n), func(t *testing.B) {
				var elems = make([]any, n+1)
				for i := range elems {
					elems[i] = 0
				}
				elems[n] = n
				benchOpcode(t, opParamPushVM(opcode.PACK, nil, elems...))
			})
		}
	})
	t.Run("UNPACK", func(t *testing.B) {
		var nums = []int{1, 255, 1024}
		for _, n := range nums {
			t.Run(strconv.Itoa(n), func(t *testing.B) {
				var elems = make([]stackitem.Item, n)
				for i := range elems {
					elems[i] = stackitem.Make(1)
				}
				benchOpcode(t, opParamPushVM(opcode.UNPACK, nil, elems))
			})
		}
	})
	t.Run("SIZE", func(t *testing.B) {
		var elems = arrayOfOnes(1024)
		var buf = maxBytes()
		t.Run("array/1", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.SIZE, nil, elems[:1])) })
		t.Run("array/1024", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.SIZE, nil, elems)) })
		t.Run("map/1024", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.SIZE, nil, bigMap())) })
		t.Run("bytes/255", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.SIZE, nil, buf[:255])) })
		t.Run("bytes/64K", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.SIZE, nil, buf[:65536])) })
		t.Run("bytes/1M", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.SIZE, nil, buf)) })
	})
	t.Run("HASKEY", func(t *testing.B) {
		var elems = arrayOfOnes(1024)
		var buf = maxBuf()
		t.Run("array/1", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.HASKEY, nil, elems, 1)) })
		t.Run("array/1023", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.HASKEY, nil, elems, 1023)) })
		t.Run("array/1024", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.HASKEY, nil, elems, 1024)) })
		t.Run("map/1", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.HASKEY, nil, bigMap(), 1)) })
		t.Run("map/1023", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.HASKEY, nil, bigMap(), 1023)) })
		t.Run("map/1024", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.HASKEY, nil, bigMap(), 1024)) })
		t.Run("buffer/255", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.HASKEY, nil, buf, 255)) })
		t.Run("buffer/64K", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.HASKEY, nil, buf, 65536)) })
		t.Run("buffer/1M", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.HASKEY, nil, buf, 1024*1024)) })
	})
	t.Run("KEYS", func(t *testing.B) {
		t.Run("map/1024", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.KEYS, nil, bigMap())) })
	})
	t.Run("VALUES", func(t *testing.B) {
		var elems = arrayOfOnes(1024)
		t.Run("array/1", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.VALUES, nil, elems[:1])) })
		t.Run("array/1024", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.VALUES, nil, elems)) })
		t.Run("map/1024", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.VALUES, nil, bigMap())) })
	})
	t.Run("PICKITEM", func(t *testing.B) {
		var elems = arrayOfOnes(1024)
		var buf = maxBytes()
		t.Run("array/1", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.PICKITEM, nil, elems, 1)) })
		t.Run("array/1023", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.PICKITEM, nil, elems, 1023)) })
		t.Run("map/1", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.PICKITEM, nil, bigMap(), 1)) })
		t.Run("map/1023", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.PICKITEM, nil, bigMap(), 1023)) })
		t.Run("bytes/255", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.PICKITEM, nil, buf, 255)) })
		t.Run("bytes/64K", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.PICKITEM, nil, buf, 65536)) })
		t.Run("bytes/1M", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.PICKITEM, nil, buf, 1024*1024-1)) })
	})
	t.Run("APPEND", func(t *testing.B) {
		var a0 = arrayOfOnes(0)
		var a1023 = arrayOfOnes(1023)
		t.Run("array/1", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.APPEND, nil, a0, 1)) })
		t.Run("array/1023", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.APPEND, nil, a1023, 1)) })
		t.Run("struct/1", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.APPEND, nil, stackitem.NewStruct(a0), 1)) })
		t.Run("struct/1023", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.APPEND, nil, stackitem.NewStruct(a1023), 1)) })
		t.Run("array/struct", func(t *testing.B) {
			benchOpcode(t, opParamPushVM(opcode.APPEND, nil, a1023, stackitem.NewStruct(a1023)))
		})
	})
	t.Run("SETITEM", func(t *testing.B) {
		var elems = arrayOfOnes(1024)
		var buf = maxBuf()
		t.Run("array/1", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.SETITEM, nil, elems, 1, 1)) })
		t.Run("array/1023", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.SETITEM, nil, elems, 1023, 1)) })
		t.Run("map/1", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.SETITEM, nil, bigMap(), 1, 1)) })
		t.Run("map/1023", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.SETITEM, nil, bigMap(), 1023, 1)) })
		t.Run("buffer/255", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.SETITEM, nil, buf, 255, 1)) })
		t.Run("buffer/1M", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.SETITEM, nil, buf, 1024*1024-1, 1)) })
	})

	t.Run("REVERSEITEMS", func(t *testing.B) {
		var elems = arrayOfOnes(1024)
		var buf = maxBuf()
		t.Run("array/1024", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.REVERSEITEMS, nil, elems)) })
		t.Run("buffer/1M", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.REVERSEITEMS, nil, buf)) })
	})

	t.Run("REMOVE", func(t *testing.B) {
		var elems = arrayOfOnes(1024)
		t.Run("array/1", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.REMOVE, nil, elems, 1)) })
		t.Run("array/255", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.REMOVE, nil, elems, 255)) })
		t.Run("array/1023", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.REMOVE, nil, elems, 1023)) })
		t.Run("map/1", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.REMOVE, nil, bigMap(), 1)) })
		t.Run("map/255", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.REMOVE, nil, bigMap(), 255)) })
		t.Run("map/1023", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.REMOVE, nil, bigMap(), 1023)) })
	})
	t.Run("CLEARITEMS", func(t *testing.B) {
		t.Run("array/1024", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.CLEARITEMS, nil, arrayOfOnes(1024))) })
		t.Run("map/1024", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.CLEARITEMS, nil, bigMap())) })
	})

	t.Run("ISNULL", func(t *testing.B) {
		t.Run("null", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.ISNULL, nil, stackitem.Null{})) })
		t.Run("integer", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.ISNULL, nil, 1)) })
	})
	t.Run("ISTYPE", func(t *testing.B) {
		t.Run("null/null", func(t *testing.B) {
			benchOpcode(t, opParamPushVM(opcode.ISTYPE, []byte{byte(stackitem.AnyT)}, stackitem.Null{}))
		})
		t.Run("integer/integer", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.ISTYPE, []byte{byte(stackitem.IntegerT)}, 1)) })
		t.Run("null/integer", func(t *testing.B) { benchOpcode(t, opParamPushVM(opcode.ISTYPE, []byte{byte(stackitem.AnyT)}, 1)) })
	})
	t.Run("CONVERT", func(t *testing.B) {
		t.Run("bytes/integer", func(t *testing.B) {
			benchOpcode(t, opParamPushVM(opcode.CONVERT, []byte{byte(stackitem.IntegerT)}, "1012345678901234567890123456789"))
		})
		t.Run("integer/bytes", func(t *testing.B) {
			benchOpcode(t, opParamPushVM(opcode.CONVERT, []byte{byte(stackitem.ByteArrayT)}, maxNumber()))
		})
		t.Run("array/struct", func(t *testing.B) {
			benchOpcode(t, opParamPushVM(opcode.CONVERT, []byte{byte(stackitem.StructT)}, arrayOfOnes(1024)))
		})
	})
}

const countPoints = 8

func getBigInts(minBits, maxBits, num int) []*big.Int {
	step := uint((maxBits - minBits) / num)
	curr := step - 1
	res := make([]*big.Int, 0, num)
	for range num {
		res = append(res, new(big.Int).Lsh(big.NewInt(1), curr))
		curr += step
	}
	return res
}

func getBigInts2(minV, maxV *big.Int, num int) []*big.Int {
	res := make([]*big.Int, 0, num)
	diff := new(big.Int).Sub(maxV, minV)
	step := new(big.Int).Div(diff, big.NewInt(int64(num)))
	curr := new(big.Int).Add(minV, step)
	for range num {
		if curr.Cmp(maxV) < 0 {
			res = append(res, curr)
		}
		curr = new(big.Int).Add(curr, step)
	}
	return res
}

func getSizedInteropItems(n, elemSize int) []stackitem.Item {
	res := make([]stackitem.Item, 0, n)
	arrType := reflect.ArrayOf(elemSize, reflect.TypeOf(byte(0)))
	for range n {
		res = append(res, stackitem.NewInterop(reflect.New(arrType).Elem()))
	}
	return res
}

func getArrayWithSizeAndDeep(n, d int) []stackitem.Item {
	items := make([]stackitem.Item, 0, n)
	for range n {
		var item stackitem.Item = stackitem.Null{}
		for range d - 1 {
			item = stackitem.NewArray([]stackitem.Item{item})
		}
		items = append(items, item)
	}
	return items
}

func getStructWithSizeAndDeep(n, d int) []stackitem.Item {
	items := make([]stackitem.Item, 0, n)
	for range n {
		var item stackitem.Item = stackitem.Null{}
		for range d - 1 {
			item = stackitem.NewStruct([]stackitem.Item{item})
		}
		items = append(items, item)
	}
	return items
}

func BenchmarkPushInt(b *testing.B) {
	type testCase struct {
		name   string
		op     opcode.Opcode
		params []byte
	}
	var (
		testCases    []testCase
		opToNumBytes = map[opcode.Opcode]int{
			opcode.PUSHINT8:   1,
			opcode.PUSHINT16:  2,
			opcode.PUSHINT32:  4,
			opcode.PUSHINT64:  8,
			opcode.PUSHINT128: 16,
			opcode.PUSHINT256: 32,
		}
	)
	for op, numBytes := range opToNumBytes {
		buf := make([]byte, numBytes)
		buf[numBytes-1] = 0x80
		testCases = append(testCases, testCase{
			name:   fmt.Sprintf("%s %d", op, numBytes),
			op:     op,
			params: buf,
		})
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamVM(tc.op, tc.params))
		})
	}
}

func BenchmarkPushData(b *testing.B) {
	type testCase struct {
		name   string
		params []byte
	}
	var (
		testCases []testCase
		stepLen   = maxByteArraySize / countPoints
		currLen   = stepLen
	)
	for range countPoints {
		testCases = append(testCases, testCase{
			name:   fmt.Sprintf("%s %d", opcode.PUSHDATA4, currLen),
			params: make([]byte, currLen),
		})
		currLen += stepLen
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamVM(opcode.PUSHDATA4, tc.params))
		})
	}
}

func BenchmarkConvert(b *testing.B) {
	type testCase struct {
		name   string
		params []byte
		items  []any
	}
	var (
		testCases        []testCase
		stepArrayLen     = MaxStackSize / countPoints
		stepByteArrayLen = maxByteArraySize / countPoints
		stepInteropSize  = maxInteropSize / countPoints
		currArrayLen     = stepArrayLen
		currByteArrayLen = stepByteArrayLen
	)
	for range countPoints {
		currInteropSize := stepInteropSize
		for range countPoints {
			item := getSizedInteropItems(currArrayLen, currInteropSize)
			testCases = append(testCases, testCase{
				name:   fmt.Sprintf("%s %s %s %d %d", opcode.CONVERT, stackitem.ArrayT, stackitem.StructT, currArrayLen, currInteropSize),
				params: []byte{byte(stackitem.StructT)},
				items:  []any{stackitem.NewArray(item)},
			})
			currInteropSize += stepInteropSize
		}
		currArrayLen += stepArrayLen
	}
	for range countPoints {
		testCases = append(testCases, testCase{
			name:   fmt.Sprintf("%s %s %s %d", opcode.CONVERT, stackitem.ByteArrayT, stackitem.BufferT, currByteArrayLen),
			params: []byte{byte(stackitem.BufferT)},
			items:  []any{stackitem.NewByteArray(make([]byte, currByteArrayLen))},
		})
		currByteArrayLen += stepByteArrayLen
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(opcode.CONVERT, tc.params, tc.items...))
		})
	}
}

func BenchmarkInitSlot(b *testing.B) {
	type testCase struct {
		name   string
		op     opcode.Opcode
		params []byte
		items  []any
	}
	var (
		testCases       []testCase
		stepSlots       = byte(math.MaxUint8 / countPoints)
		currLocalSlots  = stepSlots
		currStaticSlots = stepSlots
	)
	for range countPoints {
		currArgsSlots := stepSlots
		for range countPoints {
			testCases = append(testCases, testCase{
				name:   fmt.Sprintf("%s %d %d", opcode.INITSLOT, currLocalSlots, currArgsSlots),
				op:     opcode.INITSLOT,
				params: []byte{currLocalSlots, currArgsSlots},
				// TODO: do not ignore the size and depth of the argument
				items: make([]any, currArgsSlots),
			})
			currArgsSlots += stepSlots
		}
		currLocalSlots += stepSlots
	}
	for range countPoints {
		testCases = append(testCases, testCase{
			name:   fmt.Sprintf("%s %d", opcode.INITSSLOT, currStaticSlots),
			op:     opcode.INITSSLOT,
			params: []byte{currStaticSlots},
		})
		currStaticSlots += stepSlots
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(tc.op, tc.params, tc.items...))
		})
	}
}

func BenchmarkLD(b *testing.B) {
	type testCase struct {
		name string
		f    func() *VM
	}
	var (
		testCases            []testCase
		stepIndependentParam = MaxStackSize / countPoints
	)
	for _, op := range []opcode.Opcode{opcode.LDSFLD0, opcode.LDLOC0, opcode.LDARG0} {
		currIndependentParam := stepIndependentParam
		for range countPoints {
			var (
				stepDependentParam = MaxStackSize / (countPoints * currIndependentParam)
				currDependentParam = stepDependentParam
			)
			for range countPoints {
				indParam := currIndependentParam
				depParam := currDependentParam
				testCases = append(testCases, testCase{
					name: fmt.Sprintf("%s %d %d", op, indParam, depParam),
					f: func() *VM {
						v := opVM(op)()
						slot := []stackitem.Item{stackitem.NewArray(getArrayWithSizeAndDeep(indParam, depParam))}
						switch op {
						case opcode.LDSFLD0:
							v.Context().sc.static = slot
						case opcode.LDLOC0:
							v.Context().local = slot
						case opcode.LDARG0:
							v.Context().arguments = slot
						}
						return v
					},
				})
				testCases = append(testCases, testCase{
					name: fmt.Sprintf("%s %d %d", op, depParam, indParam),
					f: func() *VM {
						v := opVM(op)()
						slot := []stackitem.Item{stackitem.NewArray(getArrayWithSizeAndDeep(depParam, indParam))}
						switch op {
						case opcode.LDSFLD0:
							v.Context().sc.static = slot
						case opcode.LDLOC0:
							v.Context().local = slot
						case opcode.LDARG0:
							v.Context().arguments = slot
						}
						return v
					},
				})
				currDependentParam += stepDependentParam
			}
			currIndependentParam += stepIndependentParam
		}
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, tc.f)
		})
	}
}

func BenchmarkIntegerOperands(b *testing.B) {
	unaryOpcodes := []opcode.Opcode{
		opcode.INVERT,
		opcode.SIGN,
		opcode.ABS,
		opcode.NEGATE,
		opcode.INC,
		opcode.DEC,
		opcode.SQRT,
		opcode.NZ,
	}
	binaryOpcodes := []opcode.Opcode{
		opcode.AND,
		opcode.OR,
		opcode.XOR,
		opcode.EQUAL,
		opcode.NOTEQUAL,
		opcode.ADD,
		opcode.SUB,
		opcode.MUL,
		opcode.DIV,
		opcode.MOD,
		opcode.POW,
		opcode.NUMEQUAL,
		opcode.NUMNOTEQUAL,
		opcode.LT,
		opcode.LE,
		opcode.GT,
		opcode.GE,
		opcode.MIN,
		opcode.MAX,
		opcode.SHL,
		opcode.SHR,
	}
	ternaryOpcodes := []opcode.Opcode{
		opcode.MODMUL,
		opcode.MODPOW,
	}
	type testCase struct {
		name  string
		op    opcode.Opcode
		items []any
	}
	var (
		testCases  = make([]testCase, countPoints*(len(unaryOpcodes)+len(binaryOpcodes)+len(ternaryOpcodes)))
		stepExp    = math.MaxUint / countPoints
		stepSHLArg = maxSHLArg / countPoints
		bigInts1   = getBigInts(0, stackitem.MaxBigIntegerSizeBits-1, countPoints)
	)
	for _, op := range unaryOpcodes {
		for i := range bigInts1 {
			testCases = append(testCases, testCase{
				name:  fmt.Sprintf("%s %d", op, bigInts1[i].BitLen()),
				op:    op,
				items: []any{bigInts1[i]},
			})
		}
	}
	bigInts2 := getBigInts(0, stackitem.MaxBigIntegerSizeBits-1, countPoints)
	for _, op := range binaryOpcodes {
		for i := range bigInts1 {
			if op == opcode.POW {
				currExp := stepExp
				for range countPoints {
					testCases = append(testCases, testCase{
						name:  fmt.Sprintf("%s %d %d", op, bigInts1[i].BitLen(), currExp),
						op:    op,
						items: []any{bigInts1[i], big.NewInt(int64(currExp))},
					})
					currExp += stepExp
				}
				continue
			}
			if op == opcode.SHL || op == opcode.SHR {
				currSHLArg := stepSHLArg
				for range countPoints {
					testCases = append(testCases, testCase{
						name:  fmt.Sprintf("%s %d %d", op, bigInts1[i].BitLen(), currSHLArg),
						op:    op,
						items: []any{bigInts1[i], big.NewInt(int64(currSHLArg))},
					})
					currSHLArg += stepExp
				}
			}
			testCases = append(testCases, testCase{
				name:  fmt.Sprintf("%s %d", op, bigInts1[i].BitLen()),
				op:    op,
				items: []any{bigInts1[i], bigInts2[i]},
			})
		}
	}
	bigInts3 := getBigInts(0, stackitem.MaxBigIntegerSizeBits-1, countPoints)
	for _, op := range ternaryOpcodes {
		for i := range bigInts1 {
			testCases = append(testCases, testCase{
				name:  fmt.Sprintf("%s %d", op, bigInts1[i].BitLen()),
				op:    op,
				items: []any{bigInts1[i], bigInts2[i], bigInts3[i]},
			})
		}
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(tc.op, nil, tc.items...))
		})
	}
}

func BenchmarkNew(b *testing.B) {
	type testCase struct {
		name   string
		op     opcode.Opcode
		params []byte
		item   stackitem.Item
	}
	var (
		testCases []testCase
		bigInts   = getBigInts2(big.NewInt(0), big.NewInt(MaxStackSize), countPoints)
	)
	for _, op := range []opcode.Opcode{opcode.NEWARRAY, opcode.NEWSTRUCT, opcode.NEWBUFFER} {
		for _, v := range bigInts {
			testCases = append(testCases, testCase{
				name: fmt.Sprintf("%s %s", op, v),
				op:   op,
				item: stackitem.NewBigInteger(v),
			})
		}
	}
	for _, typ := range []stackitem.Type{stackitem.BooleanT, stackitem.IntegerT, stackitem.ByteArrayT, stackitem.AnyT} {
		for _, v := range bigInts {
			testCases = append(testCases, testCase{
				name:   fmt.Sprintf("%s %s %s", opcode.NEWARRAYT, typ, v),
				op:     opcode.NEWARRAYT,
				params: []byte{byte(typ)},
				item:   stackitem.NewBigInteger(v),
			})
		}
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(tc.op, tc.params, tc.item))
		})
	}
}

func BenchmarkAppend(b *testing.B) {
	type testCase struct {
		name  string
		items []any
	}
	var (
		testCases []testCase
		sizeStep  = maxArraySize / countPoints
		deepStep  = (stackitem.MaxClonableNumOfItems - 1) / countPoints
		currSize  = sizeStep
	)
	for range countPoints {
		testCases = append(testCases, testCase{
			name:  fmt.Sprintf("%s %s %d", opcode.APPEND, stackitem.ArrayT, currSize),
			items: []any{make([]stackitem.Item, currSize), stackitem.Null{}},
		})
		currDeep := deepStep
		for range countPoints {
			var item stackitem.Item = stackitem.Null{}
			for range currDeep {
				item = stackitem.NewStruct([]stackitem.Item{item})
			}
			testCases = append(testCases, testCase{
				name:  fmt.Sprintf("%s %s %d %d", opcode.APPEND, stackitem.StructT, currSize, currDeep),
				items: []any{item, stackitem.Null{}},
			})
			currDeep += deepStep
		}
		currSize += sizeStep
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamSlotsPushReadOnlyVM(opcode.APPEND, nil, 0, 0, 0, false, tc.items...))
		})
	}
}

func BenchmarkMemcpy(b *testing.B) {
	type testCase struct {
		name  string
		items []any
	}
	var (
		testCases []testCase
		stepSize  = maxArraySize / countPoints
	)
	for currSize := stepSize; currSize <= maxArraySize; currSize += stepSize {
		buf := make([]byte, currSize)
		testCases = append(testCases, testCase{
			name: fmt.Sprintf("%s %d", opcode.MEMCPY, currSize),
			items: []any{
				stackitem.NewBuffer(buf),
				stackitem.NewBigInteger(big.NewInt(0)),
				stackitem.NewBuffer(buf),
				stackitem.NewBigInteger(big.NewInt(0)),
				stackitem.NewBigInteger(big.NewInt(int64(currSize))),
			},
		})
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamSlotsPushReadOnlyVM(opcode.MEMCPY, nil, 0, 0, 0, false, tc.items...))
		})
	}
}

func BenchmarkCat(b *testing.B) {
	type testCase struct {
		name  string
		items []any
	}
	var (
		testCases []testCase
		stepSize  = stackitem.MaxSize / countPoints
	)
	for currSize := stepSize; currSize <= stackitem.MaxSize; currSize += stepSize {
		testCases = append(testCases, testCase{
			name: fmt.Sprintf("%s %d", opcode.CAT, currSize),
			items: []any{
				[]byte{},
				make([]byte, currSize),
			},
		})
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(opcode.CAT, nil, tc.items...))
		})
	}
}

func BenchmarkSubstr(b *testing.B) {
	type testCase struct {
		name  string
		items []any
	}
	var (
		testCases []testCase
		stepSize  = maxByteArraySize / countPoints
	)
	for _, op := range []opcode.Opcode{opcode.SUBSTR, opcode.LEFT, opcode.RIGHT} {
		for currSize := stepSize; currSize <= maxByteArraySize; currSize += stepSize {
			items := []any{make([]byte, currSize)}
			if op == opcode.SUBSTR {
				items = append(items, 0)
			}
			items = append(items, currSize)
			testCases = append(testCases, testCase{
				name:  fmt.Sprintf("%s %d", op, currSize),
				items: items,
			})
		}
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(opcode.CAT, nil, tc.items...))
		})
	}
}

func BenchmarkDrop(b *testing.B) {
	type testCase struct {
		name  string
		op    opcode.Opcode
		items []any
	}
	var (
		testCases     []testCase
		stepDeep      = maxArrayDeep / countPoints
		stepStackSize = MaxStackSize / countPoints
		stepElemSize  = maxArraySize / countPoints
		currSize      = stepElemSize
	)
	for _, op := range []opcode.Opcode{opcode.DROP, opcode.NIP, opcode.XDROP, opcode.CLEAR} {
		for range countPoints {
			currDeep := stepDeep
			for range countPoints {
				items := []any{getArrayWithSizeAndDeep(currSize, currDeep)}
				if op == opcode.NIP {
					items = append(items, nil)
				}
				if op == opcode.DROP || op == opcode.NIP {
					testCases = append(testCases, testCase{
						name:  fmt.Sprintf("%s %d %d", op, currSize, currDeep*currSize),
						op:    op,
						items: items,
					})
					currDeep += stepDeep
					continue
				}
				currStackSize := stepStackSize
				for range countPoints {
					cpItems := make([]any, currStackSize)
					item := items[0]
					if op == opcode.CLEAR {
						for i := range cpItems {
							cpItems[i] = item
						}
					} else {
						items[0] = item
						cpItems[currStackSize-1] = currStackSize - 1
					}
					testCases = append(testCases, testCase{
						name:  fmt.Sprintf("%s %d %d %d", op, currSize, currDeep*currSize, currStackSize),
						op:    op,
						items: cpItems,
					})
					currStackSize += stepStackSize
				}
				currDeep += stepDeep
			}
			currSize += stepElemSize
		}
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(tc.op, nil, tc.items...))
		})
	}
}

func BenchmarkDup(b *testing.B) {
	type testCase struct {
		name  string
		op    opcode.Opcode
		items []any
	}
	var (
		testCases         []testCase
		bigInts           = getBigInts(0, stackitem.MaxBigIntegerSizeBits-1, countPoints)
		stepElemSize      = maxInteropSize / countPoints
		stepByteArraySize = maxByteArraySize / countPoints
		stepStackSize     = MaxStackSize / countPoints
	)
	itemsForOpcode := func(op opcode.Opcode, item any, n int) []any {
		switch op {
		case opcode.DUP:
			return []any{item}
		case opcode.OVER:
			return []any{item, nil}
		case opcode.TUCK:
			return []any{nil, item}
		case opcode.PICK:
			items := make([]any, 0, n)
			items = append(items, item)
			for range n - 1 {
				items = append(items, nil)
			}
			return items
		default:
			panic("unexpected opcode")
		}
	}
	for _, op := range []opcode.Opcode{opcode.DUP, opcode.OVER, opcode.TUCK} {
		for _, v := range bigInts {
			if op != opcode.TUCK {
				testCases = append(testCases, testCase{
					name:  fmt.Sprintf("%s %s %d", op, stackitem.IntegerT, v.BitLen()),
					op:    op,
					items: itemsForOpcode(op, v, 0),
				})
				continue
			}
			currN := stepStackSize
			for range countPoints {
				testCases = append(testCases, testCase{
					name:  fmt.Sprintf("%s %s %d %d", op, stackitem.IntegerT, v.BitLen(), currN),
					op:    op,
					items: itemsForOpcode(op, v, currN),
				})
				currN += stepStackSize
			}
		}
		currElemSize := stepElemSize
		for range countPoints {
			item := getSizedInteropItems(1, currElemSize)[0]
			if op != opcode.TUCK {
				testCases = append(testCases, testCase{
					name:  fmt.Sprintf("%s %s %d", op, stackitem.IntegerT, currElemSize),
					op:    op,
					items: itemsForOpcode(op, item, 0),
				})
				continue
			}
			currN := stepStackSize
			for range countPoints {
				testCases = append(testCases, testCase{
					name:  fmt.Sprintf("%s %s %d %d", op, stackitem.IntegerT, currElemSize, currN),
					op:    op,
					items: itemsForOpcode(op, item, currN),
				})
				currN += stepStackSize
			}
			currElemSize += stepElemSize
		}
		currByteArraySize := stepByteArraySize
		for range countPoints {
			item := make([]byte, currByteArraySize)
			if op != opcode.TUCK {
				testCases = append(testCases, testCase{
					name:  fmt.Sprintf("%s %s %d", op, stackitem.IntegerT, currElemSize),
					op:    op,
					items: itemsForOpcode(op, item, 0),
				})
				continue
			}
			currN := stepStackSize
			for range countPoints {
				testCases = append(testCases, testCase{
					name:  fmt.Sprintf("%s %s %d %d", op, stackitem.IntegerT, currElemSize, currN),
					op:    op,
					items: itemsForOpcode(op, item, currN),
				})
				currN += stepStackSize
			}
			currByteArraySize += stepByteArraySize
		}
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(tc.op, nil, tc.items...))
		})
	}
}

func BenchmarkRollAndReverse(b *testing.B) {
	type testCase struct {
		name  string
		op    opcode.Opcode
		items []any
	}
	var (
		testCases     []testCase
		stepStackSize = MaxStackSize / countPoints
	)
	for _, op := range []opcode.Opcode{opcode.ROLL, opcode.REVERSEN} {
		currStackSize := stepStackSize
		for range countPoints {
			items := make([]any, currStackSize)
			if op == opcode.REVERSEN {
				items[currStackSize-1] = big.NewInt(int64(currStackSize - 1))
			}
			testCases = append(testCases, testCase{
				name:  fmt.Sprintf("%s %d", op, currStackSize),
				op:    op,
				items: items,
			})
			currStackSize += stepStackSize
		}
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(tc.op, nil, tc.items...))
		})
	}
}

func BenchmarkPackMap(b *testing.B) {
	type testCase struct {
		name  string
		items []any
	}
	var (
		testCases            []testCase
		stepStackSize        = MaxStackSize / (2 * countPoints)
		stepKeyByteArraySize = stackitem.MaxKeySize / countPoints
		stepBigIntBits       = stackitem.MaxBigIntegerSizeBits / countPoints
	)
	for _, typ := range []stackitem.Type{stackitem.BooleanT, stackitem.IntegerT, stackitem.ByteArrayT} {
		currStackSize := stepStackSize
		for range countPoints {
			var (
				item                 any
				currKeyByteArraySize = stepKeyByteArraySize
				currBigIntBits       = stepBigIntBits
			)
			switch typ {
			case stackitem.BooleanT:
				item = true
			case stackitem.IntegerT:
				item = new(big.Int).Lsh(big.NewInt(1), uint(currBigIntBits-1))
			case stackitem.ByteArrayT:
				buf := make([]byte, currKeyByteArraySize)
				buf[currKeyByteArraySize-1] = 1
				item = buf
			}
		keySizeLoop:
			for range countPoints {
				items := make([]any, 0, 2*currStackSize+1)
				for range currStackSize {
					items = append(items, nil, item)
					switch it := item.(type) {
					case *big.Int:
						item = new(big.Int).Add(it, big.NewInt(1))
					case []byte:
						v := new(big.Int).SetBytes(it)
						item = v.Add(v, big.NewInt(1)).Bytes()
					case bool:
						item = !it
					}
				}
				items = append(items, currStackSize)
				switch typ {
				case stackitem.BooleanT:
					testCases = append(testCases, testCase{
						name:  fmt.Sprintf("%s %s %d", opcode.PACKMAP, stackitem.BooleanT, currStackSize),
						items: items,
					})
					break keySizeLoop
				case stackitem.IntegerT:
					testCases = append(testCases, testCase{
						name:  fmt.Sprintf("%s %s %d %d", opcode.PACKMAP, stackitem.IntegerT, currStackSize, currBigIntBits),
						items: items,
					})
					currBigIntBits += stepBigIntBits
				case stackitem.ByteArrayT:
					testCases = append(testCases, testCase{
						name:  fmt.Sprintf("%s %s %d %d", opcode.PACKMAP, stackitem.ByteArrayT, currStackSize, currKeyByteArraySize),
						items: items,
					})
					currKeyByteArraySize += stepKeyByteArraySize
				}
			}
			currStackSize += stepStackSize
		}
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(opcode.PACKMAP, nil, tc.items...))
		})
	}
}

func BenchmarkPack(b *testing.B) {
	type testCase struct {
		name  string
		op    opcode.Opcode
		items []any
	}
	var (
		testCases     []testCase
		stepStackSize = MaxStackSize / countPoints
	)
	for _, op := range []opcode.Opcode{opcode.PACK, opcode.PACKSTRUCT} {
		currStackSize := stepStackSize
		for range countPoints {
			items := make([]any, currStackSize)
			items[currStackSize-1] = currStackSize - 1
			testCases = append(testCases, testCase{
				name:  fmt.Sprintf("%s %d", op, currStackSize),
				op:    op,
				items: items,
			})
			currStackSize += stepStackSize
		}
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(tc.op, nil, tc.items...))
		})
	}
}

func BenchmarkUnpack(b *testing.B) {
	type testCase struct {
		name string
		item any
	}
	var (
		testCases   []testCase
		stepMapSize = maxMapSize / countPoints
		currMapSize = stepMapSize
	)
	for range countPoints {
		m := make([]stackitem.MapElement, 0, currMapSize)
		key := 0
		for range currMapSize {
			m = append(m, stackitem.MapElement{
				Key:   stackitem.Make(key),
				Value: nil,
			})
			key++
		}
		testCases = append(testCases, testCase{
			name: fmt.Sprintf("%s %s %d", opcode.UNPACK, stackitem.MapT, currMapSize),
			item: stackitem.NewMapWithValue(m),
		})
		currMapSize += stepMapSize
	}
	for _, typ := range []stackitem.Type{stackitem.ArrayT, stackitem.StructT} {
		step := maxArraySize / countPoints
		if typ == stackitem.StructT {
			step = maxStructSize / countPoints
		}
		curr := step
		for range countPoints {
			tc := testCase{name: fmt.Sprintf("%s %s %d", opcode.UNPACK, typ, curr)}
			if typ == stackitem.StructT {
				tc.item = stackitem.NewStruct(make([]stackitem.Item, curr))
			} else {
				tc.item = make([]stackitem.Item, curr)
			}
			testCases = append(testCases, tc)
			curr += step
		}
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(opcode.UNPACK, nil, tc.item))
		})
	}
}

func BenchmarkPickitem(b *testing.B) {
	type testCase struct {
		name  string
		items []any
	}
	var (
		testCases       []testCase
		stepInteropSize = maxInteropSize / countPoints
		stepMapSize     = maxMapSize / countPoints
	)
	for _, typ := range []stackitem.Type{stackitem.ArrayT, stackitem.StructT} {
		currInteropSize := stepInteropSize
		for range countPoints {
			item := getSizedInteropItems(1, currInteropSize)[0]
			testCases = append(testCases, testCase{
				name:  fmt.Sprintf("%s %s %d", opcode.PICKITEM, typ, currInteropSize),
				items: []any{[]stackitem.Item{item}, 0},
			})
			currInteropSize += stepInteropSize
		}
	}
	currMapSize := stepMapSize
	for range countPoints {
		currInteropSize := stepInteropSize
		for range countPoints {
			m := make([]stackitem.MapElement, 0, currMapSize)
			for key := range currMapSize {
				m = append(m, stackitem.MapElement{
					Key:   stackitem.Make(key),
					Value: nil,
				})
			}
			m[currMapSize-1].Value = getSizedInteropItems(1, currInteropSize)[0]
			testCases = append(testCases, testCase{
				name:  fmt.Sprintf("%s %s %d %d", opcode.PICKITEM, stackitem.MapT, currInteropSize, currMapSize),
				items: []any{stackitem.NewMapWithValue(m), currMapSize - 1},
			})
			currInteropSize += stepInteropSize
		}
		currMapSize += stepMapSize
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(opcode.PICKITEM, nil, tc.items...))
		})
	}
}

func BenchmarkSetitem(b *testing.B) {
	type testCase struct {
		name  string
		items []any
	}
	var (
		testCases []testCase
		stepSize  = MaxStackSize / countPoints
	)
	for _, typ := range []stackitem.Type{stackitem.ArrayT, stackitem.StructT, stackitem.MapT} {
		currRemovedSize := stepSize
		for range countPoints {
			stepRemovedDeep := MaxStackSize / (countPoints * currRemovedSize)
			currRemovedDeep := stepRemovedDeep
			for range countPoints {
				currAddedSize := stepSize
				for range countPoints {
					stepAddedDeep := MaxStackSize / (countPoints * currAddedSize)
					currAddedDeep := stepAddedDeep
					for range countPoints {
						var (
							obj       any
							addedItem any
						)
						if typ == stackitem.MapT {
							currMapSize := stepSize
							for range countPoints {
								m := make([]stackitem.MapElement, 0, currMapSize)
								for key := range currMapSize {
									m = append(m, stackitem.MapElement{
										Key:   stackitem.Make(key),
										Value: nil,
									})
								}
								m[currMapSize-1].Value = stackitem.NewStruct(getArrayWithSizeAndDeep(currRemovedSize, currRemovedDeep))
								testCases = append(testCases, testCase{
									name:  fmt.Sprintf("%s %s %d %d %d %d %d", opcode.SETITEM, stackitem.MapT, currRemovedSize, currRemovedDeep, currAddedSize, currAddedDeep, currMapSize),
									items: []any{stackitem.NewMapWithValue(m), currMapSize - 1, stackitem.NewStruct(getArrayWithSizeAndDeep(currAddedSize, currAddedDeep))},
								})
								currMapSize += stepSize
							}
							currAddedDeep += stepAddedDeep
							continue
						}
						if typ == stackitem.StructT {
							obj = stackitem.NewStruct([]stackitem.Item{
								stackitem.NewStruct(getArrayWithSizeAndDeep(currRemovedSize, currRemovedDeep)),
							})
							addedItem = stackitem.NewStruct(getArrayWithSizeAndDeep(currAddedSize, currAddedDeep))
						} else {
							obj = stackitem.NewArray([]stackitem.Item{
								stackitem.NewArray(getArrayWithSizeAndDeep(currRemovedSize, currRemovedDeep)),
							})
							addedItem = stackitem.NewArray(getArrayWithSizeAndDeep(currAddedSize, currAddedDeep))
						}
						testCases = append(testCases, testCase{
							name:  fmt.Sprintf("%s %s %d %d %d %d", opcode.SETITEM, typ, currRemovedSize, currRemovedDeep, currAddedSize, currAddedDeep),
							items: []any{obj, 0, addedItem},
						})
						currAddedDeep += stepAddedDeep
					}
					currAddedSize += stepSize
				}
				currRemovedDeep += stepRemovedDeep
			}
			currRemovedSize += stepSize
		}
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamSlotsPushReadOnlyVM(opcode.SETITEM, nil, 0, 0, 0, false, tc.items...))
		})
	}
}

func BenchmarkReverseItems(b *testing.B) {
	type testCase struct {
		name  string
		items []any
	}
	var (
		testCases []testCase
		stepSize  = MaxStackSize / countPoints
		currSize  = stepSize
	)
	for _, typ := range []stackitem.Type{stackitem.BufferT, stackitem.ArrayT} {
		for range countPoints {
			var items []any
			if typ != stackitem.BufferT {
				items = []any{stackitem.NewArray(make([]stackitem.Item, currSize))}
			} else {
				items = []any{stackitem.NewBuffer(make([]byte, currSize))}
			}
			testCases = append(testCases, testCase{
				name:  fmt.Sprintf("%s %s %d", opcode.REVERSEITEMS, typ, currSize),
				items: items,
			})
			currSize += stepSize
		}
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamSlotsPushReadOnlyVM(opcode.REVERSEITEMS, nil, 0, 0, 0, false, tc.items...))
		})
	}
}

func BenchmarkRemove(b *testing.B) {
	type testCase struct {
		name  string
		items []any
	}
	var (
		testCases            []testCase
		stepItemLen          = MaxStackSize / countPoints
		stepIndependentParam = MaxStackSize / countPoints
		currItemLen          = stepItemLen
	)
	for range countPoints {
		currIndependentParam := stepIndependentParam
		for range countPoints {
			var (
				stepDependentParam = MaxStackSize / (countPoints * currIndependentParam)
				currDependentParam = stepDependentParam
			)
			for range countPoints {
				arrWithItemIndependentLen := make([]stackitem.Item, currItemLen)
				arrWithItemIndependentLen[0] = stackitem.NewArray(getArrayWithSizeAndDeep(currIndependentParam, currDependentParam))
				testCases = append(testCases, testCase{
					name:  fmt.Sprintf("%s %s %d %d %d", opcode.REMOVE, stackitem.ArrayT, currItemLen, currIndependentParam, currDependentParam),
					items: []any{arrWithItemIndependentLen, 0},
				})
				arrWithItemIndependentDeep := make([]stackitem.Item, currItemLen)
				arrWithItemIndependentDeep[0] = stackitem.NewArray(getArrayWithSizeAndDeep(currDependentParam, currIndependentParam))
				testCases = append(testCases, testCase{
					name:  fmt.Sprintf("%s %s %d %d %d", opcode.REMOVE, stackitem.ArrayT, currItemLen, currDependentParam, currIndependentParam),
					items: []any{arrWithItemIndependentDeep, 0},
				})
				mapWithItemIndependentLen := make([]stackitem.MapElement, currItemLen)
				for key := range currItemLen {
					mapWithItemIndependentLen = append(mapWithItemIndependentLen, stackitem.MapElement{
						Key:   stackitem.Make(key),
						Value: nil,
					})
				}
				mapWithItemIndependentLen[currItemLen-1].Value = stackitem.NewStruct(getArrayWithSizeAndDeep(currIndependentParam, currDependentParam))
				testCases = append(testCases, testCase{
					name:  fmt.Sprintf("%s %s %d %d %d", opcode.REMOVE, stackitem.MapT, currItemLen, currIndependentParam, currDependentParam),
					items: []any{stackitem.NewMapWithValue(mapWithItemIndependentLen), currItemLen - 1},
				})
				mapWithItemIndependentDeep := make([]stackitem.MapElement, currItemLen)
				for key := range currItemLen {
					mapWithItemIndependentDeep = append(mapWithItemIndependentDeep, stackitem.MapElement{
						Key:   stackitem.Make(key),
						Value: nil,
					})
				}
				mapWithItemIndependentDeep[currItemLen-1].Value = stackitem.NewStruct(getArrayWithSizeAndDeep(currDependentParam, currIndependentParam))
				testCases = append(testCases, testCase{
					name:  fmt.Sprintf("%s %s %d %d %d", opcode.REMOVE, stackitem.MapT, currItemLen, currDependentParam, currIndependentParam),
					items: []any{stackitem.NewMapWithValue(mapWithItemIndependentDeep), currItemLen - 1},
				})
				currDependentParam += stepDependentParam
			}
			currIndependentParam += stepIndependentParam
		}
		currItemLen += stepItemLen
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamSlotsPushReadOnlyVM(opcode.REMOVE, nil, 0, 0, 0, false, tc.items...))
		})
	}
}

func BenchmarkClearItems(b *testing.B) {
	type testCase struct {
		name  string
		items []any
	}
	var (
		testCases            []testCase
		stepIndependentParam = MaxStackSize / countPoints
		currIndependentParam = stepIndependentParam
	)
	for range countPoints {
		var (
			stepDependentParam = MaxStackSize / (countPoints * currIndependentParam)
			currDependentParam = stepDependentParam
		)
		for range countPoints {
			testCases = append(testCases, testCase{
				name:  fmt.Sprintf("%s %s %d %d", opcode.CLEARITEMS, stackitem.ArrayT, currIndependentParam, currDependentParam),
				items: []any{getArrayWithSizeAndDeep(currIndependentParam, currDependentParam)},
			})
			testCases = append(testCases, testCase{
				name:  fmt.Sprintf("%s %s %d %d", opcode.CLEARITEMS, stackitem.ArrayT, currDependentParam, currIndependentParam),
				items: []any{getArrayWithSizeAndDeep(currDependentParam, currIndependentParam)},
			})
			currDependentParam += stepDependentParam
		}
		currIndependentParam += stepIndependentParam
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(opcode.CLEARITEMS, nil, tc.items...))
		})
	}
}

func BenchmarkPopItem(b *testing.B) {
	type testCase struct {
		name  string
		items []any
	}
	var (
		testCases            []testCase
		stepIndependentParam = MaxStackSize / countPoints
		currIndependentParam = stepIndependentParam
	)
	for range countPoints {
		var (
			stepDependentParam = MaxStackSize / (countPoints * currIndependentParam)
			currDependentParam = stepDependentParam
		)
		for range countPoints {
			testCases = append(testCases, testCase{
				name:  fmt.Sprintf("%s %s %d %d", opcode.POPITEM, stackitem.ArrayT, currIndependentParam, currDependentParam),
				items: []any{[]any{getArrayWithSizeAndDeep(currIndependentParam, currDependentParam)}},
			})
			testCases = append(testCases, testCase{
				name:  fmt.Sprintf("%s %s %d %d", opcode.POPITEM, stackitem.ArrayT, currDependentParam, currIndependentParam),
				items: []any{[]any{getArrayWithSizeAndDeep(currDependentParam, currIndependentParam)}},
			})
			currDependentParam += stepDependentParam
		}
		currIndependentParam += stepIndependentParam
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(opcode.POPITEM, nil, tc.items...))
		})
	}
}

func BenchmarkSize(b *testing.B) {
	type testCase struct {
		name  string
		items []any
	}
	var (
		testCases []testCase
		stepLen   = MaxStackSize / countPoints
		currLen   = stepLen
	)
	for range countPoints {
		testCases = append(testCases, testCase{
			name:  fmt.Sprintf("%s %s %d", opcode.SIZE, stackitem.ArrayT, currLen),
			items: []any{make([]any, currLen)},
		})
		currLen += stepLen
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(opcode.SIZE, nil, tc.items...))
		})
	}
}

func BenchmarkCall(b *testing.B) {
	type testCase struct {
		op     opcode.Opcode
		name   string
		params []byte
		items  []any
	}
	var (
		testCases    []testCase
		stepPosition = maxScriptLen / countPoints
	)
	for _, op := range []opcode.Opcode{opcode.CALL, opcode.CALLL, opcode.CALLA} {
		currPosition := stepPosition
		for range countPoints {
			var (
				items []any
				sc    = make([]byte, currPosition+1)
			)
			sc[0] = byte(op)
			if op == opcode.CALLA {
				items = []any{stackitem.NewPointer(currPosition, sc)}
			}
			testCases = append(testCases, testCase{
				op:     op,
				name:   fmt.Sprintf("%s %d", op, currPosition),
				params: sc[1:],
				items:  items,
			})
			currPosition += stepPosition
		}
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(tc.op, tc.params, tc.items...))
		})
	}
}

func BenchmarkKeys(b *testing.B) {
	type testCase struct {
		name  string
		items []any
	}
	var (
		testCases []testCase
		stepLen   = MaxStackSize / countPoints
		currLen   = stepLen
		key       = make([]byte, stackitem.MaxKeySize)
	)
	for range countPoints {
		m := make([]stackitem.MapElement, 0, currLen)
		for range currLen {
			m = append(m, stackitem.MapElement{
				Key: stackitem.NewByteArray(key),
			})
		}
		testCases = append(testCases, testCase{
			name:  fmt.Sprintf("%s %d", opcode.KEYS, currLen),
			items: []any{stackitem.NewMapWithValue(m)},
		})
		currLen += stepLen
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(opcode.KEYS, nil, tc.items...))
		})
	}
}

func BenchmarkValues(b *testing.B) {
	type testCase struct {
		name  string
		items []any
	}
	var (
		testCases            []testCase
		stepIndependentParam = MaxStackSize / countPoints
		currIndependentParam = stepIndependentParam
	)
	for range countPoints {
		var (
			stepDependentParam = MaxStackSize / (countPoints * currIndependentParam)
			currDependentParam = stepDependentParam
		)
		for range countPoints {
			testCases = append(testCases, testCase{
				name:  fmt.Sprintf("%s %s %d %d", opcode.VALUES, stackitem.StructT, currIndependentParam, currDependentParam),
				items: []any{stackitem.NewStruct(getStructWithSizeAndDeep(currIndependentParam, currDependentParam))},
			})
			testCases = append(testCases, testCase{
				name:  fmt.Sprintf("%s %s %d %d", opcode.VALUES, stackitem.StructT, currDependentParam, currIndependentParam),
				items: []any{stackitem.NewStruct(getStructWithSizeAndDeep(currDependentParam, currIndependentParam))},
			})
			currDependentParam += stepDependentParam
		}
		currIndependentParam += stepIndependentParam
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(opcode.VALUES, nil, tc.items...))
		})
	}
}

func BenchmarkHasKey(b *testing.B) {
	type testCase struct {
		name  string
		items []any
	}
	var (
		testCases []testCase
		stepLen   = MaxStackSize / countPoints
		currLen   = stepLen
		key       = make([]byte, stackitem.MaxKeySize)
	)
	key[stackitem.MaxKeySize-1] = 1
	for range countPoints {
		m := make([]stackitem.MapElement, 0, currLen)
		for range currLen {
			m = append(m, stackitem.MapElement{
				Key: stackitem.NewByteArray(key),
			})
		}
		testCases = append(testCases, testCase{
			name:  fmt.Sprintf("%s %s %d", opcode.HASKEY, stackitem.MapT, currLen),
			items: []any{stackitem.NewMapWithValue(m), make([]byte, stackitem.MaxKeySize)},
		})
		currLen += stepLen
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(opcode.HASKEY, nil, tc.items...))
		})
	}
}
