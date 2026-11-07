//go:build linux

package interop_test

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

// dropPageCache evicts pages of the file
// starting from the given offset from the page cache.
func dropPageCache(path string, off int64) error {
	maps, err := os.ReadFile("/proc/self/maps")
	if err != nil {
		return err
	}
	for line := range strings.SplitSeq(string(maps), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 6 || fields[5] != path {
			continue
		}
		var start, end, mOff uint64
		if _, err := fmt.Sscanf(fields[0]+" "+fields[2], "%x-%x %x", &start, &end, &mOff); err != nil {
			return err
		}
		if uint64(off) > mOff {
			start += uint64(off) - mOff
		}
		if start < end {
			if _, _, errno := unix.Syscall(unix.SYS_MADVISE, uintptr(start), uintptr(end-start), unix.MADV_DONTNEED); errno != 0 {
				return errno
			}
		}
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	err = f.Sync()
	if err == nil {
		err = unix.Fadvise(int(f.Fd()), off, 0, unix.FADV_DONTNEED)
	}
	_ = f.Close()
	return err
}
