//go:build windows

package input

import (
	"os"
	"syscall"

	"golang.org/x/term"
)

// readSecurePassword reads the user's password with prompt.
func readSecurePassword(prompt string) (string, error) {
	s, err := term.MakeRaw(int(syscall.Stdin))
	if err != nil {
		return "", err
	}
	defer func() { _ = term.Restore(int(syscall.Stdin), s) }()
	trm := term.NewTerminal(ReadWriter{os.Stdin, os.Stdout}, prompt)
	return trm.ReadPassword(prompt)
}
