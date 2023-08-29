package app_test

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testcli"
	"github.com/nspcc-dev/neo-go/internal/versionutil"
	"github.com/nspcc-dev/neo-go/pkg/config"
)

func TestCLIVersion(t *testing.T) {
	config.Version = versionutil.TestVersion // Zero-length version string disables '--version' completely.
	e := testcli.NewExecutor(t, false)
	e.Run(t, "neo-go", "--version")
	e.CheckNextLine(t, "^NeoGo")
	e.CheckNextLine(t, "^Version:")
	e.CheckNextLine(t, "^GoVersion:")
	e.CheckEOF(t)
}
