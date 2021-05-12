package input

import (
	"io"
	"os"
	"syscall"

	"golang.org/x/term"
)

// Terminal is a terminal used for input. If `nil`, stdin is used.
var Terminal *term.Terminal

// ReadWriter combiner reader and writer.
type ReadWriter struct {
	io.Reader
	io.Writer
}

// ReadLine reads line from the input without trailing '\n'
func ReadLine(prompt string) (string, error) {
	trm := Terminal
	if trm == nil {
		s, err := term.MakeRaw(syscall.Stdin)
		if err != nil {
			panic(err)
		}
		defer func() { _ = term.Restore(syscall.Stdin, s) }()
		trm = term.NewTerminal(ReadWriter{
			Reader: os.Stdin,
			Writer: os.Stdout,
		}, "")
	}
	return readLine(trm, prompt)
}

func readLine(trm *term.Terminal, prompt string) (string, error) {
	_, err := trm.Write([]byte(prompt))
	if err != nil {
		return "", err
	}
	return trm.ReadLine()
}

// ReadPassword reads user password with prompt.
func ReadPassword(prompt string) (string, error) {
	trm := Terminal
	if trm == nil {
		s, err := term.MakeRaw(syscall.Stdin)
		if err != nil {
			panic(err)
		}
		defer func() { _ = term.Restore(syscall.Stdin, s) }()
		trm = term.NewTerminal(ReadWriter{os.Stdin, os.Stdout}, prompt)
	}
	return trm.ReadPassword(prompt)
}
