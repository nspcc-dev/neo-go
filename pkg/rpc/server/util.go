package server

import (
	"errors"
	"math"
)

func checkUint32(i int) error {
	if i < 0 || i > math.MaxUint32 {
		return errors.New("value should fit uint32")
	}
	return nil
}

func checkInt32(i int) error {
	if i < math.MinInt32 || i > math.MaxInt32 {
		return errors.New("value should fit int32")
	}
	return nil
}
