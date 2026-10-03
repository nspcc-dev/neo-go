//go:build !linux

package interop_test

import "os"

func dropPageCache(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	err = f.Sync()
	_ = f.Close()
	return err
}
