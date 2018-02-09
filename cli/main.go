package main

import (
	"os"

	"github.com/CityOfZion/neo-go/cli/server"
	"github.com/CityOfZion/neo-go/cli/smartcontract"
	"github.com/urfave/cli"
)

func main() {
	ctl := cli.NewApp()
	ctl.Name = "neo-go"

	ctl.Commands = []cli.Command{
		server.NewCommand(),
		smartcontract.NewCommand(),
	}

	ctl.Run(os.Args)
}
