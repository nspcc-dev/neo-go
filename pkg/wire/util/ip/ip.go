package iputils

import (
	"log"
	"net"
)

//GetLocalIP returns the ip address of the current node
// https://stackoverflow.com/a/37382208
func GetLocalIP() net.IP {
	//todo: implement config(will be merged from master soon). As of now it's localhost for privatenet
	conn, err := net.Dial("udp", "127.0.0.1:20333")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP
}
