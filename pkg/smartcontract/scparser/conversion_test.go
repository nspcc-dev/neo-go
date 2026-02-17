package scparser

import (
	"bytes"
	"fmt"
	"math"
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/require"
)

func TestGetIntFromInstr(t *testing.T) {
	t.Run("small int", func(t *testing.T) {
		check := func(t *testing.T, prog []byte, expected int64, errExpected bool) {
			ctx := NewContext(prog, 0)
			instr, param, err := ctx.Next()
			require.NoError(t, err)

			actual, err := GetInt64FromInstr(Instruction{Op: instr, Param: param})
			if errExpected {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, expected, actual)
			}

			actualBig, err := GetBigIntFromInstr(Instruction{Op: instr, Param: param})
			if errExpected {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, big.NewInt(expected), actualBig)
			}
		}

		// Good.
		for _, i := range []int64{
			-1,                 // PUSHM1
			0, 1, 2, 3, 15, 16, // PUSH0..PUSH16
			17, math.MaxInt8, math.MinInt8, // PUSHINT8
			math.MaxInt8 + 1, math.MaxInt16, math.MinInt16, // PUSHINT16
			-100500, math.MaxInt16 + 1, math.MaxInt32, math.MinInt32, // PUSHINT32
			math.MaxInt32 + 1, math.MaxInt64, math.MinInt64, // PUSHINT64
		} {
			t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
				w := io.NewBufBinWriter()
				emit.Int(w.BinWriter, i)
				check(t, w.Bytes(), i, false)
			})
		}

		// Bad.
		w := io.NewBufBinWriter()
		emit.String(w.BinWriter, "abeunt studia in mores")
		check(t, w.Bytes(), 0, true)
	})

	t.Run("big int", func(t *testing.T) {
		check := func(t *testing.T, prog []byte, expected *big.Int) {
			ctx := NewContext(prog, 0)
			instr, param, err := ctx.Next()
			require.NoError(t, err)

			_, err = GetInt64FromInstr(Instruction{Op: instr, Param: param})
			require.Error(t, err)

			actualBig, err := GetBigIntFromInstr(Instruction{Op: instr, Param: param})
			require.NoError(t, err)
			require.Equal(t, expected, actualBig)
		}

		for _, i := range []*big.Int{
			new(big.Int).Add(big.NewInt(math.MaxInt64), big.NewInt(1)),
			new(big.Int).Sub(big.NewInt(math.MinInt64), big.NewInt(1)),
		} {
			w := io.NewBufBinWriter()
			emit.BigInt(w.BinWriter, i)
			check(t, w.Bytes(), i)
		}
	})
}

func TestGetStringFromInstr(t *testing.T) {
	check := func(t *testing.T, prog []byte, expected string, errExpected bool) {
		ctx := NewContext(prog, 0)
		instr, param, err := ctx.Next()
		require.NoError(t, err)

		actual, err := GetStringFromInstr(Instruction{Op: instr, Param: param})
		if errExpected {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.Equal(t, expected, actual)
		}
	}

	// Good.
	for i, s := range []string{
		"", "conscientia mille testes", // PUSHDATA1
		string(make([]byte, 0x10000-1)), // PUSHDATA2
		string(make([]byte, 0x10000)),   // PUSHDATA4
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			w := io.NewBufBinWriter()
			emit.String(w.BinWriter, s)
			check(t, w.Bytes(), s, false)
		})
	}

	// Bad.
	w := io.NewBufBinWriter()
	emit.Int(w.BinWriter, 123)
	check(t, w.Bytes(), "", true)
}

func TestGetUTF8StringFromInstr(t *testing.T) {
	check := func(t *testing.T, prog []byte, expected string, errExpected bool) {
		ctx := NewContext(prog, 0)
		instr, param, err := ctx.Next()
		require.NoError(t, err)

		actual, err := GetUTF8StringFromInstr(Instruction{Op: instr, Param: param})
		if errExpected {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.Equal(t, expected, actual)
		}
	}

	// Good.
	for i, s := range []string{
		"", "conscientia mille testes", // PUSHDATA1
		string(make([]byte, 0x10000-1)), // PUSHDATA2
		string(make([]byte, 0x10000)),   // PUSHDATA4
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			w := io.NewBufBinWriter()
			emit.String(w.BinWriter, s)
			check(t, w.Bytes(), s, false)
		})
	}

	// Bad.
	w := io.NewBufBinWriter()
	emit.Bytes(w.BinWriter, []byte{0xC0}) // non-utf8.
	check(t, w.Bytes(), "", true)
}

func TestGetBoolFromInstr(t *testing.T) {
	check := func(t *testing.T, prog []byte, expected bool, errExpected bool) {
		ctx := NewContext(prog, 0)
		instr, param, err := ctx.Next()
		require.NoError(t, err)

		actual, err := GetBoolFromInstr(Instruction{Op: instr, Param: param})
		if errExpected {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.Equal(t, expected, actual)
		}
	}

	// Good, true.
	w := io.NewBufBinWriter()
	emit.Bool(w.BinWriter, true)
	check(t, w.Bytes(), true, false)
	w.Reset()
	emit.Int(w.BinWriter, 1)
	check(t, w.Bytes(), true, false)

	// Good, false.
	w.Reset()
	emit.Bool(w.BinWriter, false)
	check(t, w.Bytes(), false, false)
	w.Reset()
	emit.Int(w.BinWriter, 0)
	check(t, w.Bytes(), false, false)

	// Bad.
	w.Reset()
	emit.Int(w.BinWriter, 123)
	check(t, w.Bytes(), false, true)
}

func TestGetUint160FromInstr(t *testing.T) {
	check := func(t *testing.T, prog []byte, expected util.Uint160, errExpected bool) {
		ctx := NewContext(prog, 0)
		instr, param, err := ctx.Next()
		require.NoError(t, err)

		actual, err := GetUint160FromInstr(Instruction{Op: instr, Param: param})
		if errExpected {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.Equal(t, expected, actual)
		}
	}

	// Good, true.
	w := io.NewBufBinWriter()
	emit.Any(w.BinWriter, util.Uint160{1, 2, 3})
	check(t, w.Bytes(), util.Uint160{1, 2, 3}, false)

	// Bad.
	w.Reset()
	emit.Int(w.BinWriter, 123)
	check(t, w.Bytes(), util.Uint160{}, true)
}

func TestGetUint256FromInstr(t *testing.T) {
	check := func(t *testing.T, prog []byte, expected util.Uint256, errExpected bool) {
		ctx := NewContext(prog, 0)
		instr, param, err := ctx.Next()
		require.NoError(t, err)

		actual, err := GetUint256FromInstr(Instruction{Op: instr, Param: param})
		if errExpected {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.Equal(t, expected, actual)
		}
	}

	// Good, true.
	w := io.NewBufBinWriter()
	emit.Any(w.BinWriter, util.Uint256{1, 2, 3})
	check(t, w.Bytes(), util.Uint256{1, 2, 3}, false)

	// Bad.
	w.Reset()
	emit.Int(w.BinWriter, 123)
	check(t, w.Bytes(), util.Uint256{}, true)
}

func TestGetSignatureFromInstr(t *testing.T) {
	check := func(t *testing.T, prog []byte, expected []byte, errExpected bool) {
		ctx := NewContext(prog, 0)
		instr, param, err := ctx.Next()
		require.NoError(t, err)

		actual, err := GetSignatureFromInstr(Instruction{Op: instr, Param: param})
		if errExpected {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.Equal(t, expected, actual)
		}
	}

	// Good, true.
	s := make([]byte, keys.SignatureLen)
	w := io.NewBufBinWriter()
	emit.Bytes(w.BinWriter, s)
	check(t, w.Bytes(), s, false)

	// Bad, invalid item.
	w.Reset()
	emit.Int(w.BinWriter, 123)
	check(t, w.Bytes(), nil, true)

	// Bad, invalid len.
	w.Reset()
	emit.Bytes(w.BinWriter, []byte{1, 2, 3})
	check(t, w.Bytes(), nil, true)
}

func TestGetPublicKeyFromInstr(t *testing.T) {
	check := func(t *testing.T, prog []byte, expected *keys.PublicKey, errExpected bool) {
		ctx := NewContext(prog, 0)
		instr, param, err := ctx.Next()
		require.NoError(t, err)

		actual, err := GetPublicKeyFromInstr(Instruction{Op: instr, Param: param})
		if errExpected {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.Equal(t, expected, actual)
		}
	}

	// Good, true.
	pk, err := keys.NewPrivateKey()
	require.NoError(t, err)
	w := io.NewBufBinWriter()
	emit.Bytes(w.BinWriter, pk.PublicKey().Bytes())
	check(t, w.Bytes(), pk.PublicKey(), false)

	// Bad, invalid item.
	w.Reset()
	emit.Int(w.BinWriter, 123)
	check(t, w.Bytes(), nil, true)

	// Bad, invalid len.
	w.Reset()
	emit.Bytes(w.BinWriter, []byte{1, 2, 3})
	check(t, w.Bytes(), nil, true)
}

func TestGetListFromContext(t *testing.T) {
	t.Run("int", func(t *testing.T) {
		check := func(t *testing.T, prog []byte, expected []int64, strict bool, errText ...string) {
			ctx := NewContext(prog, 0)
			actual, err := GetListOfEFromContext[int64](ctx, GetInt64FromInstr)
			if len(errText) > 0 {
				require.Error(t, err)
				require.ErrorContains(t, err, errText[0])
			} else {
				require.NoError(t, err)
				require.Equal(t, expected, actual)
			}
		}

		t.Run("good", func(t *testing.T) {
			w := io.NewBufBinWriter()
			emit.Any(w.BinWriter, []any{5, 6, 7})
			check(t, w.Bytes(), []int64{5, 6, 7}, true)
		})
		t.Run("empty", func(t *testing.T) {
			w := io.NewBufBinWriter()
			emit.Any(w.BinWriter, []any{})
			check(t, w.Bytes(), []int64{}, true)
		})
		t.Run("reversed", func(t *testing.T) {
			w := io.NewBufBinWriter()
			emit.Any(w.BinWriter, []any{1, 2, 3})
			emit.Opcodes(w.BinWriter, opcode.DUP, opcode.REVERSEITEMS)
			check(t, w.Bytes(), []int64{3, 2, 1}, true)
		})
		t.Run("SWAP-ed elements", func(t *testing.T) {
			w := io.NewBufBinWriter()
			emit.Any(w.BinWriter, 3)
			emit.Any(w.BinWriter, 2)
			emit.Any(w.BinWriter, 1)
			emit.Opcodes(w.BinWriter, opcode.SWAP)
			emit.Int(w.BinWriter, 3)
			emit.Opcodes(w.BinWriter, opcode.PACK)
			check(t, w.Bytes(), []int64{2, 1, 3}, true)
		})
		t.Run("REVERSE3-ed elements", func(t *testing.T) {
			w := io.NewBufBinWriter()
			emit.Any(w.BinWriter, 4)
			emit.Any(w.BinWriter, 3)
			emit.Any(w.BinWriter, 2)
			emit.Any(w.BinWriter, 1)
			emit.Opcodes(w.BinWriter, opcode.REVERSE3)
			emit.Int(w.BinWriter, 4)
			emit.Opcodes(w.BinWriter, opcode.PACK)
			check(t, w.Bytes(), []int64{3, 2, 1, 4}, true)
		})
		t.Run("REVERSE4-ed elements", func(t *testing.T) {
			w := io.NewBufBinWriter()
			emit.Any(w.BinWriter, 4)
			emit.Any(w.BinWriter, 3)
			emit.Any(w.BinWriter, 2)
			emit.Any(w.BinWriter, 1)
			emit.Opcodes(w.BinWriter, opcode.REVERSE3)
			emit.Int(w.BinWriter, 4)
			emit.Opcodes(w.BinWriter, opcode.PACK)
			check(t, w.Bytes(), []int64{3, 2, 1, 4}, true)
		})
		t.Run("REVERSEN-ed elements", func(t *testing.T) {
			w := io.NewBufBinWriter()
			emit.Any(w.BinWriter, 4)
			emit.Any(w.BinWriter, 3)
			emit.Any(w.BinWriter, 2)
			emit.Any(w.BinWriter, 1)
			emit.Any(w.BinWriter, 3)
			emit.Opcodes(w.BinWriter, opcode.REVERSEN)
			emit.Int(w.BinWriter, 4)
			emit.Opcodes(w.BinWriter, opcode.PACK)
			check(t, w.Bytes(), []int64{3, 2, 1, 4}, true)
		})
		t.Run("non-strict PACK", func(t *testing.T) {
			w := io.NewBufBinWriter()
			emit.Any(w.BinWriter, 3)
			emit.Any(w.BinWriter, 2)
			emit.Any(w.BinWriter, 1)
			emit.Int(w.BinWriter, 5)
			emit.Opcodes(w.BinWriter, opcode.PACK)
			check(t, w.Bytes(), nil, false, "insufficient number of elements for PACK: expected 5, has 3")
		})
		t.Run("REVERSEN error", func(t *testing.T) {
			w := io.NewBufBinWriter()
			emit.Opcodes(w.BinWriter, opcode.REVERSEN)
			check(t, w.Bytes(), nil, true, "REVERSEN instruction requires at least 1 element on stack")
		})
		t.Run("missing PACK", func(t *testing.T) {
			w := io.NewBufBinWriter()
			emit.String(w.BinWriter, "hello")
			check(t, w.Bytes(), []int64{}, true, "expected Array/Struct on stack, got PUSHDATA1")
		})
		t.Run("too large list", func(t *testing.T) {
			arr := make([]any, maxStackSize+1)
			for i := range arr {
				arr[i] = int64(i)
			}
			w := io.NewBufBinWriter()
			emit.Any(w.BinWriter, arr)
			check(t, w.Bytes(), []int64{}, true, "number of elements exceeds 2048")
		})
		t.Run("mismatched number of elements", func(t *testing.T) {
			w := io.NewBufBinWriter()
			emit.Int(w.BinWriter, 1)
			emit.Any(w.BinWriter, []any{2, 3, 4})
			check(t, w.Bytes(), []int64{}, true, "unexpected number of elements on stack: expected 1 Array/Struct, got 2 elements")
		})
	})
	t.Run("bool", func(t *testing.T) {
		check := func(t *testing.T, prog []byte, expected []bool, errExpected bool, errText ...string) {
			ctx := NewContext(prog, 0)

			actual, err := GetListOfEFromContext[bool](ctx, GetBoolFromInstr)
			if errExpected {
				require.Error(t, err)
				if len(errText) > 0 {
					require.ErrorContains(t, err, errText[0])
				}
			} else {
				require.NoError(t, err)
				require.Equal(t, expected, actual)
			}
		}

		t.Run("good", func(t *testing.T) {
			w := io.NewBufBinWriter()
			emit.Any(w.BinWriter, []any{true, false, true})
			check(t, w.Bytes(), []bool{true, false, true}, false)
		})
		t.Run("empty", func(t *testing.T) {
			w := io.NewBufBinWriter()
			emit.Any(w.BinWriter, []any{})
			check(t, w.Bytes(), []bool{}, false)
		})
		t.Run("reversed", func(t *testing.T) {
			w := io.NewBufBinWriter()
			emit.Any(w.BinWriter, []any{true, false})
			emit.Opcodes(w.BinWriter, opcode.DUP, opcode.REVERSEITEMS)
			check(t, w.Bytes(), []bool{false, true}, false)
		})
		t.Run("invalid list length element", func(t *testing.T) {
			w := io.NewBufBinWriter()
			emit.Bool(w.BinWriter, true)
			emit.Bool(w.BinWriter, false)
			emit.Opcodes(w.BinWriter, opcode.PACK)
			check(t, w.Bytes(), []bool{}, true, "failed to parse PACK argument")
		})
	})
	t.Run("uint160", func(t *testing.T) {
		check := func(t *testing.T, prog []byte, expected []util.Uint160, errExpected bool) {
			ctx := NewContext(prog, 0)

			actual, err := GetListOfEFromContext[util.Uint160](ctx, GetUint160FromInstr)
			if errExpected {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, expected, actual)
			}
		}

		t.Run("good", func(t *testing.T) {
			w := io.NewBufBinWriter()
			emit.Any(w.BinWriter, []any{util.Uint160{1, 2, 3}, util.Uint160{4, 5, 6}})
			check(t, w.Bytes(), []util.Uint160{{1, 2, 3}, {4, 5, 6}}, false)
		})
		t.Run("empty", func(t *testing.T) {
			w := io.NewBufBinWriter()
			emit.Any(w.BinWriter, []any{})
			check(t, w.Bytes(), []util.Uint160{}, false)
		})
		t.Run("reversed", func(t *testing.T) {
			w := io.NewBufBinWriter()
			emit.Any(w.BinWriter, []any{util.Uint160{1, 2, 3}, util.Uint160{4, 5, 6}})
			emit.Opcodes(w.BinWriter, opcode.DUP, opcode.REVERSEITEMS)
			check(t, w.Bytes(), []util.Uint160{{4, 5, 6}, {1, 2, 3}}, false)
		})
		t.Run("bad", func(t *testing.T) {
			w := io.NewBufBinWriter()
			emit.Int(w.BinWriter, 123)
			check(t, w.Bytes(), []util.Uint160{}, true)
		})
	})
	t.Run("non-strict", func(t *testing.T) {
		w := io.NewBufBinWriter()
		emit.Any(w.BinWriter, 3)
		emit.Any(w.BinWriter, 2)
		emit.Any(w.BinWriter, 1)
		emit.Int(w.BinWriter, 5)
		emit.Opcodes(w.BinWriter, opcode.PACK)
		prog := w.Bytes()

		ctx := NewContext(prog, 0)
		args, err := GetListFromContextNonStrict(ctx)
		require.NoError(t, err)
		require.Equal(t, 5, len(args))
		actualArgs, err := GetListOfEFromPushedItems(args[:3], GetInt64FromInstr)
		require.NoError(t, err)
		require.Equal(t, []int64{1, 2, 3}, actualArgs)
		require.True(t, args[3].IsEmpty())
		require.True(t, args[4].IsEmpty())
	})
}

func TestGetAppCallNoArgsFromContext(t *testing.T) {
	h := util.Uint160{1, 2, 3}
	m := "myPrettyMethod"
	f := callflag.ReadStates | callflag.AllowNotify
	check := func(t *testing.T, prog []byte, errText string) {
		ctx := NewContext(prog, 0)

		actualH, actualM, actualF, err := GetAppCallNoArgsFromContext(ctx)
		if len(errText) > 0 {
			require.ErrorContains(t, err, errText)
		} else {
			require.NoError(t, err)
			require.Equal(t, h, actualH)
			require.Equal(t, m, actualM)
			require.Equal(t, f, actualF)
		}
	}
	t.Run("good", func(t *testing.T) {
		w := io.NewBufBinWriter()
		emit.AppCallNoArgs(w.BinWriter, h, m, f)
		check(t, w.Bytes(), "")
	})
	t.Run("missing callflag", func(t *testing.T) {
		w := io.NewBufBinWriter()
		emit.String(w.BinWriter, m)
		emit.Bytes(w.BinWriter, h.BytesBE())
		emit.Syscall(w.BinWriter, interopnames.SystemContractCall)
		check(t, w.Bytes(), "failed to parse callflag")
	})
	t.Run("invalid callflag", func(t *testing.T) {
		w := io.NewBufBinWriter()
		emit.Int(w.BinWriter, 123)
		emit.String(w.BinWriter, m)
		emit.Bytes(w.BinWriter, h.BytesBE())
		emit.Syscall(w.BinWriter, interopnames.SystemContractCall)
		check(t, w.Bytes(), "invalid callflag value")
	})
	t.Run("invalid method", func(t *testing.T) {
		w := io.NewBufBinWriter()
		emit.Int(w.BinWriter, int64(f))
		emit.Bool(w.BinWriter, true)
		emit.Bytes(w.BinWriter, h.BytesBE())
		emit.Syscall(w.BinWriter, interopnames.SystemContractCall)
		check(t, w.Bytes(), "failed to parse method")
	})
	t.Run("invalid scripthash", func(t *testing.T) {
		w := io.NewBufBinWriter()
		emit.Int(w.BinWriter, int64(f))
		emit.String(w.BinWriter, m)
		emit.Bytes(w.BinWriter, []byte{1, 2, 3})
		emit.Syscall(w.BinWriter, interopnames.SystemContractCall)
		check(t, w.Bytes(), "failed to parse contract scripthash")
	})
	t.Run("not a SYSCALL", func(t *testing.T) {
		w := io.NewBufBinWriter()
		emit.Int(w.BinWriter, int64(f))
		emit.String(w.BinWriter, m)
		emit.Bytes(w.BinWriter, h.BytesBE())
		emit.Opcodes(w.BinWriter, opcode.PUSH0)
		check(t, w.Bytes(), "expected SYSCALL, got PUSH0")
	})
	t.Run("unknown SYSCALL", func(t *testing.T) {
		w := io.NewBufBinWriter()
		emit.Int(w.BinWriter, int64(f))
		emit.String(w.BinWriter, m)
		emit.Bytes(w.BinWriter, h.BytesBE())
		emit.Syscall(w.BinWriter, "Neo.Ledger.Unknown")
		check(t, w.Bytes(), "failed to parse SYSCALL parameter: interop not found")
	})
	t.Run("invalid SYSCALL", func(t *testing.T) {
		w := io.NewBufBinWriter()
		emit.Int(w.BinWriter, int64(f))
		emit.String(w.BinWriter, m)
		emit.Bytes(w.BinWriter, h.BytesBE())
		emit.Syscall(w.BinWriter, interopnames.SystemRuntimeCheckWitness)
		check(t, w.Bytes(), "expected System.Contract.Call SYSCALL")
	})
}

func TestGetAppCallFromContext(t *testing.T) {
	h := util.Uint160{1, 2, 3}
	m := "myPrettyMethod"
	f := callflag.ReadStates | callflag.AllowNotify

	t.Run("good", func(t *testing.T) {
		w := io.NewBufBinWriter()
		emit.AppCall(w.BinWriter, h, m, f, 1, 2, 3)
		prog := w.Bytes()

		ctx := NewContext(prog, 0)
		actualH, actualM, actualF, args, err := GetAppCallFromContext(ctx)
		require.NoError(t, err)
		actualArgs, err := GetListOfEFromPushedItems(args, GetInt64FromInstr)
		require.NoError(t, err)
		require.Equal(t, []int64{1, 2, 3}, actualArgs)
		require.Equal(t, h, actualH)
		require.Equal(t, m, actualM)
		require.Equal(t, f, actualF)

		// Ensure there's no data in the end.
		require.Equal(t, len(prog), ctx.NextIP())
		op, _, err := ctx.Next()
		require.NoError(t, err)
		require.Equal(t, opcode.RET, op)
	})
	t.Run("invalid args", func(t *testing.T) {
		w := io.NewBufBinWriter()
		emit.Int(w.BinWriter, 0) // extra arg.
		emit.AppCall(w.BinWriter, h, m, f, 1, 2, 3)
		prog := w.Bytes()

		ctx := NewContext(prog, 0)
		_, _, _, _, err := GetAppCallFromContext(ctx) // try to parse strings instead of ints.
		require.ErrorContains(t, err, "failed to parse arguments")
	})
	t.Run("invalid appcall", func(t *testing.T) {
		w := io.NewBufBinWriter()
		emit.AppCall(w.BinWriter, h, m, 123, 1, 2, 3) // invalid callflag.
		prog := w.Bytes()

		ctx := NewContext(prog, 0)
		_, _, _, _, err := GetAppCallFromContext(ctx)
		require.ErrorContains(t, err, "ailed to parse AppCall without args")
	})
}

func TestParseAppCall(t *testing.T) {
	h := util.Uint160{1, 2, 3}
	m := "myPrettyMethod"
	f := callflag.ReadStates | callflag.AllowNotify

	t.Run("good", func(t *testing.T) {
		w := io.NewBufBinWriter()
		emit.AppCall(w.BinWriter, h, m, f, 1, 2, 3)
		prog := w.Bytes()

		actualH, actualM, actualF, args, err := ParseAppCall(prog)
		require.NoError(t, err)
		actualArgs, err := GetListOfEFromPushedItems(args, GetInt64FromInstr)
		require.NoError(t, err)
		require.Equal(t, []int64{1, 2, 3}, actualArgs)
		require.Equal(t, h, actualH)
		require.Equal(t, m, actualM)
		require.Equal(t, f, actualF)
	})
	t.Run("non-strict", func(t *testing.T) {
		w := io.NewBufBinWriter()
		emit.Any(w.BinWriter, 3)
		emit.Any(w.BinWriter, 2)
		emit.Any(w.BinWriter, 1)
		emit.Int(w.BinWriter, 5)
		emit.Opcodes(w.BinWriter, opcode.PACK)
		emit.AppCallNoArgs(w.BinWriter, h, m, f)
		prog := w.Bytes()

		actualH, actualM, actualF, args, err := ParseAppCallNonStrict(prog)
		require.NoError(t, err)
		require.Equal(t, h, actualH)
		require.Equal(t, m, actualM)
		require.Equal(t, f, actualF)
		require.Equal(t, 5, len(args))
		actualArgs, err := GetListOfEFromPushedItems(args[:3], GetInt64FromInstr)
		require.NoError(t, err)
		require.Equal(t, []int64{1, 2, 3}, actualArgs)
		require.True(t, args[3].IsEmpty())
		require.True(t, args[4].IsEmpty())
	})
	t.Run("bad", func(t *testing.T) {
		w := io.NewBufBinWriter()
		emit.AppCall(w.BinWriter, h, m, 123, 1, 2, 3)
		prog := w.Bytes()

		_, _, _, _, err := ParseAppCall(prog) // invalid callflag.
		require.ErrorContains(t, err, "failed to parse AppCall")
	})
}

func TestParseSomething(t *testing.T) {
	t.Run("map", func(t *testing.T) {
		t.Run("strict", func(t *testing.T) {
			t.Run("good", func(t *testing.T) {
				w := io.NewBufBinWriter()
				emit.Any(w.BinWriter, "val2")
				emit.Any(w.BinWriter, "key2")
				emit.Any(w.BinWriter, "val1")
				emit.Any(w.BinWriter, "key1")
				emit.Int(w.BinWriter, 2)
				emit.Opcodes(w.BinWriter, opcode.PACKMAP)
				prog := w.Bytes()

				res, err := ParseSomething(prog, true)
				require.NoError(t, err)
				require.Equal(t, 1, len(res))
				e := res[0].Map
				require.NotNil(t, e)
				require.Equal(t, 2, len(e))
				require.Equal(t, "key1", string(e[0].Key.Param))
				require.Equal(t, "val1", string(e[0].Value.Param))
				require.Equal(t, "key2", string(e[1].Key.Param))
				require.Equal(t, "val2", string(e[1].Value.Param))
			})
			t.Run("bad", func(t *testing.T) {
				w := io.NewBufBinWriter()
				emit.Any(w.BinWriter, "key2")
				emit.Any(w.BinWriter, "val1")
				emit.Any(w.BinWriter, "key1")
				emit.Int(w.BinWriter, 2)
				emit.Opcodes(w.BinWriter, opcode.PACKMAP)
				prog := w.Bytes()

				_, err := ParseSomething(prog, true)
				require.ErrorContains(t, err, "insufficient number of elements for PACKMAP: expected 4, has 3")
			})
		})
		t.Run("non-strict", func(t *testing.T) {
			t.Run("good", func(t *testing.T) {
				w := io.NewBufBinWriter()
				emit.Any(w.BinWriter, "key2")
				emit.Any(w.BinWriter, "val1")
				emit.Any(w.BinWriter, "key1")
				emit.Int(w.BinWriter, 2)
				emit.Opcodes(w.BinWriter, opcode.PACKMAP)
				prog := w.Bytes()

				res, err := ParseSomething(prog, false)
				require.NoError(t, err)
				require.Equal(t, 1, len(res))
				e := res[0].Map
				require.NotNil(t, e)
				require.Equal(t, 2, len(e))
				require.Equal(t, "key1", string(e[0].Key.Param))
				require.Equal(t, "val1", string(e[0].Value.Param))
				require.Equal(t, "key2", string(e[1].Key.Param))
				require.True(t, e[1].Value.IsEmpty())
			})
		})
	})
	t.Run("unsupported instruction", func(t *testing.T) {
		w := io.NewBufBinWriter()
		emit.Opcodes(w.BinWriter, opcode.NEWMAP)
		prog := w.Bytes()

		_, err := ParseSomething(prog, false)
		require.ErrorContains(t, err, "static parser does not support NEWMAP instruction")
	})
	t.Run("[]byte", func(t *testing.T) {
		t.Run("reversed", func(t *testing.T) {
			w := io.NewBufBinWriter()
			emit.Any(w.BinWriter, []byte{1, 2, 3})
			emit.Opcodes(w.BinWriter, opcode.DUP, opcode.REVERSEITEMS)
			expectedProg := w.Bytes()
			actualProg := bytes.Clone(expectedProg)

			res, err := ParseSomething(actualProg, true)
			require.NoError(t, err)
			require.Equal(t, 1, len(res))
			require.False(t, res[0].IsNested())
			actual, err := GetBytesFromInstr(res[0].Instruction)
			require.NoError(t, err)
			require.Equal(t, []byte{3, 2, 1}, actual)
			require.Equal(t, expectedProg, actualProg) // ensure the original prog is not suffered.
		})
	})
}
