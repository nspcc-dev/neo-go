package main

import (
	"fmt"
	"net"
	"time"

	"github.com/CityOfZion/neo-go/pkg/wire"
	"github.com/CityOfZion/neo-go/pkg/wire/payload"
	"github.com/CityOfZion/neo-go/pkg/wire/protocol"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

func main() {

	listener, err := net.Listen("tcp", ":20338")
	if err != nil {
		fmt.Println(err)
		return
	}

	defer func() {
		listener.Close()
		fmt.Println("Listener closed")
	}()

	for {

		conn, err := listener.Accept()
		conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if err != nil {
			fmt.Println(err)
			break
		}
		fmt.Println("Someone connected", conn)

		tcpAddrMe := &net.TCPAddr{IP: net.ParseIP("82.2.97.142"), Port: 20338}
		nonce := uint32(100)
		messageVer, err := payload.NewVersionMessage(tcpAddrMe, 2595770, false, protocol.DefaultVersion, protocol.UserAgent, nonce, protocol.NodePeerService)

		if err != nil {
			fmt.Println(err)
		}

		if err := wire.WriteMessage(conn, protocol.MainNet, messageVer); err != nil {
			fmt.Println(err)
			return
		}

		readmsg, err := wire.ReadMessage(conn, protocol.MainNet)
		if err != nil {
			fmt.Println("Error reading msg: ", err)
		}
		s1, ok := readmsg.(*payload.VersionMessage)
		if !ok {
			fmt.Println("Cannot assert to Version Message")
			return
		}

		fmt.Printf("%+v\n", s1)

		messageVrck, _ := payload.NewVerackMessage()
		if err := wire.WriteMessage(conn, protocol.MainNet, messageVrck); err != nil {
			fmt.Println(err)
			return
		}

		readmsg, err = wire.ReadMessage(conn, protocol.MainNet)
		if err != nil {
			fmt.Println("Error reading msg: ", err)
		}
		s2, ok := readmsg.(*payload.VerackMessage)
		if !ok {
			fmt.Println("Cannot assert to Verack Message")
			return
		}

		fmt.Printf(string(s2.Command()))

		hash, _ := util.Uint256DecodeString("d42561e3d30e15be6400b6df2f328e02d2bf6354c41dce433bc57687c82144bf")
		messageHeaders, _ := payload.NewGetHeadersMessage(
			[]util.Uint256{
				hash,
			},
			util.Uint256{})
		if err := wire.WriteMessage(conn, protocol.MainNet, messageHeaders); err != nil {
			fmt.Println(err)
			return
		}
	}
}
