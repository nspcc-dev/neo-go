package main

import (
	"testing"
)

func TestCLIVersion(t *testing.T) {
	e := newExecutor(t, false)
	e.Run(t, "neo-go", "--version")
	e.checkNextLine(t, "^neo-go version")
	e.checkEOF(t)
}
