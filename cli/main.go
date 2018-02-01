package main

import (
	"flag"
	"strings"

	"github.com/CityOfZion/neo-go/pkg/network"
)

var (
	tcp  = flag.Int("tcp", 3000, "port TCP listener will listen on.")
	seed = flag.String("seed", "", "initial seed servers.")
	net  = flag.Int("net", 56753, "the mode the server will operate in.")
	rpc  = flag.Int("rpc", 0, "let this server also respond to rpc calls on this port")
)

// Simple dirty and quick bootstrapping for the sake of development.
// e.g run 2 nodes:
// neoserver -tcp :4000
// neoserver -tcp :3000 -seed 127.0.0.1:4000
func main() {
	flag.Parse()

	opts := network.StartOpts{
		Seeds: parseSeeds(*seed),
		TCP:   *tcp,
		RPC:   *rpc,
	}

	s := network.NewServer(network.NetMode(*net))
	s.Start(opts)
}

func parseSeeds(s string) []string {
	if len(s) == 0 {
		return nil
	}
	seeds := strings.Split(s, ",")
	if len(seeds) == 0 {
		return nil
	}
	return seeds
}
