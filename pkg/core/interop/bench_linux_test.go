package interop_test

import (
	"os"

	"golang.org/x/sys/unix"
)

func dropPageCache(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	err = f.Sync()
	if err == nil {
		err = unix.Fadvise(int(f.Fd()), 0, 0, unix.FADV_DONTNEED)
	}
	_ = f.Close()
	return err
}
