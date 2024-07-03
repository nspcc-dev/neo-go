package main

import (
	"fmt"
	"os"

	"github.com/nspcc-dev/neo-go/cli/app"
)

func main() {
	ctl := app.New()

	if err := ctl.Run(os.Args); err != nil {
		fmt.Fprintln(ctl.ErrWriter, err)
		os.Exit(1)
	}
}
