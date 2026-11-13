//go:build linux

package interop_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"
)

// dropPageCache evicts pages of the file
// starting from the given offset from the page cache.
func dropPageCache(path string, off int64) error {
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	maps, err := os.ReadFile("/proc/self/maps")
	if err != nil {
		return err
	}
	for line := range strings.SplitSeq(string(maps), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 6 || fields[5] != realPath {
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
	f, err := os.Open(realPath)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}
	if off >= info.Size() {
		return nil
	}

	if err = f.Sync(); err != nil {
		return err
	}
	if err = unix.Fadvise(int(f.Fd()), off, 0, unix.FADV_DONTNEED); err != nil {
		return err
	}

	size := info.Size() - off
	mapped, err := unix.Mmap(int(f.Fd()), off, int(size), unix.PROT_READ, unix.MAP_SHARED)
	if err != nil {
		return err
	}
	defer unix.Munmap(mapped)

	pageSize := os.Getpagesize()
	pages := (len(mapped) + pageSize - 1) / pageSize
	vec := make([]byte, pages)
	_, _, errno := unix.Syscall(unix.SYS_MINCORE, uintptr(unsafe.Pointer(&mapped[0])), uintptr(len(mapped)), uintptr(unsafe.Pointer(&vec[0])))
	if errno != 0 {
		return errno
	}
	for i, v := range vec {
		if v&1 != 0 {
			return fmt.Errorf("page %d still resident after FADV_DONTNEED: %s offset %d", i, realPath, off)
		}
	}
	return nil
}
