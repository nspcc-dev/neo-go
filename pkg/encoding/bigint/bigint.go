package bigint

import (
	"math"
	"math/big"
	"math/bits"

	"github.com/holiman/uint256"
	"github.com/nspcc-dev/neo-go/pkg/util/slice"
)

const (
	// MaxBytesLen is the maximum length of a serialized integer suitable for Neo VM.
	MaxBytesLen = 32 // 256-bit signed integer
	// wordSizeBytes is a size of a big.Word (uint) in bytes.
	wordSizeBytes = bits.UintSize / 8
)

var bigOne = big.NewInt(1)

// FromBytesUnsigned converts data in little-endian format to an unsigned integer.
func FromBytesUnsigned(data []byte) *big.Int {
	bs := slice.CopyReverse(data)
	return new(big.Int).SetBytes(bs)
}

// Uint256FromBytesUnsigned converts data in little-endian format to an unsigned integer.
func Uint256FromBytesUnsigned(data []byte) *uint256.Int {
	bs := slice.CopyReverse(data)
	return new(uint256.Int).SetBytes(bs)
}

// FromBytes converts data in little-endian format to
// an integer.
// FromBytes converts data in little-endian format to
// an integer.
func FromBytes(data []byte) *big.Int {
	n := new(big.Int)
	size := len(data)
	if size == 0 {
		if data == nil {
			panic("nil slice provided to `FromBytes`")
		}
		return big.NewInt(0)
	}

	isNeg := data[size-1]&0x80 != 0

	size = getEffectiveSize(data, isNeg)
	if size == 0 {
		if isNeg {
			return big.NewInt(-1)
		}

		return big.NewInt(0)
	}

	lw := size / wordSizeBytes
	ws := make([]big.Word, lw+1)
	for i := 0; i < lw; i++ {
		base := i * wordSizeBytes
		for j := base + 7; j >= base; j-- {
			ws[i] <<= 8
			ws[i] ^= big.Word(data[j])
		}
	}

	for i := size - 1; i >= lw*wordSizeBytes; i-- {
		ws[lw] <<= 8
		ws[lw] ^= big.Word(data[i])
	}

	if isNeg {
		for i := 0; i <= lw; i++ {
			ws[i] = ^ws[i]
		}

		shift := byte(wordSizeBytes-size%wordSizeBytes) * 8
		ws[lw] = ws[lw] & (^big.Word(0) >> shift)

		n.SetBits(ws)
		n.Neg(n)

		return n.Sub(n, bigOne)
	}

	return n.SetBits(ws)
}

func Uint256FromBytes(data []byte) *uint256.Int {
	n := new(uint256.Int)
	size := len(data)
	if size == 0 {
		if data == nil {
			panic("nil slice provided to `FromBytes`")
		}
		return uint256.NewInt(0)
	}
	isNeg := data[len(data)-1]&0x80 != 0
	slice.Reverse(data)
	if !isNeg {
		n.SetBytes(data)
	} else {
		carry := true
		for i := len(data) - 1; i >= 0; i-- {
			if carry {
				data[i]--
				carry = data[i] == math.MaxUint8
			}
			data[i] = ^data[i]
		}
		n.SetBytes(data)
		n.Neg(n)
	}
	return n
}

// getEffectiveSize returns the minimal number of bytes required
// to represent a number (two's complement for negatives).
func getEffectiveSize(buf []byte, isNeg bool) int {
	var b byte
	if isNeg {
		b = 0xFF
	}

	size := len(buf)
	for ; size > 0; size-- {
		if buf[size-1] != b {
			break
		}
	}

	return size
}

// ToBytes converts an integer to a slice in little-endian format.
// Note: NEO3 serialization differs from default C# BigInteger.ToByteArray()
// when n == 0. For zero is equal to empty slice in NEO3.
//
// https://github.com/neo-project/neo-vm/blob/master/src/neo-vm/Types/Integer.cs#L16
func ToBytes(n *big.Int) []byte {
	return ToPreallocatedBytes(n, []byte{})
}

// ToPreallocatedBytes converts an integer to a slice in little-endian format using the given
// byte array for conversion result.
// ToPreallocatedBytes converts an integer to a slice in little-endian format using the given
// byte array for conversion result.
func ToPreallocatedBytes(n *big.Int, data []byte) []byte {
	sign := n.Sign()
	if sign == 0 {
		return data[:0]
	}

	if sign < 0 {
		bits := n.Bits()
		carry := true
		nonZero := false
		for i := range bits {
			if carry {
				bits[i]--
				carry = (bits[i] == math.MaxUint)
			}
			nonZero = nonZero || (bits[i] != 0)
		}
		defer func() {
			var carry = true
			for i := range bits {
				if carry {
					bits[i]++
					carry = (bits[i] == 0)
				} else {
					break
				}
			}
		}()
		if !nonZero { // n == -1
			return append(data[:0], 0xFF)
		}
	}

	lb := n.BitLen()/8 + 1

	if c := cap(data); c < lb {
		data = make([]byte, lb)
	} else {
		data = data[:lb]
	}
	_ = n.FillBytes(data)
	slice.Reverse(data)

	if sign == -1 {
		for i := range data {
			data[i] = ^data[i]
		}
	}

	return data
}

func Uint256ToBytes(n *uint256.Int) []byte {
	if n.Sign() == 0 {
		return []byte{}
	}
	fill := true
	var filler byte
	b := n.Bytes()
	if n.Sign() < 0 {
		var sig int
		for ; sig < len(b); sig++ {
			if b[sig] < 0xff {
				if b[sig] >= 0x80 {
					fill = false
				}
				break
			}
		}
		b = b[sig:]
		filler = 0xff
	} else {
		filler = 0
		if b[0] < 0x80 {
			fill = false
		}
	}
	slice.Reverse(b)
	if fill {
		b = append(b, filler)
	}
	return b
}
