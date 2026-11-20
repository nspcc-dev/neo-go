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

// dropPageCache evicts file pages starting at off and checks that none remain
// resident in the page cache.
func dropPageCache(path string, off int64) error {
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	maps, err := os.ReadFile("/proc/self/maps")
	if err != nil {
		return err
	}
	var mappedFile bool
	for line := range strings.SplitSeq(string(maps), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 6 || fields[5] != realPath {
			continue
		}
		mappedFile = true
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
	if !mappedFile {
		return fmt.Errorf("no mapping found for %s", realPath)
	}
	f, err := os.Open(realPath)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := f.Sync(); err != nil {
		return err
	}
	if err := unix.Fadvise(int(f.Fd()), off, 0, unix.FADV_DONTNEED); err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		return err
	}
	size := info.Size() - off
	if size <= 0 || int64(int(size)) != size {
		return fmt.Errorf("invalid page cache range: %s offset %d size %d", realPath, off, size)
	}
	mapped, err := unix.Mmap(int(f.Fd()), off, int(size), unix.PROT_READ, unix.MAP_SHARED)
	if err != nil {
		return err
	}
	ps := os.Getpagesize()
	pages := (len(mapped) + ps - 1) / ps
	vec := make([]byte, pages)
	_, _, errno := unix.Syscall(unix.SYS_MINCORE, uintptr(unsafe.Pointer(unsafe.SliceData(mapped))), uintptr(len(mapped)), uintptr(unsafe.Pointer(unsafe.SliceData(vec))))
	err = unix.Munmap(mapped)
	if errno != 0 {
		return errno
	}
	if err != nil {
		return err
	}
	for i, v := range vec {
		if v&1 != 0 {
			return fmt.Errorf("file page %d still resident after FADV_DONTNEED: %s offset %d", off/int64(ps)+int64(i), realPath, off)
		}
	}
	return nil
}
