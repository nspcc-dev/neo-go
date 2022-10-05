package vm

import (
	"os"

	"github.com/chzyer/readline"
	"github.com/nspcc-dev/neo-go/cli/cmdargs"
	"github.com/urfave/cli"
)

// NewCommands returns 'vm' command.
func NewCommands() []cli.Command {
	return []cli.Command{{
		Name:   "vm",
		Usage:  "start the virtual machine",
		Action: startVMPrompt,
		Flags:  []cli.Flag{},
	}}
}

func startVMPrompt(ctx *cli.Context) error {
	if err := cmdargs.EnsureNone(ctx); err != nil {
		return err
	}
	p := NewWithConfig(true, os.Exit, &readline.Config{})
	return p.Run()
}
