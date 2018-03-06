package main

import (
	"log"

	"github.com/CityOfZion/neo-go/pkg/network"
)

func main() {
	config := network.Config{
		UserAgent: "/neo-go/",
		Seeds: []string{
			"127.0.0.1:20333",
			"127.0.0.1:20334",
			"127.0.0.1:20335",
			"127.0.0.1:20336",
		},
		ListenTCP: 20333,
		Net:       network.ModePrivNet,
		Relay:     true,
	}
	s := network.NewServer(config)
	if err := s.Start(); err != nil {
		log.Fatal(err)
	}
}
