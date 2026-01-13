package stackitem

import (
	"fmt"
	"math"

	"github.com/nspcc-dev/neo-go/pkg/util"
)

// ToUint160 converts [stackitem.Item] to [util.Uint160] or returns an error if
// so.
func ToUint160(s Item) (util.Uint160, error) {
	buf, err := s.TryBytes()
	if err != nil {
		return util.Uint160{}, err
	}
	u, err := util.Uint160DecodeBytesBE(buf)
	if err != nil {
		return util.Uint160{}, fmt.Errorf("%w: %w", ErrInvalidValue, err)
	}
	return u, nil
}

// ToUint256 converts [stackitem.Item] to [util.Uint256] or returns an error if
// so.
func ToUint256(s Item) (util.Uint256, error) {
	buf, err := s.TryBytes()
	if err != nil {
		return util.Uint256{}, err
	}
	u, err := util.Uint256DecodeBytesBE(buf)
	if err != nil {
		return util.Uint256{}, fmt.Errorf("%w: %w", ErrInvalidValue, err)
	}
	return u, nil
}

// ToInt32 converts [stackitem.Item] to int32 with the bounds check or returns
// an error if so.
func ToInt32(s Item) (int32, error) {
	i, err := ToInt64(s)
	if err != nil {
		return 0, err
	}
	if i < math.MinInt32 || math.MaxInt32 < i {
		return 0, fmt.Errorf("%w: bigint is not in int32 range", ErrInvalidValue)
	}
	return int32(i), nil
}

// ToInt64 converts [stackitem.Item] to int64 with the bounds check or returns
// an error if so.
func ToInt64(s Item) (int64, error) {
	bigInt, err := s.TryInteger()
	if err != nil {
		return 0, err
	}
	if !bigInt.IsInt64() {
		return 0, fmt.Errorf("%w: bigint is not in int64 range", ErrInvalidValue)
	}
	return bigInt.Int64(), nil
}

// ToUint8 converts [stackitem.Item] to uint8 with the bounds check or returns
// an error if so.
func ToUint8(s Item) (uint8, error) {
	i, err := ToInt64(s)
	if err != nil {
		return 0, err
	}
	if i < 0 || math.MaxUint8 < i {
		return 0, fmt.Errorf("%w: bigint is not in uint8 range", ErrInvalidValue)
	}
	return uint8(i), nil
}

// ToUint16 converts [stackitem.Item] to uint16 with the bounds check or
// returns an error if so.
func ToUint16(s Item) (uint16, error) {
	i, err := ToInt64(s)
	if err != nil {
		return 0, err
	}
	if i < 0 || math.MaxUint16 < i {
		return 0, fmt.Errorf("%w: bigint is not in uint16 range", ErrInvalidValue)
	}
	return uint16(i), nil
}

// ToUint32 converts [stackitem.Item] to uint32 with the bounds check or
// returns an error if so.
func ToUint32(s Item) (uint32, error) {
	i, err := ToInt64(s)
	if err != nil {
		return 0, err
	}
	if i < 0 || math.MaxUint32 < i {
		return 0, fmt.Errorf("%w: bigint is not in uint32 range", ErrInvalidValue)
	}
	return uint32(i), nil
}

// ToUint64 converts [stackitem.Item] to uint64 with the bounds check or
// returns an error if so.
func ToUint64(s Item) (uint64, error) {
	bigInt, err := s.TryInteger()
	if err != nil {
		return 0, err
	}
	if !bigInt.IsUint64() {
		return 0, fmt.Errorf("%w: bigint is not in uint64 range", ErrInvalidValue)
	}
	return bigInt.Uint64(), nil
}
