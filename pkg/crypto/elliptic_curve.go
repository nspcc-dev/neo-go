package crypto

// Original work completed by @vsergeev: https://github.com/vsergeev/btckeygenie
// Expanded and tweaked upon here under MIT license.

import (
	"bytes"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/CityOfZion/neo-go/pkg/util"
)

type (
	// EllipticCurve represents the parameters of a short Weierstrass equation elliptic
	// curve.
	EllipticCurve struct {
		A *big.Int
		H *big.Int
		elliptic.Curve
	}

	// ECPoint represents a point on the EllipticCurve.
	ECPoint struct {
		X *big.Int
		Y *big.Int
	}
)

// NewEllipticCurve returns a ready to use EllipticCurve with preconfigured
// fields for the NEO protocol.
func NewEllipticCurve() EllipticCurve {
	c := EllipticCurve{
		Curve: elliptic.P256(),
	}

	c.A, _ = new(big.Int).SetString(
		"FFFFFFFF00000001000000000000000000000000FFFFFFFFFFFFFFFFFFFFFFFC", 16,
	)
	c.H, _ = new(big.Int).SetString("01", 16)

	return c
}

// RandomECPoint returns a random generated ECPoint, mostly used
// for testing.
func RandomECPoint() ECPoint {
	c := NewEllipticCurve()
	b := make([]byte, c.Params().N.BitLen()/8+8)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return ECPoint{}
	}

	d := new(big.Int).SetBytes(b)
	d.Mod(d, new(big.Int).Sub(c.Params().N, big.NewInt(1)))
	d.Add(d, big.NewInt(1))

	q := new(big.Int).SetBytes(d.Bytes())
	return c.ScalarBaseMult(q)
}

// ECPointFromReader return a new point from the given reader.
// f == 4, 6 or 7 are not implemented.
func ECPointFromReader(r io.Reader) (point ECPoint, err error) {
	var f uint8
	if err = binary.Read(r, binary.LittleEndian, &f); err != nil {
		return
	}

	// Infinity
	if f == 0 {
		return ECPoint{
			X: new(big.Int),
			Y: new(big.Int),
		}, nil
	}

	if f == 2 || f == 3 {
		y := new(big.Int).SetBytes([]byte{f & 1})
		data := make([]byte, 32)
		if err = binary.Read(r, binary.LittleEndian, data); err != nil {
			return
		}
		data = util.ArrayReverse(data)
		data = append(data, byte(0x00))

		return ECPoint{
			X: new(big.Int).SetBytes(data),
			Y: y,
		}, nil
	}
	return
}

// EncodeBinary encodes the point to the given io.Writer.
func (p ECPoint) EncodeBinary(w io.Writer) error {
	bx := p.X.Bytes()
	padded := append(
		bytes.Repeat(
			[]byte{0x00},
			32-len(bx),
		),
		bx...,
	)

	prefix := byte(0x03)
	if p.Y.Bit(0) == 0 {
		prefix = byte(0x02)
	}
	buf := make([]byte, len(padded)+1)
	buf[0] = prefix
	copy(buf[1:], padded)

	return binary.Write(w, binary.LittleEndian, buf)
}

// String implements the Stringer interface.
func (p *ECPoint) String() string {
	if p.IsInfinity() {
		return "00"
	}
	bx := hex.EncodeToString(p.X.Bytes())
	by := hex.EncodeToString(p.Y.Bytes())
	return fmt.Sprintf("%s%s", bx, by)
}

// IsInfinity checks if point P is infinity on EllipticCurve ec.
func (p *ECPoint) IsInfinity() bool {
	return p.X == nil && p.Y == nil
}

// IsInfinity checks if point P is infinity on EllipticCurve ec.
func (c *EllipticCurve) IsInfinity(P ECPoint) bool {
	return P.X == nil && P.Y == nil
}

// IsOnCurve checks if point P is on EllipticCurve ec.
func (c *EllipticCurve) IsOnCurve(P ECPoint) bool {
	if c.IsInfinity(P) {
		return false
	}
	lhs := mulMod(P.Y, P.Y, c.Params().P)
	rhs := addMod(
		addMod(
			expMod(P.X, big.NewInt(3), c.Params().P),
			mulMod(c.A, P.X, c.Params().P), c.Params().P),
		c.Params().B, c.Params().P)

	if lhs.Cmp(rhs) == 0 {
		return true
	}
	return false
}

// Add computes R = P + Q on EllipticCurve ec.
func (c *EllipticCurve) Add(P, Q ECPoint) (R ECPoint) {
	R.X, R.Y = c.Curve.Add(P.X, P.Y, Q.X, Q.Y)
	return
}

// ScalarMult computes Q = k * P on EllipticCurve ec.
func (c *EllipticCurve) ScalarMult(k *big.Int, P ECPoint) (Q ECPoint) {
	// Implementation based on pseudocode here:
	// https://en.wikipedia.org/wiki/Elliptic_curve_point_multiplication#Montgomery_ladder
	var R0 ECPoint
	var R1 ECPoint

	R0.X = nil
	R0.Y = nil
	R1.X = new(big.Int).Set(P.X)
	R1.Y = new(big.Int).Set(P.Y)

	for i := c.Params().N.BitLen() - 1; i >= 0; i-- {
		if k.Bit(i) == 0 {
			R1 = c.Add(R0, R1)
			R0 = c.Add(R0, R0)
		} else {
			R0 = c.Add(R0, R1)
			R1 = c.Add(R1, R1)
		}
	}
	return R0
}

// ScalarBaseMult computes Q = k * G on EllipticCurve ec.
func (c *EllipticCurve) ScalarBaseMult(k *big.Int) (Q ECPoint) {
	Q.X, Q.Y = c.Curve.ScalarBaseMult(k.Bytes())
	return
}

// Decompress decompresses coordinate x and ylsb (y's least significant bit) into a ECPoint P on EllipticCurve ec.
func (c *EllipticCurve) Decompress(x *big.Int, ylsb uint) (P ECPoint, err error) {
	/* y**2 = x**3 + a*x + b  % p */
	rhs := addMod(
		addMod(
			expMod(x, big.NewInt(3), c.Params().P),
			mulMod(c.A, x, c.Params().P),
			c.Params().P),
		c.Params().B, c.Params().P)

	y := sqrtMod(rhs, c.Params().P)
	if y.Bit(0) != (ylsb & 0x1) {
		y = subMod(big.NewInt(0), y, c.Params().P)
	}

	P.X = x
	P.Y = y

	if !c.IsOnCurve(P) {
		return P, errors.New("compressed (x, ylsb) not on curve")
	}

	return P, nil
}
