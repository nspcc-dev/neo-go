package main

import (
	"flag"
	"strings"

	"github.com/anthdm/neo-go/pkg/network"
)

var (
	port = flag.String("port", ":3000", "port the TCP listener will listen on.")
	seed = flag.String("seed", "", "initial seed servers.")
)

// Simple dirty and quick bootstrapping for the sake of development.
// e.g run 2 nodes:
// neoserver -port :4000
// neoserver -port :3000 -seed 127.0.0.1:4000
func main() {
	flag.Parse()

	s := network.NewServer(network.ModeTestNet)
	seeds := strings.Split(*seed, ",")
	if len(seeds) == 0 {
		seeds = []string{*seed}
	}
	if *seed == "" {
		seeds = []string{}
	}
	s.Start(*port, seeds)
}
