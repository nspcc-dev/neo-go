package unwrap

import (
	"errors"
	"math"
	"math/big"
	"testing"

	"github.com/google/uuid"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestStdErrors(t *testing.T) {
	funcs := []func(r *result.Invoke, err error) (any, error){
		func(r *result.Invoke, err error) (any, error) {
			return BigInt(r, err)
		},
		func(r *result.Invoke, err error) (any, error) {
			return Bool(r, err)
		},
		func(r *result.Invoke, err error) (any, error) {
			return Int64(r, err)
		},
		func(r *result.Invoke, err error) (any, error) {
			return LimitedInt64(r, err, 0, 1)
		},
		func(r *result.Invoke, err error) (any, error) {
			return Bytes(r, err)
		},
		func(r *result.Invoke, err error) (any, error) {
			return UTF8String(r, err)
		},
		func(r *result.Invoke, err error) (any, error) {
			return PrintableASCIIString(r, err)
		},
		func(r *result.Invoke, err error) (any, error) {
			return Uint160(r, err)
		},
		func(r *result.Invoke, err error) (any, error) {
			return Uint256(r, err)
		},
		func(r *result.Invoke, err error) (any, error) {
			return PublicKey(r, err)
		},
		func(r *result.Invoke, err error) (any, error) {
			_, _, err = SessionIterator(r, err)
			return nil, err
		},
		func(r *result.Invoke, err error) (any, error) {
			_, _, _, err = ArrayAndSessionIterator(r, err)
			return nil, err
		},
		func(r *result.Invoke, err error) (any, error) {
			return Array(r, err)
		},
		func(r *result.Invoke, err error) (any, error) {
			return ArrayOfBools(r, err)
		},
		func(r *result.Invoke, err error) (any, error) {
			return ArrayOfBigInts(r, err)
		},
		func(r *result.Invoke, err error) (any, error) {
			return ArrayOfBytes(r, err)
		},
		func(r *result.Invoke, err error) (any, error) {
			return ArrayOfUTF8Strings(r, err)
		},
		func(r *result.Invoke, err error) (any, error) {
			return ArrayOfUint160(r, err)
		},
		func(r *result.Invoke, err error) (any, error) {
			return ArrayOfUint256(r, err)
		},
		func(r *result.Invoke, err error) (any, error) {
			return ArrayOfPublicKeys(r, err)
		},
		func(r *result.Invoke, err error) (any, error) {
			return Map(r, err)
		},
	}
	t.Run("error on input", func(t *testing.T) {
		for _, f := range funcs {
			_, err := f(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make(42)}}, errors.New("some"))
			require.Error(t, err)
		}
	})

	t.Run("FAULT state", func(t *testing.T) {
		for _, f := range funcs {
			_, err := f(&result.Invoke{State: "FAULT", Stack: []stackitem.Item{stackitem.Make(42)}}, nil)
			require.Error(t, err)
		}
	})
	t.Run("nothing returned", func(t *testing.T) {
		for _, f := range funcs {
			_, err := f(&result.Invoke{State: "HALT"}, errors.New("some"))
			require.Error(t, err)
		}
	})
	t.Run("HALT state with empty stack", func(t *testing.T) {
		for _, f := range funcs {
			_, err := f(&result.Invoke{State: "HALT"}, nil)
			require.Error(t, err)
		}
	})
	t.Run("multiple return values", func(t *testing.T) {
		for _, f := range funcs {
			_, err := f(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make(42), stackitem.Make(42)}}, nil)
			require.Error(t, err)
		}
	})
}

func TestBigInt(t *testing.T) {
	_, err := BigInt(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make([]stackitem.Item{})}}, nil)
	require.Error(t, err)

	i, err := BigInt(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make(42)}}, nil)
	require.NoError(t, err)
	require.Equal(t, big.NewInt(42), i)
}

func TestBool(t *testing.T) {
	_, err := Bool(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make("0x03c564ed28ba3d50beb1a52dcb751b929e1d747281566bd510363470be186bc0")}}, nil)
	require.Error(t, err)

	b, err := Bool(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make(true)}}, nil)
	require.NoError(t, err)
	require.True(t, b)
}

func TestNothing(t *testing.T) {
	// Error on input.
	err := Nothing(&result.Invoke{State: "HALT", Stack: []stackitem.Item{}}, errors.New("some"))
	require.Error(t, err)

	// Nonempty stack.
	err = Nothing(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make(42)}}, nil)
	require.Error(t, err)

	// FAULT state.
	err = Nothing(&result.Invoke{State: "FAULT", Stack: []stackitem.Item{}}, nil)
	require.Error(t, err)

	// Positive.
	err = Nothing(&result.Invoke{State: "HALT", Stack: []stackitem.Item{}}, nil)
	require.NoError(t, err)
}

func TestInt64(t *testing.T) {
	_, err := Int64(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make("0x03c564ed28ba3d50beb1a52dcb751b929e1d747281566bd510363470be186bc0")}}, nil)
	require.Error(t, err)

	_, err = Int64(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make(uint64(math.MaxUint64))}}, nil)
	require.Error(t, err)

	i, err := Int64(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make(42)}}, nil)
	require.NoError(t, err)
	require.Equal(t, int64(42), i)
}

func TestLimitedInt64(t *testing.T) {
	_, err := LimitedInt64(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make("0x03c564ed28ba3d50beb1a52dcb751b929e1d747281566bd510363470be186bc0")}}, nil, math.MinInt64, math.MaxInt64)
	require.Error(t, err)

	_, err = LimitedInt64(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make(uint64(math.MaxUint64))}}, nil, math.MinInt64, math.MaxInt64)
	require.Error(t, err)

	_, err = LimitedInt64(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make(42)}}, nil, 128, 256)
	require.Error(t, err)

	_, err = LimitedInt64(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make(42)}}, nil, 0, 40)
	require.Error(t, err)

	i, err := LimitedInt64(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make(42)}}, nil, 0, 128)
	require.NoError(t, err)
	require.Equal(t, int64(42), i)
}

func TestBytes(t *testing.T) {
	_, err := Bytes(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make([]stackitem.Item{})}}, nil)
	require.Error(t, err)

	b, err := Bytes(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make([]byte{1, 2, 3})}}, nil)
	require.NoError(t, err)
	require.Equal(t, []byte{1, 2, 3}, b)
}

func TestUTF8String(t *testing.T) {
	_, err := UTF8String(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make([]stackitem.Item{})}}, nil)
	require.Error(t, err)

	_, err = UTF8String(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make("\xff")}}, nil)
	require.Error(t, err)

	s, err := UTF8String(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make("value")}}, nil)
	require.NoError(t, err)
	require.Equal(t, "value", s)
}

func TestPrintableASCIIString(t *testing.T) {
	_, err := PrintableASCIIString(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make([]stackitem.Item{})}}, nil)
	require.Error(t, err)

	_, err = PrintableASCIIString(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make("\xff")}}, nil)
	require.Error(t, err)

	_, err = PrintableASCIIString(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make("\n\r")}}, nil)
	require.Error(t, err)

	s, err := PrintableASCIIString(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make("value")}}, nil)
	require.NoError(t, err)
	require.Equal(t, "value", s)
}

func TestUint160(t *testing.T) {
	_, err := Uint160(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make(util.Uint256{1, 2, 3}.BytesBE())}}, nil)
	require.Error(t, err)

	u, err := Uint160(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make(util.Uint160{1, 2, 3}.BytesBE())}}, nil)
	require.NoError(t, err)
	require.Equal(t, util.Uint160{1, 2, 3}, u)
}

func TestUint256(t *testing.T) {
	_, err := Uint256(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make(util.Uint160{1, 2, 3}.BytesBE())}}, nil)
	require.Error(t, err)

	u, err := Uint256(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make(util.Uint256{1, 2, 3}.BytesBE())}}, nil)
	require.NoError(t, err)
	require.Equal(t, util.Uint256{1, 2, 3}, u)
}

func TestPublicKey(t *testing.T) {
	k, err := keys.NewPrivateKey()
	require.NoError(t, err)

	_, err = PublicKey(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make(util.Uint160{1, 2, 3}.BytesBE())}}, nil)
	require.Error(t, err)

	pk, err := PublicKey(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make(k.PublicKey().Bytes())}}, nil)
	require.NoError(t, err)
	require.Equal(t, k.PublicKey(), pk)
}

func TestSessionIterator(t *testing.T) {
	_, _, err := SessionIterator(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make(42)}}, nil)
	require.Error(t, err)

	_, _, err = SessionIterator(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.NewInterop(42)}}, nil)
	require.Error(t, err)

	iid := uuid.New()
	iter := result.Iterator{ID: &iid}
	_, _, err = SessionIterator(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.NewInterop(iter)}}, nil)
	require.Error(t, err)

	sid := uuid.New()
	rs, ri, err := SessionIterator(&result.Invoke{Session: sid, State: "HALT", Stack: []stackitem.Item{stackitem.NewInterop(iter)}}, nil)
	require.NoError(t, err)
	require.Equal(t, sid, rs)
	require.Equal(t, iter, ri)
}

func TestArraySessionIterator(t *testing.T) {
	_, _, _, err := ArrayAndSessionIterator(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make(42)}}, nil)
	require.Error(t, err)

	_, _, _, err = ArrayAndSessionIterator(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.NewInterop(42)}}, nil)
	require.Error(t, err)

	arr := stackitem.NewArray([]stackitem.Item{stackitem.Make(42)})
	ra, rs, ri, err := ArrayAndSessionIterator(&result.Invoke{State: "HALT", Stack: []stackitem.Item{arr}}, nil)
	require.NoError(t, err)
	require.Equal(t, arr.Value(), ra)
	require.Empty(t, rs)
	require.Empty(t, ri)

	_, _, _, err = ArrayAndSessionIterator(&result.Invoke{State: "HALT", Stack: []stackitem.Item{arr, stackitem.NewInterop(42)}}, nil)
	require.Error(t, err)

	iid := uuid.New()
	iter := result.Iterator{ID: &iid}
	_, _, _, err = ArrayAndSessionIterator(&result.Invoke{State: "HALT", Stack: []stackitem.Item{arr, stackitem.NewInterop(iter)}}, nil)
	require.ErrorIs(t, err, ErrNoSessionID)

	sid := uuid.New()
	_, rs, ri, err = ArrayAndSessionIterator(&result.Invoke{Session: sid, State: "HALT", Stack: []stackitem.Item{arr, stackitem.NewInterop(iter)}}, nil)
	require.NoError(t, err)
	require.Equal(t, arr.Value(), ra)
	require.Equal(t, sid, rs)
	require.Equal(t, iter, ri)

	_, _, _, err = ArrayAndSessionIterator(&result.Invoke{Session: sid, State: "HALT", Stack: []stackitem.Item{arr, stackitem.NewInterop(iter), stackitem.Make(42)}}, nil)
	require.Error(t, err)
}

func TestArray(t *testing.T) {
	_, err := Array(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make(42)}}, nil)
	require.Error(t, err)

	a, err := Array(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make([]stackitem.Item{stackitem.Make(42)})}}, nil)
	require.NoError(t, err)
	require.Equal(t, 1, len(a))
	require.Equal(t, stackitem.Make(42), a[0])
}

func TestArrayOfBools(t *testing.T) {
	_, err := ArrayOfBools(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make(42)}}, nil)
	require.Error(t, err)

	_, err = ArrayOfBools(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make([]stackitem.Item{stackitem.Make("reallybigstringthatcantbeanumberandthuscantbeconvertedtobool")})}}, nil)
	require.Error(t, err)

	a, err := ArrayOfBools(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make([]stackitem.Item{stackitem.Make(true)})}}, nil)
	require.NoError(t, err)
	require.Equal(t, 1, len(a))
	require.Equal(t, true, a[0])
}

func TestArrayOfBigInts(t *testing.T) {
	_, err := ArrayOfBigInts(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make(42)}}, nil)
	require.Error(t, err)

	_, err = ArrayOfBigInts(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make([]stackitem.Item{stackitem.Make([]stackitem.Item{})})}}, nil)
	require.Error(t, err)

	a, err := ArrayOfBigInts(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make([]stackitem.Item{stackitem.Make(42)})}}, nil)
	require.NoError(t, err)
	require.Equal(t, 1, len(a))
	require.Equal(t, big.NewInt(42), a[0])
}

func TestArrayOfBytes(t *testing.T) {
	_, err := ArrayOfBytes(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make(42)}}, nil)
	require.Error(t, err)

	_, err = ArrayOfBytes(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make([]stackitem.Item{stackitem.Make([]stackitem.Item{})})}}, nil)
	require.Error(t, err)

	a, err := ArrayOfBytes(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make([]stackitem.Item{stackitem.Make([]byte("some"))})}}, nil)
	require.NoError(t, err)
	require.Equal(t, 1, len(a))
	require.Equal(t, []byte("some"), a[0])
}

func TestArrayOfUTF8Strings(t *testing.T) {
	_, err := ArrayOfUTF8Strings(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make(42)}}, nil)
	require.Error(t, err)

	_, err = ArrayOfUTF8Strings(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make([]stackitem.Item{stackitem.Make([]stackitem.Item{})})}}, nil)
	require.Error(t, err)

	_, err = ArrayOfUTF8Strings(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make([]stackitem.Item{stackitem.Make([]byte{0, 0xff})})}}, nil)
	require.Error(t, err)

	a, err := ArrayOfUTF8Strings(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make([]stackitem.Item{stackitem.Make("some")})}}, nil)
	require.NoError(t, err)
	require.Equal(t, 1, len(a))
	require.Equal(t, "some", a[0])
}

func TestArrayOfUint160(t *testing.T) {
	_, err := ArrayOfUint160(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make(42)}}, nil)
	require.Error(t, err)

	_, err = ArrayOfUint160(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make([]stackitem.Item{stackitem.Make([]stackitem.Item{})})}}, nil)
	require.Error(t, err)

	_, err = ArrayOfUint160(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make([]stackitem.Item{stackitem.Make([]byte("some"))})}}, nil)
	require.Error(t, err)

	u160 := util.Uint160{1, 2, 3}
	uints, err := ArrayOfUint160(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make([]stackitem.Item{stackitem.Make(u160.BytesBE())})}}, nil)
	require.NoError(t, err)
	require.Equal(t, 1, len(uints))
	require.Equal(t, u160, uints[0])
}

func TestArrayOfUint256(t *testing.T) {
	_, err := ArrayOfUint256(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make(42)}}, nil)
	require.Error(t, err)

	_, err = ArrayOfUint256(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make([]stackitem.Item{stackitem.Make([]stackitem.Item{})})}}, nil)
	require.Error(t, err)

	_, err = ArrayOfUint256(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make([]stackitem.Item{stackitem.Make([]byte("some"))})}}, nil)
	require.Error(t, err)

	u256 := util.Uint256{1, 2, 3}
	uints, err := ArrayOfUint256(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make([]stackitem.Item{stackitem.Make(u256.BytesBE())})}}, nil)
	require.NoError(t, err)
	require.Equal(t, 1, len(uints))
	require.Equal(t, u256, uints[0])
}

func TestArrayOfPublicKeys(t *testing.T) {
	_, err := ArrayOfPublicKeys(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make(42)}}, nil)
	require.Error(t, err)

	_, err = ArrayOfPublicKeys(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make([]stackitem.Item{stackitem.Make([]stackitem.Item{})})}}, nil)
	require.Error(t, err)

	_, err = ArrayOfPublicKeys(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make([]stackitem.Item{stackitem.Make([]byte("some"))})}}, nil)
	require.Error(t, err)

	k, err := keys.NewPrivateKey()
	require.NoError(t, err)

	pks, err := ArrayOfPublicKeys(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make([]stackitem.Item{stackitem.Make(k.PublicKey().Bytes())})}}, nil)
	require.NoError(t, err)
	require.Equal(t, 1, len(pks))
	require.Equal(t, k.PublicKey(), pks[0])
}

func TestMap(t *testing.T) {
	_, err := Map(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.Make(42)}}, nil)
	require.Error(t, err)

	m, err := Map(&result.Invoke{State: "HALT", Stack: []stackitem.Item{stackitem.NewMapWithValue([]stackitem.MapElement{{Key: stackitem.Make(42), Value: stackitem.Make("string")}})}}, nil)
	require.NoError(t, err)
	require.Equal(t, 1, m.Len())
	require.Equal(t, 0, m.Index(stackitem.Make(42)))
}
