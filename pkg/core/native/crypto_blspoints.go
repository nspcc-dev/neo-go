package native

import (
	"errors"
	"fmt"
	"math/big"

	bls12381 "github.com/consensys/gnark-crypto/ecc/bls12-381"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// blsPoint is a wrapper around bls12381 point types that must be used as
// stackitem.Interop values and implement stackitem.Equatable interface.
type blsPoint struct {
	point any
}

var _ = stackitem.Equatable(blsPoint{})

// Equals implements stackitem.Equatable interface.
func (p blsPoint) Equals(other stackitem.Equatable) bool {
	res, err := p.EqualsCheckType(other)
	return err == nil && res
}

// EqualsCheckType checks whether other is of the same type as p and returns an error if not.
// It also returns whether other and p are equal.
func (p blsPoint) EqualsCheckType(other stackitem.Equatable) (bool, error) {
	b, ok := other.(blsPoint)
	if !ok {
		return false, errors.New("not a bls12-381 point")
	}
	var (
		res bool
		err error
	)
	switch x := p.point.(type) {
	case *bls12381.G1Affine:
		y, ok := b.point.(*bls12381.G1Affine)
		if !ok {
			err = fmt.Errorf("equal: unexpected y bls12381 point type: %T vs G1Affine", y)
			break
		}
		res = x.Equal(y)
	case *bls12381.G1Jac:
		y, ok := b.point.(*bls12381.G1Jac)
		if !ok {
			err = fmt.Errorf("equal: unexpected y bls12381 point type: %T vs G1Jac", y)
			break
		}
		res = x.Equal(y)
	case *bls12381.G2Affine:
		y, ok := b.point.(*bls12381.G2Affine)
		if !ok {
			err = fmt.Errorf("equal: unexpected y bls12381 point type: %T vs G2Affine", y)
			break
		}
		res = x.Equal(y)
	case *bls12381.G2Jac:
		y, ok := b.point.(*bls12381.G2Jac)
		if !ok {
			err = fmt.Errorf("equal: unexpected y bls12381 point type: %T vs G2Jac", y)
			break
		}
		res = x.Equal(y)
	case *bls12381.GT:
		y, ok := b.point.(*bls12381.GT)
		if !ok {
			err = fmt.Errorf("equal: unexpected y bls12381 point type: %T vs GT", y)
			break
		}
		res = x.Equal(y)
	default:
		err = fmt.Errorf("equal: unexpected x bls12381 point type: %T", x)
	}

	return res, err
}

// Bytes returns serialized representation of the provided point in compressed form.
func (p blsPoint) Bytes() []byte {
	switch p := p.point.(type) {
	case *bls12381.G1Affine:
		compressed := p.Bytes()
		return compressed[:]
	case *bls12381.G1Jac:
		g1Affine := new(bls12381.G1Affine)
		g1Affine.FromJacobian(p)
		compressed := g1Affine.Bytes()
		return compressed[:]
	case *bls12381.G2Affine:
		compressed := p.Bytes()
		return compressed[:]
	case *bls12381.G2Jac:
		g2Affine := new(bls12381.G2Affine)
		g2Affine.FromJacobian(p)
		compressed := g2Affine.Bytes()
		return compressed[:]
	case *bls12381.GT:
		compressed := p.Bytes()
		return compressed[:]
	default:
		panic(errors.New("unknown bls12381 point type"))
	}
}

// FromBytes deserializes BLS12-381 point from the given byte slice in compressed form.
func (p *blsPoint) FromBytes(buf []byte) error {
	switch l := len(buf); l {
	case bls12381.SizeOfG1AffineCompressed:
		g1Affine := new(bls12381.G1Affine)
		_, err := g1Affine.SetBytes(buf)
		if err != nil {
			return fmt.Errorf("failed to decode bls12381 G1Affine point: %w", err)
		}
		p.point = g1Affine
	case bls12381.SizeOfG2AffineCompressed:
		g2Affine := new(bls12381.G2Affine)
		_, err := g2Affine.SetBytes(buf)
		if err != nil {
			return fmt.Errorf("failed to decode bls12381 G2Affine point: %w", err)
		}
		p.point = g2Affine
	case bls12381.SizeOfGT:
		gt := new(bls12381.GT)
		err := gt.SetBytes(buf)
		if err != nil {
			return fmt.Errorf("failed to decode GT point: %w", err)
		}
		p.point = gt
	}

	return nil
}

// blsPointAdd performs addition of two BLS12-381 points.
func blsPointAdd(a, b blsPoint) (blsPoint, error) {
	var (
		res any
		err error
	)
	switch x := a.point.(type) {
	case *bls12381.G1Affine:
		switch y := b.point.(type) {
		case *bls12381.G1Affine:
			xJac := new(bls12381.G1Jac)
			xJac.FromAffine(x)
			xJac.AddMixed(y)
			res = xJac
		case *bls12381.G1Jac:
			yJac := new(bls12381.G1Jac)
			yJac.Set(y)
			yJac.AddMixed(x)
			res = yJac
		default:
			err = fmt.Errorf("add: inconsistent bls12381 point types: %T and %T", x, y)
		}
	case *bls12381.G1Jac:
		resJac := new(bls12381.G1Jac)
		resJac.Set(x)
		switch y := b.point.(type) {
		case *bls12381.G1Affine:
			resJac.AddMixed(y)
		case *bls12381.G1Jac:
			resJac.AddAssign(y)
		default:
			err = fmt.Errorf("add: inconsistent bls12381 point types: %T and %T", x, y)
		}
		res = resJac
	case *bls12381.G2Affine:
		switch y := b.point.(type) {
		case *bls12381.G2Affine:
			xJac := new(bls12381.G2Jac)
			xJac.FromAffine(x)
			xJac.AddMixed(y)
			res = xJac
		case *bls12381.G2Jac:
			yJac := new(bls12381.G2Jac)
			yJac.Set(y)
			yJac.AddMixed(x)
			res = yJac
		default:
			err = fmt.Errorf("add: inconsistent bls12381 point types: %T and %T", x, y)
		}
	case *bls12381.G2Jac:
		resJac := new(bls12381.G2Jac)
		resJac.Set(x)
		switch y := b.point.(type) {
		case *bls12381.G2Affine:
			resJac.AddMixed(y)
		case *bls12381.G2Jac:
			resJac.AddAssign(y)
		default:
			err = fmt.Errorf("add: inconsistent bls12381 point types: %T and %T", x, y)
		}
		res = resJac
	case *bls12381.GT:
		resGT := new(bls12381.GT)
		resGT.Set(x)
		switch y := b.point.(type) {
		case *bls12381.GT:
			// It's multiplication, see https://github.com/neo-project/Neo.Cryptography.BLS12_381/issues/4.
			resGT.Mul(x, y)
		default:
			err = fmt.Errorf("add: inconsistent bls12381 point types: %T and %T", x, y)
		}
		res = resGT
	default:
		err = fmt.Errorf("add: unexpected bls12381 point type: %T", x)
	}

	return blsPoint{point: res}, err
}

// blsPointAdd performs scalar multiplication of two BLS12-381 points.
func blsPointMul(a blsPoint, alphaBi *big.Int) (blsPoint, error) {
	var (
		res any
		err error
	)
	switch x := a.point.(type) {
	case *bls12381.G1Affine:
		// The result is in Jacobian form in the reference implementation.
		g1Jac := new(bls12381.G1Jac)
		g1Jac.FromAffine(x)
		g1Jac.ScalarMultiplication(g1Jac, alphaBi)
		res = g1Jac
	case *bls12381.G1Jac:
		g1Jac := new(bls12381.G1Jac)
		g1Jac.ScalarMultiplication(x, alphaBi)
		res = g1Jac
	case *bls12381.G2Affine:
		// The result is in Jacobian form in the reference implementation.
		g2Jac := new(bls12381.G2Jac)
		g2Jac.FromAffine(x)
		g2Jac.ScalarMultiplication(g2Jac, alphaBi)
		res = g2Jac
	case *bls12381.G2Jac:
		g2Jac := new(bls12381.G2Jac)
		g2Jac.ScalarMultiplication(x, alphaBi)
		res = g2Jac
	case *bls12381.GT:
		gt := new(bls12381.GT)

		// C# implementation differs a bit from go's. They use double-and-add algorithm, see
		// https://github.com/neo-project/Neo.Cryptography.BLS12_381/blob/844bc3a4f7d8ba2c545ace90ca124f8ada4c8d29/src/Neo.Cryptography.BLS12_381/Gt.cs#L102
		// and https://en.wikipedia.org/wiki/Elliptic_curve_point_multiplication#Double-and-add,
		// Pay attention that C#'s Gt.Double() squares (not doubles!) the initial GT point.
		// Thus.C#'s scalar multiplication operation over Gt and Scalar is effectively an exponent.
		// Go's exponent algorithm differs a bit from the C#'s double-and-add in that go's one
		// uses 2-bits windowed method for multiplication. However, the resulting GT point is
		// absolutely the same between two implementations.
		gt.Exp(*x, alphaBi)

		res = gt
	default:
		err = fmt.Errorf("mul: unexpected bls12381 point type: %T", x)
	}

	return blsPoint{point: res}, err
}

func blsPointPairing(a, b blsPoint) (blsPoint, error) {
	var (
		x *bls12381.G1Affine
		y *bls12381.G2Affine
	)
	switch p := a.point.(type) {
	case *bls12381.G1Affine:
		x = p
	case *bls12381.G1Jac:
		x = new(bls12381.G1Affine)
		x.FromJacobian(p)
	default:
		return blsPoint{}, fmt.Errorf("pairing: unexpected bls12381 point type (g1): %T", p)
	}
	switch p := b.point.(type) {
	case *bls12381.G2Affine:
		y = p
	case *bls12381.G2Jac:
		y = new(bls12381.G2Affine)
		y.FromJacobian(p)
	default:
		return blsPoint{}, fmt.Errorf("pairing: unexpected bls12381 point type (g2): %T", p)
	}

	gt, err := bls12381.Pair([]bls12381.G1Affine{*x}, []bls12381.G2Affine{*y})
	if err != nil {
		return blsPoint{}, fmt.Errorf("failed to perform pairing operation: %w", err)
	}

	return blsPoint{&gt}, nil
}
