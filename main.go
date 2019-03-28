package main

import (
	"fmt"

	"github.com/CityOfZion/neo-go/pkg/server"
	"github.com/CityOfZion/neo-go/pkg/wire/protocol"
)

func main() {
	s, err := server.New(protocol.MainNet, 10332)
	if err != nil {
		fmt.Println(err)
		return
	}
	err = s.Run()
	if err != nil {
		fmt.Println("Server has stopped from the following error: ", err.Error())
	}
}
