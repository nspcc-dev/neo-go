package peer

import (
	"fmt"
	"net"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/wire/payload"
)

func TestHandshake(t *testing.T) {
	conn, err := net.Dial("tcp", ":20338")
	if err != nil {
		t.Fatal(err.Error())
	}
	p := newPeer(conn)
	p.inbound = true
	err = p.handshake()
	go p.readLoop()
	go p.startProtocol()
	verack, err := payload.NewVerackMessage()
	if err != nil {
		t.Fail()
	}

	fmt.Println("sending verack; handshake complete")
	p.Write(verack)

}
