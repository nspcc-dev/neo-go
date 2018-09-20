package main

import (
	"fmt"
	"net"

	"github.com/CityOfZion/neo-go/pkg/wire"
	"github.com/CityOfZion/neo-go/pkg/wire/payload"
	"github.com/CityOfZion/neo-go/pkg/wire/protocol"
)

// This file is to test the wire package
// by communicating with a node on the network

func main() {
	conn, err := net.Dial("tcp", ":20338") // in real impl, DialWithTimeout or could hang

	if err != nil {
		fmt.Println("Conn err", err)
		return
	}

	readmsg, err := wire.ReadMessage(conn, protocol.MainNet)
	if err != nil {
		fmt.Println(err)
		return
	}
	s, ok := readmsg.(*payload.VersionMessage)
	if !ok {
		fmt.Println("Cannot assert to VersionMessage")
		return
	}

	fmt.Printf("%+v\n", s)

	expectedIP := "82.2.97.142"
	expectedPort := 10334
	tcpAddrMe := &net.TCPAddr{IP: net.ParseIP(expectedIP), Port: expectedPort}
	messageVer, err := payload.NewVersionMessage(tcpAddrMe, 2595770, false, protocol.DefaultVersion)

	if err != nil {
		fmt.Println(err)
	}
	if err := wire.WriteMessage(conn, protocol.MainNet, messageVer); err != nil {
		fmt.Println(err)
		return
	}

	readmsg, err = wire.ReadMessage(conn, protocol.MainNet)
	if err != nil {
		fmt.Println("Error reading msg: ", err)
	}
	s1, ok := readmsg.(*payload.VerackMessage)
	if !ok {
		fmt.Println("Cannot assert to VerackMessage")
		return
	}

	fmt.Printf("%+v\n", s1)

	messageVrck, _ := payload.NewVerackMessage()
	if err := wire.WriteMessage(conn, protocol.MainNet, messageVrck); err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Handshake complete")
}
