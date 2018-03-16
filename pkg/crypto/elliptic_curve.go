package crypto

// Original work completed by @vsergeev: https://github.com/vsergeev/btckeygenie
// Expanded and tweaked upon here under MIT license.

import (
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
		B *big.Int
		P *big.Int
		G EllipticCurvePoint
		N *big.Int
		H *big.Int
	}

	// EllipticCurveEllipticCurvePoint represents a point on the EllipticCurve.
	EllipticCurvePoint struct {
		X *big.Int
		Y *big.Int
	}
)

// NewEllipticCurvePointFromReader return a new point from the given reader.
// f == 4, 6 or 7 are not implemented.
func NewEllipticCurvePointFromReader(r io.Reader) (point EllipticCurvePoint, err error) {
	var f uint8
	if err = binary.Read(r, binary.LittleEndian, &f); err != nil {
		return
	}

	// Infinity
	if f == 0 {
		return EllipticCurvePoint{
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

		return EllipticCurvePoint{
			X: new(big.Int).SetBytes(data),
			Y: y,
		}, nil
	}
	return
}

// NewEllipticCurve returns a ready to use EllipticCurve with preconfigured
// fields for the NEO protocol.
func NewEllipticCurve() EllipticCurve {
	c := EllipticCurve{}

	c.P, _ = new(big.Int).SetString(
		"FFFFFFFF00000001000000000000000000000000FFFFFFFFFFFFFFFFFFFFFFFF", 16,
	)
	c.A, _ = new(big.Int).SetString(
		"FFFFFFFF00000001000000000000000000000000FFFFFFFFFFFFFFFFFFFFFFFC", 16,
	)
	c.B, _ = new(big.Int).SetString(
		"5AC635D8AA3A93E7B3EBBD55769886BC651D06B0CC53B0F63BCE3C3E27D2604B", 16,
	)
	c.G.X, _ = new(big.Int).SetString(
		"6B17D1F2E12C4247F8BCE6E563A440F277037D812DEB33A0F4A13945D898C296", 16,
	)
	c.G.Y, _ = new(big.Int).SetString(
		"4FE342E2FE1A7F9B8EE7EB4A7C0F9E162BCE33576B315ECECBB6406837BF51F5", 16,
	)
	c.N, _ = new(big.Int).SetString(
		"FFFFFFFF00000000FFFFFFFFFFFFFFFFBCE6FAADA7179E84F3B9CAC2FC632551", 16,
	)
	c.H, _ = new(big.Int).SetString("01", 16)

	return c
}

func (p *EllipticCurvePoint) format() string {
	if p.X == nil && p.Y == nil {
		return "(inf,inf)"
	}
	bx := hex.EncodeToString(p.X.Bytes())
	by := hex.EncodeToString(p.Y.Bytes())
	return fmt.Sprintf("(%s, %s)", bx, by)
}

// IsInfinity checks if point P is infinity on EllipticCurve ec.
func (c *EllipticCurve) IsInfinity(P EllipticCurvePoint) bool {
	if P.X == nil && P.Y == nil {
		return true
	}
	return false
}

// IsOnCurve checks if point P is on EllipticCurve ec.
func (c *EllipticCurve) IsOnCurve(P EllipticCurvePoint) bool {
	if c.IsInfinity(P) {
		return false
	}
	lhs := mulMod(P.Y, P.Y, c.P)
	rhs := addMod(
		addMod(
			expMod(P.X, big.NewInt(3), c.P),
			mulMod(c.A, P.X, c.P), c.P),
		c.B, c.P)

	if lhs.Cmp(rhs) == 0 {
		return true
	}
	return false
}

// Add computes R = P + Q on EllipticCurve ec.
func (c *EllipticCurve) Add(P, Q EllipticCurvePoint) (R EllipticCurvePoint) {
	// See rules 1-5 on SEC1 pg.7 http://www.secg.org/collateral/sec1_final.pdf
	if c.IsInfinity(P) && c.IsInfinity(Q) {
		R.X = nil
		R.Y = nil
	} else if c.IsInfinity(P) {
		R.X = new(big.Int).Set(Q.X)
		R.Y = new(big.Int).Set(Q.Y)
	} else if c.IsInfinity(Q) {
		R.X = new(big.Int).Set(P.X)
		R.Y = new(big.Int).Set(P.Y)
	} else if P.X.Cmp(Q.X) == 0 && addMod(P.Y, Q.Y, c.P).Sign() == 0 {
		R.X = nil
		R.Y = nil
	} else if P.X.Cmp(Q.X) == 0 && P.Y.Cmp(Q.Y) == 0 && P.Y.Sign() != 0 {
		num := addMod(
			mulMod(big.NewInt(3),
				mulMod(P.X, P.X, c.P), c.P),
			c.A, c.P)
		den := invMod(mulMod(big.NewInt(2), P.Y, c.P), c.P)
		lambda := mulMod(num, den, c.P)
		R.X = subMod(
			mulMod(lambda, lambda, c.P),
			mulMod(big.NewInt(2), P.X, c.P),
			c.P)
		R.Y = subMod(
			mulMod(lambda, subMod(P.X, R.X, c.P), c.P),
			P.Y, c.P)
	} else if P.X.Cmp(Q.X) != 0 {
		num := subMod(Q.Y, P.Y, c.P)
		den := invMod(subMod(Q.X, P.X, c.P), c.P)
		lambda := mulMod(num, den, c.P)
		R.X = subMod(
			subMod(
				mulMod(lambda, lambda, c.P),
				P.X, c.P),
			Q.X, c.P)
		R.Y = subMod(
			mulMod(lambda,
				subMod(P.X, R.X, c.P), c.P),
			P.Y, c.P)
	} else {
		panic(fmt.Sprintf("Unsupported point addition: %v + %v", P.format(), Q.format()))
	}

	return R
}

// ScalarMult computes Q = k * P on EllipticCurve ec.
func (c *EllipticCurve) ScalarMult(k *big.Int, P EllipticCurvePoint) (Q EllipticCurvePoint) {
	// Implementation based on pseudocode here:
	// https://en.wikipedia.org/wiki/Elliptic_curve_point_multiplication#Montgomery_ladder
	var R0 EllipticCurvePoint
	var R1 EllipticCurvePoint

	R0.X = nil
	R0.Y = nil
	R1.X = new(big.Int).Set(P.X)
	R1.Y = new(big.Int).Set(P.Y)

	for i := c.N.BitLen() - 1; i >= 0; i-- {
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
func (c *EllipticCurve) ScalarBaseMult(k *big.Int) (Q EllipticCurvePoint) {
	return c.ScalarMult(k, c.G)
}

// Decompress decompresses coordinate x and ylsb (y's least significant bit) into a EllipticCurvePoint P on EllipticCurve ec.
func (c *EllipticCurve) Decompress(x *big.Int, ylsb uint) (P EllipticCurvePoint, err error) {
	/* y**2 = x**3 + a*x + b  % p */
	rhs := addMod(
		addMod(
			expMod(x, big.NewInt(3), c.P),
			mulMod(c.A, x, c.P),
			c.P),
		c.B, c.P)

	y := sqrtMod(rhs, c.P)
	if y.Bit(0) != (ylsb & 0x1) {
		y = subMod(big.NewInt(0), y, c.P)
	}

	P.X = x
	P.Y = y

	if !c.IsOnCurve(P) {
		return P, errors.New("compressed (x, ylsb) not on curve")
	}

	return P, nil
}
