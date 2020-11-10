package binary

import (
	"encoding/hex"
	"errors"
	"math/big"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
)

var (
	// ErrInvalidBase is returned when base is invalid.
	ErrInvalidBase = errors.New("invalid base")
	// ErrInvalidFormat is returned when string is not a number.
	ErrInvalidFormat = errors.New("invalid format")
)

// Itoa converts number to string.
func Itoa(ic *interop.Context) error {
	num := ic.VM.Estack().Pop().BigInt()
	base := ic.VM.Estack().Pop().BigInt()
	if !base.IsInt64() {
		return ErrInvalidBase
	}
	var s string
	switch b := base.Int64(); b {
	case 10:
		s = num.Text(10)
	case 16:
		if num.Sign() == 0 {
			s = "0"
			break
		}
		bs := bigint.ToBytes(num)
		reverse(bs)
		s = hex.EncodeToString(bs)
		if pad := bs[0] & 0xF8; pad == 0 || pad == 0xF8 {
			s = s[1:]
		}
		s = strings.ToUpper(s)
	default:
		return ErrInvalidBase
	}
	ic.VM.Estack().PushVal(s)
	return nil
}

// Atoi converts string to number.
func Atoi(ic *interop.Context) error {
	num := ic.VM.Estack().Pop().String()
	base := ic.VM.Estack().Pop().BigInt()
	if !base.IsInt64() {
		return ErrInvalidBase
	}
	var bi *big.Int
	switch b := base.Int64(); b {
	case 10:
		var ok bool
		bi, ok = new(big.Int).SetString(num, int(b))
		if !ok {
			return ErrInvalidFormat
		}
	case 16:
		changed := len(num)%2 != 0
		if changed {
			num = "0" + num
		}
		bs, err := hex.DecodeString(num)
		if err != nil {
			return ErrInvalidFormat
		}
		if changed && bs[0]&0x8 != 0 {
			bs[0] |= 0xF0
		}
		reverse(bs)
		bi = bigint.FromBytes(bs)
	default:
		return ErrInvalidBase
	}
	ic.VM.Estack().PushVal(bi)
	return nil
}

func reverse(b []byte) {
	l := len(b)
	for i := 0; i < l/2; i++ {
		b[i], b[l-i-1] = b[l-i-1], b[i]
	}
}
