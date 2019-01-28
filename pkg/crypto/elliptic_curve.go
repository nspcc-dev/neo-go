package crypto

// Original work completed by @vsergeev: https://github.com/vsergeev/btckeygenie
// Expanded and tweaked upon here under MIT license.

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"
)

type (
	// EllipticCurve represents elliptic.P256.
	EllipticCurve struct {
		curve elliptic.Curve
	}

	// ECPoint represents a point on the EllipticCurve.
	ECPoint struct {
		X *big.Int
		Y *big.Int
	}
)

var curve = elliptic.P256()

// NewEllipticCurve returns a ready to use EllipticCurve with preconfigured
// fields for the NEO protocol.
func NewEllipticCurve() EllipticCurve {
	return EllipticCurve{curve: curve}
}

// RandomECPoint returns a random generated ECPoint, mostly used
// for testing.
func RandomECPoint() (p ECPoint) {
	pub, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		return
	}
	p.X, p.Y = pub.X, pub.Y
	return
}

// ECPointFromReader return a new point from the given reader.
// f == 4, 6 or 7 are not implemented.
func ECPointFromReader(r io.Reader) (point ECPoint, err error) {
	var f uint8
	if err = binary.Read(r, binary.LittleEndian, &f); err != nil {
		return
	}

	switch f {
	// Infinity
	case 0:
		point.X = new(big.Int)
		point.Y = new(big.Int)
	case 2, 3:
		var sk *ecdsa.PrivateKey
		if sk, err = ecdsa.GenerateKey(curve, r); err != nil {
			return
		}
		point.X, point.Y = sk.X, sk.Y // copying PublicKey
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

// Params returns the parameters for the curve.
func (c *EllipticCurve) Params() *elliptic.CurveParams {
	return c.curve.Params()
}

// IsInfinity checks if point P is infinity on EllipticCurve ec.
func (c *EllipticCurve) IsInfinity(P ECPoint) bool {
	return P.X == nil && P.Y == nil
}

// IsOnCurve checks if point P is on EllipticCurve ec.
func (c *EllipticCurve) IsOnCurve(P ECPoint) bool {
	return c.curve.IsOnCurve(P.X, P.Y)
}

// Add computes R = P + Q on EllipticCurve ec.
func (c *EllipticCurve) Add(P, Q ECPoint) (R ECPoint) {
	R.X, R.Y = c.curve.Add(P.X, P.Y, Q.X, Q.Y)
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

	for i := c.curve.Params().N.BitLen() - 1; i >= 0; i-- {
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
	Q.X, Q.Y = c.curve.ScalarBaseMult(k.Bytes())
	return
}

// Decompress decompresses coordinate x and ylsb (y's least significant bit) into a ECPoint P on EllipticCurve ec.
func (c *EllipticCurve) Decompress(x *big.Int, ylsb uint) (P ECPoint, err error) {
	/* y**2 = x**3 + a*x + b  % p */
	rhs := addMod(
		addMod(
			expMod(x, big.NewInt(3), c.curve.Params().P),
			mulMod(big.NewInt(-3), x, c.curve.Params().P),
			c.curve.Params().P),
		c.curve.Params().B, c.curve.Params().P)

	y := new(big.Int).ModSqrt(rhs, c.curve.Params().P)
	if y.Bit(0) != (ylsb & 0x1) {
		y = subMod(big.NewInt(0), y, c.curve.Params().P)
	}

	P.X = x
	P.Y = y

	if !c.IsOnCurve(P) {
		return P, errors.New("compressed (x, ylsb) not on curve")
	}

	return P, nil
}
