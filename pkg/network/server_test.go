package network

import (
	"sync"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/network"
	"github.com/CityOfZion/neo-go/pkg/network/payload"
)

func TestVersion(t *testing.T) {
	s := NewServer()
	var wg sync.WaitGroup
	wg.Add(1)
	go s.Start()

	s.proto <- protoTuple{
		msg: network.NewMessage(
			network.ModePrivNet, network.CMDVersion, payload.NewVersion(
				0, 0, "hello", 0, true,
			),
		),
		peer: &TCPPeer{},
	}
	wg.Wait()
}
