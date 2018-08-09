package main

import (
	"os"
	"github.com/urfave/cli"
)

func main() {
	ctl := cli.NewApp()
	ctl.Name = "neo-go"
	ctl.Usage = "Official Go client for Neo"

	ctl.Commands = []cli.Command{
	}

	ctl.Run(os.Args)
}
