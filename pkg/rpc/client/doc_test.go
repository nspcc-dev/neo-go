package client_test

import (
	"context"
	"fmt"
	"os"

	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/rpc/client"
)

func Example() {
	endpoint := "http://seed5.bridgeprotocol.io:10332"
	opts := client.Options{}

	c, err := client.New(context.TODO(), endpoint, opts)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	err = c.Init()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if err := c.Ping(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	addr, err := address.StringToUint160("ATySFJAbLW7QHsZGHScLhxq6EyNBxx3eFP")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	resp, err := c.GetNEP17Balances(addr)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println(resp.Address)
	fmt.Println(resp.Balances)
}
