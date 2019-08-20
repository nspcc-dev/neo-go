package iputils

import (
	"log"
	"net"
)

//GetLocalIP returns the ip address of the current node
// https://stackoverflow.com/a/37382208
func GetLocalIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP
}
