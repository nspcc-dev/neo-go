package native

import (
	"errors"
	"fmt"

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
