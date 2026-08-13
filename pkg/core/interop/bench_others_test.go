//go:build !linux

package interop_test

import "errors"

func dropPageCache(_ string, _ int64) error {
	return errors.New("page cache eviction is only supported on Linux")
}
