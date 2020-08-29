package main

import (
	"testing"
)

func TestCLIVersion(t *testing.T) {
	e := newExecutor(t, false)
	defer e.Close(t)
	e.Run(t, "neo-go", "--version")
	e.checkNextLine(t, "^neo-go version")
	e.checkEOF(t)
}
