package vm

import (
	"fmt"
	"os"

	"github.com/chzyer/readline"
	"github.com/nspcc-dev/neo-go/cli/cmdargs"
	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/pkg/core/storage/dbconfig"
	"github.com/urfave/cli/v2"
)

// NewCommands returns 'vm' command.
func NewCommands() []*cli.Command {
	cfgFlags := []cli.Flag{options.Config, options.ConfigFile, options.RelativePath}
	cfgFlags = append(cfgFlags, options.Network...)
	return []*cli.Command{{
		Name:   "vm",
		Usage:  "Start the virtual machine",
		Action: startVMPrompt,
		Flags:  cfgFlags,
	}}
}

func startVMPrompt(ctx *cli.Context) error {
	if err := cmdargs.EnsureNone(ctx); err != nil {
		return err
	}

	cfg, err := options.GetConfigFromContext(ctx)
	if err != nil {
		return cli.Exit(err, 1)
	}
	if ctx.NumFlags() == 0 {
		cfg.ApplicationConfiguration.DBConfiguration.Type = dbconfig.InMemoryDB
	}
	if cfg.ApplicationConfiguration.DBConfiguration.Type != dbconfig.InMemoryDB {
		cfg.ApplicationConfiguration.DBConfiguration.LevelDBOptions.ReadOnly = true
		cfg.ApplicationConfiguration.DBConfiguration.BoltDBOptions.ReadOnly = true
	}

	p, err := NewWithConfig(true, os.Exit, &readline.Config{}, cfg)
	if err != nil {
		return cli.Exit(fmt.Errorf("failed to create VM CLI: %w", err), 1)
	}
	return p.Run()
}
