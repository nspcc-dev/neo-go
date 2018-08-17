package peer

import (
	"fmt"
	"net"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/wire/payload"
)

func TestHandshake(t *testing.T) {

	// startListener in integtest/local

	conn, err := net.Dial("tcp", ":20338")
	if err != nil {
		t.Fatal(err.Error())
	}
	p := NewPeer(conn, true, DefaultConfig())
	err = p.Run()
	verack, err := payload.NewVerackMessage()
	if err != nil {
		t.Fail()
	}

	fmt.Println("sending verack; handshake complete")
	p.Write(verack)

}
