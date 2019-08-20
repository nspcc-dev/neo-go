/*
This file has been modified under the MIT license.
Original: https://github.com/vsergeev/btckeygenie
*/

package elliptic

import (
	nativeelliptic "crypto/elliptic"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
)

// Point represents a point on an EllipticCurve.
type Point struct {
	X *big.Int
	Y *big.Int
}

// Curve represents the parameters of a short Weierstrass equation elliptic curve.
/* y**2 = x**3 + a*x + b  % p */
type Curve struct {
	A    *big.Int
	B    *big.Int
	P    *big.Int
	G    Point
	N    *big.Int
	H    *big.Int
	Name string
}

// dump dumps the bytes of a point for debugging.
func (p *Point) dump() {
	fmt.Print(p.format())
}

// format formats the bytes of a point for debugging.
func (p *Point) format() string {
	if p.X == nil && p.Y == nil {
		return "(inf,inf)"
	}
	return fmt.Sprintf("(%s,%s)", hex.EncodeToString(p.X.Bytes()), hex.EncodeToString(p.Y.Bytes()))
}

// Params represent the paramters for the Elliptic Curve
func (ec Curve) Params() *nativeelliptic.CurveParams {
	return &nativeelliptic.CurveParams{
		P:       ec.P,
		N:       ec.N,
		B:       ec.B,
		Gx:      ec.G.X,
		Gy:      ec.G.Y,
		BitSize: 256,
		Name:    ec.Name,
	}
}

/*** Modular Arithmetic ***/

/* NOTE: Returning a new z each time below is very space inefficient, but the
 * alternate accumulator based design makes the point arithmetic functions look
 * absolutely hideous. I may still change this in the future. */

// addMod computes z = (x + y) % p.
func addMod(x *big.Int, y *big.Int, p *big.Int) (z *big.Int) {
	z = new(big.Int).Add(x, y)
	z.Mod(z, p)
	return z
}

// subMod computes z = (x - y) % p.
func subMod(x *big.Int, y *big.Int, p *big.Int) (z *big.Int) {
	z = new(big.Int).Sub(x, y)
	z.Mod(z, p)
	return z
}

// mulMod computes z = (x * y) % p.
func mulMod(x *big.Int, y *big.Int, p *big.Int) (z *big.Int) {
	n := new(big.Int).Set(x)
	z = big.NewInt(0)

	for i := 0; i < y.BitLen(); i++ {
		if y.Bit(i) == 1 {
			z = addMod(z, n, p)
		}
		n = addMod(n, n, p)
	}

	return z
}

// invMod computes z = (1/x) % p.
func invMod(x *big.Int, p *big.Int) (z *big.Int) {
	z = new(big.Int).ModInverse(x, p)
	return z
}

// expMod computes z = (x^e) % p.
func expMod(x *big.Int, y *big.Int, p *big.Int) (z *big.Int) {
	z = new(big.Int).Exp(x, y, p)
	return z
}

// sqrtMod computes z = sqrt(x) % p.
func sqrtMod(x *big.Int, p *big.Int) (z *big.Int) {
	/* assert that p % 4 == 3 */
	if new(big.Int).Mod(p, big.NewInt(4)).Cmp(big.NewInt(3)) != 0 {
		panic("p is not equal to 3 mod 4!")
	}

	/* z = sqrt(x) % p = x^((p+1)/4) % p */

	/* e = (p+1)/4 */
	e := new(big.Int).Add(p, big.NewInt(1))
	e = e.Rsh(e, 2)

	z = expMod(x, e, p)
	return z
}

/*** Point Arithmetic on Curve ***/

// IsInfinity checks if point P is infinity on EllipticCurve ec.
func (ec *Curve) IsInfinity(P Point) bool {
	/* We use (nil,nil) to represent O, the point at infinity. */

	if P.X == nil && P.Y == nil {
		return true
	}

	return false
}

// IsOnCurve checks if point P is on EllipticCurve ec.
func (ec Curve) IsOnCurve(P1, P2 *big.Int) bool {
	P := Point{P1, P2}
	if ec.IsInfinity(P) {
		return false
	}

	/* y**2 = x**3 + a*x + b  % p */
	lhs := mulMod(P.Y, P.Y, ec.P)
	rhs := addMod(
		addMod(
			expMod(P.X, big.NewInt(3), ec.P),
			mulMod(ec.A, P.X, ec.P), ec.P),
		ec.B, ec.P)

	if lhs.Cmp(rhs) == 0 {
		return true
	}

	return false
}

// Add computes R = P + Q on EllipticCurve ec.
func (ec Curve) Add(P1, P2, Q1, Q2 *big.Int) (R1 *big.Int, R2 *big.Int) {
	/* See rules 1-5 on SEC1 pg.7 http://www.secg.org/collateral/sec1_final.pdf */
	P := Point{P1, P2}
	Q := Point{Q1, Q2}
	R := Point{}
	if ec.IsInfinity(P) && ec.IsInfinity(Q) {
		/* Rule #1 Identity */
		/* R = O + O = O */

		R.X = nil
		R.Y = nil

	} else if ec.IsInfinity(P) {
		/* Rule #2 Identity */
		/* R = O + Q = Q */

		R.X = new(big.Int).Set(Q.X)
		R.Y = new(big.Int).Set(Q.Y)

	} else if ec.IsInfinity(Q) {
		/* Rule #2 Identity */
		/* R = P + O = P */

		R.X = new(big.Int).Set(P.X)
		R.Y = new(big.Int).Set(P.Y)

	} else if P.X.Cmp(Q.X) == 0 && addMod(P.Y, Q.Y, ec.P).Sign() == 0 {
		/* Rule #3 Identity */
		/* R = (x,y) + (x,-y) = O */

		R.X = nil
		R.Y = nil

	} else if P.X.Cmp(Q.X) == 0 && P.Y.Cmp(Q.Y) == 0 && P.Y.Sign() != 0 {
		/* Rule #5 Point doubling */
		/* R = P + P */

		/* Lambda = (3*P.X*P.X + a) / (2*P.Y) */
		num := addMod(
			mulMod(big.NewInt(3),
				mulMod(P.X, P.X, ec.P), ec.P),
			ec.A, ec.P)
		den := invMod(mulMod(big.NewInt(2), P.Y, ec.P), ec.P)
		lambda := mulMod(num, den, ec.P)

		/* R.X = lambda*lambda - 2*P.X */
		R.X = subMod(
			mulMod(lambda, lambda, ec.P),
			mulMod(big.NewInt(2), P.X, ec.P),
			ec.P)
		/* R.Y = lambda*(P.X - R.X) - P.Y */
		R.Y = subMod(
			mulMod(lambda, subMod(P.X, R.X, ec.P), ec.P),
			P.Y, ec.P)

	} else if P.X.Cmp(Q.X) != 0 {
		/* Rule #4 Point addition */
		/* R = P + Q */

		/* Lambda = (Q.Y - P.Y) / (Q.X - P.X) */
		num := subMod(Q.Y, P.Y, ec.P)
		den := invMod(subMod(Q.X, P.X, ec.P), ec.P)
		lambda := mulMod(num, den, ec.P)

		/* R.X = lambda*lambda - P.X - Q.X */
		R.X = subMod(
			subMod(
				mulMod(lambda, lambda, ec.P),
				P.X, ec.P),
			Q.X, ec.P)

		/* R.Y = lambda*(P.X - R.X) - P.Y */
		R.Y = subMod(
			mulMod(lambda,
				subMod(P.X, R.X, ec.P), ec.P),
			P.Y, ec.P)
	} else {
		panic(fmt.Sprintf("Unsupported point addition: %v + %v", P.format(), Q.format()))
	}

	return R.X, R.Y
}

// ScalarMult computes Q = k * P on EllipticCurve ec.
func (ec Curve) ScalarMult(P1, P2 *big.Int, l []byte) (Q1, Q2 *big.Int) {
	/* Note: this function is not constant time, due to the branching nature of
	 * the underlying point Add() function. */

	/* Montgomery Ladder Point Multiplication
	 *
	 * Implementation based on pseudocode here:
	 * See https://en.wikipedia.org/wiki/Elliptic_curve_point_multiplication#Montgomery_ladder */

	P := Point{P1, P2}
	k := big.Int{}
	k.SetBytes(l)

	var R0 Point
	var R1 Point

	R0.X = nil
	R0.Y = nil
	R1.X = new(big.Int).Set(P.X)
	R1.Y = new(big.Int).Set(P.Y)

	for i := ec.N.BitLen() - 1; i >= 0; i-- {
		if k.Bit(i) == 0 {
			R1.X, R1.Y = ec.Add(R0.X, R0.Y, R1.X, R1.Y)
			R0.X, R0.Y = ec.Add(R0.X, R0.Y, R0.X, R0.Y)
		} else {
			R0.X, R0.Y = ec.Add(R0.X, R0.Y, R1.X, R1.Y)
			R1.X, R1.Y = ec.Add(R1.X, R1.Y, R1.X, R1.Y)
		}
	}

	return R0.X, R0.Y
}

// ScalarBaseMult computes Q = k * G on EllipticCurve ec.
func (ec Curve) ScalarBaseMult(k []byte) (Q1, Q2 *big.Int) {

	return ec.ScalarMult(ec.G.X, ec.G.Y, k)
}

// Decompress decompresses coordinate x and ylsb (y's least significant bit) into a Point P on EllipticCurve ec.
func (ec *Curve) Decompress(x *big.Int, ylsb uint) (P Point, err error) {
	/* y**2 = x**3 + a*x + b  % p */
	rhs := addMod(
		addMod(
			expMod(x, big.NewInt(3), ec.P),
			mulMod(ec.A, x, ec.P),
			ec.P),
		ec.B, ec.P)

	/* y = sqrt(rhs) % p */
	y := sqrtMod(rhs, ec.P)

	/* Use -y if opposite lsb is required */
	if y.Bit(0) != (ylsb & 0x1) {
		y = subMod(big.NewInt(0), y, ec.P)
	}

	P.X = x
	P.Y = y

	if !ec.IsOnCurve(P.X, P.Y) {
		return P, errors.New("compressed (x, ylsb) not on curve")
	}

	return P, nil
}

// Double will return the (x1+x1,y1+y1)
func (ec Curve) Double(x1, y1 *big.Int) (x, y *big.Int) {
	x = &big.Int{}
	x.SetBytes([]byte{0x00})
	y = &big.Int{}
	y.SetBytes([]byte{0x00})
	return x, y
}
