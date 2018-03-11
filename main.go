package main

import "github.com/CityOfZion/neo-go/pkg/network/p2p"

func main() {
	s := p2p.NewServer()
	s.Start()
}
