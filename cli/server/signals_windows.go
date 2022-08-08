//go:build windows

package server

import "syscall"

const (
	// Doesn't really matter, Windows can't do it.
	sighup  = syscall.SIGHUP
	sigusr1 = syscall.Signal(0xa)
	sigusr2 = syscall.Signal(0xc)
)
