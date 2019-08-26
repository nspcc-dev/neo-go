package main

import (
	"os"

	"github.com/CityOfZion/neo-go/cli/server"
	"github.com/CityOfZion/neo-go/cli/smartcontract"
	"github.com/CityOfZion/neo-go/cli/vm"
	"github.com/CityOfZion/neo-go/cli/wallet"
	"github.com/CityOfZion/neo-go/config"
	"github.com/urfave/cli"
)

func main() {
	ctl := cli.NewApp()
	ctl.Name = "neo-go"
	ctl.Version = config.Version
	ctl.Usage = "Official Go client for Neo"

	ctl.Commands = []cli.Command{
		server.NewCommand(),
		smartcontract.NewCommand(),
		wallet.NewCommand(),
		vm.NewCommand(),
	}

	if err := ctl.Run(os.Args); err != nil {
		panic(err)
	}
}
