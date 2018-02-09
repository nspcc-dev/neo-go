package main

import (
	"os"

	"github.com/urfave/cli"
)

// Simple dirty and quick bootstrapping for the sake of development.
// e.g run 2 nodes:
// neoserver -tcp :4000
// neoserver -tcp :3000 -seed 127.0.0.1:4000
func main() {
	ctl := cli.NewApp()
	ctl.Name = "neo-go"

	ctl.Commands = []cli.Command{
		{
			Name:   "node",
			Usage:  "start a NEO node",
			Action: startServer,
			Flags: []cli.Flag{
				cli.IntFlag{Name: "tcp"},
				cli.IntFlag{Name: "rpc"},
				cli.StringFlag{Name: "seed"},
				cli.BoolFlag{Name: "privnet, p"},
				cli.BoolFlag{Name: "mainnet, m"},
				cli.BoolFlag{Name: "testnet, t"},
			},
		},
		{
			Name:  "contract",
			Usage: "compile - debug - deploy smart contracts",
			Subcommands: []cli.Command{
				{
					Name:   "compile",
					Usage:  "compile a smart contract to a .avm file",
					Action: contractCompile,
				},
				{
					Name:   "opdump",
					Usage:  "dump the opcode of a .go file",
					Action: contractDumpOpcode,
				},
			},
		},
	}

	ctl.Run(os.Args)
}
