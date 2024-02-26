package rpcclient_test

import (
	"context"
	"fmt"
	"os"

	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
)

func Example() {
	endpoint := "https://rpc.t5.n3.nspcc.ru:20331"
	opts := rpcclient.Options{}

	c, err := rpcclient.New(context.TODO(), endpoint, opts)
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

	addr, err := address.StringToUint160("NUkaBmzsZq1qdgaHfKrtRUcHNhtVJ2hTpv")
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
