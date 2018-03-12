package main

import (
	"github.com/CityOfZion/neo-go/pkg/network/p2p"
)

func main() {
	cfg := p2p.Config{
		UserAgent: "/NEO-GO:0.0.1/",
		Net:       p2p.ModeMainNet,
		Seeds: []string{
			"seed2.neo.org:10333",
		},
		Relay:    true,
		MaxPeers: 50,
	}

	s := p2p.NewServer(cfg)
	s.Start()
}
