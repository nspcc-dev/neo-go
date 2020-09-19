package input

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"golang.org/x/crypto/ssh/terminal"
)

// Terminal is a terminal used for input. If `nil`, stdin is used.
var Terminal *terminal.Terminal

// ReadLine reads line from the input without trailing '\n'
func ReadLine(w io.Writer, prompt string) (string, error) {
	if Terminal != nil {
		_, err := Terminal.Write([]byte(prompt))
		if err != nil {
			return "", err
		}
		raw, err := Terminal.ReadLine()
		return strings.TrimRight(raw, "\n"), err
	}
	fmt.Fprint(w, prompt)
	buf := bufio.NewReader(os.Stdin)
	return buf.ReadString('\n')
}

// ReadPassword reads user password with prompt.
func ReadPassword(w io.Writer, prompt string) (string, error) {
	if Terminal != nil {
		return Terminal.ReadPassword(prompt)
	}
	fmt.Fprint(w, prompt)
	rawPass, err := terminal.ReadPassword(syscall.Stdin)
	if err != nil {
		return "", err
	}
	fmt.Fprintln(w)
	return strings.TrimRight(string(rawPass), "\n"), nil
}
