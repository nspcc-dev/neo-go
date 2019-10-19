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

	ctl.Commands = append(ctl.Commands, server.NewCommands()...)
	ctl.Commands = append(ctl.Commands, smartcontract.NewCommands()...)
	ctl.Commands = append(ctl.Commands, wallet.NewCommands()...)
	ctl.Commands = append(ctl.Commands, vm.NewCommands()...)

	if err := ctl.Run(os.Args); err != nil {
		panic(err)
	}
}
