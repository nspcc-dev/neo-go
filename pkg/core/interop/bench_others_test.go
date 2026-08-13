//go:build !linux

package interop_test

func dropPageCache(_ string, _ int64) error {
	return nil
}
