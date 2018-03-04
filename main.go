package main

import (
	"context"
	"fmt"
	"log"

	"github.com/CityOfZion/neo-go/pkg/rpc"
)

func main() {
	client, err := rpc.NewClient(context.TODO(), "http://seed5.bridgeprotocol.io:10332", rpc.ClientOptions{})
	if err != nil {
		log.Fatal(err)
	}
	if err := client.Ping(); err != nil {
		log.Fatal(err)
	}

	resp, err := client.GetAccountState("AcsdonGS7EYbXzFWuMV2WKZ4DnKd9ddcc1")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%+v\n", resp.ID)
}
