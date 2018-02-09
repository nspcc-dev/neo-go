package main

import (
	"fmt"

	"github.com/CityOfZion/neo-go/pkg/vm"
	"github.com/urfave/cli"
)

func contractCompile(ctx *cli.Context) error {
	fmt.Println("compile")
	return nil
}

func contractDumpOpcode(ctx *cli.Context) error {
	src := ctx.Args()[0]

	c := vm.NewCompiler()
	if err := c.CompileSource(src); err != nil {
		return err
	}
	c.DumpOpcode()
	return nil
}
