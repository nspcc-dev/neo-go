package vm

import (
	"encoding/binary"
	"strconv"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func benchOpcodeInt(t *testing.B, f func() *VM, fail bool) {
	t.ResetTimer()
	for n := 0; n < t.N; n++ {
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

func opParamPushVM(op opcode.Opcode, param []byte, items ...interface{}) func() *VM {
	return opParamSlotsPushVM(op, param, 0, 0, 0, items...)
}

func opParamSlotsPushVM(op opcode.Opcode, param []byte, sslot int, slotloc int, slotarg int, items ...interface{}) func() *VM {
	return func() *VM {
		script := []byte{byte(op)}
		script = append(script, param...)
		v := load(script)
		v.SyscallHandler = func(_ *VM, _ uint32) error {
			return nil
		}
		if sslot != 0 {
			v.Context().static.init(sslot)
		}
		if slotloc != 0 && slotarg != 0 {
			v.Context().local = v.newSlot(slotloc)
			v.Context().arguments = v.newSlot(slotarg)
		}
		for i := range items {
			item, ok := items[i].(stackitem.Item)
			if ok {
				item = stackitem.DeepCopy(item)
			} else {
				item = stackitem.Make(items[i])
			}
			v.estack.PushVal(item)
		}
		return v
	}
}

func exceptParamPushVM(op opcode.Opcode, param []byte, ilen int, elen int, exception bool, items ...interface{}) func() *VM {
	return func() *VM {
		regVMF := opParamPushVM(op, param, items...)
		v := regVMF()
		if ilen != 0 {
			eCtx := newExceptionHandlingContext(1, 2)
			v.Context().tryStack.PushVal(eCtx)
			for i := 0; i < ilen; i++ {
				v.call(v.Context(), 0)
			}
		} else if elen != 0 {
			for i := 0; i < elen; i++ {
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

func arrayOfIfaces(size int) []interface{} {
	var elems = make([]interface{}, size)
	for i := range elems {
		elems[i] = 1
	}
	return elems
}

func bigMap() *stackitem.Map {
	var m = stackitem.NewMap()
	for i := 0; i < 1024; i++ {
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
		var getitems = func(element interface{}) []interface{} {
			l := cp.l
			pos := cp.pos
			if cp.op == opcode.PICK {
				pos++
				l++
			}
			var items = make([]interface{}, l)
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
		var getitems = func(element interface{}) []interface{} {
			l := sw.num
			if sw.op == opcode.ROLL || sw.op == opcode.REVERSEN {
				l++
			}
			var items = make([]interface{}, l)
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
				var elems = make([]interface{}, n+1)
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
