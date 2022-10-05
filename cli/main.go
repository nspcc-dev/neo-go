package main

import (
	"os"

	"github.com/nspcc-dev/neo-go/cli/app"
)

func main() {
	ctl := app.New()

	if err := ctl.Run(os.Args); err != nil {
		panic(err)
	}
}
