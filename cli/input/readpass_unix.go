//go:build !windows
// +build !windows

package input

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

// readSecurePassword reads the user's password with prompt directly from /dev/tty.
func readSecurePassword(prompt string) (string, error) {
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return "", err
	}
	defer f.Close()
	_, err = f.WriteString(prompt)
	if err != nil {
		return "", err
	}
	pass, err := term.ReadPassword(int(f.Fd()))
	if err != nil {
		return "", fmt.Errorf("failed to read password: %w", err)
	}
	_, err = f.WriteString("\n")
	return string(pass), err
}
