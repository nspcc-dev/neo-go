//go:build !windows

package server

import "syscall"

const (
	sighup  = syscall.SIGHUP
	sigusr1 = syscall.SIGUSR1
	sigusr2 = syscall.SIGUSR2
)
